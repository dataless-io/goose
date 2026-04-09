package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/dataless-io/goose-broker-service/broker"
	"github.com/dataless-io/goose-broker-service/statics"
)

type Config struct {
	Statics string
	APIKey  string
}

// NewHTTPHandler wires broker API + health endpoints + static UI.
func NewHTTPHandler(manager *broker.BrokerManager, cfg Config) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	api := withAPIKeyWriteGuard(broker.NewHandler(manager), strings.TrimSpace(cfg.APIKey))
	mux.Handle("/brokers", api)
	mux.Handle("/brokers/", api)

	staticHandler := statics.ServeStatics(cfg.Statics)
	mux.HandleFunc("/", staticHandler)

	return mux
}

func withAPIKeyWriteGuard(next http.Handler, apiKey string) http.Handler {
	if apiKey == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isWriteMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		provided := strings.TrimSpace(r.Header.Get("X-API-Key"))
		if provided == "" {
			auth := strings.TrimSpace(r.Header.Get("Authorization"))
			const bearer = "Bearer "
			if len(auth) >= len(bearer) && strings.EqualFold(auth[:len(bearer)], bearer) {
				provided = strings.TrimSpace(auth[len(bearer):])
			}
		}

		if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(apiKey)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"unauthorized","message":"missing or invalid API key"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
