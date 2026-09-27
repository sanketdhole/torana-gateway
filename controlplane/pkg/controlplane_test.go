package controlplane_test

import (
	"context"
	"crypto/ed25519"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"testing"
	"time"

	controlplanev1 "github.com/phaselume/torana/api/proto/controlplane/v1"
	"github.com/phaselume/torana/controlplane/pkg/auth"
	"github.com/phaselume/torana/controlplane/pkg/crypto"
	"github.com/phaselume/torana/controlplane/pkg/fleet"
	cprpc "github.com/phaselume/torana/controlplane/pkg/grpc"
	"github.com/phaselume/torana/controlplane/pkg/state"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCryptoSigner(t *testing.T) {
	tempKey := filepath.Join(t.TempDir(), "test-ed25519.key")
	signer, err := crypto.NewSigner(tempKey)
	if err != nil {
		t.Fatalf("crypto.NewSigner failed: %v", err)
	}

	if len(signer.PublicKeyBytes()) != ed25519.PublicKeySize {
		t.Errorf("expected public key size %d, got %d", ed25519.PublicKeySize, len(signer.PublicKeyBytes()))
	}

	sig := signer.SignSnapshot(42)
	if !signer.VerifySnapshot(42, sig) {
		t.Errorf("signature verification failed for version 42")
	}

	if signer.VerifySnapshot(43, sig) {
		t.Errorf("signature should not verify for wrong version 43")
	}

	// Reload from key file
	reloaded, err := crypto.NewSigner(tempKey)
	if err != nil {
		t.Fatalf("reloading signer failed: %v", err)
	}
	if reloaded.PublicKeyHex() != signer.PublicKeyHex() {
		t.Errorf("expected reloaded key hex %s, got %s", signer.PublicKeyHex(), reloaded.PublicKeyHex())
	}
}

func TestStateStoreVersioningAndRollback(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	signer, _ := crypto.NewSigner("")

	tempStorage := filepath.Join(t.TempDir(), "state.json")
	store, err := state.NewStore(tempStorage, signer, logger)
	if err != nil {
		t.Fatalf("state.NewStore failed: %v", err)
	}

	if store.ConfigVersion() != 1 {
		t.Errorf("expected initial version 1, got %d", store.ConfigVersion())
	}

	initialSnap := store.CurrentSnapshot()
	if initialSnap == nil || len(initialSnap.Ed25519Signature) == 0 {
		t.Fatalf("expected signed initial snapshot")
	}

	// Publish new config version 2
	schema := store.GetSchema()
	schema.Routes = append(schema.Routes, &controlplanev1.Route{
		Id:   "route-anthropic",
		Path: "/v1/messages",
	})

	snap2, err := store.PublishNewConfig(schema, "test-user", "Added Anthropic route")
	if err != nil {
		t.Fatalf("PublishNewConfig failed: %v", err)
	}
	if snap2.ConfigVersion != 2 {
		t.Errorf("expected version 2, got %d", snap2.ConfigVersion)
	}
	if len(snap2.Routes) != len(initialSnap.Routes)+1 {
		t.Errorf("expected %d routes, got %d", len(initialSnap.Routes)+1, len(snap2.Routes))
	}

	// Rollback to version 1 (which becomes version 3 with v1's content)
	snap3, err := store.Rollback(1, "test-admin")
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}
	if snap3.ConfigVersion != 3 {
		t.Errorf("expected rollback to create version 3, got %d", snap3.ConfigVersion)
	}
	if len(snap3.Routes) != len(initialSnap.Routes) {
		t.Errorf("expected %d routes after rollback, got %d", len(initialSnap.Routes), len(snap3.Routes))
	}
}

func TestFleetManagerAndGRPCStreaming(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	signer, _ := crypto.NewSigner("")
	authMgr := auth.NewManager("admin-secret", "dev-enroll-token")
	stateStore, _ := state.NewStore("", signer, logger)
	fleetMgr := fleet.NewManager(logger)

	// Start in-memory bufconn listener
	lis := bufconn.Listen(1024 * 1024)
	defer lis.Close()

	grpcServer := grpc.NewServer()
	cpServer := cprpc.NewServer("test-cluster", stateStore, fleetMgr, authMgr, logger)
	controlplanev1.RegisterControlPlaneServiceServer(grpcServer, cpServer)

	go func() {
		_ = grpcServer.Serve(lis)
	}()
	defer grpcServer.Stop()

	// Connect gRPC client via in-memory dialer
	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial gRPC: %v", err)
	}
	defer conn.Close()

	client := controlplanev1.NewControlPlaneServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Test Enrollment with valid token
	enrollResp, err := client.Enroll(ctx, &controlplanev1.EnrollRequest{
		Namespace:   "default",
		NodeId:      "node-test-1",
		EnrollToken: "dev-enroll-token",
		Version:     "0.1.0",
	})
	if err != nil {
		t.Fatalf("enroll RPC failed: %v", err)
	}
	if !enrollResp.Accepted || enrollResp.ClusterId != "test-cluster" {
		t.Errorf("expected accepted enrollment, got: %+v", enrollResp)
	}

	// 2. Test Enrollment with invalid token
	badEnrollResp, err := client.Enroll(ctx, &controlplanev1.EnrollRequest{
		Namespace:   "default",
		NodeId:      "node-bad",
		EnrollToken: "wrong-token",
	})
	if err != nil {
		t.Fatalf("enroll RPC failed: %v", err)
	}
	if badEnrollResp.Accepted {
		t.Errorf("expected rejected enrollment for bad token")
	}

	// 3. Test Stream connection and snapshot dispatch
	stream, err := client.Stream(ctx)
	if err != nil {
		t.Fatalf("failed to open stream: %v", err)
	}

	// Send initial Hello
	err = stream.Send(&controlplanev1.NodeMessage{
		NodeId:    "node-test-1",
		Namespace: "default",
		Timestamp: timestamppb.Now(),
		Payload: &controlplanev1.NodeMessage_Hello{
			Hello: &controlplanev1.Hello{
				Version:            "0.1.0",
				SupportedProtocols: []string{"http", "grpc"},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to send Hello: %v", err)
	}

	// Receive initial snapshot
	msg, err := stream.Recv()
	if err != nil {
		t.Fatalf("failed to receive initial snapshot: %v", err)
	}
	snap := msg.GetSnapshot()
	if snap == nil {
		t.Fatalf("expected Snapshot payload in control message, got: %+v", msg)
	}
	if snap.ConfigVersion != 1 {
		t.Errorf("expected initial snapshot config_version 1, got %d", snap.ConfigVersion)
	}

	// Verify Ed25519 signature
	if !signer.VerifySnapshot(snap.ConfigVersion, snap.Ed25519Signature) {
		t.Errorf("ed25519 signature verification on received snapshot failed")
	}

	// Send ACK back
	err = stream.Send(&controlplanev1.NodeMessage{
		NodeId:    "node-test-1",
		Namespace: "default",
		Timestamp: timestamppb.Now(),
		Payload: &controlplanev1.NodeMessage_Ack{
			Ack: &controlplanev1.Ack{
				AppliedConfigVersion: snap.ConfigVersion,
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to send Ack: %v", err)
	}

	// Verify fleet record updated
	time.Sleep(100 * time.Millisecond)
	nodes := fleetMgr.GetNodes()
	if len(nodes) != 1 || nodes[0].LastAckStatus != "ACK" {
		t.Errorf("expected node in fleet with ACK status, got: %+v", nodes)
	}
}
