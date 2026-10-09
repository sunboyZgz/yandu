package localapi

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"yandu/internal/adapters"
	"yandu/internal/model"
	"yandu/internal/pathpolicy"
	"yandu/internal/reconcile"
	"yandu/internal/secrets"
)

//go:embed static/*
var assets embed.FS

type session struct {
	CSRF    string
	Expires time.Time
}
type API struct {
	Engine   *reconcile.Engine
	Token    string
	mu       sync.Mutex
	Tickets  map[string]time.Time
	Sessions map[string]session
}

func New(e *reconcile.Engine, token string) *API {
	return &API{Engine: e, Token: token, Tickets: map[string]time.Time{}, Sessions: map[string]session{}}
}
func send(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func failure(w http.ResponseWriter, err error) {
	var e *model.Error
	if !errors.As(err, &e) {
		e = &model.Error{Code: "INTERNAL_ERROR", Message: "操作失败，请查看本机诊断"}
	}
	status := 400
	switch e.Code {
	case "REVISION_CONFLICT", "ROUTE_CONFLICT", "OPERATION_CONFLICT":
		status = 409
	case "ACCESS_DENIED", "CLOUD_AUTH_DENIED", "SESSION_REQUIRED", "CSRF_INVALID":
		status = 403
	case "PATH_NOT_FOUND", "PROJECT_NOT_FOUND", "OPERATION_NOT_FOUND":
		status = 404
	}
	send(w, status, map[string]any{"error": e})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return model.Err("INVALID_REQUEST", "请求字段或格式无效")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return model.Err("INVALID_REQUEST", "只允许一个 JSON 请求")
	}
	return nil
}
func equal(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	if r.Host != model.ManagementAddr {
		http.Error(w, "Host denied", 403)
		return
	}
	origin := r.Header.Get("Origin")
	if origin != "" && origin != "http://"+model.ManagementAddr {
		http.Error(w, "Origin denied", 403)
		return
	}
	if r.URL.Path == "/api/v1/health" && r.Method == "GET" {
		send(w, 200, map[string]any{"version": model.Version, "ready": true})
		return
	}
	if r.URL.Path == "/api/v1/launch" && r.Method == "POST" {
		if !equal(r.Header.Get("Authorization"), "Bearer "+a.Token) {
			failure(w, model.Err("SESSION_REQUIRED", "请通过 yandu ui 打开管理页"))
			return
		}
		ticket := model.ID()
		a.mu.Lock()
		for k, v := range a.Tickets {
			if time.Now().After(v) {
				delete(a.Tickets, k)
			}
		}
		a.Tickets[ticket] = time.Now().Add(time.Minute)
		a.mu.Unlock()
		send(w, 200, map[string]string{"ticket": ticket})
		return
	}
	if r.URL.Path == "/api/v1/session" && r.Method == "POST" {
		var v struct {
			Ticket string `json:"ticket"`
		}
		if err := decode(w, r, &v); err != nil {
			failure(w, err)
			return
		}
		a.mu.Lock()
		expires, ok := a.Tickets[v.Ticket]
		delete(a.Tickets, v.Ticket)
		if !ok || time.Now().After(expires) {
			a.mu.Unlock()
			failure(w, model.Err("SESSION_REQUIRED", "启动票据已过期，请重新运行 yandu ui"))
			return
		}
		id := model.ID()
		csrf := model.ID()
		a.Sessions[id] = session{csrf, time.Now().Add(8 * time.Hour)}
		for k, v := range a.Sessions {
			if time.Now().After(v.Expires) {
				delete(a.Sessions, k)
			}
		}
		a.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "yandu_session", Value: id, HttpOnly: true, SameSite: http.SameSiteStrictMode, Path: "/", MaxAge: 8 * 3600})
		send(w, 200, map[string]string{"csrf": csrf})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		cli := equal(r.Header.Get("Authorization"), "Bearer "+a.Token)
		var ss session
		if !cli {
			c, err := r.Cookie("yandu_session")
			a.mu.Lock()
			if err == nil {
				ss = a.Sessions[c.Value]
			}
			a.mu.Unlock()
			if ss.Expires.Before(time.Now()) {
				failure(w, model.Err("SESSION_REQUIRED", "请从桌面快捷方式或 yandu ui 打开管理页"))
				return
			}
			if r.Method != "GET" && !equal(r.Header.Get("X-CSRF-Token"), ss.CSRF) {
				failure(w, model.Err("CSRF_INVALID", "本机会话校验失败，请刷新页面"))
				return
			}
		}
		if r.URL.Path == "/api/v1/session" && r.Method == "GET" {
			send(w, 200, map[string]string{"csrf": ss.CSRF})
			return
		}
		a.dispatch(w, r)
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(405)
		return
	}
	sub, _ := fs.Sub(assets, "static")
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if _, err := fs.Stat(sub, path); err != nil {
		if strings.Contains(filepath.Base(path), ".") {
			http.NotFound(w, r)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		http.FileServer(http.FS(sub)).ServeHTTP(w, r2)
		return
	}
	http.FileServer(http.FS(sub)).ServeHTTP(w, r)
}
func (a *API) dispatch(w http.ResponseWriter, r *http.Request) {
	s, err := a.Engine.Store.Read()
	if err != nil {
		failure(w, err)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	switch {
	case path == "/status" && r.Method == "GET":
		projects := []model.Project{}
		for _, p := range s.Projects {
			projects = append(projects, p.Public())
		}
		sites := []model.Site{}
		device := ""
		fp := ""
		if s.Profile != nil {
			sites = s.Profile.Sites
			device = s.Profile.DeviceID
			fp = s.Profile.SSHFingerprint
		}
		send(w, 200, map[string]any{"version": model.Version, "revision": s.Revision, "appliedRevision": s.AppliedRevision, "projects": projects, "sites": sites, "roots": s.Roots, "operations": s.Operations, "reserved": s.Reserved, "cloudRevisions": s.CloudRevisions, "deviceId": device, "sshFingerprint": fp, "components": map[string]any{"agent": true, "fileService": a.Engine.Caddy.Alive(), "tunnel": a.Engine.FRPC.Alive(), "connection": s.Profile != nil}})
	case path == "/projects" && r.Method == "GET":
		out := []model.Project{}
		for _, p := range s.Projects {
			out = append(out, p.Public())
		}
		send(w, 200, out)
	case (path == "/projects" && r.Method == "POST") || (len(parts) == 2 && parts[0] == "projects" && r.Method == "PUT"):
		var v struct {
			Project          model.Project `json:"project"`
			Password         string        `json:"password,omitempty"`
			ExpectedRevision int64         `json:"expectedRevision"`
		}
		if err = decode(w, r, &v); err == nil {
			if r.Method == "PUT" && v.Project.ID != parts[1] {
				err = model.Err("INVALID_REQUEST", "项目标识与路径不匹配")
			} else {
				var p model.Project
				p, err = a.Engine.Save(v.Project, v.Password, v.ExpectedRevision)
				if err == nil {
					send(w, 200, p)
					return
				}
			}
		}
		failure(w, err)
	case len(parts) == 3 && parts[0] == "projects" && r.Method == "POST":
		id := parts[1]
		action := parts[2]
		if action == "validate" {
			for _, p := range s.Projects {
				if p.ID == id {
					err = pathpolicy.ValidateProject(p, s)
					if err != nil {
						failure(w, err)
					} else {
						send(w, 200, map[string]bool{"valid": true})
					}
					return
				}
			}
			failure(w, model.Err("PROJECT_NOT_FOUND", "项目不存在"))
			return
		}
		if action == "verify" {
			var v struct {
				Password string `json:"password"`
			}
			if err = decode(w, r, &v); err == nil {
				err = a.Engine.CheckPassword(r.Context(), id, v.Password)
			}
			if err != nil {
				failure(w, err)
			} else {
				send(w, 200, map[string]bool{"verified": true})
			}
			return
		}
		var v struct {
			ExpectedRevision   int64 `json:"expectedRevision"`
			ConfirmPublication bool  `json:"confirmPublication"`
		}
		if err = decode(w, r, &v); err != nil {
			failure(w, err)
			return
		}
		op, err := a.Engine.Queue(id, action, v.ExpectedRevision, v.ConfirmPublication)
		if err != nil {
			failure(w, err)
		} else {
			send(w, 202, map[string]string{"operationId": op})
		}
	case path == "/roots" && r.Method == "POST":
		var v struct {
			Path string `json:"path"`
		}
		if err = decode(w, r, &v); err == nil {
			v.Path = filepath.Clean(v.Path)
			if !filepath.IsAbs(v.Path) || filepath.Dir(v.Path) == v.Path || pathpolicy.Within(v.Path, a.Engine.Dir) || pathpolicy.Within(a.Engine.Dir, v.Path) {
				err = model.Err("PATH_OUTSIDE_ROOT", "请授权独立的素材目录，不能授权磁盘根或工具状态目录")
			} else {
				err = pathpolicy.CheckDirectory(v.Path)
			}
		}
		if err == nil {
			err = a.Engine.Store.Update(func(s *model.Snapshot) error {
				for _, p := range s.Roots {
					if p == v.Path {
						return nil
					}
				}
				s.Roots = append(s.Roots, v.Path)
				s.Revision++
				return nil
			})
		}
		if err != nil {
			failure(w, err)
		} else {
			send(w, 200, map[string]string{"path": v.Path})
		}
	case path == "/files" && r.Method == "GET":
		id := r.URL.Query().Get("projectId")
		rel := r.URL.Query().Get("path")
		for _, p := range s.Projects {
			if p.ID == id {
				a.files(w, r, p, rel, s)
				return
			}
		}
		failure(w, model.Err("PROJECT_NOT_FOUND", "项目不存在"))
	case path == "/directories" && r.Method == "POST":
		var v struct {
			ProjectID string `json:"projectId"`
			Path      string `json:"path"`
		}
		if err = decode(w, r, &v); err != nil {
			failure(w, err)
			return
		}
		for _, p := range s.Projects {
			if p.ID == v.ProjectID {
				if _, err = pathpolicy.Relative(p.ProjectBase+"/"+v.Path, p.ProjectBase, p.ProjectBase+"/"); err != nil {
					failure(w, err)
					return
				}
				target := filepath.Join(p.RootPath, filepath.FromSlash(v.Path))
				if !pathpolicy.Within(p.RootPath, target) {
					failure(w, model.Err("PATH_OUTSIDE_ROOT", "目录超出授权根"))
					return
				}
				if err = pathpolicy.NoLinks(filepath.Dir(target)); err == nil {
					var root *os.Root
					root, err = os.OpenRoot(p.RootPath)
					if err == nil {
						err = root.Mkdir(filepath.FromSlash(v.Path), 0750)
						root.Close()
					}
				}
				if err != nil {
					failure(w, model.Err("ACCESS_DENIED", "无法创建目录，检查服务账号写入权限或目录是否存在"))
				} else {
					send(w, 201, map[string]string{"path": v.Path})
				}
				return
			}
		}
		failure(w, model.Err("PROJECT_NOT_FOUND", "项目不存在"))
	case path == "/connection/import" && r.Method == "POST":
		var v struct {
			Bundle     json.RawMessage `json:"bundle"`
			Passphrase string          `json:"passphrase"`
		}
		if err = decode(w, r, &v); err != nil {
			failure(w, err)
			return
		}
		p, err := secrets.Decrypt(v.Bundle, v.Passphrase)
		if err != nil {
			failure(w, err)
			return
		}
		err = a.Engine.Import(p)
		if err != nil {
			failure(w, err)
		} else {
			send(w, 200, map[string]any{"sites": p.Sites, "deviceId": p.DeviceID})
		}
	case path == "/connection/test" && r.Method == "POST":
		if s.Profile == nil {
			failure(w, model.Err("CONNECTION_REQUIRED", "未导入连接包"))
			return
		}
		receipt, err := a.Engine.Call(r.Context(), *s.Profile, structRequest(s.Profile.Sites[0].ID))
		if err != nil {
			failure(w, err)
		} else {
			send(w, 200, receipt)
		}
	case path == "/routes/release" && r.Method == "POST":
		var v struct {
			SiteID  string `json:"siteId"`
			RouteID string `json:"routeId"`
			Confirm bool   `json:"confirm"`
		}
		if err = decode(w, r, &v); err == nil {
			if !v.Confirm {
				err = model.Err("CONFIRMATION_REQUIRED", "释放后该路径将交回原网站，请明确确认")
			} else {
				err = a.Engine.Release(r.Context(), v.SiteID, v.RouteID)
			}
		}
		if err != nil {
			failure(w, err)
		} else {
			send(w, 200, map[string]bool{"released": true})
		}
	case path == "/config/export" && r.Method == "GET":
		projects := []model.Project{}
		for _, p := range s.Projects {
			q := p.Public()
			q.Publication = "disabled"
			projects = append(projects, q)
		}
		send(w, 200, map[string]any{"schemaVersion": 1, "projects": projects})
	case path == "/config/import" && r.Method == "POST":
		var v struct {
			SchemaVersion int             `json:"schemaVersion"`
			Projects      []model.Project `json:"projects"`
		}
		if err = decode(w, r, &v); err != nil {
			failure(w, err)
			return
		}
		if v.SchemaVersion != 1 || len(v.Projects) > 100 {
			failure(w, model.Err("INVALID_REQUEST", "配置版本或数量无效"))
			return
		}
		if err = a.Engine.ImportDrafts(v.Projects); err != nil {
			failure(w, err)
			return
		}
		send(w, 200, map[string]int{"imported": len(v.Projects)})
	case path == "/diagnostics" && r.Method == "GET":
		components := map[string]string{}
		for _, name := range []string{"caddy", "frpc"} {
			if _, e := adapters.Binary(name); e == nil {
				components[name] = "installed"
			} else {
				components[name] = "missing"
			}
		}
		ops := append([]model.Operation{}, s.Operations...)
		for i := range ops {
			if ops[i].Error != "" {
				ops[i].Error = "操作失败；请在本机操作记录中查看详情"
			}
		}
		send(w, 200, map[string]any{"version": model.Version, "revision": s.Revision, "cloudRevisions": s.CloudRevisions, "components": components, "operations": ops, "managementBind": model.ManagementAddr, "resourceBind": model.ResourceAddr, "projectCount": len(s.Projects)})
	case len(parts) == 2 && parts[0] == "operations" && r.Method == "GET":
		for _, op := range s.Operations {
			if op.ID == parts[1] {
				send(w, 200, op)
				return
			}
		}
		failure(w, model.Err("OPERATION_NOT_FOUND", "操作不存在"))
	default:
		http.NotFound(w, r)
	}
}

type File struct {
	Name      string    `json:"name"`
	Directory bool      `json:"directory"`
	Size      int64     `json:"size"`
	Modified  time.Time `json:"modified"`
	URL       string    `json:"url,omitempty"`
	Denied    bool      `json:"denied"`
}

func (a *API) files(w http.ResponseWriter, r *http.Request, p model.Project, rel string, s model.Snapshot) {
	if rel != "" {
		if _, err := pathpolicy.Relative(p.ProjectBase+"/"+rel, p.ProjectBase, p.ProjectBase+"/"); err != nil {
			failure(w, err)
			return
		}
	}
	target := filepath.Join(p.RootPath, filepath.FromSlash(rel))
	if !pathpolicy.Within(p.RootPath, target) {
		failure(w, model.Err("PATH_OUTSIDE_ROOT", "目录超出授权根"))
		return
	}
	if err := pathpolicy.CheckDirectory(target); err != nil {
		failure(w, err)
		return
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		failure(w, model.Err("ACCESS_DENIED", "目录无法读取"))
		return
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	if offset > len(entries) {
		offset = len(entries)
	}
	end := offset + 100
	if end > len(entries) {
		end = len(entries)
	}
	out := []File{}
	site, _ := s.Site(p.SiteID)
	for _, v := range entries[offset:end] {
		info, e := v.Info()
		if e != nil {
			continue
		}
		f := File{Name: v.Name(), Directory: v.IsDir(), Size: info.Size(), Modified: info.ModTime(), Denied: pathpolicy.NoLinks(filepath.Join(target, v.Name())) != nil}
		if !f.Directory && !f.Denied {
			rpath := filepath.ToSlash(filepath.Join(rel, v.Name()))
			reqPath := p.ProjectBase + "/" + rpath
			for _, pre := range p.ResourcePrefixes {
				if strings.HasPrefix(reqPath, pre) {
					f.URL = pathpolicy.URL(site.Domain, p.ProjectBase, rpath)
				}
			}
		}
		out = append(out, f)
	}
	send(w, 200, map[string]any{"entries": out, "total": len(entries), "offset": offset, "path": rel, "projectId": p.ID, "root": p.RootPath})
}
func (a *API) String() string { return fmt.Sprintf("Yandu management at %s", model.ManagementAddr) }
