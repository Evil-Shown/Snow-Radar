// Package api: HTTP surface of the control plane.
package api

import (
	"crypto/rsa"
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
	SGPWgPublicKey  string
	SGPAwgPublicKey string
	FSNWgPublicKey  string
	FSNAwgPublicKey string
}

type Server struct {
	cfg     Config
	store   store.Store
	issuer  *auth.TokenIssuer
}

func NewRouter(cfg Config, st store.Store, issuer *auth.TokenIssuer) http.Handler {
	s := &Server{cfg: cfg, store: st, issuer: issuer}
	g := gin.New()
	g.Use(gin.Recovery())

	g.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	v1 := g.Group("/api/v1")
	v1.POST("/auth/signup", s.signup)
	v1.POST("/auth/login", s.login)
	v1.POST("/webhooks/paddle", s.paddleWebhook)
	v1.POST("/webhooks/payhere", s.payHereWebhook)

	authed := v1.Group("/", s.requireAuth())
	authed.POST("/connect", s.connect)
	authed.GET("/peers", s.listPeers)

	return g
}

type credentials struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=12,max=128"`
}

func (s *Server) signup(c *gin.Context) {
	var req credentials
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "email and a password of at least 12 characters are required"})
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}
	u := &store.User{Email: strings.ToLower(strings.TrimSpace(req.Email)), PasswordHash: hash}
	if err := s.store.CreateUser(u); err != nil {
		c.JSON(409, gin.H{"error": "email already registered"})
		return
	}
	c.JSON(201, gin.H{"user_id": u.ID})
}

func (s *Server) login(c *gin.Context) {
	var req credentials
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	u, err := s.store.GetUserByEmail(strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil || !auth.VerifyPassword(req.Password, u.PasswordHash) {
		// Uniform error: never reveal whether the email exists.
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	access, err1 := s.issuer.Issue(u.ID, false)
	refresh, err2 := s.issuer.Issue(u.ID, true)
	if err1 != nil || err2 != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}
	c.JSON(200, gin.H{"access_token": access, "refresh_token": refresh})
}

func (s *Server) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(h, prefix) {
			c.AbortWithStatusJSON(401, gin.H{"error": "missing bearer token"})
			return
		}
		claims, err := s.issuer.Verify(strings.TrimPrefix(h, prefix))
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
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

func (s *Server) peerService(pubKeys map[string]map[bool]string) *peer.Service {
	svc, _ := peer.NewService(s.store,
		map[string]string{
			"sgp": pubKeys["sgp"][false],
			"fsn": pubKeys["fsn"][false],
		},
		map[string]map[bool]string{
			"sgp": {false: "sgp.snowradar.app:51820", true: "sgp.snowradar.app:51821"},
			"fsn": {false: "fsn.snowradar.app:51820", true: "fsn.snowradar.app:51821"},
		},
	)
	return svc
}

func (s *Server) connect(c *gin.Context) {
	var req connectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "node_id must be sgp|fsn; public_key required"})
		return
	}
	userID := c.GetString("user_id")

	pubKeys := map[string]map[bool]string{
		"sgp": {false: s.cfg.SGPWgPublicKey, true: s.cfg.SGPAwgPublicKey},
		"fsn": {false: s.cfg.FSNWgPublicKey, true: s.cfg.FSNAwgPublicKey},
	}
	for node := range pubKeys {
		if pubKeys[node][false] == "" || pubKeys[node][true] == "" {
			c.JSON(503, gin.H{"error": "node keys not provisioned yet"})
			return
		}
	}

	p, clientCfg, err := s.peerService(pubKeys).Connect(userID, req.NodeID, req.PublicKey, req.StealthMode)
	if err != nil {
		switch {
		case errors.Is(err, peer.ErrNotSubscribed):
			c.JSON(402, gin.H{"error": "no active subscription"})
		case errors.Is(err, peer.ErrTooManyPeers):
			c.JSON(409, gin.H{"error": "device limit reached"})
		default:
			c.JSON(400, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(200, gin.H{"peer_id": p.ID, "address": p.Address.String(), "config": clientCfg})
}

func (s *Server) listPeers(c *gin.Context) {
	peers, _ := s.store.PeersByUser(c.GetString("user_id"))
	c.JSON(200, peers)
}

func (s *Server) paddleWebhook(c *gin.Context) {
	raw, err := billing.ReadBody(c.Request, maxWebhookBody)
	if err != nil {
		c.JSON(400, gin.H{"error": "body too large or unreadable"})
		return
	}
	if s.cfg.PaddleSecret == "" {
		c.JSON(503, gin.H{"error": "webhook secret not configured"})
		return
	}
	if err := billing.VerifyPaddle(s.cfg.PaddleSecret, c.GetHeader("Paddle-Signature"), raw); err != nil {
		c.JSON(403, gin.H{"error": "signature verification failed"})
		return
	}
	ev, err := billing.ParsePaddle(raw)
	if err != nil {
		c.JSON(400, gin.H{"error": "unrecognized payload"})
		return
	}
	if err := s.store.UpsertSubscription(&store.Subscription{
		UserID: ev.UserID, Provider: ev.Provider, External: ev.External, State: string(ev.State),
	}); err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (s *Server) payHereWebhook(c *gin.Context) {
	if s.cfg.PayHereSecret == "" {
		c.JSON(503, gin.H{"error": "webhook secret not configured"})
		return
	}
	// PayHere POSTs form-encoded values; hash covers the documented concat.
	if err := c.Request.ParseForm(); err != nil {
		c.JSON(400, gin.H{"error": "bad form"})
		return
	}
	f := c.Request.PostForm
	ok := billing.VerifyPayHere(
		s.cfg.PayHereSecret,
		f.Get("merchant_id"), f.Get("order_id"), f.Get("payhere_amount"),
		f.Get("payhere_currency"), f.Get("status_code"), f.Get("md5sig"),
	)
	if !ok {
		c.JSON(403, gin.H{"error": "signature verification failed"})
		return
	}
	state := map[string]string{"2": "active", "0": "past_due", "-1": "cancelled", "-2": "past_due", "-3": "cancelled"}[f.Get("status_code")]
	if state == "" {
		c.JSON(400, gin.H{"error": "unknown status_code"})
		return
	}
	// PayHere identifies the payer by custom fields we set at checkout.
	userID := f.Get("custom_1")
	if userID == "" {
		c.JSON(400, gin.H{"error": "missing user reference"})
		return
	}
	if err := s.store.UpsertSubscription(&store.Subscription{
		UserID: userID, Provider: "payhere", External: f.Get("subscription_id"), State: state,
	}); err != nil {
		c.JSON(500, gin.H{"error": "internal"})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
