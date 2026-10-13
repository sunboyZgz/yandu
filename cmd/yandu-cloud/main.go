package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"yandu/internal/cloud"
	"yandu/internal/cloudsync"
	"yandu/internal/model"
	"yandu/internal/pathpolicy"
	"yandu/internal/secrets"
	"yandu/internal/toolchain"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "命令：list、disks、tools、storage-init、storage-gateway、serve、pair、version")
		os.Exit(1)
	}
	var e error
	switch os.Args[1] {
	case "tools":
		if e = toolchain.Command(os.Args[2:], "/etc/yandu/tools.json", "cloud", os.Stdout, os.Stderr); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		return
	case "storage-init":
		fs := flag.NewFlagSet("storage-init", flag.ContinueOnError)
		dir := fs.String("dir", "/etc/yandu", "managed config directory")
		server := fs.String("server", "", "TLS server IP/DNS")
		if e = fs.Parse(os.Args[2:]); e == nil {
			e = cloud.InitializeStorage(*dir, *server)
		}
	case "list", "disks":
		if e = inventoryCommand(os.Args[1], os.Args[2:], os.Stdout, os.Stderr); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		return
	case "version":
		fmt.Println(model.Version)
		return
	case "serve":
		e = serve(os.Args[2:])
	case "storage-gateway":
		e = gateway(os.Args[2:])
	case "pair":
		e = pair(os.Args[2:])
	default:
		e = fmt.Errorf("未知命令")
	}
	if e != nil {
		receipt := model.Receipt{Error: &model.Error{Code: "CLOUD_HELPER_FAILED", Message: e.Error()}}
		if me, ok := e.(*model.Error); ok {
			receipt.Error = me
		}
		json.NewEncoder(os.Stdout).Encode(receipt)
		os.Exit(1)
	}
}
func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	dir := fs.String("dir", "/etc/yandu", "受管目录")
	principal := fs.String("principal", "", "固定设备身份")
	nginx := fs.String("nginx", "/usr/sbin/nginx", "Nginx 程序")
	if e := fs.Parse(args); e != nil {
		return e
	}
	if !pathpolicy.Identifier(*principal) {
		return model.Err("CLOUD_AUTH_DENIED", "无效 principal")
	}
	if e := trusted(filepath.Join(*dir, "registry.json")); e != nil {
		return e
	}
	if e := trusted(os.Args[0]); e != nil {
		return e
	}
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 65537))
	if err != nil || len(data) > 65536 {
		return model.Err("INVALID_MANIFEST", "清单超过 64 KiB 限制")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var req cloudsync.Request
	if e := dec.Decode(&req); e != nil {
		return model.Err("INVALID_MANIFEST", "无效清单")
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return model.Err("INVALID_MANIFEST", "接收体超过限制或有多余内容")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	receipt, e := (cloud.Helper{Dir: *dir, RegistryPath: filepath.Join(*dir, "registry.json"), Nginx: *nginx}).Call(ctx, *principal, req)
	if e != nil {
		return e
	}
	return json.NewEncoder(os.Stdout).Encode(receipt)
}
func pair(args []string) error {
	fs := flag.NewFlagSet("pair", flag.ContinueOnError)
	scope := fs.String("scope", "readonly", "readonly 或 storage-only")
	refreshMonitor := fs.Bool("refresh-monitor", false, "为现有存储设备重新交付只读监控凭证和连接包")
	dir := fs.String("dir", "/etc/yandu", "受管目录")
	device := fs.String("device", "owner", "设备 ID")
	server := fs.String("server", "", "服务器地址")
	sshAddr := fs.String("ssh", "", "SSH host:port")
	user := fs.String("user", "yandu-sync", "设备专用 SSH 用户")
	tlsName := fs.String("tls-name", "", "frps TLS 名称")
	siteID := fs.String("site", "main", "站点 ID")
	out := fs.String("out", "main.yandu-profile", "加密连接包")
	hostKey := fs.String("host-key", "/etc/ssh/ssh_host_ed25519_key.pub", "主机公钥")
	if e := fs.Parse(args); e != nil {
		return e
	}
	if !pathpolicy.Identifier(*device) || *server == "" {
		return fmt.Errorf("需要有效 --device 和 --server")
	}
	if *sshAddr == "" {
		*sshAddr = net.JoinHostPort(*server, "22")
	}
	if *tlsName == "" {
		*tlsName = *server
	}
	if *scope == "storage-only" {
		return storagePair(*dir, *device, *server, *tlsName, *out, *refreshMonitor)
	}
	if *refreshMonitor {
		return fmt.Errorf("--refresh-monitor 仅用于 storage-only 配对")
	}
	if *scope != "readonly" {
		return fmt.Errorf("未知配对模式")
	}
	var reg cloud.Registry
	b, e := os.ReadFile(filepath.Join(*dir, "registry.json"))
	if e != nil {
		return e
	}
	if e = json.Unmarshal(b, &reg); e != nil {
		return e
	}
	var sites []model.Site
	for _, s := range reg.Sites {
		if s.ID == *siteID {
			sites = append(sites, s.Site)
		}
	}
	if len(sites) == 0 {
		return fmt.Errorf("站点尚未安装")
	}
	if len(reg.Devices) > 0 {
		return fmt.Errorf("v0.1 为单一可信所有者；已有设备时请走轮换流程，不能自动新增第二个身份")
	}
	pub, priv, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return e
	}
	der, e := x509.MarshalPKCS8PrivateKey(priv)
	if e != nil {
		return e
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	pubSSH, e := ssh.NewPublicKey(pub)
	if e != nil {
		return e
	}
	hb, e := os.ReadFile(*hostKey)
	if e != nil {
		return e
	}
	hk, _, _, _, e := ssh.ParseAuthorizedKey(hb)
	if e != nil {
		return e
	}
	ca, e := os.ReadFile(filepath.Join(*dir, "tls", "ca.crt"))
	if e != nil {
		return e
	}
	token, e := os.ReadFile(filepath.Join(*dir, "tunnel.token"))
	if e != nil {
		return e
	}
	fmt.Fprint(os.Stderr, "连接包口令（至少 12 字符，不回显）：")
	var pass []byte
	if term.IsTerminal(int(os.Stdin.Fd())) {
		pass, e = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
	} else {
		v, er := bufio.NewReader(os.Stdin).ReadString('\n')
		e = er
		if e == io.EOF {
			e = nil
		}
		pass = []byte(strings.TrimSpace(v))
	}
	if e != nil {
		return e
	}
	p := model.Profile{SchemaVersion: 1, DeviceID: *device, ServerAddr: *server, ServerPort: 7000, TLSName: *tlsName, TunnelCA: string(ca), TunnelToken: strings.TrimSpace(string(token)), SSHAddr: *sshAddr, SSHUser: *user, SSHFingerprint: ssh.FingerprintSHA256(hk), SSHPrivateKey: string(keyPEM), Sites: sites}
	if e = secrets.Validate(p); e != nil {
		return e
	}
	bundle, e := secrets.Encrypt(p, string(pass))
	clear(pass)
	clear(keyPEM)
	if e != nil {
		return e
	}
	if _, e = os.Stat(*out); e == nil {
		return fmt.Errorf("连接包已存在，拒绝覆盖")
	}
	if e = os.WriteFile(*out, bundle, 0600); e != nil {
		return e
	}
	// Only the encrypted bundle is written; the generated private key has no plaintext disk copy.
	auth := fmt.Sprintf("restrict,command=\"/usr/bin/sudo -n /usr/local/lib/yandu/yandu-cloud serve --principal %s\" %s", *device, ssh.MarshalAuthorizedKey(pubSSH))
	if e = cloud.Atomic(filepath.Join(*dir, "authorized_keys"), []byte(auth), 0600); e != nil {
		return e
	}
	reg.Devices[*device] = []string{*siteID}
	b, _ = json.MarshalIndent(reg, "", "  ")
	if e = cloud.Atomic(filepath.Join(*dir, "registry.json"), b, 0600); e != nil {
		return e
	}
	fmt.Fprintf(os.Stderr, "已写入加密连接包 %s；请安全交付并在导入后删除传输副本。\n", *out)
	return nil
}

func gateway(args []string) error {
	fs := flag.NewFlagSet("storage-gateway", flag.ContinueOnError)
	config := fs.String("config", "/etc/yandu/gateway.json", "网关配置")
	if e := fs.Parse(args); e != nil {
		return e
	}
	data, e := os.ReadFile(*config)
	if e != nil {
		return e
	}
	var c cloud.GatewayConfig
	if e = json.Unmarshal(data, &c); e != nil {
		return e
	}
	h, e := cloud.NewGateway(c)
	if e != nil {
		return e
	}
	server := &http.Server{Addr: c.Listen, Handler: h, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() {
		<-ctx.Done()
		shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		server.Shutdown(shutdown)
	}()
	if c.TLSCert != "" {
		e = server.ListenAndServeTLS(c.TLSCert, c.TLSKey)
	} else {
		e = server.ListenAndServe()
	}
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}
func storagePair(dir, device, server, tlsName, out string, refreshMonitor bool) error {
	data, e := os.ReadFile(filepath.Join(dir, "storage-owner.json"))
	if e == nil && len(data) > 0 {
		var owner struct {
			DeviceID string `json:"deviceId"`
		}
		if !refreshMonitor || json.Unmarshal(data, &owner) != nil || owner.DeviceID != device {
			return fmt.Errorf("已有存储设备；仅相同设备可用 --refresh-monitor 更新监控连接包")
		}
	} else if refreshMonitor {
		return fmt.Errorf("监控更新需要现有 storage-only 设备归属记录")
	}
	if !os.IsNotExist(e) && e != nil {
		return e
	}
	ca, e := os.ReadFile(filepath.Join(dir, "tls", "ca.crt"))
	if e != nil {
		return e
	}
	token, e := os.ReadFile(filepath.Join(dir, "tunnel.token"))
	if e != nil {
		return e
	}
	p := model.Profile{SchemaVersion: 2, DeviceID: device, ServerAddr: server, ServerPort: 7000, TLSName: tlsName, TunnelCA: string(ca), TunnelToken: strings.TrimSpace(string(token)), Sites: []model.Site{}, Storage: &model.StorageTunnel{RemotePort: 19090, GatewayURL: "http://127.0.0.1:19091", MonitorToken: "ym_" + model.ID() + model.ID()}}
	if e = secrets.Validate(p); e != nil {
		return e
	}
	fmt.Fprint(os.Stderr, "连接包口令（至少 12 字符）：")
	var pass []byte
	if term.IsTerminal(int(os.Stdin.Fd())) {
		pass, e = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
	} else {
		line, er := bufio.NewReader(os.Stdin).ReadString('\n')
		if er != nil && er != io.EOF {
			return er
		}
		pass = []byte(strings.TrimSpace(line))
	}
	if e != nil {
		return e
	}
	bundle, e := secrets.Encrypt(p, string(pass))
	clear(pass)
	if e != nil {
		return e
	}
	f, e := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(bundle)
	f.Close()
	if e != nil {
		return e
	}
	owner, _ := json.Marshal(map[string]string{"deviceId": device})
	if e = cloud.Atomic(filepath.Join(dir, "storage-owner.json"), owner, 0600); e != nil {
		return e
	}
	monitor, _ := json.Marshal(cloud.MonitorConfig{DeviceID: device, GatewayURL: p.Storage.GatewayURL, MonitorToken: p.Storage.MonitorToken})
	if e = cloud.Atomic(filepath.Join(dir, "monitor.json"), monitor, 0600); e != nil {
		return e
	}
	fmt.Fprintln(os.Stderr, "已生成 schema v2 存储连接包：", out)
	return nil
}
