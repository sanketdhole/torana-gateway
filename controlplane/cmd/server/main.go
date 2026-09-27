package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	controlplanev1 "github.com/phaselume/torana/api/proto/controlplane/v1"
	"github.com/phaselume/torana/controlplane/pkg/auth"
	"github.com/phaselume/torana/controlplane/pkg/crypto"
	"github.com/phaselume/torana/controlplane/pkg/fleet"
	cprpc "github.com/phaselume/torana/controlplane/pkg/grpc"
	"github.com/phaselume/torana/controlplane/pkg/rest"
	"github.com/phaselume/torana/controlplane/pkg/state"
	"github.com/phaselume/torana/controlplane/ui"
	"google.golang.org/grpc"
)

var (
	Version = "0.2.0-enterprise"
)

func main() {
	grpcPort := flag.String("grpc-port", ":9090", "gRPC control plane listen address (for gateway-data nodes)")
	httpPort := flag.String("http-port", ":8080", "HTTP REST API and React UI listen address")
	storageFile := flag.String("storage-file", "controlplane/data/config-state.json", "Persistent configuration state JSON file")
	keyFile := flag.String("key-file", "controlplane/data/ed25519.key", "Ed25519 signing keypair file")
	adminToken := flag.String("admin-token", "torana-admin-secret-key", "Secret admin token for UI & API operations")
	enrollToken := flag.String("enroll-token", "torana-enroll-dev-secret", "Enrollment token expected from data plane gateways")
	uiDir := flag.String("ui-dir", "controlplane/ui/dist", "Path to React compiled UI assets")
	clusterID := flag.String("cluster-id", "torana-cluster-primary", "Cluster identifier")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	logger.Info("starting Torana Enterprise Control Plane Service",
		"version", Version,
		"cluster_id", *clusterID,
		"grpc_port", *grpcPort,
		"http_port", *httpPort,
		"storage_file", *storageFile,
	)

	// Ensure data directory exists
	if dir := filepath.Dir(*storageFile); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}
	if dir := filepath.Dir(*keyFile); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	// 1. Initialize Ed25519 Signer
	signer, err := crypto.NewSigner(*keyFile)
	if err != nil {
		logger.Error("failed to initialize Ed25519 signer", "error", err)
		os.Exit(1)
	}
	logger.Info("Ed25519 cryptographic signer ready", "public_key_hex", signer.PublicKeyHex())

	// 2. Initialize Auth & RBAC Manager
	authMgr := auth.NewManager(*adminToken, *enrollToken)

	// 3. Initialize State Store
	stateStore, err := state.NewStore(*storageFile, signer, logger)
	if err != nil {
		logger.Error("failed to initialize state store", "error", err)
		os.Exit(1)
	}
	logger.Info("configuration store initialized", "active_version", stateStore.ConfigVersion())

	// 4. Initialize Fleet Manager
	fleetMgr := fleet.NewManager(logger)

	// 5. Start gRPC Control Plane Server
	grpcLis, err := net.Listen("tcp", *grpcPort)
	if err != nil {
		logger.Error("failed to listen on gRPC port", "addr", *grpcPort, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	cpGrpcServer := cprpc.NewServer(*clusterID, stateStore, fleetMgr, authMgr, logger)
	controlplanev1.RegisterControlPlaneServiceServer(grpcServer, cpGrpcServer)

	go func() {
		logger.Info("gRPC Control Plane listening", "addr", *grpcPort)
		if err := grpcServer.Serve(grpcLis); err != nil {
			logger.Error("gRPC server terminated", "error", err)
		}
	}()

	// 6. Start HTTP REST & React UI Server
	restServer := rest.NewServer(stateStore, fleetMgr, authMgr, signer, logger, *uiDir, ui.Dist())
	httpServer := &http.Server{
		Addr:         *httpPort,
		Handler:      restServer.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // 0 for SSE streaming
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("HTTP Control Panel & REST API listening",
			"url", fmt.Sprintf("http://localhost%s", *httpPort),
			"ui_dir", *uiDir,
		)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
		}
	}()

	// Graceful shutdown on SIGINT / SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutting down Torana Control Plane service gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = httpServer.Shutdown(shutdownCtx)
	grpcServer.GracefulStop()

	logger.Info("Torana Control Plane service exited cleanly")
}
