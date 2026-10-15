package localapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"yandu/internal/model"
	"yandu/internal/reconcile"
	"yandu/internal/state"
)

func directoryAPI(t *testing.T) (*API, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s, err := state.Open(filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	a := New(&reconcile.Engine{Store: s, Dir: filepath.Join(root, "state")}, "secret")
	a.Sessions["browser"] = session{CSRF: "csrf", Expires: time.Now().Add(time.Hour)}
	return a, root
}

func directoryRequest(a *API, method, path, csrf string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://"+model.ManagementAddr+"/api/v1"+path, nil)
	r.AddCookie(&http.Cookie{Name: "yandu_session", Value: "browser"})
	r.Header.Set("X-CSRF-Token", csrf)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}

func TestBrowseDirectories(t *testing.T) {
	a, root := directoryAPI(t)
	for _, name := range []string{"素材 空格", "empty", ".hidden"} {
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "private.txt"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "empty"), filepath.Join(root, "link")); err != nil {
		t.Log("symlink unavailable:", err)
	}
	w := directoryRequest(a, "GET", "/directories/browse?path="+url.QueryEscape(root), "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct {
		Path, Parent string
		Directories  []directory
		Selectable   bool
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Path != root || result.Parent != filepath.Dir(root) || result.Selectable {
		t.Fatal("incorrect root navigation or state ancestor selectable", w.Body.String())
	}
	if len(result.Directories) != 3 || strings.Contains(w.Body.String(), "private.txt") || strings.Contains(w.Body.String(), `"name":"link"`) || strings.Contains(w.Body.String(), ".hidden") {
		t.Fatal("unexpected entries", w.Body.String())
	}
	w = directoryRequest(a, "GET", "/directories/browse?path="+url.QueryEscape(filepath.Join(root, "empty")), "")
	if !strings.Contains(w.Body.String(), `"directories":[]`) || !strings.Contains(w.Body.String(), `"selectable":true`) {
		t.Fatal("empty folder cannot be selected", w.Body.String())
	}
	for _, invalid := range []string{"relative/path", filepath.Join(root, "missing"), filepath.Join(root, "private.txt")} {
		if w := directoryRequest(a, "GET", "/directories/browse?path="+url.QueryEscape(invalid), ""); w.Code == 200 {
			t.Fatal("invalid directory accepted", invalid)
		}
	}
}

func TestPickDirectoryAuthorizationAndCancellation(t *testing.T) {
	a, root := directoryAPI(t)
	folder := filepath.Join(root, "素材 空格")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	calls := 0
	a.directoryPicker = func(context.Context) (string, error) { calls++; return folder, nil }
	if w := directoryRequest(a, "POST", "/directories/pick", ""); w.Code != 403 || calls != 0 {
		t.Fatal("picker bypassed CSRF")
	}
	request := httptest.NewRequest("POST", "http://"+model.ManagementAddr+"/api/v1/directories/pick", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, request)
	if w.Code != 403 || calls != 0 {
		t.Fatal("anonymous picker opened")
	}
	w = directoryRequest(a, "POST", "/directories/pick", "csrf")
	var picked map[string]string
	json.Unmarshal(w.Body.Bytes(), &picked)
	if w.Code != 200 || picked["path"] != folder {
		t.Fatal(w.Code, w.Body.String())
	}
	snapshot, _ := a.Engine.Store.Read()
	if len(snapshot.Roots) != 0 {
		t.Fatal("selection silently granted directory access")
	}
	a.directoryPicker = func(context.Context) (string, error) { return "", nil }
	if w = directoryRequest(a, "POST", "/directories/pick", "csrf"); !strings.Contains(w.Body.String(), `"cancelled":true`) {
		t.Fatal(w.Body.String())
	}
	a.directoryPicker = func(context.Context) (string, error) { return "", errors.New("no desktop") }
	if w = directoryRequest(a, "POST", "/directories/pick", "csrf"); w.Code != 400 || !strings.Contains(w.Body.String(), "PICKER_UNAVAILABLE") {
		t.Fatal(w.Body.String())
	}
	for _, invalid := range []string{root, a.Engine.Dir, filepath.Join(a.Engine.Dir, "private"), filepath.VolumeName(root) + string(filepath.Separator)} {
		a.directoryPicker = func(context.Context) (string, error) { return invalid, nil }
		if w = directoryRequest(a, "POST", "/directories/pick", "csrf"); w.Code == 200 {
			t.Fatal("unsafe selection accepted", invalid)
		}
	}
	a.pickerMu.Lock()
	w = directoryRequest(a, "POST", "/directories/pick", "csrf")
	a.pickerMu.Unlock()
	if w.Code != 409 {
		t.Fatal("concurrent system dialogs allowed")
	}
}
