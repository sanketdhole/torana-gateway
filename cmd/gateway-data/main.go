package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/phaselume/torana/internal/config"
	"github.com/phaselume/torana/internal/supervisor"
)

var (
	// Version is injected at build time using -ldflags "-X main.Version=x.y.z"
	Version = "1.0.0"
)

func main() {
	cfg, showVersion, err := config.ParseFlags(os.Args[1:])
	if err != nil {
		os.Exit(2)
	}

	if showVersion {
		fmt.Printf("gateway-data version %s\n", Version)
		os.Exit(0)
	}

	var level slog.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		if cfg.Debug {
			level = slog.LevelDebug
		} else if cfg.Environment == "production" {
			level = slog.LevelInfo
		} else {
			level = slog.LevelDebug
		}
	}

	format := strings.ToLower(cfg.LogFormat)
	if format == "" {
		if cfg.Environment == "production" {
			format = "json"
		} else {
			format = "text"
		}
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var logger *slog.Logger
	if format == "json" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, opts))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, opts))
	}

	logger.Info("bootstrapping gateway-data",
		"version", Version,
		"namespace", cfg.Namespace,
		"platform_url", cfg.PlatformURL,
		"listen_http", cfg.ListenHTTP,
		"listen_grpc", cfg.ListenGRPC,
		"environment", cfg.Environment,
		"debug", cfg.Debug,
		"log_level", level.String(),
	)

	if cfg.Token != "" {
		logger.Info("loaded connection parameters from TORANA_TOKEN",
			"org_id", cfg.OrgID,
			"namespace", cfg.Namespace,
			"platform_url", cfg.PlatformURL,
		)
	}

	sup := supervisor.New(cfg, logger)

	if err := sup.Run(context.Background()); err != nil {
		logger.Error("gateway supervisor terminated with error", "error", err)
		os.Exit(1)
	}
}
