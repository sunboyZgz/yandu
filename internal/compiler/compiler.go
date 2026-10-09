package compiler

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"yandu/internal/model"
)

func quote(v string) string { return strconv.Quote(v) }
func Caddy(s model.Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "{\n admin %s {\n enforce_origin\n }\n persist_config off\n auto_https off\n}\nhttp://%s {\n bind 127.0.0.1\n header X-Content-Type-Options nosniff\n", model.CaddyAdmin, strings.TrimPrefix(model.ResourceAddr, "127.0.0.1"))
	for i, p := range s.Effective {
		site, ok := s.Site(p.SiteID)
		if !ok || p.Publication != "active" {
			continue
		}
		fmt.Fprintf(&b, " @p%d {\n host %s\n path", i, site.Domain)
		for _, pre := range p.ResourcePrefixes {
			fmt.Fprintf(&b, " %s*", pre)
		}
		fmt.Fprintf(&b, "\n }\n handle @p%d {\n reverse_proxy %s {\n method GET\n rewrite /authorize\n header_up X-Forwarded-Method {method}\n header_up X-Forwarded-Uri {uri}\n header_up X-Yandu-Generation %s\n @allowed status 2xx\n handle_response @allowed {\n route {\n", i, model.GuardAddr, p.Generation)
		fmt.Fprintf(&b, " @probe%d path", i)
		for _, pre := range p.ResourcePrefixes {
			fmt.Fprintf(&b, " %s.yandu-probe-%s", pre, p.ProbeToken)
		}
		fmt.Fprintf(&b, "\n respond @probe%d %s 200\n uri strip_prefix %s\n root * %s\n header Content-Type {http.reverse_proxy.header.X-Yandu-Type}\n", i, quote(p.ProbeToken), p.ProjectBase, quote(strings.ReplaceAll(p.RootPath, "\\", "/")))
		cache := "no-cache, must-revalidate"
		if p.CachePolicy == "immutable" {
			cache = "public, max-age=31536000, immutable"
		}
		if p.AccessPolicy == "basic_auth" {
			cache = "private, no-store"
		}
		fmt.Fprintf(&b, " header Cache-Control %s\n file_server {\n index __yandu_no_directory_index__\n }\n }\n }\n }\n }\n", quote(cache))
	}
	b.WriteString(" handle {\n respond 404\n }\n}\n")
	return b.String()
}
func FRPC(s model.Snapshot, caPath, adminPassword string) string {
	if s.Profile == nil {
		return ""
	}
	p := s.Profile
	var b strings.Builder
	fmt.Fprintf(&b, "serverAddr = %s\nserverPort = %d\nloginFailExit = false\nauth.method = \"token\"\nauth.token = %s\ntransport.tls.enable = true\ntransport.tls.trustedCaFile = %s\ntransport.tls.serverName = %s\nwebServer.addr = \"127.0.0.1\"\nwebServer.port = 18768\nwebServer.user = \"yandu\"\nwebServer.password = %s\nlog.to = \"console\"\nlog.level = \"warn\"\n", quote(p.ServerAddr), p.ServerPort, quote(p.TunnelToken), quote(caPath), quote(p.TLSName), quote(adminPassword))
	for _, pr := range s.Effective {
		site, ok := s.Site(pr.SiteID)
		if !ok || pr.Publication != "active" {
			continue
		}
		for _, pre := range pr.ResourcePrefixes {
			fmt.Fprintf(&b, "\n[[proxies]]\nname = %s\ntype = \"http\"\nlocalIP = \"127.0.0.1\"\nlocalPort = 18766\ncustomDomains = [%s]\nlocations = [%s]\n", quote(p.DeviceID+"-"+model.RouteID(pr.ID, pre)), quote(site.Domain), quote(pre))
		}
	}
	return b.String()
}
func Routes(s model.Snapshot, site string) []model.Route {
	out := append([]model.Route{}, s.Reserved[site]...)
	for _, p := range s.Effective {
		if p.SiteID != site {
			continue
		}
		for _, pre := range p.ResourcePrefixes {
			out = append(out, model.Route{ID: model.RouteID(p.ID, pre), ProjectID: p.ID, Prefix: pre, State: p.Publication, AccessPolicy: p.AccessPolicy})
		}
	}
	dedup := map[string]model.Route{}
	for _, r := range out {
		dedup[r.ID] = r
	}
	out = []model.Route{}
	for _, r := range dedup {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if out == nil {
		out = []model.Route{}
	}
	return out
}
func Digest(routes []model.Route) string { b, _ := json.Marshal(routes); return model.Hex(b) }
func Nginx(routes []model.Route, generation string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Yandu managed generation %s; do not edit\n", generation)
	for _, r := range routes {
		fmt.Fprintf(&b, "location ^~ %s {\n add_header X-Yandu-Generation %s always;\n add_header X-Content-Type-Options nosniff always;\n", r.Prefix, generation)
		if r.State != "active" {
			b.WriteString(" return 404;\n}\n")
			continue
		}
		b.WriteString(" limit_except GET HEAD { deny all; }\n proxy_set_header Host $host;\n proxy_set_header Cookie \"\";\n proxy_set_header X-Yandu-Generation \"\";\n proxy_hide_header Set-Cookie;\n proxy_hide_header X-Yandu-Type;\n proxy_hide_header X-Content-Type-Options;\n")
		if r.AccessPolicy == "public_read" {
			b.WriteString(" proxy_set_header Authorization \"\";\n")
		}
		b.WriteString(" proxy_pass http://127.0.0.1:18080;\n proxy_cache off;\n proxy_buffering off;\n proxy_connect_timeout 5s;\n proxy_read_timeout 60s;\n}\n")
	}
	return b.String()
}
