package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Bootstrap uses the one-time default admin password. Persist only the scoped admin API key afterwards.
func Bootstrap(ctx context.Context, endpoint, password string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: nil}}
	req, e := http.NewRequestWithContext(ctx, "GET", endpoint+"/api/v2/token", nil)
	if e != nil {
		return "", e
	}
	req.SetBasicAuth("yandu-provisioner", password)
	res, e := client.Do(req)
	if e != nil {
		return "", e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fmt.Errorf("SFTPGo bootstrap authentication HTTP %d", res.StatusCode)
	}
	var token struct {
		Access string `json:"access_token"`
	}
	if e = json.NewDecoder(res.Body).Decode(&token); e != nil {
		return "", e
	}
	call := func(method, path string, body any, out any) error {
		data, _ := json.Marshal(body)
		r, e := http.NewRequestWithContext(ctx, method, endpoint+path, bytes.NewReader(data))
		if e != nil {
			return e
		}
		r.Header.Set("Authorization", "Bearer "+token.Access)
		r.Header.Set("Content-Type", "application/json")
		response, e := client.Do(r)
		if e != nil {
			return e
		}
		defer response.Body.Close()
		if response.StatusCode >= 300 {
			msg, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
			return fmt.Errorf("SFTPGo bootstrap HTTP %d: %s", response.StatusCode, msg)
		}
		if out != nil {
			return json.NewDecoder(response.Body).Decode(out)
		}
		return nil
	}
	var admin map[string]any
	if e = call("GET", "/api/v2/admins/yandu-provisioner", nil, &admin); e != nil {
		return "", e
	}
	admin["filters"] = map[string]any{"allow_api_key_auth": true}
	if e = call("PUT", "/api/v2/admins/yandu-provisioner", admin, nil); e != nil {
		return "", e
	}
	var key struct {
		Key string `json:"key"`
	}
	e = call("POST", "/api/v2/apikeys", map[string]any{"name": "Yandu internal provisioning", "scope": 1, "admin": "yandu-provisioner"}, &key)
	return key.Key, e
}
