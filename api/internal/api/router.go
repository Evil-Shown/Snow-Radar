// Package api: HTTP surface of the control plane.
package api

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/evil-shown/snow-radar/api/internal/auth"
	"github.com/evil-shown/snow-radar/api/internal/billing"
	"github.com/evil-shown/snow-radar/api/internal/peer"
	"github.com/evil-shown/snow-radar/api/internal/store"
)

const (
	AccessTTL         = 15 * time.Minute
	RefreshTTL        = 30 * 24 * time.Hour
	ReadHeaderTimeout = 5 * time.Second
	maxWebhookBody    = 64 << 10
)

type Config struct {
	Addr            string
	JWTPrivateKey   *rsa.PrivateKey
	PaddleSecret    string
	PayHereSecret   string
	CheckoutSecret  string // HMAC key for signed checkout sessions
	SGPWgPublicKey  string
	SGPAwgPublicKey string
	FSNWgPublicKey  string
	FSNAwgPublicKey string
}

type Server struct {
	cfg           Config
	store         store.Store
	issuer        *auth.TokenIssuer
	peers         *peer.Service
	refreshTokens *sessionManager
	checkout      *billing.CheckoutService
	limiter       *rateLimiter
}

// NewServer wires the full dependency graph ONCE at startup.
//
// SECURITY NOTE (audit finding #1/#34): constructing peer.Service per request
// reset allocator state on every call, handing duplicate tunnel IPs to
// different peers. Long-lived singletons are mandatory here.
func NewServer(cfg Config, st store.Store, issuer *auth.TokenIssuer) (*Server, error) {
	s := &Server{
		cfg:           cfg,
		store:         st,
		issuer:        issuer,
		refreshTokens: newSessionManager(st),
		checkout:      billing.NewCheckoutService([]byte(cfg.CheckoutSecret)),
		limiter:       newRateLimiter(),
	}
	pubKeys := map[string]string{
		"sgp": cfg.SGPWgPublicKey,
		"fsn": cfg.FSNWgPublicKey,
	}
	endpoints := map[string]map[bool]string{
		"sgp": {false: "sgp.snowradar.app:51820", true: "sgp.snowradar.app:51821"},
		"fsn": {false: "fsn.snowradar.app:51820", true: "fsn.snowradar.app:51821"},
	}
	svc, err := peer.NewService(st, pubKeys, endpoints)
	if err != nil {
		return nil, err
	}
	s.peers = svc
	return s, nil
}

func NewRouter(cfg Config, st store.Store, issuer *auth.TokenIssuer) (http.Handler, error) {
	s, err := NewServer(cfg, st, issuer)
	if err != nil {
		return nil, err
	}
	g := gin.New()
	g.Use(gin.Recovery())
	g.Use(s.bodyLimit(1 << 20)) // global 1 MiB cap: no endpoint needs more

	g.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	v1 := g.Group("/api/v1")
	v1.POST("/auth/signup", s.rateLimit(10, time.Minute), s.signup)
	v1.POST("/auth/login", s.rateLimit(10, time.Minute), s.login)
	v1.POST("/auth/refresh", s.rateLimit(30, time.Minute), s.refresh)
	v1.POST("/webhooks/paddle", s.paddleWebhook)
	v1.POST("/webhooks/payhere", s.payHereWebhook)

	authed := v1.Group("/", s.requireAuth())
	authed.POST("/connect", s.connect)
	authed.GET("/peers", s.listPeers)
	authed.DELETE("/peers/:id", s.revokePeer)
	authed.POST("/billing/checkout", s.checkoutSession)
	authed.POST("/auth/logout", s.logout)

	return g, nil
}

func newJTI() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type credentials struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=12,max=128"`
}

func (s *Server) signup(c *gin.Context) {
	var req credentials
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and a password of at least 12 characters are required"})
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	u := &store.User{Email: strings.ToLower(strings.TrimSpace(req.Email)), PasswordHash: hash}
	if err := s.store.CreateUser(u); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user_id": u.ID})
}

