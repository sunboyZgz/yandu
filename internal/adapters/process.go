package adapters

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type Process struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	ctx       context.Context
	alive     bool
	name, log string
	args      []string
	lastError string
}

func Binary(name string) (string, error) {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	exe, _ := os.Executable()
	for _, p := range []string{filepath.Join(filepath.Dir(exe), "bin", name+suffix), filepath.Join(filepath.Dir(exe), name+suffix)} {
		if v, e := os.Stat(p); e == nil && !v.IsDir() {
			return p, nil
		}
	}
	return exec.LookPath(name)
}
func Run(ctx context.Context, name string, args ...string) error {
	bin, e := Binary(name)
	if e != nil {
		return fmt.Errorf("COMPONENT_MISSING: 未找到 %s", name)
	}
	command := exec.CommandContext(ctx, bin, args...)
	if name == "caddy" {
		for i, arg := range args {
			if arg == "--config" && i+1 < len(args) {
				base := filepath.Dir(filepath.Dir(filepath.Dir(args[i+1])))
				command.Env = append(os.Environ(), "XDG_CONFIG_HOME="+filepath.Join(base, "caddy-config"), "XDG_DATA_HOME="+filepath.Join(base, "caddy-data"))
				break
			}
		}
	}
	b, e := command.CombinedOutput()
	if e != nil {
		return fmt.Errorf("%s: %s", name, string(b))
	}
	return nil
}
func (p *Process) Start(parent context.Context, name, log string, args ...string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		return nil
	}
	bin, e := Binary(name)
	if e != nil {
		return fmt.Errorf("COMPONENT_MISSING: 未找到 %s", name)
	}
	ctx, cancel := context.WithCancel(parent)
	p.ctx = ctx
	p.cancel = cancel
	p.name = bin
	p.log = log
	p.args = args
	if e = p.launch(); e != nil {
		cancel()
		p.cancel = nil
		return e
	}
	go p.watch(ctx)
	return nil
}
func (p *Process) launch() error {
	f, e := os.OpenFile(p.log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if e != nil {
		return e
	}
	cmd := exec.CommandContext(p.ctx, p.name, p.args...)
	if filepath.Base(p.name) == "caddy" || filepath.Base(p.name) == "caddy.exe" {
		base := filepath.Dir(p.log)
		cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+filepath.Join(base, "caddy-config"), "XDG_DATA_HOME="+filepath.Join(base, "caddy-data"))
	}
	cmd.Stdout = f
	cmd.Stderr = f
	if e = cmd.Start(); e != nil {
		f.Close()
		p.lastError = e.Error()
		return e
	}
	p.cmd = cmd
	p.alive = true
	go func() {
		e := cmd.Wait()
		f.Close()
		p.mu.Lock()
		if p.cmd == cmd {
			p.alive = false
			if e != nil {
				p.lastError = e.Error()
			}
		}
		p.mu.Unlock()
	}()
	return nil
}
func (p *Process) watch(ctx context.Context) {
	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		p.mu.Lock()
		if p.ctx == ctx && !p.alive {
			if e := p.launch(); e != nil {
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
			} else {
				backoff = time.Second
			}
		}
		p.mu.Unlock()
	}
}
func (p *Process) Stop() {
	p.mu.Lock()
	cancel := p.cancel
	p.cancel = nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	for i := 0; i < 50; i++ {
		if !p.Alive() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}
func (p *Process) SetArgs(args ...string) { p.mu.Lock(); defer p.mu.Unlock(); p.args = args }
func (p *Process) Alive() bool            { p.mu.Lock(); defer p.mu.Unlock(); return p.alive }
func Listening(addr string) bool {
	c, e := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if e == nil {
		c.Close()
		return true
	}
	return false
}
