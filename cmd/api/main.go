package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"

	"git.assilvestrar.club/lourenco/k8s-mtp/internal/config"
	"git.assilvestrar.club/lourenco/k8s-mtp/internal/handlers"
	"git.assilvestrar.club/lourenco/k8s-mtp/internal/middleware"
	"git.assilvestrar.club/lourenco/k8s-mtp/internal/server"
	"git.assilvestrar.club/lourenco/k8s-mtp/internal/store"
)

func main() {
	configFile := flag.String("config", "config.json", "config file")
	debug := flag.Bool("debug", false, "enable debug mode")
	flag.Parse()

	_ = debug

	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config file: %v\n", err)
		os.Exit(1)
	}

	server.Init(cfg)
	logger := server.Logger()

	db, err := store.New(cfg, logger)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err = db.RunMigrationsEmbedded(); err != nil {
		if err == migrate.ErrNoChange {
			logger.Info("migrations already up to date")
		} else {
			logger.Error("failed to run migrations", "error", err)
			os.Exit(1)
		}
	}

	auth := middleware.NewAuthMiddleware(cfg, logger)
	if auth == nil {
		logger.Error("failed to wire the auth middlware")
		os.Exit(1)
	}
	rl := middleware.NewRateLimiter(cfg, logger)
	if rl == nil {
		logger.Error("failed to wire the rate limiter middleware")
		os.Exit(1)
	}
	tenantHandler := handlers.NewTenantHandler(db, logger)
	if tenantHandler == nil {
		logger.Error("failed to wire the tenant handler middleware")
		os.Exit(1)
	}
	memberHandler := handlers.NewMemberHandler(db, logger)
	if memberHandler == nil {
		logger.Error("failed to wire the member handler middlware")
		os.Exit(1)
	}

	srv := server.NewServer(cfg, logger, db, auth, rl, tenantHandler, memberHandler)

	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	// graceful shutdown
	logger.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped gracefully")
}
