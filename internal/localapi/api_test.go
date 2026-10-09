package localapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"yandu/internal/model"
	"yandu/internal/reconcile"
	"yandu/internal/state"
)

func TestHostOriginSessionAndCSRF(t *testing.T) {
	s, e := state.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a := New(&reconcile.Engine{Store: s, Ctx: context.Background()}, "secret")
	call := func(method, path, host, origin, bearer, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://"+host+path, bytes.NewBufferString(body))
		req.Host = host
		req.Header.Set("Origin", origin)
		req.Header.Set("Authorization", bearer)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, req)
		return w
	}
	if w := call("GET", "/api/v1/health", "evil.com", "", "", ""); w.Code != 403 {
		t.Fatal("DNS rebinding allowed")
	}
	if w := call("GET", "/api/v1/health", model.ManagementAddr, "http://evil.com", "", ""); w.Code != 403 {
		t.Fatal("cross-origin allowed")
	}
	if w := call("GET", "/api/v1/status", model.ManagementAddr, "", "", ""); w.Code != 403 {
		t.Fatal("anonymous API allowed")
	}
	w := call("POST", "/api/v1/launch", model.ManagementAddr, "", "Bearer secret", "{}")
	var ticket map[string]string
	json.Unmarshal(w.Body.Bytes(), &ticket)
	if ticket["ticket"] == "" {
		t.Fatal("no ticket")
	}
	body, _ := json.Marshal(ticket)
	w = call("POST", "/api/v1/session", model.ManagementAddr, "http://"+model.ManagementAddr, "", string(body))
	if w.Code != 200 || len(w.Result().Cookies()) == 0 {
		t.Fatal("session exchange failed")
	}
	if w2 := call("POST", "/api/v1/session", model.ManagementAddr, "http://"+model.ManagementAddr, "", string(body)); w2.Code == 200 {
		t.Fatal("ticket reused")
	}
	req := httptest.NewRequest("POST", "http://"+model.ManagementAddr+"/api/v1/roots", bytes.NewBufferString(`{"path":"/tmp"}`))
	req.Host = model.ManagementAddr
	req.AddCookie(w.Result().Cookies()[0])
	out := httptest.NewRecorder()
	a.ServeHTTP(out, req)
	if out.Code != 403 {
		t.Fatal("CSRF accepted")
	}
	if !w.Result().Cookies()[0].HttpOnly {
		t.Fatal("session cookie readable")
	}
}
