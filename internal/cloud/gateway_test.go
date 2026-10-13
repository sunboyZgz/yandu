package cloud

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }
func TestGatewayOfflineAndPrivatePaths(t *testing.T) {
	h, e := NewGateway(GatewayConfig{Listen: "127.0.0.1:19091", Upstream: "http://127.0.0.1:1"})
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("GET", "/storage/v1/projects/example/capacity", nil)
	r.RemoteAddr = "127.0.0.1:3000"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	r.URL.Path = "/api/v1/status"
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
	r.URL.Path = "/storage/v1/projects/example/credentials"
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
	if _, e = NewGateway(GatewayConfig{Listen: "0.0.0.0:19091", Upstream: "http://127.0.0.1:19090"}); e == nil {
		t.Fatal("external plaintext bind accepted")
	}
}

func TestGatewayMonitorReadOnly(t *testing.T) {
	var called atomic.Int64
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		if r.URL.Path != "/storage/v1/monitor" || r.Header.Get("Authorization") != "Bearer monitoring" {
			t.Error("monitor forwarding changed")
		}
		w.Write([]byte(`{"deviceId":"owner"}`))
	}))
	defer backend.Close()
	h, e := NewGateway(GatewayConfig{Listen: "127.0.0.1:19091", Upstream: backend.URL})
	if e != nil {
		t.Fatal(e)
	}
	for _, method := range []string{"POST", "PATCH", "DELETE", "HEAD", "GET"} {
		r := httptest.NewRequest(method, "/storage/v1/monitor", nil)
		r.RemoteAddr = "127.0.0.1:3000"
		r.Header.Set("Authorization", "Bearer monitoring")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if method == "GET" && w.Code != 200 || method != "GET" && w.Code != 405 {
			t.Fatalf("method %s: %d", method, w.Code)
		}
	}
	for _, path := range []string{"/storage/v1/%6donitor", "/storage/v1/monitor?foo=bar", "/api/v1/status"} {
		r := httptest.NewRequest("GET", path, nil)
		r.RemoteAddr = "127.0.0.1:3000"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 404 {
			t.Fatal("private path accepted", path)
		}
	}
	if called.Load() != 1 {
		t.Fatal("write/private route forwarded")
	}
}
func TestGatewayGiBStreamingBudget(t *testing.T) {
	if os.Getenv("YANDU_BENCH_GIB") != "1" {
		t.Skip("set YANDU_BENCH_GIB=1 for 1 GiB streaming acceptance")
	}
	const size = int64(1 << 30)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		io.CopyBuffer(w, io.LimitReader(zeros{}, size), make([]byte, 64<<10))
	}))
	defer backend.Close()
	h, e := NewGateway(GatewayConfig{Listen: "127.0.0.1:19091", Upstream: backend.URL})
	if e != nil {
		t.Fatal(e)
	}
	gateway := httptest.NewServer(h)
	defer gateway.Close()
	runtime.GC()
	var initial runtime.MemStats
	runtime.ReadMemStats(&initial)
	var peak atomic.Uint64
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				var now runtime.MemStats
				runtime.ReadMemStats(&now)
				for {
					old := peak.Load()
					if now.HeapAlloc <= old || peak.CompareAndSwap(old, now.HeapAlloc) {
						break
					}
				}
			}
		}
	}()
	res, e := http.Get(gateway.URL + "/storage/v1/projects/example/objects/object/content")
	if e != nil {
		close(done)
		t.Fatal(e)
	}
	n, e := io.CopyBuffer(io.Discard, res.Body, make([]byte, 64<<10))
	res.Body.Close()
	close(done)
	if e != nil || n != size {
		t.Fatal(n, e)
	}
	increase := int64(peak.Load()) - int64(initial.HeapAlloc)
	if increase > 128<<20 {
		t.Fatalf("heap increase %d exceeds 128 MiB", increase)
	}
	t.Log(fmt.Sprintf("streamBytes=%d peakHeapIncreaseBytes=%d cloudStagingBytes=0 (gateway performs no filesystem operations)", n, increase))
}
