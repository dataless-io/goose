package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dataless-io/goose-broker-service/api"
	"github.com/dataless-io/goose-broker-service/broker"
	"github.com/fulldump/goconfig"
)

type Config struct {
	HTTPAddr          string `usage:"http bind address"`
	PersistenceDir    string `usage:"broker logs directory"`
	DefaultBrokerName string `usage:"default broker name"`
	Statics           string `usage:"optional statics directory override"`
	APIKey            string `usage:"api key for write operations"`

	HTTPReadTimeout       time.Duration `usage:"http read timeout"`
	HTTPReadHeaderTimeout time.Duration `usage:"http read header timeout"`
	HTTPWriteTimeout      time.Duration `usage:"http write timeout"`
	HTTPIdleTimeout       time.Duration `usage:"http idle timeout"`
	HTTPShutdownTimeout   time.Duration `usage:"http graceful shutdown timeout"`

	MaxLogSizeBytes   int64 `usage:"max size of a log file before rotation"`
	MaxRetainedLogs   int   `usage:"number of rotated log files retained"`
	UnsafeBatchWrites bool  `usage:"skip flush on every append"`
	SyncOnWrite       bool  `usage:"force fsync on every append"`
}

func main() {
	cfg := Config{
		HTTPAddr:              ":8081",
		PersistenceDir:        "./data",
		DefaultBrokerName:     "default",
		HTTPReadTimeout:       10 * time.Second,
		HTTPReadHeaderTimeout: 5 * time.Second,
		HTTPWriteTimeout:      0,
		HTTPIdleTimeout:       60 * time.Second,
		HTTPShutdownTimeout:   15 * time.Second,
		MaxLogSizeBytes:       10 * 1024 * 1024,
		MaxRetainedLogs:       5,
	}
	goconfig.Read(&cfg)

	manager, err := broker.NewManagerWithConfig(broker.ManagerConfig{
		BasePath: cfg.PersistenceDir,
		BrokerConfig: broker.Config{
			MaxLogSize:        cfg.MaxLogSizeBytes,
			MaxRetainedLogs:   cfg.MaxRetainedLogs,
			UnsafeBatchWrites: cfg.UnsafeBatchWrites,
			SyncOnWrite:       cfg.SyncOnWrite,
		},
		AutoLoadFiles: true,
	})
	if err != nil {
		log.Fatalf("failed to initialize broker manager: %v", err)
	}

	if cfg.DefaultBrokerName != "" {
		if _, exists := manager.GetBroker(cfg.DefaultBrokerName); !exists {
			if _, err := manager.CreateBroker(cfg.DefaultBrokerName); err != nil {
				log.Fatalf("failed to create default broker %q: %v", cfg.DefaultBrokerName, err)
			}
		}
	}

	handler := api.NewHTTPHandler(manager, api.Config{
		Statics: cfg.Statics,
		APIKey:  cfg.APIKey,
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	log.Printf("broker service listening on %s", cfg.HTTPAddr)
	log.Printf("persistence dir: %s", cfg.PersistenceDir)
	log.Printf("default broker: %s", cfg.DefaultBrokerName)
	if cfg.APIKey == "" {
		log.Println("write auth: disabled")
	} else {
		log.Println("write auth: enabled")
	}

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		log.Fatalf("http server failed: %v", err)
	case sig := <-sigCh:
		log.Printf("received signal %s, shutting down", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		if closeErr := server.Close(); closeErr != nil {
			log.Printf("force-close failed: %v", closeErr)
		}
	}

	if err := manager.Close(); err != nil {
		log.Printf("manager close failed: %v", err)
	}
	log.Println("broker service stopped")
}
