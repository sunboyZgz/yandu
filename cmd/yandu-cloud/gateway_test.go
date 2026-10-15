package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type gatewayListener struct {
	net.Listener
	closed chan struct{}
	once   sync.Once
}

func (l *gatewayListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return l.Listener.Close()
}

func TestGatewayWaitsForStreamingResponseBeforeExit(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	l := &gatewayListener{Listener: listener, closed: make(chan struct{})}
	finish := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "first chunk\n")
		w.(http.Flusher).Flush()
		select {
		case <-finish:
			io.WriteString(w, "last chunk\n")
		case <-r.Context().Done():
		}
	})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveGateway(ctx, server, l, 5*time.Second, "", "") }()
	t.Cleanup(func() { cancel(); server.Close() })
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Get("http://" + l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	if chunk, err := reader.ReadString('\n'); err != nil || chunk != "first chunk\n" {
		t.Fatal(chunk, err)
	}
	cancel()
	select {
	case <-l.closed:
	case <-time.After(5 * time.Second):
		t.Fatal("listener did not stop accepting requests")
	}
	select {
	case err := <-done:
		t.Fatalf("gateway exited during an active stream: %v", err)
	default:
	}
	close(finish)
	if tail, err := io.ReadAll(reader); err != nil || string(tail) != "last chunk\n" {
		t.Fatalf("stream interrupted: %q %v", tail, err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("gateway did not finish draining")
	}
}

func TestGatewayDrainDeadlineClosesRemainingConnections(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	stopped := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "first chunk\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(stopped)
	})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveGateway(ctx, server, listener, 20*time.Millisecond, "", "") }()
	t.Cleanup(func() { cancel(); server.Close() })
	response, err := (&http.Client{Timeout: 5 * time.Second}).Get("http://" + listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("drain exceeded deadline")
	}
	if _, err := io.ReadAll(reader); err == nil {
		t.Fatal("timed-out stream reported success")
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("handler context not cancelled")
	}
}

func TestGatewayCheckConfigDoesNotListenAndRejectsInvalidOptions(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	config := filepath.Join(t.TempDir(), "gateway.json")
	if err := os.WriteFile(config, []byte(`{"listen":"`+listener.Addr().String()+`","upstream":"http://127.0.0.1:19090"}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YANDU_GATEWAY_SHUTDOWN_TIMEOUT", "10m")
	if err := gateway([]string{"--config", config, "--check-config"}); err != nil {
		t.Fatal("check tried to bind occupied port:", err)
	}
	for _, timeout := range []string{"0s", "-1s", "25h", "invalid"} {
		if err := gateway([]string{"--config", config, "--check-config", "--shutdown-timeout", timeout}); err == nil {
			t.Fatal("invalid timeout accepted:", timeout)
		}
	}
	if err := os.WriteFile(config, []byte(`{"listen":"127.0.0.1:19091","upstream":"http://127.0.0.1:19090","tlsCert":"missing.crt","tlsKey":"missing.key"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := gateway([]string{"--config", config, "--check-config"}); err == nil || !strings.Contains(err.Error(), "TLS") {
		t.Fatal("missing TLS files accepted:", err)
	}
}
