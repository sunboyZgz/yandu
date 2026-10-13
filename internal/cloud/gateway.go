package cloud

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
	"yandu/internal/pathpolicy"
)

type GatewayConfig struct {
	Listen       string   `json:"listen"`
	Upstream     string   `json:"upstream"`
	AllowedCIDRs []string `json:"allowedCidrs"`
	TLSCert      string   `json:"tlsCert,omitempty"`
	TLSKey       string   `json:"tlsKey,omitempty"`
}

func NewGateway(c GatewayConfig) (http.Handler, error) {
	host, _, e := net.SplitHostPort(c.Listen)
	if e != nil {
		return nil, e
	}
	ip := net.ParseIP(host)
	local := ip != nil && ip.IsLoopback()
	if !local && (c.TLSCert == "" || c.TLSKey == "" || len(c.AllowedCIDRs) == 0) {
		return nil, fmt.Errorf("non-loopback listener requires TLS and explicit allowed CIDRs")
	}
	upstream, e := url.Parse(c.Upstream)
	if e != nil || upstream.Scheme != "http" || upstream.User != nil || upstream.Path != "" || upstream.RawQuery != "" {
		return nil, fmt.Errorf("gateway requires loopback HTTP tunnel upstream")
	}
	upHost, _, e := net.SplitHostPort(upstream.Host)
	if e != nil || net.ParseIP(upHost) == nil || !net.ParseIP(upHost).IsLoopback() {
		return nil, fmt.Errorf("upstream must bind loopback")
	}
	cidrs := []*net.IPNet{}
	for _, v := range c.AllowedCIDRs {
		_, n, e := net.ParseCIDR(v)
		if e != nil {
			return nil, e
		}
		cidrs = append(cidrs, n)
	}
	if local && len(cidrs) == 0 {
		_, v, _ := net.ParseCIDR("127.0.0.0/8")
		cidrs = append(cidrs, v)
		_, v, _ = net.ParseCIDR("::1/128")
		cidrs = append(cidrs, v)
	}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.FlushInterval = -1
	proxy.Transport = &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext, ResponseHeaderTimeout: 30 * time.Second, IdleConnTimeout: 60 * time.Second, MaxIdleConnsPerHost: 16, MaxResponseHeaderBytes: 16384}
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		for _, h := range []string{"Forwarded", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Forwarded-For", "Cookie"} {
			r.Header.Del(h)
		}
		r.Host = upstream.Host
	}
	proxy.ModifyResponse = func(r *http.Response) error {
		r.Header.Del("Set-Cookie")
		if loc := r.Header.Get("Location"); loc != "" {
			u, e := url.Parse(loc)
			if e != nil || !strings.HasPrefix(u.Path, "/storage/v1/") {
				return fmt.Errorf("invalid storage upload Location")
			}
			r.Header.Set("Location", u.RequestURI())
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(503)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "DEVICE_OFFLINE", "message": "local storage device unavailable"}})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, _, e := net.SplitHostPort(r.RemoteAddr)
		if e != nil {
			w.WriteHeader(403)
			return
		}
		allowed := false
		for _, n := range cidrs {
			if n.Contains(net.ParseIP(h)) {
				allowed = true
			}
		}
		if !allowed {
			w.WriteHeader(403)
			return
		}
		// Separate operator credential; the device validates it. No write or file
		// endpoint can be reached with this credential.
		if r.URL.Path == "/storage/v1/monitor" && r.URL.EscapedPath() == r.URL.Path && r.URL.RawQuery == "" {
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			proxy.ServeHTTP(w, r)
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 4 || parts[0] != "storage" || parts[1] != "v1" || parts[2] != "projects" || !pathpolicy.Identifier(parts[3]) || strings.ContainsAny(r.URL.EscapedPath(), "%\\") || strings.Contains(r.URL.Path, "//") || (len(parts) > 4 && parts[4] == "credentials") {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case "GET", "HEAD", "POST", "PATCH", "DELETE", "OPTIONS":
		default:
			w.WriteHeader(405)
			return
		}
		proxy.ServeHTTP(w, r)
	}), nil
}
