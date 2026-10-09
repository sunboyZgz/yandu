package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"yandu/internal/adapters"
	"yandu/internal/cloud"
	"yandu/internal/cloudsync"
	"yandu/internal/compiler"
	"yandu/internal/model"
	"yandu/internal/pathpolicy"
	"yandu/internal/state"
)

type Engine struct {
	Store         *state.Store
	Dir           string
	Ctx           context.Context
	mu            sync.Mutex
	Caddy, FRPC   adapters.Process
	HTTPClient    *http.Client
	AdminPassword string
	Sync          func(context.Context, model.Profile, cloudsync.Request) (model.Receipt, error)
}

func (e *Engine) Call(ctx context.Context, p model.Profile, r cloudsync.Request) (model.Receipt, error) {
	if e.Sync != nil {
		return e.Sync(ctx, p, r)
	}
	return (cloudsync.Client{Profile: p}).Call(ctx, r)
}
func (e *Engine) Save(p model.Project, password string, expected int64) (model.Project, error) {
	p.PasswordHash = ""
	p.Generation = ""
	p.ProbeToken = ""
	p.LastError = ""
	err := e.Store.Update(func(s *model.Snapshot) error {
		index := -1
		for i, v := range s.Projects {
			if v.ID == p.ID {
				index = i
				if v.SiteID != p.SiteID || v.ProjectBase != p.ProjectBase {
					return model.Err("ROUTE_OWNERSHIP_CHANGE", "站点或页面基础路径变更请先停用、删除映射并释放旧路径")
				}
				if v.Revision != expected {
					return model.Err("REVISION_CONFLICT", "项目已被修改，请刷新后再保存")
				}
				p.PasswordHash = v.PasswordHash
				p.Publication = v.Publication
				p.AppliedRevision = v.AppliedRevision
				p.Status = v.Status
			}
		}
		if index < 0 {
			if expected != 0 {
				return model.Err("REVISION_CONFLICT", "项目不存在")
			}
			p.Publication = "disabled"
			p.Status = "draft"
		} else {
			p.Status = "pending"
		}
		if password != "" {
			if len(password) < 12 || len(password) > 72 {
				return model.Err("WEAK_PASSWORD", "资源密码需要 12–72 个字符")
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			p.PasswordHash = string(hash)
		}
		if err := pathpolicy.ValidateProject(p, *s); err != nil {
			return err
		}
		if len(s.Projects) >= 100 && index < 0 {
			return model.Err("PROJECT_LIMIT", "最多允许 100 个项目")
		}
		if index >= 0 {
			old := s.Projects[index]
			if old.AccessPolicy != p.AccessPolicy || old.PasswordHash != p.PasswordHash || old.Username != p.Username {
				for i := range s.Effective {
					if s.Effective[i].ID == p.ID {
						s.Effective[i].Publication = "disabled"
					}
				}
				p.Publication = "disabled"
				p.Status = "disabled_pending"
			}
			s.Projects[index] = p
		} else {
			s.Projects = append(s.Projects, p)
		}
		s.Revision++
		p.Revision = s.Revision
		if index >= 0 {
			s.Projects[index] = p
		} else {
			s.Projects[len(s.Projects)-1] = p
		}
		return nil
	})
	return p.Public(), err
}
func (e *Engine) Queue(id, action string, expected int64, confirm bool) (string, error) {
	op := model.Operation{ID: model.ID(), ProjectID: id, Action: action, Stage: "queued", Status: "running", StartedAt: time.Now(), UpdatedAt: time.Now()}
	err := e.Store.Update(func(s *model.Snapshot) error {
		found := false
		for i, p := range s.Projects {
			if p.ID != id {
				continue
			}
			found = true
			if expected != p.Revision {
				return model.Err("REVISION_CONFLICT", "项目版本已改变，请刷新")
			}
			if action == "apply" {
				if !confirm {
					return model.Err("PUBLICATION_CONFIRMATION_REQUIRED", "请明确确认将这些资源公开或按密码保护发布")
				}
				if err := pathpolicy.ValidateProject(p, *s); err != nil {
					return err
				}
				s.Projects[i].Publication = "active"
				s.Projects[i].Status = "applying"
			} else if action == "disable" || action == "remove" {
				s.Projects[i].Publication = "disabled"
				s.Projects[i].Status = "disabled_pending"
				for j := range s.Effective {
					if s.Effective[j].ID == id {
						s.Effective[j].Publication = "disabled"
					}
				}
			} else {
				return model.Err("INVALID_ACTION", "操作无效")
			}
			s.Revision++
			s.Projects[i].Revision = s.Revision
		}
		if !found {
			return model.Err("PROJECT_NOT_FOUND", "项目不存在")
		}
		s.Operations = append(s.Operations, op)
		if len(s.Operations) > 200 {
			s.Operations = s.Operations[len(s.Operations)-200:]
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	go e.apply(id, op.ID, action)
	return op.ID, nil
}
func (e *Engine) stage(opID, stage string, err error) {
	e.Store.Update(func(s *model.Snapshot) error {
		for i := range s.Operations {
			if s.Operations[i].ID == opID {
				s.Operations[i].Stage = stage
				s.Operations[i].UpdatedAt = time.Now()
				if err != nil {
					s.Operations[i].Status = "failed"
					s.Operations[i].Error = err.Error()
				} else if stage == "complete" {
					s.Operations[i].Status = "succeeded"
				}
			}
		}
		return nil
	})
}
func (e *Engine) apply(id, op, action string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	ctx, cancel := context.WithTimeout(e.Ctx, 90*time.Second)
	defer cancel()
	err := e.applyLocked(ctx, id, op, action)
	if err != nil {
		var block *model.Error
		if errors.As(err, &block) && block.Code == "CLOUD_BLOCKED" {
			snapshot, readErr := e.Store.Read()
			if readErr == nil {
				for _, p := range snapshot.Projects {
					if p.ID == id {
						e.blockSite(p.SiteID, block.Message)
						break
					}
				}
			}
		}
		e.stage(op, "failed", err)
		e.Store.Update(func(s *model.Snapshot) error {
			for i := range s.Projects {
				if s.Projects[i].ID == id {
					s.Projects[i].LastError = err.Error()
					if s.Projects[i].Status == "cloud_blocked" {
						continue
					}
					if s.Projects[i].Publication == "disabled" {
						s.Projects[i].Status = "disabled_pending"
					} else if s.Projects[i].Status == "applying" {
						s.Projects[i].Status = "local_invalid"
					}
				}
			}
			return nil
		})
	} else {
		e.stage(op, "complete", nil)
	}
}
func (e *Engine) applyLocked(ctx context.Context, id, op, action string) error {
	s, err := e.Store.Read()
	if err != nil {
		return err
	}
	var p model.Project
	for _, v := range s.Projects {
		if v.ID == id {
			p = v
		}
	}
	if p.ID == "" {
		return model.Err("PROJECT_NOT_FOUND", "项目不存在")
	}
	if s.Profile != nil {
		receipt, preflight := e.Call(ctx, *s.Profile, cloudsync.Request{Action: "status", SiteID: p.SiteID})
		var blocked *model.Error
		if errors.As(preflight, &blocked) && blocked.Code == "CLOUD_BLOCKED" {
			e.blockSite(p.SiteID, blocked.Message)
			return blocked
		}
		if preflight == nil {
			s.CloudRevisions[p.SiteID] = receipt.Revision
			e.Store.Update(func(v *model.Snapshot) error { v.CloudRevisions[p.SiteID] = receipt.Revision; return nil })
		}
	}

	if p.Publication == "active" {
		if err = pathpolicy.ValidateProject(p, s); err != nil {
			return err
		}
	}
	e.stage(op, "validate", nil)
	p.Generation = model.ID()
	p.ProbeToken = model.ID()
	p.AppliedRevision = p.Revision
	candidate := s
	candidate.Effective = append([]model.Project{}, s.Effective...)
	replaced := false
	for i, old := range candidate.Effective {
		if old.ID != id {
			continue
		}
		replaced = true
		for _, pre := range old.ResourcePrefixes {
			has := false
			for _, np := range p.ResourcePrefixes {
				if pre == np {
					has = true
				}
			}
			if !has {
				candidate.Reserved[p.SiteID] = append(candidate.Reserved[p.SiteID], model.Route{ID: model.RouteID(p.ID, pre), ProjectID: p.ID, Prefix: pre, State: "disabled", AccessPolicy: old.AccessPolicy})
			}
		}
		candidate.Effective[i] = p
	}
	if !replaced {
		candidate.Effective = append(candidate.Effective, p)
	}
	caddyPath, frpcPath, err := e.writeConfigs(candidate)
	if err != nil {
		return err
	}
	if err = adapters.Run(ctx, "caddy", "validate", "--config", caddyPath, "--adapter", "caddyfile"); err != nil {
		return err
	}
	if s.Profile != nil {
		if err = adapters.Run(ctx, "frpc", "verify", "-c", frpcPath); err != nil {
			return err
		}
	}
	// Update the guard before reloading: stale generations immediately stop serving.
	if err = e.Store.Update(func(v *model.Snapshot) error {
		for _, q := range v.Projects {
			if q.ID == id && q.Revision != p.Revision {
				return model.Err("REVISION_CONFLICT", "应用前项目已被修改，请重新应用")
			}
		}
		merged := append([]model.Project{}, v.Effective...)
		found := false
		for i := range merged {
			if merged[i].ID == id {
				merged[i] = p
				found = true
			}
		}
		if !found {
			merged = append(merged, p)
		}
		v.Effective = merged
		v.Reserved = candidate.Reserved
		return nil
	}); err != nil {
		return err
	}
	e.stage(op, "local", nil)
	if err = e.loadCaddy(ctx, caddyPath); err != nil {
		return err
	}
	if s.Profile == nil {
		return model.Err("CONNECTION_REQUIRED", "请先导入连接包")
	}
	e.setStatus(id, "waiting_tunnel", "")
	e.stage(op, "tunnel", nil)
	if err = e.loadFRPC(ctx, frpcPath); err != nil {
		return err
	}
	if p.Publication == "active" {
		if err = e.waitTunnel(ctx, candidate); err != nil {
			return err
		}
	}
	e.setStatus(id, "waiting_ingress", "")
	e.stage(op, "cloud", nil)
	fresh, err := e.Store.Read()
	if err != nil {
		return err
	}
	if err = e.syncSite(ctx, fresh, p.SiteID, nil); err != nil {
		return err
	}
	if p.Publication == "active" {
		e.setStatus(id, "unverified", "")
		e.stage(op, "probe", nil)
		if err = e.probe(ctx, p, *s.Profile); err != nil {
			return err
		}
	}
	return e.Store.Update(func(v *model.Snapshot) error {
		for i, q := range v.Projects {
			if q.ID == id {
				if q.Revision != p.Revision {
					return model.Err("REVISION_CONFLICT", "应用期间项目被修改，请应用新版本")
				}
				v.Projects[i].AppliedRevision = p.Revision
				v.Projects[i].LastError = ""
				if p.Publication == "active" {
					v.Projects[i].Status = "active"
				} else {
					v.Projects[i].Status = "disabled"
				}
			}
		}
		v.AppliedRevision = p.Revision
		if action == "remove" {
			for _, pre := range p.ResourcePrefixes {
				v.Reserved[p.SiteID] = append(v.Reserved[p.SiteID], model.Route{ID: model.RouteID(p.ID, pre), ProjectID: p.ID, Prefix: pre, State: "disabled", AccessPolicy: p.AccessPolicy})
			}
			effective := []model.Project{}
			for _, q := range v.Effective {
				if q.ID != id {
					effective = append(effective, q)
				}
			}
			v.Effective = effective
			out := []model.Project{}
			for _, q := range v.Projects {
				if q.ID != id {
					out = append(out, q)
				}
			}
			v.Projects = out
		}
		return nil
	})
}
func (e *Engine) setStatus(id, status, message string) {
	e.Store.Update(func(s *model.Snapshot) error {
		for i := range s.Projects {
			if s.Projects[i].ID == id {
				s.Projects[i].Status = status
				s.Projects[i].LastError = message
			}
		}
		return nil
	})
}
func (e *Engine) writeConfigs(s model.Snapshot) (string, string, error) {
	dir := filepath.Join(e.Dir, "generated", model.ID())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", err
	}
	cp := filepath.Join(dir, "Caddyfile")
	fp := filepath.Join(dir, "frpc.toml")
	ca := filepath.Join(e.Dir, "tunnel-ca.pem")
	if s.Profile != nil {
		if err := cloud.Atomic(ca, []byte(s.Profile.TunnelCA), 0600); err != nil {
			return "", "", err
		}
	}
	if err := os.WriteFile(cp, []byte(compiler.Caddy(s)), 0600); err != nil {
		return "", "", err
	}
	if s.Profile != nil {
		if err := os.WriteFile(fp, []byte(compiler.FRPC(s, ca, e.AdminPassword)), 0600); err != nil {
			return "", "", err
		}
	}
	return cp, fp, nil
}
func (e *Engine) loadCaddy(ctx context.Context, path string) error {
	if e.Caddy.Alive() {
		err := adapters.Run(ctx, "caddy", "reload", "--config", path, "--adapter", "caddyfile", "--address", model.CaddyAdmin)
		if err == nil {
			e.Caddy.SetArgs("run", "--config", path, "--adapter", "caddyfile")
		}
		return err
	}
	if adapters.Listening(model.ResourceAddr) || adapters.Listening(model.CaddyAdmin) {
		return model.Err("PORT_IN_USE", "资源或 Caddy 内部端口已被其他进程占用")
	}
	if err := e.Caddy.Start(e.Ctx, "caddy", filepath.Join(e.Dir, "caddy.log"), "run", "--config", path, "--adapter", "caddyfile"); err != nil {
		return err
	}
	for i := 0; i < 30; i++ {
		if adapters.Listening(model.ResourceAddr) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return model.Err("CADDY_START_FAILED", "文件服务没有就绪")
}
func (e *Engine) loadFRPC(ctx context.Context, path string) error {
	// frpc's reload API rereads the file used at startup, not the CLI -c path.
	stable := filepath.Join(e.Dir, "frpc.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err = cloud.Atomic(stable, data, 0600); err != nil {
		return err
	}
	path = stable

	if e.FRPC.Alive() {
		err := adapters.Run(ctx, "frpc", "reload", "-c", path)
		if err == nil {
			e.FRPC.SetArgs("-c", path)
		}
		return err
	}
	if adapters.Listening("127.0.0.1:18768") {
		return model.Err("PORT_IN_USE", "隧道内部端口被其他进程占用")
	}
	return e.FRPC.Start(e.Ctx, "frpc", filepath.Join(e.Dir, "frpc.log"), "-c", path)
}
func (e *Engine) waitTunnel(ctx context.Context, s model.Snapshot) error {
	expected := map[string]bool{}
	for _, p := range s.Effective {
		if p.Publication == "active" {
			for _, pre := range p.ResourcePrefixes {
				expected[s.Profile.DeviceID+"-"+model.RouteID(p.ID, pre)] = true
			}
		}
	}
	client := &http.Client{Timeout: time.Second}
	for i := 0; i < 30; i++ {
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1:18768/api/status", nil)
		req.SetBasicAuth("yandu", e.AdminPassword)
		resp, err := client.Do(req)
		if err == nil {
			var data map[string][]struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			}
			json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&data)
			resp.Body.Close()
			running := map[string]bool{}
			for _, list := range data {
				for _, v := range list {
					if v.Status == "running" {
						running[v.Name] = true
					}
				}
			}
			ok := true
			for n := range expected {
				if !running[n] {
					ok = false
				}
			}
			if ok {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return model.Err("TUNNEL_OFFLINE", "资源隧道未注册，检查服务器、TLS CA 与认证材料")
}
func (e *Engine) syncSite(ctx context.Context, s model.Snapshot, site string, release []string) error {
	routes := compiler.Routes(s, site)
	digest := compiler.Digest(routes)
	if s.CloudDigests[site] == digest && len(release) == 0 && s.Pending[site].OperationID == "" {
		return nil
	}
	if s.Profile == nil {
		return model.Err("CONNECTION_REQUIRED", "未导入连接包")
	}
	m, has := s.Pending[site]
	if has {
		receipt, err := e.Call(ctx, *s.Profile, cloudsync.Request{Action: "result", OperationID: m.OperationID, SiteID: site})
		if err == nil {
			if receipt.Error != nil {
				return receipt.Error
			}
			if receipt.Verified {
				e.recordReceipt(site, m, receipt)
				return e.syncSiteLatest(ctx, site, release)
			}
		} else if er, ok := err.(*model.Error); ok && er.Code == "INGRESS_VERIFY_FAILED" {
			status, se := e.Call(ctx, *s.Profile, cloudsync.Request{Action: "status", SiteID: site})
			if se != nil {
				return se
			}
			if se = e.Store.Update(func(v *model.Snapshot) error {
				v.CloudRevisions[site] = status.Revision
				delete(v.Pending, site)
				return nil
			}); se != nil {
				return se
			}
			return e.syncSiteLatest(ctx, site, release)
		} else if er, ok := err.(*model.Error); !ok || er.Code != "OPERATION_NOT_FOUND" {
			return err
		}
	}
	if !has {
		m = model.Manifest{SchemaVersion: 1, OperationID: model.ID(), SiteID: site, ExpectedCloudRevision: s.CloudRevisions[site], SourceRevision: s.Revision, Routes: routes, Release: release}
		if err := e.Store.Update(func(v *model.Snapshot) error { v.Pending[site] = m; return nil }); err != nil {
			return err
		}
	}
	receipt, err := e.Call(ctx, *s.Profile, cloudsync.Request{Action: "apply", Manifest: &m})
	if err != nil {
		if er, ok := err.(*model.Error); ok && (er.Code == "REVISION_CONFLICT" || er.Code == "NGINX_CONFIG_INVALID" || er.Code == "INGRESS_VERIFY_FAILED") {
			status, se := e.Call(ctx, *s.Profile, cloudsync.Request{Action: "status", SiteID: site})
			if se == nil {
				e.Store.Update(func(v *model.Snapshot) error {
					v.CloudRevisions[site] = status.Revision
					delete(v.Pending, site)
					return nil
				})
			}
		}
		return err
	}
	if !receipt.Verified {
		return model.Err("INGRESS_VERIFY_FAILED", "云端尚未验证实际入口版本")
	}
	if err = e.recordReceipt(site, m, receipt); err != nil {
		return err
	}
	return e.syncSiteLatest(ctx, site, nil)
}
func (e *Engine) syncSiteLatest(ctx context.Context, site string, release []string) error {
	s, err := e.Store.Read()
	if err != nil {
		return err
	}
	return e.syncSite(ctx, s, site, release)
}
func (e *Engine) recordReceipt(site string, m model.Manifest, r model.Receipt) error {
	return e.Store.Update(func(v *model.Snapshot) error {
		for _, id := range m.Release {
			out := []model.Route{}
			for _, route := range v.Reserved[site] {
				if route.ID != id {
					out = append(out, route)
				}
			}
			v.Reserved[site] = out
			for i := range v.Effective {
				p := &v.Effective[i]
				if p.SiteID != site {
					continue
				}
				prefixes := []string{}
				for _, pre := range p.ResourcePrefixes {
					if model.RouteID(p.ID, pre) != id {
						prefixes = append(prefixes, pre)
					}
				}
				p.ResourcePrefixes = prefixes
			}
		}
		v.CloudRevisions[site] = r.Revision
		v.CloudDigests[site] = compiler.Digest(m.Routes)
		delete(v.Pending, site)
		return nil
	})
}
func (e *Engine) probe(ctx context.Context, p model.Project, profile model.Profile) error {
	var domain string
	for _, s := range profile.Sites {
		if s.ID == p.SiteID {
			domain = s.Domain
		}
	}
	client := e.probeClient()
	for _, pre := range p.ResourcePrefixes {
		req, _ := http.NewRequestWithContext(ctx, "GET", "https://"+domain+pre+".yandu-probe-"+p.ProbeToken, nil)
		// Probe credentials are never stored in plaintext; obtain a one-use loopback verification through the guard.
		if p.AccessPolicy == "basic_auth" {
			return model.Err("PROTECTED_PROBE_REQUIRED", "密码资源已接入；请通过验证功能输入资源密码完成外网核验")
		}
		resp, err := client.Do(req)
		if err != nil {
			return model.Err("INGRESS_VERIFY_FAILED", "外网 HTTPS 资源探针无法连接")
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 || string(b) != p.ProbeToken {
			return model.Err("INGRESS_VERIFY_FAILED", "外网响应不是当前资源探针，不能标记已生效")
		}
	}
	return nil
}
func (e *Engine) Boot() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.Store.Read()
	if err != nil {
		return err
	}
	if err = e.Store.Update(func(v *model.Snapshot) error {
		for i := range v.Operations {
			if v.Operations[i].Status == "running" {
				v.Operations[i].Status = "interrupted"
				v.Operations[i].Error = "Agent 重启，等待重新核对"
			}
		}
		for i := range v.Projects {
			if v.Projects[i].Publication == "active" {
				v.Projects[i].Status = "recovering"
			}
		}
		return nil
	}); err != nil {
		return err
	}
	// Invalid or unavailable roots stay blocked until a later apply/retry.
	for i := range s.Effective {
		if s.Effective[i].Publication == "active" {
			if err = pathpolicy.ValidateProject(s.Effective[i], s); err != nil {
				s.Effective[i].Publication = "disabled"
			}
		}
	}
	e.Store.Update(func(v *model.Snapshot) error {
		for i := range s.Effective {
			for _, current := range v.Effective {
				if current.ID == s.Effective[i].ID && current.Publication == "disabled" {
					s.Effective[i].Publication = "disabled"
				}
			}
			exists := false
			for _, p := range v.Projects {
				if p.ID == s.Effective[i].ID {
					exists = true
					if p.Publication == "disabled" || p.Status == "cloud_blocked" {
						s.Effective[i].Publication = "disabled"
					}
				}
			}
			if !exists {
				s.Effective[i].Publication = "disabled"
			}
		}
		v.Effective = s.Effective
		return nil
	})
	cp, fp, err := e.writeConfigs(s)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(e.Ctx, 15*time.Second)
	defer cancel()
	if err = adapters.Run(ctx, "caddy", "validate", "--config", cp, "--adapter", "caddyfile"); err != nil {
		return err
	}
	if err = e.loadCaddy(ctx, cp); err != nil {
		return err
	}
	if s.Profile != nil {
		if err = adapters.Run(ctx, "frpc", "verify", "-c", fp); err == nil {
			err = e.loadFRPC(ctx, fp)
		}
		if err == nil {
			for _, site := range s.Profile.Sites {
				_, readErr := e.Call(ctx, *s.Profile, cloudsync.Request{Action: "status", SiteID: site.ID})
				var blocked *model.Error
				if errors.As(readErr, &blocked) && blocked.Code == "CLOUD_BLOCKED" {
					e.blockSite(site.ID, blocked.Message)
				}
			}
		}
	}
	return err
}
func (e *Engine) Release(ctx context.Context, site, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.Store.Read()
	if err != nil {
		return err
	}
	if s.Pending[site].OperationID != "" {
		if err = e.syncSite(ctx, s, site, nil); err != nil {
			return err
		}
		s, err = e.Store.Read()
		if err != nil {
			return err
		}
	}
	candidate := s
	candidate.Reserved = map[string][]model.Route{}
	for k, v := range s.Reserved {
		candidate.Reserved[k] = append([]model.Route{}, v...)
	}
	candidate.Effective = append([]model.Project{}, s.Effective...)
	found := false
	out := []model.Route{}
	for _, r := range s.Reserved[site] {
		if r.ID == id {
			if r.State != "disabled" {
				return model.Err("ROUTE_ACTIVE", "请先停用路由")
			}
			found = true
		} else {
			out = append(out, r)
		}
	}
	candidate.Reserved[site] = out
	for i := range candidate.Effective {
		p := &candidate.Effective[i]
		if p.SiteID != site {
			continue
		}
		prefixes := []string{}
		for _, pre := range p.ResourcePrefixes {
			if model.RouteID(p.ID, pre) == id {
				if p.Publication == "active" {
					return model.Err("ROUTE_ACTIVE", "请先停用项目")
				}
				found = true
			} else {
				prefixes = append(prefixes, pre)
			}
		}
		p.ResourcePrefixes = prefixes
	}
	if !found {
		return model.Err("ROUTE_NOT_FOUND", "保留路由不存在")
	}
	return e.syncSite(ctx, candidate, site, []string{id})
}
func (e *Engine) RetryLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-e.Ctx.Done():
			return
		case <-ticker.C:
		}
		s, err := e.Store.Read()
		if err != nil {
			continue
		}
		for site, m := range s.Pending {
			if len(m.Release) > 0 {
				e.mu.Lock()
				ctx, cancel := context.WithTimeout(e.Ctx, 45*time.Second)
				e.syncSiteLatest(ctx, site, nil)
				cancel()
				e.mu.Unlock()
			}
		}
		for _, p := range s.Projects {
			if p.Status == "active" {
				if !e.Caddy.Alive() {
					e.setStatus(p.ID, "recovering", "文件服务正在恢复")
					p.Status = "recovering"
				} else if !e.FRPC.Alive() {
					e.setStatus(p.ID, "waiting_tunnel", "隧道进程正在恢复")
					p.Status = "waiting_tunnel"
				} else if err := pathpolicy.CheckDirectory(p.RootPath); err != nil {
					e.setStatus(p.ID, "local_invalid", err.Error())
					p.Status = "local_invalid"
				}
			}
			if p.Status == "active" || p.Status == "draft" || p.Status == "disabled" || p.Status == "pending" || p.Status == "cloud_blocked" || (p.Status == "unverified" && p.AccessPolicy == "basic_auth") {
				continue
			}
			op := model.ID()
			e.Store.Update(func(v *model.Snapshot) error {
				v.Operations = append(v.Operations, model.Operation{ID: op, ProjectID: p.ID, Action: "retry", Stage: "queued", Status: "running", StartedAt: time.Now(), UpdatedAt: time.Now()})
				if len(v.Operations) > 200 {
					v.Operations = v.Operations[len(v.Operations)-200:]
				}
				return nil
			})
			e.apply(p.ID, op, "retry")
		}
	}
}
func (e *Engine) Close() { e.Caddy.Stop(); e.FRPC.Stop() }
func (e *Engine) CheckPassword(ctx context.Context, id, password string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.Store.Read()
	if err != nil {
		return err
	}
	for _, p := range s.Effective {
		if p.ID != id || p.Publication != "active" {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(p.PasswordHash), []byte(password)) != nil {
			return model.Err("ACCESS_DENIED", "资源密码错误")
		}
		site, _ := s.Site(p.SiteID)
		for _, pre := range p.ResourcePrefixes {
			req, _ := http.NewRequestWithContext(ctx, "GET", "https://"+site.Domain+pre+".yandu-probe-"+p.ProbeToken, nil)
			req.SetBasicAuth(p.Username, password)
			client := e.probeClient()
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			resp.Body.Close()
			if resp.StatusCode != 200 || strings.TrimSpace(string(b)) != p.ProbeToken {
				return model.Err("INGRESS_VERIFY_FAILED", "密码资源未通过外网验证")
			}
		}
		return e.Store.Update(func(v *model.Snapshot) error {
			for i := range v.Projects {
				if v.Projects[i].ID == id && v.Projects[i].Publication == "active" {
					v.Projects[i].Status = "active"
					v.Projects[i].AppliedRevision = p.AppliedRevision
					v.Projects[i].LastError = ""
				}
			}
			return nil
		})
	}
	return fmt.Errorf("项目未启用")
}
func (e *Engine) Import(p model.Profile) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.FRPC.Stop()
	if err := e.Store.Update(func(s *model.Snapshot) error {
		s.Profile = &p
		s.CloudRevisions = map[string]int64{}
		s.CloudDigests = map[string]string{}
		s.Pending = map[string]model.Manifest{}
		for i := range s.Effective {
			s.Effective[i].Publication = "disabled"
		}
		for i := range s.Projects {
			s.Projects[i].Publication = "disabled"
			s.Projects[i].Status = "disabled_pending"
		}
		s.Revision++
		return nil
	}); err != nil {
		return err
	}
	s, err := e.Store.Read()
	if err != nil {
		return err
	}
	cp, fp, err := e.writeConfigs(s)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(e.Ctx, 20*time.Second)
	defer cancel()
	if err = adapters.Run(ctx, "caddy", "validate", "--config", cp, "--adapter", "caddyfile"); err != nil {
		return err
	}
	if err = e.loadCaddy(ctx, cp); err != nil {
		return err
	}
	if err = adapters.Run(ctx, "frpc", "verify", "-c", fp); err != nil {
		return err
	}
	return e.loadFRPC(ctx, fp)
}
func (e *Engine) probeClient() *http.Client {
	if e.HTTPClient != nil {
		return e.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
}

func (e *Engine) ImportDrafts(projects []model.Project) error {
	return e.Store.Update(func(s *model.Snapshot) error {
		if len(s.Projects)+len(projects) > 100 {
			return model.Err("PROJECT_LIMIT", "最多 100 个项目")
		}
		for _, p := range projects {
			for _, q := range s.Projects {
				if p.ID == q.ID {
					return model.Err("PROJECT_EXISTS", "导入标识已存在，请在管理页编辑项目")
				}
			}
			p.Publication = "disabled"
			p.Status = "draft"
			p.Revision = 0
			p.AppliedRevision = 0
			p.PasswordHash = ""
			p.Generation = ""
			p.ProbeToken = ""
			p.LastError = ""
			if err := pathpolicy.ValidateProject(p, *s); err != nil {
				return err
			}
			s.Revision++
			p.Revision = s.Revision
			s.Projects = append(s.Projects, p)
		}
		return nil
	})
}

func (e *Engine) blockSite(site, message string) {
	e.Store.Update(func(v *model.Snapshot) error {
		for i := range v.Projects {
			if v.Projects[i].SiteID == site {
				v.Projects[i].Publication = "disabled"
				v.Projects[i].Status = "cloud_blocked"
				v.Projects[i].LastError = message
				v.Revision++
				v.Projects[i].Revision = v.Revision
			}
		}
		for i := range v.Effective {
			if v.Effective[i].SiteID == site {
				v.Effective[i].Publication = "disabled"
			}
		}
		return nil
	})
}