func (s *Server) issueTokenPair(userID string) (access, refresh string, err error) {
	jti := newJTI()
	if access, err = s.issuer.Issue(userID, auth.TokenAccess, ""); err != nil {
		return "", "", err
	}
	if refresh, err = s.issuer.Issue(userID, auth.TokenRefresh, jti); err != nil {
		return "", "", err
	}
	if err := s.refreshTokens.issue(jti, userID); err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

func (s *Server) login(c *gin.Context) {
	var req credentials
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	u, err := s.store.GetUserByEmail(strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil || !auth.VerifyPassword(req.Password, u.PasswordHash) {
		// Uniform error: never reveal whether the email exists.
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	access, refresh, err := s.issueTokenPair(u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": access, "refresh_token": refresh})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// refresh implements rotate-on-use: a refresh token is valid exactly once.
// Replaying a consumed token burns every outstanding session for that user.
func (s *Server) refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	claims, err := s.issuer.Verify(req.RefreshToken)
	if err != nil || claims.TokenType != auth.TokenRefresh {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	userID, err := s.refreshTokens.consume(claims.ID)
	if err != nil || userID != claims.UserID {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token revoked or replayed"})
		return
	}
	access, newRefresh, err := s.issueTokenPair(claims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": access, "refresh_token": newRefresh})
}

func (s *Server) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(h, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		claims, err := s.issuer.Verify(strings.TrimPrefix(h, prefix))
		if err != nil || claims.TokenType != auth.TokenAccess {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Next()
	}
}

type connectRequest struct {
	NodeID      string `json:"node_id" binding:"required,oneof=sgp fsn"`
	PublicKey   string `json:"public_key" binding:"required"`
	StealthMode bool   `json:"stealth_mode"`
}

func (s *Server) connect(c *gin.Context) {
	var req connectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id must be sgp|fsn; public_key required"})
		return
	}
	userID := c.GetString("user_id")

	pubKeys := map[string]map[bool]string{
		"sgp": {false: s.cfg.SGPWgPublicKey, true: s.cfg.SGPAwgPublicKey},
		"fsn": {false: s.cfg.FSNWgPublicKey, true: s.cfg.FSNAwgPublicKey},
	}
	for node := range pubKeys {
		if pubKeys[node][false] == "" || pubKeys[node][true] == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "node keys not provisioned yet"})
			return
		}
	}

	p, clientCfg, err := s.peers.Connect(userID, req.NodeID, req.PublicKey, req.StealthMode)
	if err != nil {
		switch {
		case errors.Is(err, peer.ErrNotSubscribed):
			c.JSON(http.StatusPaymentRequired, gin.H{"error": "no active subscription"})
		case errors.Is(err, peer.ErrTooManyPeers):
			c.JSON(http.StatusConflict, gin.H{"error": "device limit reached"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"peer_id": p.ID, "address": p.Address.String(), "config": clientCfg})
}

func (s *Server) listPeers(c *gin.Context) {
	peers, _ := s.store.PeersByUser(c.GetString("user_id"))
	c.JSON(http.StatusOK, peers)
}

func (s *Server) paddleWebhook(c *gin.Context) {
	raw, err := billing.ReadBody(c.Request, maxWebhookBody)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body too large or unreadable"})
		return
	}
	if s.cfg.PaddleSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "webhook secret not configured"})
		return
	}
	if err := billing.VerifyPaddle(s.cfg.PaddleSecret, c.GetHeader("Paddle-Signature"), raw); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "signature verification failed"})
		return
	}
	ev, err := billing.ParsePaddle(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unrecognized payload"})
		return
	}

	// Identity comes exclusively from verifying our signed checkout session;
	// the provider-controlled user identifier is never trusted.
	checkout := billing.NewCheckoutService([]byte(s.cfg.PaddleSecret))
	userID, err := checkout.Verify(ev.CheckoutToken)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid checkout session"})
		return
	}
	if _, err := s.store.GetUser(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown user reference"})
		return
	}
	existing, err := s.store.GetSubscription(userID)
	if err == nil && existing.External != ev.External &&
		existing.State == string(billing.StateActive) {
		c.JSON(http.StatusConflict, gin.H{"error": "user already holds an active subscription"})
		return
	}

	if err := s.store.UpsertSubscription(&store.Subscription{
		UserID: userID, Provider: ev.Provider, External: ev.External, State: string(ev.State),
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) payHereWebhook(c *gin.Context) {
	if s.cfg.PayHereSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "webhook secret not configured"})
		return
	}
	// PayHere POSTs form-encoded values; hash covers the documented concat.
	if err := c.Request.ParseForm(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad form"})
		return
	}
	f := c.Request.PostForm
	ok := billing.VerifyPayHere(
		s.cfg.PayHereSecret,
		f.Get("merchant_id"), f.Get("order_id"), f.Get("payhere_amount"),
		f.Get("payhere_currency"), f.Get("status_code"), f.Get("md5sig"),
	)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "signature verification failed"})
		return
	}
	state := map[string]string{"2": "active", "0": "past_due", "-1": "cancelled", "-2": "past_due", "-3": "cancelled"}[f.Get("status_code")]
	if state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown status_code"})
		return
	}
	// Same checkout-session binding as the Paddle path: custom_1 carries
	// OUR signed token, not a raw user id.
	checkout := billing.NewCheckoutService([]byte(s.cfg.PayHereSecret))
	userID, err := checkout.Verify(f.Get("custom_1"))
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid checkout session"})
		return
	}
	if _, err := s.store.GetUser(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown user reference"})
		return
	}
	if err := s.store.UpsertSubscription(&store.Subscription{
		UserID: userID, Provider: "payhere", External: f.Get("subscription_id"), State: state,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// checkoutSession mints a short-lived signed token binding THIS user to the
// upcoming payment. The client attaches it to the provider checkout
// (custom_data.checkout_token / custom_1); our webhook verifies OUR
// signature - never a provider-supplied user id.
func (s *Server) checkoutSession(c *gin.Context) {
	var req struct {
		Provider string `json:"provider" binding:"required,oneof=paddle payhere"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider must be paddle or payhere"})
		return
	}
	userID := c.GetString("user_id")

	const ttl = 30 * time.Minute // long enough to complete payment, short enough to be useless if leaked
	token, err := s.checkout.Mint(userID, ttl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	field := map[string]string{"paddle": "checkout_token", "payhere": "custom_1"}[req.Provider]
	c.JSON(http.StatusCreated, gin.H{
		"token":      token,
		"field":      field,
		"expires_in": int(ttl.Seconds()),
	})
}

// revokePeer removes one of the caller's own devices. Ownership check is
// implicit: Revoke refuses to touch peers belonging to another user.
func (s *Server) revokePeer(c *gin.Context) {
	userID := c.GetString("user_id")
	err := s.peers.Revoke(userID, c.Param("id"))
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"revoked": true})
	case errors.Is(err, store.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no such peer for this user"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
	}
}

// logout revokes every outstanding refresh token for the caller. Access
// tokens stay valid for at most AccessTTL (15 min) by design.
func (s *Server) logout(c *gin.Context) {
	if err := s.refreshTokens.revokeAllFor(c.GetString("user_id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logged_out": true})
}

