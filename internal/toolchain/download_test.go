package toolchain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func downloadFixture(t *testing.T, handler http.HandlerFunc, payload string) (asset, string, *http.Client) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	digest := sha256.Sum256([]byte(payload))
	return asset{URL: server.URL + "/frp.tar.gz", SHA256: fmt.Sprintf("%x", digest)}, filepath.Join(t.TempDir(), "frp.tar.gz"), &http.Client{Timeout: time.Second}
}

func assertNoPartialDownload(t *testing.T, archive string) {
	t.Helper()
	entries, e := filepath.Glob(filepath.Join(filepath.Dir(archive), ".download-*"))
	if e != nil || len(entries) != 0 {
		t.Fatalf("partial downloads left behind: %v, %v", entries, e)
	}
}

func TestDownloadRetriesTransientHTTPAndPublishesOnlyVerifiedArchive(t *testing.T) {
	var calls atomic.Int32
	payload := "verified release archive"
	a, path, client := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, payload)
	}, payload)
	var progress bytes.Buffer
	if e := downloadArchive(context.Background(), a, path, client, &progress); e != nil {
		t.Fatal(e)
	}
	actual, e := os.ReadFile(path)
	if e != nil || string(actual) != payload || calls.Load() != 2 {
		t.Fatalf("retry did not publish verified archive: %q %v, calls %d", actual, e, calls.Load())
	}
	if !strings.Contains(progress.String(), a.URL) || !strings.Contains(progress.String(), "2/3") || !strings.Contains(progress.String(), "校验完成") {
		t.Fatalf("missing download diagnostics: %s", &progress)
	}
	assertNoPartialDownload(t, path)
}

func TestInterruptedDownloadRetriesWithoutPublishingPartialContent(t *testing.T) {
	var calls atomic.Int32
	payload := "complete archive"
	a, path, client := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Length", "100")
			fmt.Fprint(w, "partial")
			return
		}
		fmt.Fprint(w, payload)
	}, payload)
	if e := downloadArchive(context.Background(), a, path, client, nil); e != nil {
		t.Fatal(e)
	}
	actual, _ := os.ReadFile(path)
	if string(actual) != payload || calls.Load() != 2 {
		t.Fatal("interrupted download was published or not retried")
	}
	assertNoPartialDownload(t, path)
}

func TestDownloadChecksumMismatchDoesNotRetryOrReplaceExistingFile(t *testing.T) {
	var calls atomic.Int32
	a, path, client := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, "tampered")
	}, "expected")
	if e := os.WriteFile(path, []byte("keep existing archive"), 0644); e != nil {
		t.Fatal(e)
	}
	e := downloadArchive(context.Background(), a, path, client, nil)
	if e == nil || !strings.Contains(e.Error(), "SHA256") || calls.Load() != 1 {
		t.Fatalf("checksum failure retried or hidden: %v, calls %d", e, calls.Load())
	}
	actual, _ := os.ReadFile(path)
	if string(actual) != "keep existing archive" {
		t.Fatal("unverified download replaced existing archive")
	}
	assertNoPartialDownload(t, path)
}

func TestDownloadPermanentHTTPErrorDoesNotRetry(t *testing.T) {
	var calls atomic.Int32
	a, path, client := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}, "expected")
	e := downloadArchive(context.Background(), a, path, client, nil)
	if e == nil || !strings.Contains(e.Error(), "404") || !strings.Contains(e.Error(), a.URL) || calls.Load() != 1 {
		t.Fatalf("permanent error was retried or lost: %v, calls %d", e, calls.Load())
	}
}

func TestDownloadTimeoutHasBoundedRetriesAndURLInError(t *testing.T) {
	var calls atomic.Int32
	a, path, client := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		<-r.Context().Done()
	}, "expected")
	client.Timeout = 50 * time.Millisecond
	e := downloadArchive(context.Background(), a, path, client, nil)
	if e == nil || !errors.Is(e, context.DeadlineExceeded) || !strings.Contains(e.Error(), a.URL) || calls.Load() != 3 {
		t.Fatalf("timeout retry bound or cause missing: %v, calls %d", e, calls.Load())
	}
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("timeout published an archive")
	}
}

func TestDownloadCancellationInterruptsRetryBackoff(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, path, client := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}, "expected")
	// Cancel as soon as the first retry is announced, avoiding timing races.
	progress := cancelOnRetry{cancel: cancel}
	e := downloadArchive(ctx, a, path, client, progress)
	if !errors.Is(e, context.Canceled) || calls.Load() != 1 {
		t.Fatalf("cancellation retried request: %v, calls %d", e, calls.Load())
	}
}

type cancelOnRetry struct{ cancel context.CancelFunc }

func (w cancelOnRetry) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte("秒后重试")) {
		w.cancel()
	}
	return len(p), nil
}
