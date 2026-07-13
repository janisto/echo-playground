package main

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestServeReturnsListenErrorWithoutListeningLog(t *testing.T) {
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := listener.Close(); closeErr != nil {
			t.Errorf("close listener: %v", closeErr)
		}
	})
	core, observed := observer.New(zap.InfoLevel)
	server := &http.Server{
		Addr:              listener.Addr().String(),
		Handler:           http.NotFoundHandler(),
		ReadHeaderTimeout: time.Second,
	}
	err = serve(t.Context(), server, time.Second, zap.New(core))
	if err == nil || !strings.Contains(err.Error(), "address already in use") {
		t.Fatalf("expected address-in-use error, got %v", err)
	}
	if observed.FilterMessage("server listening").Len() != 0 {
		t.Fatal("bind failure emitted a listening event")
	}
}

func TestServeDoesNotStartWithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	server := &http.Server{
		Addr:              "127.0.0.1:0",
		Handler:           http.NotFoundHandler(),
		ReadHeaderTimeout: time.Second,
	}
	if err := serve(ctx, server, time.Second, zap.NewNop()); err != nil {
		t.Fatalf("expected canceled startup to be a clean no-op, got %v", err)
	}
}
