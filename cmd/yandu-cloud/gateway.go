package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"yandu/internal/cloud"
)

func gateway(args []string) error {
	fs := flag.NewFlagSet("storage-gateway", flag.ContinueOnError)
	config := fs.String("config", "/etc/yandu/gateway.json", "网关配置")
	check := fs.Bool("check-config", false, "校验配置和 TLS 文件后退出，不监听端口")
	defaultTimeout := os.Getenv("YANDU_GATEWAY_SHUTDOWN_TIMEOUT")
	if defaultTimeout == "" {
		defaultTimeout = "5m"
	}
	timeout := fs.String("shutdown-timeout", defaultTimeout, "退出时等待正在传输的请求完成的最长时间，例如 5m")
	if err := fs.Parse(args); err != nil {
		return err
	}
	drain, err := time.ParseDuration(*timeout)
	if err != nil || drain <= 0 || drain > 24*time.Hour || fs.NArg() != 0 {
		return fmt.Errorf("shutdown-timeout 必须大于 0 且不超过 24h，不接受位置参数")
	}
	data, err := os.ReadFile(*config)
	if err != nil {
		return err
	}
	var c cloud.GatewayConfig
	if err = json.Unmarshal(data, &c); err != nil {
		return err
	}
	h, err := cloud.NewGateway(c)
	if err != nil {
		return err
	}
	if (c.TLSCert == "") != (c.TLSKey == "") {
		return fmt.Errorf("网关 TLS 证书和私钥必须同时配置")
	}
	if c.TLSCert != "" {
		if _, err := tls.LoadX509KeyPair(c.TLSCert, c.TLSKey); err != nil {
			return fmt.Errorf("网关 TLS 文件无效：%w", err)
		}
	}
	if *check {
		return nil
	}
	listener, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return serveGateway(ctx, server, listener, drain, c.TLSCert, c.TLSKey)
}

// Keep the process alive until Shutdown has finished, rather than returning as
// soon as Serve stops accepting new requests. Close only after the drain limit.
func serveGateway(ctx context.Context, server *http.Server, listener net.Listener, drain time.Duration, cert, key string) error {
	defer listener.Close()
	served := make(chan error, 1)
	go func() {
		if cert != "" {
			served <- server.ServeTLS(listener, cert, key)
		} else {
			served <- server.Serve(listener)
		}
	}()
	select {
	case err := <-served:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), drain)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			<-served
			return fmt.Errorf("网关等待传输完成超过退出期限 %s，剩余连接已关闭：%w", drain, err)
		}
		if err := <-served; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
