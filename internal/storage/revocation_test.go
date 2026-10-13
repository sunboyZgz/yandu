package storage

import (
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRevocationInterruptsBlockedTusBody(t *testing.T) {
	f := setup(t)
	v := f.intent(t, "stalled.bin", "", "")
	created := f.call(t, "POST", "/tus/", nil, map[string]string{"Tus-Resumable": "1.0.0", "Upload-Length": "3", "Upload-Metadata": "intentId " + base64.StdEncoding.EncodeToString([]byte(v["id"]))})
	if created.Code != 201 {
		t.Fatal(created.Body.String())
	}
	server := httptest.NewServer(f.s.PublicHandler())
	defer server.Close()
	conn, e := net.Dial("tcp", server.Listener.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "PATCH /storage/v1/projects/one/tus/%s HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer %s\r\nTus-Resumable: 1.0.0\r\nUpload-Offset: 0\r\nContent-Type: application/offset+octet-stream\r\nContent-Length: 3\r\n\r\na", v["id"], server.Listener.Addr(), f.token)
	time.Sleep(150 * time.Millisecond)
	start := time.Now()
	f.s.DB.Exec("UPDATE credentials SET revoked=1")
	conn.SetReadDeadline(time.Now().Add(4500 * time.Millisecond))
	_, e = io.ReadAll(conn)
	if n, ok := e.(net.Error); ok && n.Timeout() {
		t.Fatal("revocation did not interrupt blocked upload within five seconds")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("revocation exceeded five seconds")
	}
}
