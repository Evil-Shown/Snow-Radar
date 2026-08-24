package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log"
	"net/http"
	"os"

	"github.com/evil-shown/snow-radar/api/internal/api"
	"github.com/evil-shown/snow-radar/api/internal/auth"
	"github.com/evil-shown/snow-radar/api/internal/store"
)

func loadRSAPrivateKey(path string) *rsa.PrivateKey {
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read private key %s: %v", path, err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		log.Fatalf("no PEM block in %s", path)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		log.Fatalf("parse private key: %v", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		log.Fatalf("key is not RSA")
	}
	return rsaKey
}

func main() {
	cfg := api.Config{
		// Loopback default protects bare-metal runs; containers override via
		// ENV in the Dockerfile (audit finding #8: loopback-only bind made
		// the container unreachable through published ports).
		Addr:            getenv("LISTEN_ADDR", "127.0.0.1:8080"),
		JWTPrivateKey:   loadRSAPrivateKey(getenv("JWT_PRIVATE_KEY_PATH", "")),
		PaddleSecret:    os.Getenv("PADDLE_WEBHOOK_SECRET"),
		PayHereSecret:   os.Getenv("PAYHERE_MERCHANT_SECRET"),
		SGPWgPublicKey:  os.Getenv("SGP_WG_PUBLIC_KEY"),
		SGPAwgPublicKey: os.Getenv("SGP_AWG_PUBLIC_KEY"),
		FSNWgPublicKey:  os.Getenv("FSN_WG_PUBLIC_KEY"),
		FSNAwgPublicKey: os.Getenv("FSN_AWG_PUBLIC_KEY"),
	}
	if cfg.JWTPrivateKey == nil {
		log.Fatal("JWT_PRIVATE_KEY_PATH is required")
	}

	st := store.NewMemory() // Postgres-backed store lands with migrations
	issuer := auth.NewTokenIssuer(cfg.JWTPrivateKey, api.AccessTTL, api.RefreshTTL)
	router, err := api.NewRouter(cfg, st, issuer)
	if err != nil {
		log.Fatalf("router init: %v", err)
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: api.ReadHeaderTimeout,
	}
	log.Printf("snowradar-api listening on %s (loopback default; front with a TLS reverse proxy)", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
