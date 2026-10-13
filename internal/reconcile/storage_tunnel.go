package reconcile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func (e *Engine) StorageTunnelStatus(ctx context.Context) error {
	s, err := e.Store.Read()
	if err != nil {
		return err
	}
	if s.Profile == nil || s.Profile.Storage == nil {
		return fmt.Errorf("未配置存储隧道")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1:18768/api/status", nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth("yandu", e.AdminPassword)
	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("隧道管理接口暂不可用")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("隧道管理鉴权失败")
	}
	var status map[string][]struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&status); err != nil {
		return err
	}
	for _, proxies := range status {
		for _, proxy := range proxies {
			if proxy.Name == s.Profile.DeviceID+"-storage" && proxy.Status == "running" {
				return nil
			}
		}
	}
	return fmt.Errorf("存储隧道尚未注册，请检查服务器及 TLS 连接")
}
