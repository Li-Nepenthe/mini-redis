package main

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestRunCancellationStopsBackgroundWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, "127.0.0.1:0", "") }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("canceled server did not stop")
	}
}

func TestRunListenFailureStopsBackgroundWork(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := run(context.Background(), listener.Addr().String(), ""); err == nil {
		t.Fatal("run hid a listen failure")
	}
}
