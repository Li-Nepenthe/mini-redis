package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Li-Nepenthe/mini-redis/ai-backend/internal/auth"
	"github.com/Li-Nepenthe/mini-redis/ai-backend/internal/config"
	"github.com/Li-Nepenthe/mini-redis/ai-backend/internal/httpapi"
	"github.com/Li-Nepenthe/mini-redis/ai-backend/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	repo, err := store.Open(startup, cfg.DSN, cfg.DBMaxOpen, cfg.DBMaxIdle, cfg.DBConnLifetime, cfg.MonthlyLimit)
	if err != nil {
		return err
	}
	defer repo.DB.Close()
	if err := repo.Migrate(startup); err != nil {
		return err
	}
	tokens, err := auth.NewTokens(cfg.JWTSecret, cfg.JWTTTL)
	if err != nil {
		return err
	}
	api := &httpapi.API{Repo: repo, Tokens: tokens, Quota: cfg.MonthlyLimit, Logger: logger, Ready: repo.DB.PingContext}
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: api.Router(),
		ReadHeaderTimeout: 5 * time.Second, IdleTimeout: time.Minute, MaxHeaderBytes: 32 << 10}
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()
	logger.Info("api listening", "address", listener.Addr().String(), "provider", "mock-not-yet-enabled")
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		// 用独立context排空，不能把已取消的信号context传给Shutdown。
		shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		err := server.Shutdown(shutdown)
		if err != nil {
			_ = server.Close()
		}
		serveErr := <-result
		if !errors.Is(serveErr, http.ErrServerClosed) {
			return errors.Join(err, serveErr)
		}
		return err
	}
}
