package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gofrs/flock"
	"golang.org/x/term"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"yandu/internal/localapi"
	"yandu/internal/model"
	"yandu/internal/reconcile"
	"yandu/internal/resource"
	"yandu/internal/service"
	"yandu/internal/state"
)

var stateDir string

func main() {
	fs := flag.NewFlagSet("yandu", flag.ExitOnError)
	fs.StringVar(&stateDir, "state-dir", service.StateDir(), "状态目录")
	fs.Parse(os.Args[1:])
	args := fs.Args()
	if len(args) == 0 {
		args = []string{"ui"}
	}
	var err error
	if args[0] == "serve" {
		handled, e := service.Dispatch(serve)
		if e != nil {
			err = e
		} else if !handled {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			err = serve(ctx)
		}
	} else {
		err = command(args)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func serve(ctx context.Context) error {
	if !filepath.IsAbs(stateDir) {
		p, e := filepath.Abs(stateDir)
		if e != nil {
			return e
		}
		stateDir = p
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(stateDir, "agent.lock"))
	ok, err := lock.TryLock()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Agent 已在运行")
	}
	defer lock.Unlock()
	tokenPath := filepath.Join(stateDir, "local.token")
	token, err := os.ReadFile(tokenPath)
	if os.IsNotExist(err) {
		token = []byte(model.ID() + model.ID())
		err = os.WriteFile(tokenPath, token, 0600)
	}
	if err != nil {
		return err
	}
	if len(token) != 64 {
		return fmt.Errorf("本机凭据损坏")
	}
	store, err := state.Open(stateDir)
	if err != nil {
		return err
	}
	defer store.Close()
	e := &reconcile.Engine{Store: store, Dir: stateDir, Ctx: ctx, AdminPassword: model.Hex(token)}
	defer e.Close()
	api := &http.Server{Addr: model.ManagementAddr, Handler: localapi.New(e, string(token)), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	guard := &http.Server{Addr: model.GuardAddr, Handler: resource.Guard{Store: store}, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384}
	errors := make(chan error, 2)
	go func() { errors <- api.ListenAndServe() }()
	go func() { errors <- guard.ListenAndServe() }()
	if err = e.Boot(); err != nil {
		fmt.Fprintln(os.Stderr, "组件初始化：", err)
	}
	go e.RetryLoop()
	fmt.Fprintln(os.Stderr, "檐渡 Agent 已启动：", model.ManagementAddr)
	select {
	case <-ctx.Done():
	case err = <-errors:
		if err != http.ErrServerClosed {
			return err
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	api.Shutdown(shutdown)
	guard.Shutdown(shutdown)
	return nil
}
func request(method, path string, payload any) (json.RawMessage, error) {
	token, err := os.ReadFile(filepath.Join(stateDir, "local.token"))
	if err != nil {
		return nil, fmt.Errorf("Agent 尚未启动，请运行 yandu ui")
	}
	var body io.Reader
	if payload != nil {
		b, e := json.Marshal(payload)
		if e != nil {
			return nil, e
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, "http://"+model.ManagementAddr+"/api/v1"+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+string(token))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s", strings.TrimSpace(string(b)))
	}
	return b, err
}
func printJSON(b []byte) {
	var pretty bytes.Buffer
	if json.Indent(&pretty, b, "", "  ") == nil {
		fmt.Println(pretty.String())
	} else {
		fmt.Println(string(b))
	}
}
func password() (string, error) {
	fmt.Fprint(os.Stderr, "连接包口令（不回显）：")
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, e := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		return string(b), e
	}
	b, e := io.ReadAll(io.LimitReader(os.Stdin, 1024))
	return strings.TrimSpace(string(b)), e
}
func command(args []string) error {
	switch args[0] {
	case "version":
		fmt.Println(model.Version)
		return nil
	case "ui":
		client := &http.Client{Timeout: 500 * time.Millisecond}
		resp, err := client.Get("http://" + model.ManagementAddr + "/api/v1/health")
		if err == nil {
			resp.Body.Close()
		}
		if err != nil {
			if err = startService(); err != nil {
				return err
			}
			ready := false
			for i := 0; i < 80; i++ {
				resp, err = client.Get("http://" + model.ManagementAddr + "/api/v1/health")
				if err == nil {
					resp.Body.Close()
					ready = true
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if !ready {
				return fmt.Errorf("本地服务未启动，请查看状态目录中的 agent.log")
			}
		}
		b, err := request("POST", "/launch", map[string]any{})
		if err != nil {
			return err
		}
		var v struct {
			Ticket string `json:"ticket"`
		}
		if err = json.Unmarshal(b, &v); err != nil {
			return err
		}
		if len(args) > 1 && args[1] == "--no-open" {
			fmt.Println("本地管理页已就绪")
			return nil
		}
		return openBrowser("http://" + model.ManagementAddr + "/?launch=" + model.ID() + "#ticket=" + v.Ticket)
	case "status", "doctor":
		path := "/status"
		if args[0] == "doctor" {
			path = "/diagnostics"
		}
		b, e := request("GET", path, nil)
		if e == nil {
			printJSON(b)
		}
		return e
	case "connection":
		if len(args) != 3 || args[1] != "import" {
			return fmt.Errorf("用法：yandu connection import main.yandu-profile")
		}
		b, e := os.ReadFile(args[2])
		if e != nil {
			return e
		}
		pass, e := password()
		if e != nil {
			return e
		}
		out, e := request("POST", "/connection/import", map[string]any{"bundle": json.RawMessage(b), "passphrase": pass})
		if e == nil {
			printJSON(out)
		}
		return e
	case "root":
		if len(args) != 3 || args[1] != "grant" {
			return fmt.Errorf("用法：yandu root grant 绝对目录")
		}
		b, e := request("POST", "/roots", map[string]string{"path": args[2]})
		if e == nil {
			printJSON(b)
		}
		return e
	case "project":
		return project(args[1:])
	case "route":
		if len(args) != 4 || args[1] != "release" {
			return fmt.Errorf("用法：yandu route release 站点 路由ID；此命令明确释放路径给网站")
		}
		b, e := request("POST", "/routes/release", map[string]any{"siteId": args[2], "routeId": args[3], "confirm": true})
		if e == nil {
			printJSON(b)
		}
		return e
	case "config":
		if len(args) != 3 {
			return fmt.Errorf("用法：yandu config import|export 文件")
		}
		if args[1] == "export" {
			b, e := request("GET", "/config/export", nil)
			if e != nil {
				return e
			}
			return os.WriteFile(args[2], b, 0600)
		}
		if args[1] == "import" {
			b, e := os.ReadFile(args[2])
			if e != nil {
				return e
			}
			out, e := request("POST", "/config/import", json.RawMessage(b))
			if e == nil {
				printJSON(out)
			}
			return e
		}
	}
	return fmt.Errorf("命令：ui、serve、status、doctor、connection import、root grant、project add/list/apply/disable/remove、route release、config import/export")
}
func project(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("缺少项目操作")
	}
	if args[0] == "list" {
		b, e := request("GET", "/projects", nil)
		if e == nil {
			printJSON(b)
		}
		return e
	}
	if args[0] == "add" {
		fs := flag.NewFlagSet("project add", flag.ContinueOnError)
		p := model.Project{Publication: "disabled", AccessPolicy: "public_read", CachePolicy: "revalidate", ContentPolicy: "media_only"}
		var prefixes string
		fs.StringVar(&p.ID, "id", "", "项目标识")
		fs.StringVar(&p.DisplayName, "name", "", "项目名")
		fs.StringVar(&p.SiteID, "site", "main", "站点")
		fs.StringVar(&p.RootPath, "root", "", "绝对目录")
		fs.StringVar(&p.ProjectBase, "base", "/blog", "页面基础路径")
		fs.StringVar(&prefixes, "prefix", "/blog/xxx/", "资源前缀，多个用逗号分隔")
		if e := fs.Parse(args[1:]); e != nil {
			return e
		}
		p.ResourcePrefixes = strings.Split(prefixes, ",")
		b, e := request("POST", "/projects", map[string]any{"project": p, "expectedRevision": 0})
		if e == nil {
			printJSON(b)
		}
		return e
	}
	if len(args) < 2 {
		return fmt.Errorf("缺少项目标识")
	}
	action := args[0]
	if action != "apply" && action != "disable" && action != "remove" {
		return fmt.Errorf("不支持的项目操作")
	}
	confirm := len(args) == 3 && args[2] == "--confirm-publish"
	if action == "apply" && !confirm {
		return fmt.Errorf("应用会发布资源，请加 --confirm-publish 明确确认")
	}
	b, e := request("GET", "/projects", nil)
	if e != nil {
		return e
	}
	var ps []model.Project
	json.Unmarshal(b, &ps)
	for _, p := range ps {
		if p.ID == args[1] {
			out, e := request("POST", "/projects/"+p.ID+"/"+action, map[string]any{"expectedRevision": p.Revision, "confirmPublication": confirm})
			if e == nil {
				printJSON(out)
			}
			return e
		}
	}
	return fmt.Errorf("项目不存在")
}
func openBrowser(url string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		c = exec.Command("open", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	return c.Start()
}
