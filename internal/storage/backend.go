package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Backend owns formal files. No API handler writes or renames files in its home.
type Backend interface {
	Provision(context.Context, string, string, int64) (string, error)
	Upload(context.Context, string, string, io.Reader, int64) error
	Download(context.Context, string, string, string) (*http.Response, error)
	Mkdir(context.Context, string, string) error
	Move(context.Context, string, string, string) error
	Remove(context.Context, string, string) error
}
type SFTPGo struct {
	URL, AdminKey, AdminPassword string
	Client                       *http.Client
}

func (b *SFTPGo) call(ctx context.Context, key, method, endpoint string, body io.Reader) (*http.Response, error) {
	req, e := http.NewRequestWithContext(ctx, method, b.URL+endpoint, body)
	if e != nil {
		return nil, e
	}
	req.Header.Set("X-SFTPGO-API-KEY", key)
	if strings.HasPrefix(endpoint, "/api/v2/apikeys") {
		token, e := b.adminBearer(ctx)
		if e != nil {
			return nil, e
		}
		req.Header.Del("X-SFTPGO-API-KEY")
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if strings.HasPrefix(endpoint, "/api/v2/users") || strings.HasPrefix(endpoint, "/api/v2/apikeys") {
		req.Header.Set("Content-Type", "application/json")
	}
	client := b.Client
	if client == nil {
		client = &http.Client{Transport: &http.Transport{Proxy: nil, ResponseHeaderTimeout: 15 * time.Second, MaxIdleConnsPerHost: 8}}
	}
	res, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	if res.StatusCode == 404 && method == "DELETE" {
		res.Body.Close()
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
	}
	if res.StatusCode >= 300 {
		defer res.Body.Close()
		buf, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return nil, fmt.Errorf("SFTPGo %s: HTTP %d %s", method, res.StatusCode, buf)
	}
	return res, nil
}
func (b *SFTPGo) json(ctx context.Context, method, path string, v any, out any) error {
	data, e := json.Marshal(v)
	if e != nil {
		return e
	}
	res, e := b.call(ctx, b.AdminKey, method, path, bytes.NewReader(data))
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if out != nil {
		return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(out)
	}
	return nil
}
func (b *SFTPGo) Provision(ctx context.Context, id, root string, quota int64) (string, error) {
	user := map[string]any{"username": "yandu-" + id, "status": 1, "home_dir": root, "permissions": map[string][]string{"/": {"list", "download", "upload", "overwrite", "delete", "rename", "create_dirs", "delete_dirs"}}, "filters": map[string]any{"allow_api_key_auth": true, "denied_protocols": []string{"SSH", "FTP", "DAV"}}, "filesystem": map[string]any{"provider": 0}}
	existing, e := b.call(ctx, b.AdminKey, "GET", "/api/v2/users/"+url.PathEscape("yandu-"+id), nil)
	if e == nil {
		var previous struct {
			Home string `json:"home_dir"`
		}
		err := json.NewDecoder(existing.Body).Decode(&previous)
		existing.Body.Close()
		if err != nil || previous.Home != root {
			return "", fmt.Errorf("SFTPGo home differs; explicit migration required")
		}
	} else if e = b.json(ctx, "POST", "/api/v2/users", user, nil); e != nil {
		return "", e
	}
	var key struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	}
	if e := b.json(ctx, "POST", "/api/v2/apikeys", map[string]any{"name": "Yandu " + id, "scope": 2, "user": "yandu-" + id}, &key); e != nil {
		return "", e
	}
	return key.Key, nil
}
func (b *SFTPGo) Mkdir(ctx context.Context, key, path string) error {
	r, e := b.call(ctx, key, "POST", "/api/v2/user/dirs?path="+url.QueryEscape(path), nil)
	if e == nil {
		r.Body.Close()
	}
	return e
}
func (b *SFTPGo) Upload(ctx context.Context, key, path string, body io.Reader, length int64) error {
	req, e := http.NewRequestWithContext(ctx, "POST", b.URL+"/api/v2/user/files/upload?path="+url.QueryEscape(path), body)
	if e != nil {
		return e
	}
	req.ContentLength = length
	req.Header.Set("X-SFTPGO-API-KEY", key)
	req.Header.Set("Content-Type", "application/octet-stream")
	c := b.Client
	if c == nil {
		c = &http.Client{Transport: &http.Transport{Proxy: nil, ResponseHeaderTimeout: 30 * time.Second}}
	}
	r, e := c.Do(req)
	if e != nil {
		return e
	}
	defer r.Body.Close()
	io.Copy(io.Discard, io.LimitReader(r.Body, 2048))
	if r.StatusCode >= 300 {
		return fmt.Errorf("SFTPGo upload HTTP %d", r.StatusCode)
	}
	return nil
}
func (b *SFTPGo) Download(ctx context.Context, key, path, rangeHeader string) (*http.Response, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", b.URL+"/api/v2/user/files?path="+url.QueryEscape(path), nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("X-SFTPGO-API-KEY", key)
	req.Header.Set("Range", rangeHeader)
	c := b.Client
	if c == nil {
		c = &http.Client{Transport: &http.Transport{Proxy: nil, ResponseHeaderTimeout: 15 * time.Second}}
	}
	return c.Do(req)
}
func (b *SFTPGo) Move(ctx context.Context, key, from, to string) error {
	r, e := b.call(ctx, key, "POST", "/api/v2/user/file-actions/move?path="+url.QueryEscape(from)+"&target="+url.QueryEscape(to), nil)
	if e == nil {
		r.Body.Close()
	}
	return e
}
func (b *SFTPGo) Remove(ctx context.Context, key, path string) error {
	r, e := b.call(ctx, key, "DELETE", "/api/v2/user/files?path="+url.QueryEscape(path), nil)
	if e == nil {
		r.Body.Close()
	}
	return e
}

func (b *SFTPGo) adminBearer(ctx context.Context) (string, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", b.URL+"/api/v2/token", nil)
	if e != nil {
		return "", e
	}
	req.SetBasicAuth("yandu-provisioner", b.AdminPassword)
	c := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: nil}}
	r, e := c.Do(req)
	if e != nil {
		return "", e
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return "", fmt.Errorf("SFTPGo provisioning token HTTP %d", r.StatusCode)
	}
	var token struct {
		Access string `json:"access_token"`
	}
	e = json.NewDecoder(io.LimitReader(r.Body, 8192)).Decode(&token)
	return token.Access, e
}

func (b *SFTPGo) ValidateHome(ctx context.Context, id, root string) error {
	res, e := b.call(ctx, b.AdminKey, "GET", "/api/v2/users/"+url.PathEscape("yandu-"+id), nil)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	var user struct {
		Home string `json:"home_dir"`
	}
	if e = json.NewDecoder(res.Body).Decode(&user); e != nil {
		return e
	}
	if user.Home != root {
		return fmt.Errorf("SFTPGo home differs from storage metadata")
	}
	return nil
}
