package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gofrs/flock"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"yandu/internal/cloudsync"
	"yandu/internal/compiler"
	"yandu/internal/model"
	"yandu/internal/pathpolicy"
)

type RegisteredSite struct {
	model.Site
	ProbeURL string `json:"probeURL"`
	Blocked  bool   `json:"blocked,omitempty"`
}
type Registry struct {
	Sites   []RegisteredSite    `json:"sites"`
	Devices map[string][]string `json:"devices"`
}
type OwnedRoute struct {
	model.Route
	Owner string `json:"owner"`
}
type Recorded struct {
	Digest  string        `json:"digest"`
	Receipt model.Receipt `json:"receipt"`
}
type State struct {
	Generation string                  `json:"generation"`
	Revisions  map[string]int64        `json:"revisions"`
	Sources    map[string]int64        `json:"sources"`
	Routes     map[string][]OwnedRoute `json:"routes"`
	Operations map[string]Recorded     `json:"operations"`
}
type Helper struct {
	Dir          string
	RegistryPath string
	Nginx        string
	Runner       func(context.Context, string, ...string) error
	Verify       func(context.Context, RegisteredSite, []model.Route, string) error
}

func Atomic(path string, b []byte, mode os.FileMode) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	tmp := path + ".tmp-" + model.ID()
	if e := os.WriteFile(tmp, b, mode); e != nil {
		return e
	}
	defer os.Remove(tmp)
	f, e := os.OpenFile(tmp, os.O_RDWR, mode)
	if e != nil {
		return e
	}
	e = f.Sync()
	f.Close()
	if e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func (h Helper) run(ctx context.Context, args ...string) error {
	if h.Runner != nil {
		return h.Runner(ctx, h.Nginx, args...)
	}
	out, e := exec.CommandContext(ctx, h.Nginx, args...).CombinedOutput()
	if e != nil {
		return fmt.Errorf("%s: %s", e, string(out))
	}
	return nil
}
func readJSON(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func (h Helper) Call(ctx context.Context, principal string, req cloudsync.Request) (model.Receipt, error) {
	var result model.Receipt
	if !pathpolicy.Identifier(principal) {
		return result, model.Err("CLOUD_AUTH_DENIED", "设备身份无效")
	}
	if e := os.MkdirAll(h.Dir, 0700); e != nil {
		return result, e
	}
	lock := flock.New(filepath.Join(h.Dir, "apply.lock"))
	ok, e := lock.TryLockContext(ctx, 100*time.Millisecond)
	if e != nil || !ok {
		return result, model.Err("CLOUD_BUSY", "配置助手正在处理其他操作")
	}
	defer lock.Unlock()
	var reg Registry
	if e = readJSON(h.RegistryPath, &reg); e != nil {
		return result, e
	}
	siteID := req.SiteID
	if req.Manifest != nil {
		siteID = req.Manifest.SiteID
	}
	var site RegisteredSite
	authorized := false
	for _, id := range reg.Devices[principal] {
		if id == siteID {
			authorized = true
		}
	}
	for _, v := range reg.Sites {
		if v.ID == siteID {
			site = v
		}
	}
	if !authorized || site.ID == "" {
		return result, model.Err("CLOUD_AUTH_DENIED", "设备未被授权管理该站点")
	}
	if site.Blocked {
		return result, model.Err("CLOUD_BLOCKED", "资源入口已由云端管理员封禁，普通同步不能恢复")
	}
	s := State{Revisions: map[string]int64{}, Sources: map[string]int64{}, Routes: map[string][]OwnedRoute{}, Operations: map[string]Recorded{}}
	if e = readJSON(filepath.Join(h.Dir, "state.json"), &s); e != nil && !os.IsNotExist(e) {
		return result, e
	}
	// Resume an interrupted transaction before accepting another mutation.
	if e = h.recover(ctx, &s); e != nil {
		return result, e
	}
	if req.Action == "status" {
		return model.Receipt{SiteID: siteID, Revision: s.Revisions[siteID], Generation: s.Generation, Verified: s.Generation != ""}, nil
	}
	if req.Action == "result" {
		v, ok := s.Operations[principal+":"+req.OperationID]
		if !ok {
			return result, model.Err("OPERATION_NOT_FOUND", "未找到操作回执")
		}
		return v.Receipt, nil
	}
	if req.Action != "apply" || req.Manifest == nil {
		return result, model.Err("INVALID_MANIFEST", "不支持的配置操作")
	}
	m := *req.Manifest
	b, _ := json.Marshal(m)
	digest := model.Hex(b)
	opKey := principal + ":" + m.OperationID
	if old, ok := s.Operations[opKey]; ok {
		if old.Digest != digest {
			return result, model.Err("OPERATION_CONFLICT", "相同 operationId 对应不同清单")
		}
		return old.Receipt, nil
	}
	if m.ExpectedCloudRevision != s.Revisions[siteID] {
		return result, model.Err("REVISION_CONFLICT", "云端版本已改变，请读取状态后重新应用")
	}
	if m.SchemaVersion != 1 || len(m.OperationID) != 32 || !hexID(m.OperationID) || len(m.Routes) > 100 || len(m.Release) > 100 || m.SourceRevision < s.Sources[siteID+":"+principal] {
		return result, model.Err("INVALID_MANIFEST", "清单版本、数量或操作标识无效")
	}
	seen := map[string]bool{}
	prefixes := []string{}
	for _, r := range m.Routes {
		if r.ID != model.RouteID(r.ProjectID, r.Prefix) || !pathpolicy.Identifier(r.ProjectID) || seen[r.ID] || (r.State != "active" && r.State != "disabled") || (r.AccessPolicy != "public_read" && r.AccessPolicy != "basic_auth") {
			return result, model.Err("INVALID_MANIFEST", "路由字段无效")
		}
		seen[r.ID] = true
		if e = pathpolicy.Prefix(r.Prefix, true); e != nil {
			return result, e
		}
		allow := false
		for _, p := range site.AllowedPrefixes {
			if strings.HasPrefix(r.Prefix, p) && r.Prefix != p {
				allow = true
			}
		}
		if !allow {
			return result, model.Err("CLOUD_AUTH_DENIED", "路由不在授权页面范围内")
		}
		for _, p := range site.ProtectedPrefixes {
			if pathpolicy.Overlap(r.Prefix, p) {
				return result, model.Err("PAGE_ROUTE_PROTECTED", "路由覆盖受保护业务路径")
			}
		}
		for _, p := range prefixes {
			if pathpolicy.Overlap(r.Prefix, p) {
				return result, model.Err("ROUTE_CONFLICT", "路由重复或相互包含")
			}
		}
		prefixes = append(prefixes, r.Prefix)
	}
	oldRoutes := s.Routes[siteID]
	merged := []OwnedRoute{}
	released := map[string]bool{}
	for _, id := range m.Release {
		released[id] = true
	}
	for _, old := range oldRoutes {
		if released[old.ID] {
			if old.Owner != principal || old.State != "disabled" {
				return result, model.Err("CLOUD_AUTH_DENIED", "只能释放该设备已停用的路由")
			}
			delete(released, old.ID)
			continue
		}
		if old.Owner == principal {
			if seen[old.ID] {
				continue
			}
			old.State = "disabled"
		} else {
			for _, pre := range prefixes {
				if pathpolicy.Overlap(old.Prefix, pre) {
					return result, model.Err("ROUTE_CONFLICT", "路径已由其他设备拥有")
				}
			}
		}
		merged = append(merged, old)
	}
	if len(released) > 0 {
		return result, model.Err("INVALID_MANIFEST", "要释放的保留路由不存在")
	}
	for _, r := range m.Routes {
		for _, old := range merged {
			if pathpolicy.Overlap(r.Prefix, old.Prefix) {
				return result, model.Err("ROUTE_CONFLICT", "路径与尚未释放的保留路由重叠")
			}
		}
		merged = append(merged, OwnedRoute{r, principal})
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Prefix < merged[j].Prefix })
	if e = h.drift(reg, s); e != nil {
		return result, e
	}
	previous := s.Generation
	s.Routes[siteID] = merged
	s.Generation = model.ID()
	s.Revisions[siteID]++
	s.Sources[siteID+":"+principal] = m.SourceRevision
	result = model.Receipt{OperationID: m.OperationID, SiteID: siteID, Revision: s.Revisions[siteID], Generation: s.Generation}
	for _, v := range reg.Sites {
		var routes []model.Route
		for _, r := range s.Routes[v.ID] {
			routes = append(routes, r.Route)
		}
		file := filepath.Join(h.Dir, "nginx", "generations", s.Generation, v.ID+".locations.conf")
		if e = Atomic(file, []byte(compiler.Nginx(routes, s.Generation)), 0644); e != nil {
			return result, e
		}
	}
	journal := transaction{Previous: previous, Next: s, OperationKey: opKey, Digest: digest, Receipt: result}
	if e = h.saveJournal(journal); e != nil {
		return result, e
	}
	if e = h.switchTo(s.Generation); e != nil {
		return result, e
	}
	if e = h.run(ctx, "-t"); e != nil {
		h.switchTo(previous)
		os.Remove(filepath.Join(h.Dir, "transaction.json"))
		return result, model.Err("NGINX_CONFIG_INVALID", "Nginx 校验失败，已恢复受管指针："+e.Error())
	}
	journal.ReloadRequested = true
	if e = h.saveJournal(journal); e != nil {
		return result, e
	}
	if e = h.run(ctx, "-s", "reload"); e != nil {
		return result, model.Err("NGINX_RELOAD_FAILED", "重载失败，保留事务以便核对恢复")
	}
	routes := []model.Route{}
	for _, r := range merged {
		routes = append(routes, r.Route)
	}
	if h.Verify != nil {
		e = h.Verify(ctx, site, routes, s.Generation)
	} else {
		e = verify(ctx, site, routes, s.Generation)
	}
	if e != nil {
		result.Error = &model.Error{Code: "INGRESS_VERIFY_FAILED", Message: "入口版本尚未通过探针验证"}
	} else {
		result.Verified = true
	}
	s.Operations[opKey] = Recorded{digest, result}
	if e = h.save(s); e != nil {
		return result, e
	}
	os.Remove(filepath.Join(h.Dir, "transaction.json"))
	return result, nil
}
func hexID(s string) bool {
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func (h Helper) save(s State) error {
	b, _ := json.MarshalIndent(s, "", "  ")
	return Atomic(filepath.Join(h.Dir, "state.json"), b, 0600)
}
func (h Helper) switchTo(g string) error {
	current := filepath.Join(h.Dir, "nginx", "current")
	if g == "" {
		return os.Remove(current)
	}
	tmp := current + "-" + model.ID()
	if e := os.Symlink(filepath.Join("generations", g), tmp); e != nil {
		return e
	}
	defer os.Remove(tmp)
	return os.Rename(tmp, current)
}
func (h Helper) drift(reg Registry, s State) error {
	if s.Generation == "" {
		return nil
	}
	target, e := os.Readlink(filepath.Join(h.Dir, "nginx", "current"))
	if e != nil || target != filepath.Join("generations", s.Generation) {
		return model.Err("CONFIG_DRIFT", "受管配置指针被外部修改")
	}
	for _, site := range reg.Sites {
		var r []model.Route
		for _, v := range s.Routes[site.ID] {
			r = append(r, v.Route)
		}
		b, e := os.ReadFile(filepath.Join(h.Dir, "nginx", "current", site.ID+".locations.conf"))
		if e != nil || string(b) != compiler.Nginx(r, s.Generation) {
			return model.Err("CONFIG_DRIFT", "受管 Nginx 配置内容已改变")
		}
	}
	return nil
}

type transaction struct {
	Previous        string        `json:"previous"`
	Next            State         `json:"next"`
	OperationKey    string        `json:"operationKey"`
	Digest          string        `json:"digest"`
	Receipt         model.Receipt `json:"receipt"`
	ReloadRequested bool          `json:"reloadRequested"`
}

func (h Helper) saveJournal(t transaction) error {
	b, _ := json.Marshal(t)
	return Atomic(filepath.Join(h.Dir, "transaction.json"), b, 0600)
}
func (h Helper) recover(ctx context.Context, s *State) error {
	var t transaction
	if e := readJSON(filepath.Join(h.Dir, "transaction.json"), &t); os.IsNotExist(e) {
		return nil
	} else if e != nil {
		return e
	}
	if !t.ReloadRequested {
		if e := h.switchTo(t.Previous); e != nil && !os.IsNotExist(e) {
			return e
		}
		return os.Remove(filepath.Join(h.Dir, "transaction.json"))
	}
	if e := h.switchTo(t.Next.Generation); e != nil {
		return e
	}
	if e := h.run(ctx, "-t"); e != nil {
		return model.Err("NGINX_CONFIG_INVALID", "恢复事务时配置校验失败")
	}
	if e := h.run(ctx, "-s", "reload"); e != nil {
		return e
	}
	t.Receipt.Verified = false
	t.Receipt.Error = &model.Error{Code: "INGRESS_VERIFY_FAILED", Message: "崩溃恢复完成，需要重新验证资源入口"}
	t.Next.Operations[t.OperationKey] = Recorded{t.Digest, t.Receipt}
	if e := h.save(t.Next); e != nil {
		return e
	}
	*s = t.Next
	return os.Remove(filepath.Join(h.Dir, "transaction.json"))
}
func verify(ctx context.Context, site RegisteredSite, routes []model.Route, generation string) error {
	base := site.ProbeURL
	if base == "" {
		base = "https://" + site.Domain
	}
	u, e := url.Parse(base)
	if e != nil || u.Host == "" {
		return fmt.Errorf("invalid probe URL")
	}
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	for _, route := range routes {
		success := false
		for attempt := 0; attempt < 8; attempt++ {
			u.Path = route.Prefix + ".yandu-ingress-check"
			req, _ := http.NewRequestWithContext(ctx, "HEAD", u.String(), nil)
			req.Host = site.Domain
			resp, e := client.Do(req)
			if e == nil {
				success = resp.Header.Get("X-Yandu-Generation") == generation
				resp.Body.Close()
			}
			if success {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
		}
		if !success {
			return fmt.Errorf("generation not served")
		}
	}
	return nil
}
