package cloudsync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"time"
	"yandu/internal/model"
)

type Request struct {
	Action      string          `json:"action"`
	Manifest    *model.Manifest `json:"manifest,omitempty"`
	OperationID string          `json:"operationId,omitempty"`
	SiteID      string          `json:"siteId,omitempty"`
}
type Client struct{ Profile model.Profile }

func (c Client) Call(ctx context.Context, req Request) (model.Receipt, error) {
	var result model.Receipt
	signer, e := ssh.ParsePrivateKey([]byte(c.Profile.SSHPrivateKey))
	if e != nil {
		return result, e
	}
	cfg := &ssh.ClientConfig{User: c.Profile.SSHUser, Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, Timeout: 10 * time.Second, HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if ssh.FingerprintSHA256(key) != c.Profile.SSHFingerprint {
			return model.Err("SSH_HOST_KEY_MISMATCH", "云端 SSH 主机指纹改变，连接已阻断")
		}
		return nil
	}}
	conn, e := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", c.Profile.SSHAddr)
	if e != nil {
		return result, model.Err("SSH_OFFLINE", "无法连接云端配置通道")
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(40 * time.Second))
	ch, channels, requests, e := ssh.NewClientConn(conn, c.Profile.SSHAddr, cfg)
	if e != nil {
		return result, fmt.Errorf("SSH_CONNECT_FAILED: %w", e)
	}
	client := ssh.NewClient(ch, channels, requests)
	defer client.Close()
	session, e := client.NewSession()
	if e != nil {
		return result, e
	}
	defer session.Close()
	b, _ := json.Marshal(req)
	session.Stdin = bytes.NewReader(b)
	out := &limitedBuffer{max: 65536}
	session.Stdout = out
	session.Stderr = io.Discard
	done := make(chan error, 1)
	go func() { done <- session.Run("yandu-sync") }()
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	case e = <-done:
	}
	if out.overflow {
		return result, model.Err("CLOUD_PROTOCOL_ERROR", "云端响应超过限制")
	}
	if json.Unmarshal(out.Bytes(), &result) != nil {
		return result, model.Err("CLOUD_PROTOCOL_ERROR", "云端未返回有效回执")
	}
	if result.Error != nil {
		return result, result.Error
	}
	if e != nil {
		return result, model.Err("CLOUD_SYNC_FAILED", "云端同步命令失败")
	}
	return result, nil
}

type limitedBuffer struct {
	bytes.Buffer
	max      int
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	left := b.max - b.Len()
	if len(p) > left {
		b.overflow = true
		p = p[:left]
	}
	b.Buffer.Write(p)
	return n, nil
}
