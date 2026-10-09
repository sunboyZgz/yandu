package tests

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"yandu/internal/cloud"
	"yandu/internal/cloudsync"
	"yandu/internal/model"
	"yandu/internal/reconcile"
	"yandu/internal/resource"
	"yandu/internal/secrets"
	"yandu/internal/state"
)

func binary(t *testing.T, name, env string) string {
	t.Helper()
	if p := os.Getenv(env); p != "" {
		return p
	}
	p, e := exec.LookPath(name)
	if e != nil {
		t.Skip(name + " not available; set " + env)
	}
	return p
}
func freePort(t *testing.T) int {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
func run(t *testing.T, bin string, args ...string) {
	t.Helper()
	out, e := exec.Command(bin, args...).CombinedOutput()
	if e != nil {
		t.Fatalf("%s: %s", filepath.Base(bin), out)
	}
}
func start(t *testing.T, bin string, args ...string) {
	t.Helper()
	f, e := os.Create(filepath.Join(t.TempDir(), filepath.Base(bin)+".log"))
	if e != nil {
		t.Fatal(e)
	}
	c := exec.Command(bin, args...)
	c.Stdout = f
	c.Stderr = f
	if e = c.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		c.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { c.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			c.Process.Kill()
			<-done
		}
		f.Close()
	})
}
func waitPort(t *testing.T, addr string) {
	t.Helper()
	for i := 0; i < 80; i++ {
		c, e := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if e == nil {
			c.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("listener not ready: " + addr)
}
func makeTLS(t *testing.T, dir string) (string, string, string) {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, e := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(2), DNSNames: []string{"example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, e := x509.CreateCertificate(rand.Reader, cert, ca, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	capath := filepath.Join(dir, "ca.crt")
	cp := filepath.Join(dir, "server.crt")
	kp := filepath.Join(dir, "server.key")
	os.WriteFile(capath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600)
	os.WriteFile(cp, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600)
	os.WriteFile(kp, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600)
	return capath, cp, kp
}
func sshEndpoint(t *testing.T, h cloud.Helper) (addr, fingerprint, private string, stop func()) {
	t.Helper()
	_, hostPriv, _ := ed25519.GenerateKey(rand.Reader)
	hostSigner, _ := ssh.NewSignerFromKey(hostPriv)
	pub, clientPriv, _ := ed25519.GenerateKey(rand.Reader)
	public, _ := ssh.NewPublicKey(pub)
	der, _ := x509.MarshalPKCS8PrivateKey(clientPriv)
	private = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(conn ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
		if conn.User() != "yandu-sync" || !bytes.Equal(k.Marshal(), public.Marshal()) {
			return nil, fmt.Errorf("denied")
		}
		return &ssh.Permissions{}, nil
	}}
	cfg.AddHostKey(hostSigner)
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	stop = func() { l.Close() }
	t.Cleanup(stop)
	go func() {
		for {
			conn, e := l.Accept()
			if e != nil {
				return
			}
			go func() {
				defer conn.Close()
				sc, chans, reqs, e := ssh.NewServerConn(conn, cfg)
				if e != nil {
					return
				}
				defer sc.Close()
				go ssh.DiscardRequests(reqs)
				for ch := range chans {
					if ch.ChannelType() != "session" {
						ch.Reject(ssh.Prohibited, "restricted")
						continue
					}
					channel, requests, e := ch.Accept()
					if e != nil {
						continue
					}
					go func() {
						defer channel.Close()
						for req := range requests {
							if req.Type != "exec" {
								req.Reply(false, nil)
								continue
							}
							req.Reply(true, nil)
							var q cloudsync.Request
							e := json.NewDecoder(io.LimitReader(channel, 65536)).Decode(&q)
							var r model.Receipt
							if e == nil {
								ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
								r, e = h.Call(ctx, "owner", q)
								cancel()
							}
							if e != nil {
								me, ok := e.(*model.Error)
								if !ok {
									me = &model.Error{Code: "TEST_HELPER_ERROR", Message: e.Error()}
								}
								r.Error = me
							}
							json.NewEncoder(channel).Encode(r)
							exit := uint32(0)
							if e != nil {
								exit = 1
							}
							channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{exit}))
							return
						}
					}()
				}
			}()
		}
	}()
	return l.Addr().String(), ssh.FingerprintSHA256(hostSigner.PublicKey()), private, stop
}
func waitOperation(t *testing.T, s *state.Store, id string, success bool) {
	t.Helper()
	deadline := time.Now().Add(50 * time.Second)
	for time.Now().Before(deadline) {
		v, _ := s.Read()
		for _, op := range v.Operations {
			if op.ID == id && op.Status != "running" {
				if (op.Status == "succeeded") != success {
					t.Fatalf("operation %s: %s (%s)", op.Action, op.Status, op.Error)
				}
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("operation timeout")
}
func TestRealDataChainAndAutomaticConfiguration(t *testing.T) {
	caddy := binary(t, "caddy", "YANDU_TEST_CADDY")
	frpc := binary(t, "frpc", "YANDU_TEST_FRPC")
	frps := binary(t, "frps", "YANDU_TEST_FRPS")
	nginx := binary(t, "nginx", "YANDU_TEST_NGINX")
	t.Setenv("PATH", filepath.Dir(caddy)+string(os.PathListSeparator)+filepath.Dir(frpc)+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, addr := range []string{model.ResourceAddr, model.CaddyAdmin, model.GuardAddr, "127.0.0.1:18768", "127.0.0.1:18080"} {
		l, e := net.Listen("tcp", addr)
		if e != nil {
			t.Fatalf("integration port in use: %s", addr)
		}
		l.Close()
	}
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	ca, cert, key := makeTLS(t, dir)
	port := freePort(t)
	token := model.ID() + model.ID()
	frpsConfig := filepath.Join(dir, "frps.toml")
	os.WriteFile(frpsConfig, []byte(fmt.Sprintf("bindAddr = \"127.0.0.1\"\nbindPort = %d\nproxyBindAddr = \"127.0.0.1\"\nvhostHTTPPort = 18080\nauth.method = \"token\"\nauth.token = %q\ntransport.tls.force = true\ntransport.tls.certFile = %q\ntransport.tls.keyFile = %q\nlog.level = \"warn\"\n", port, token, cert, key)), 0600)
	run(t, frps, "verify", "-c", frpsConfig)
	start(t, frps, "-c", frpsConfig)
	waitPort(t, fmt.Sprintf("127.0.0.1:%d", port))
	cloudDir := filepath.Join(dir, "cloud")
	os.MkdirAll(filepath.Join(cloudDir, "nginx", "generations", "bootstrap"), 0700)
	os.WriteFile(filepath.Join(cloudDir, "nginx", "generations", "bootstrap", "main.locations.conf"), []byte("# Yandu managed generation bootstrap; do not edit\n"), 0644)
	os.Symlink(filepath.Join("generations", "bootstrap"), filepath.Join(cloudDir, "nginx", "current"))
	cs := cloud.State{Generation: "bootstrap", Revisions: map[string]int64{}, Sources: map[string]int64{}, Routes: map[string][]cloud.OwnedRoute{}, Operations: map[string]cloud.Recorded{}}
	b, _ := json.Marshal(cs)
	os.WriteFile(filepath.Join(cloudDir, "state.json"), b, 0600)
	nginxPort := freePort(t)
	probeURL := fmt.Sprintf("http://127.0.0.1:%d", nginxPort)
	site := model.Site{ID: "main", Domain: "example.com", AllowedPrefixes: []string{"/blog/", "/love/", "/gallery/"}, ProtectedPrefixes: []string{"/blog/api/"}}
	reg := cloud.Registry{Sites: []cloud.RegisteredSite{{Site: site, ProbeURL: probeURL}}, Devices: map[string][]string{"owner": {"main"}}}
	b, _ = json.Marshal(reg)
	os.WriteFile(filepath.Join(cloudDir, "registry.json"), b, 0600)
	conf := filepath.Join(dir, "nginx.conf")
	os.WriteFile(conf, []byte(fmt.Sprintf("pid %s;\nerror_log %s warn;\nevents {}\nhttp { access_log off; server { listen 127.0.0.1:%d; server_name example.com; include %s; location / { return 200 'cloud-page'; } } }\n", filepath.Join(dir, "nginx.pid"), filepath.Join(dir, "nginx.log"), nginxPort, filepath.Join(cloudDir, "nginx", "current", "main.locations.conf"))), 0600)
	run(t, nginx, "-p", dir, "-c", conf, "-t")
	start(t, nginx, "-p", dir, "-c", conf, "-g", "daemon off;")
	waitPort(t, fmt.Sprintf("127.0.0.1:%d", nginxPort))
	helper := cloud.Helper{Dir: cloudDir, RegistryPath: filepath.Join(cloudDir, "registry.json"), Nginx: nginx, Runner: func(ctx context.Context, bin string, args ...string) error {
		args = append([]string{"-p", dir, "-c", conf}, args...)
		out, e := exec.CommandContext(ctx, bin, args...).CombinedOutput()
		if e != nil {
			return fmt.Errorf("%s", out)
		}
		return nil
	}}
	sshAddr, fp, private, stopSSH := sshEndpoint(t, helper)
	cab, _ := os.ReadFile(ca)
	profile := model.Profile{SchemaVersion: 1, DeviceID: "owner", ServerAddr: "127.0.0.1", ServerPort: port, TLSName: "example.com", TunnelCA: string(cab), TunnelToken: token, SSHAddr: sshAddr, SSHUser: "yandu-sync", SSHFingerprint: fp, SSHPrivateKey: private, Sites: []model.Site{site}}
	// The pairing format exercises authenticated encryption, validation and corruption detection.
	encrypted, e := secrets.Encrypt(profile, "test-pairing-password")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = secrets.Decrypt(encrypted, "wrong-password"); e == nil {
		t.Fatal("bad profile password accepted")
	}
	if output := os.Getenv("YANDU_BROWSER_FIXTURE_OUT"); output != "" {
		if err := os.WriteFile(output, encrypted, 0600); err != nil {
			t.Fatal(err)
		}
	}
	profile, e = secrets.Decrypt(encrypted, "test-pairing-password")
	if e != nil {
		t.Fatal(e)
	}
	upstream, _ := url.Parse(probeURL)
	gateway := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(upstream))
	defer gateway.Close()
	gatewayAddr := strings.TrimPrefix(gateway.URL, "https://")
	client := gateway.Client()
	transport := client.Transport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, gatewayAddr)
	}
	client.Transport = transport
	client.CheckRedirect = func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	store, e := state.Open(filepath.Join(dir, "agent"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	store.Update(func(s *model.Snapshot) error { s.Profile = &profile; s.Roots = []string{dir}; return nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	engine := &reconcile.Engine{Store: store, Dir: filepath.Join(dir, "agent"), Ctx: ctx, AdminPassword: "admin-password-for-tests", HTTPClient: client}
	defer engine.Close()
	var leakedWebsiteCredential atomic.Bool
	guard := &http.Server{Addr: model.GuardAddr, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" || strings.HasPrefix(r.Header.Get("Authorization"), "Bearer website-") {
			leakedWebsiteCredential.Store(true)
		}
		(resource.Guard{Store: store}).ServeHTTP(w, r)
	})}
	go guard.ListenAndServe()
	defer guard.Close()
	waitPort(t, model.GuardAddr)
	if e = engine.Boot(); e != nil {
		t.Fatal(e)
	}
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a6V8AAAAASUVORK5CYII=")
	add := func(id, base, pre string) model.Project {
		root := filepath.Join(dir, id)
		os.MkdirAll(filepath.Join(root, strings.TrimPrefix(pre, base+"/")), 0700)
		os.WriteFile(filepath.Join(root, strings.TrimPrefix(pre, base+"/"), "中文 a.png"), png, 0600)
		p, e := engine.Save(model.Project{ID: id, DisplayName: id, SiteID: "main", ProjectBase: base, RootPath: root, ResourcePrefixes: []string{pre}, AccessPolicy: "public_read", CachePolicy: "revalidate", ContentPolicy: "media_only"}, "", 0)
		if e != nil {
			t.Fatal(e)
		}
		op, e := engine.Queue(id, "apply", p.Revision, true)
		if e != nil {
			t.Fatal(e)
		}
		waitOperation(t, store, op, true)
		return p
	}
	add("blog", "/blog", "/blog/xxx/")
	add("love", "/love", "/love/photos/")
	add("gallery", "/gallery", "/gallery/media/")
	get := func(method, path string, headers map[string]string) (int, []byte, http.Header) {
		req, _ := http.NewRequest(method, "https://example.com"+path, nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b, resp.Header
	}
	for _, path := range []string{"/blog", "/blog/api/users", "/blog/posts/123", "/blog/assets/bundle.png", "/blog/xxxevil/a.png"} {
		code, b, _ := get("GET", path, nil)
		if code != 200 || string(b) != "cloud-page" {
			t.Fatalf("cloud page hijacked: %s %d %s", path, code, b)
		}
	}
	for _, path := range []string{"/blog/xxx/中文%20a.png", "/love/photos/中文%20a.png", "/gallery/media/中文%20a.png"} {
		code, b, h := get("GET", path, nil)
		if code != 200 || !bytes.Equal(b, png) || h.Get("Content-Type") != "image/png" {
			t.Fatalf("resource mismatch: %s %d (%q)", path, code, b)
		}
	}
	code, _, _ := get("GET", "/blog/xxx/中文%20a.png", map[string]string{"Cookie": "website-session=private", "Authorization": "Bearer website-login"})
	if code != 200 || leakedWebsiteCredential.Load() {
		t.Fatal("public resource forwarded website credentials")
	}
	code, b, h := get("GET", "/blog/xxx/中文%20a.png", map[string]string{"Range": "bytes=0-7"})
	if code != 206 || !bytes.Equal(b, png[:8]) || h.Get("Content-Range") == "" {
		t.Fatal("Range failed")
	}
	code, b, _ = get("HEAD", "/blog/xxx/中文%20a.png", nil)
	if code != 200 || len(b) != 0 {
		t.Fatal("HEAD failed")
	}
	code, _, h = get("GET", "/blog/xxx/中文%20a.png", nil)
	code, _, _ = get("GET", "/blog/xxx/中文%20a.png", map[string]string{"If-None-Match": h.Get("ETag")})
	if code != 304 {
		t.Fatal("conditional request failed")
	}
	code, _, _ = get("GET", "/blog/xxx/中文%20a.png", map[string]string{"Range": "bytes=99999-"})
	if code != 416 {
		t.Fatal("invalid Range accepted")
	}
	code, b, _ = get("GET", "/blog/xxx/missing.png", nil)
	if code != 404 || bytes.Contains(b, []byte("cloud-page")) {
		t.Fatal("missing resource fell back to SPA")
	}
	code, _, _ = get("GET", "/api/v1/status", nil)
	if code != 200 {
		t.Fatal("website behavior changed")
	}
	local, _ := url.Parse("http://" + model.ResourceAddr + "/api/v1/status")
	req, _ := http.NewRequest("GET", local.String(), nil)
	req.Host = "example.com"
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	if resp.StatusCode != 404 {
		t.Fatal("management leaked to resource port")
	}
	resp.Body.Close()
	// Tightening a public route to password protection blocks before component reload.
	current, _ := store.Read()
	gallery := current.Projects[2]
	gallery.AccessPolicy = "basic_auth"
	gallery.Username = "reader"
	gallery, err := engine.Save(gallery, "resource-password", gallery.Revision)
	if err != nil {
		t.Fatal(err)
	}
	code, _, _ = get("GET", "/gallery/media/中文%20a.png", nil)
	if code == 200 {
		t.Fatal("permission tightening did not block")
	}
	opBasic, err := engine.Queue(gallery.ID, "apply", gallery.Revision, true)
	if err != nil {
		t.Fatal(err)
	}
	waitOperation(t, store, opBasic, false)
	code, _, _ = get("GET", "/gallery/media/中文%20a.png", nil)
	if code != 401 {
		t.Fatalf("expected password challenge, got %d", code)
	}
	authorization := "Basic " + base64.StdEncoding.EncodeToString([]byte("reader:resource-password"))
	code, protectedBody, _ := get("GET", "/gallery/media/中文%20a.png", map[string]string{"Authorization": authorization})
	if code != 200 || !bytes.Equal(protectedBody, png) {
		t.Fatal("Nginx stripped independent resource authorization")
	}
	if err = engine.CheckPassword(context.Background(), gallery.ID, "resource-password"); err != nil {
		t.Fatal(err)
	}
	// Removing a mapping retains 404 until an explicit route release returns ownership to the website.
	current, _ = store.Read()
	gallery = current.Projects[2]
	opRemove, err := engine.Queue(gallery.ID, "remove", gallery.Revision, false)
	if err != nil {
		t.Fatal(err)
	}
	waitOperation(t, store, opRemove, true)
	code, _, _ = get("GET", "/gallery/media/中文%20a.png", nil)
	if code != 404 {
		t.Fatal("removed mapping not denied")
	}
	if err = engine.Release(context.Background(), "main", model.RouteID("gallery", "/gallery/media/")); err != nil {
		t.Fatal(err)
	}
	code, bodyAfterRelease, _ := get("GET", "/gallery/media/中文%20a.png", nil)
	if code != 200 || string(bodyAfterRelease) != "cloud-page" {
		t.Fatal("release did not restore original website ownership")
	}
	if os.Getenv("YANDU_TEST_CAPACITY") == "1" {
		capacityStart := time.Now()
		s, _ := store.Read()
		for _, old := range s.Projects {
			p := old
			for i := 1; i < 10; i++ {
				p.ResourcePrefixes = append(p.ResourcePrefixes, fmt.Sprintf("%s/extra%d/", p.ProjectBase, i))
			}
			p, err = engine.Save(p, "", p.Revision)
			if err != nil {
				t.Fatal(err)
			}
			op, err := engine.Queue(p.ID, "apply", p.Revision, true)
			if err != nil {
				t.Fatal(err)
			}
			waitOperation(t, store, op, true)
		}
		for n := 0; n < 8; n++ {
			id := fmt.Sprintf("capacity-%d", n)
			root := filepath.Join(dir, id)
			os.MkdirAll(filepath.Join(root, fmt.Sprintf("p%d/media0", n)), 0700)
			os.WriteFile(filepath.Join(root, fmt.Sprintf("p%d/media0", n), "中文 a.png"), png, 0600)
			prefixes := []string{}
			for i := 0; i < 10; i++ {
				prefixes = append(prefixes, fmt.Sprintf("/gallery/p%d/media%d/", n, i))
			}
			p, er := engine.Save(model.Project{ID: id, DisplayName: id, SiteID: "main", ProjectBase: "/gallery", RootPath: root, ResourcePrefixes: prefixes, AccessPolicy: "public_read", CachePolicy: "revalidate", ContentPolicy: "media_only"}, "", 0)
			if er != nil {
				t.Fatal(er)
			}
			op, er := engine.Queue(p.ID, "apply", p.Revision, true)
			if er != nil {
				t.Fatal(er)
			}
			waitOperation(t, store, op, true)
		}
		s, _ = store.Read()
		routeCount := 0
		for _, p := range s.Projects {
			routeCount += len(p.ResourcePrefixes)
		}
		if len(s.Projects) != 10 || routeCount != 100 {
			t.Fatalf("capacity fixture %d projects %d routes", len(s.Projects), routeCount)
		}
		var wg sync.WaitGroup
		var mu sync.Mutex
		problems := []string{}
		loadStart := time.Now()
		for _, p := range s.Projects {
			wg.Add(1)
			go func(p model.Project) {
				defer wg.Done()
				req, _ := http.NewRequest("GET", "https://example.com"+p.ResourcePrefixes[0]+"中文%20a.png", nil)
				resp, er := client.Do(req)
				if er != nil {
					mu.Lock()
					problems = append(problems, er.Error())
					mu.Unlock()
					return
				}
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if resp.StatusCode != 200 || !bytes.Equal(body, png) {
					mu.Lock()
					problems = append(problems, p.ID)
					mu.Unlock()
				}
			}(p)
		}
		wg.Wait()
		if len(problems) > 0 {
			t.Fatalf("concurrent reads failed: %v", problems)
		}
		t.Logf("Capacity: 100 registered routes across 10 projects, 10 concurrent exact-byte reads in %s; configuration expansion in %s (loopback, 68-byte fixture; not WAN throughput).", time.Since(loadStart), time.Since(capacityStart))
	}
	wrongProfile := profile
	wrongProfile.SSHFingerprint = "SHA256:wrong"
	if _, err = (cloudsync.Client{Profile: wrongProfile}).Call(context.Background(), cloudsync.Request{Action: "status", SiteID: "main"}); err == nil || !strings.Contains(err.Error(), "SSH_HOST_KEY_MISMATCH") {
		t.Fatalf("host key pin not enforced: %v", err)
	}
	before, _ := store.Read()
	revision := before.CloudRevisions["main"]
	p := before.Projects[0]
	migrated := filepath.Join(dir, "migrated")
	os.MkdirAll(filepath.Join(migrated, "xxx"), 0700)
	os.WriteFile(filepath.Join(migrated, "xxx", "中文 a.png"), png, 0600)
	p.RootPath = migrated
	p, e = engine.Save(p, "", p.Revision)
	if e != nil {
		t.Fatal(e)
	}
	op, e := engine.Queue(p.ID, "apply", p.Revision, true)
	if e != nil {
		t.Fatal(e)
	}
	waitOperation(t, store, op, true)
	after, _ := store.Read()
	if after.CloudRevisions["main"] != revision {
		t.Fatal("directory migration reloaded Nginx")
	}
	// Losing the configuration channel must not stop an already established data channel.
	stopSSH()
	code, _, _ = get("GET", "/love/photos/中文%20a.png", nil)
	if code != 200 {
		t.Fatal("SSH loss stopped resources")
	}
	p = after.Projects[0]
	op, e = engine.Queue(p.ID, "disable", p.Revision, false)
	if e != nil {
		t.Fatal(e)
	}
	code, _, _ = get("GET", "/blog/xxx/中文%20a.png", nil)
	if code == 200 {
		t.Fatal("disable did not block immediately")
	}
	waitOperation(t, store, op, false)
	code, _, _ = get("GET", "/blog", nil)
	if code != 200 {
		t.Fatal("disable harmed page")
	}
	if _, e = os.Stat(filepath.Join(migrated, "xxx", "中文 a.png")); e != nil {
		t.Fatal("disable deleted file")
	}
	t.Log("Verified: three projects, automatic ingress+frpc registration, exact bytes, Chinese/space names, HEAD/Range/ETag, page/API isolation, unchanged ingress on directory migration, password tightening/authentication, delete/release, host key pinning, immediate disable, retained files, SSH channel failure isolation.")
}
