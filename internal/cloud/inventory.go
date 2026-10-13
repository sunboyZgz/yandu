package cloud

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
	"yandu/internal/inventory"
	"yandu/internal/pathpolicy"
	"yandu/internal/secrets"
)

type MonitorConfig struct {
	DeviceID     string `json:"deviceId"`
	GatewayURL   string `json:"gatewayUrl"`
	MonitorToken string `json:"monitorToken"`
}

func LoadMonitorConfig(path string) (MonitorConfig, error) {
	var c MonitorConfig
	f, e := os.Open(path)
	if e != nil {
		return c, fmt.Errorf("无法读取监控配置；新安装会自动生成，旧安装需重新交付监控连接包：%w", e)
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return c, e
	}
	if !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
		return c, fmt.Errorf("监控配置须为仅所有者可读的普通文件（chmod 600）")
	}
	dec := json.NewDecoder(io.LimitReader(f, 65537))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&c); e != nil {
		return c, fmt.Errorf("监控配置格式无效")
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return c, fmt.Errorf("监控配置存在多余内容")
	}
	if !pathpolicy.Identifier(c.DeviceID) || !secrets.ValidMonitorToken(c.MonitorToken) {
		return c, fmt.Errorf("监控配置中的设备或凭证无效")
	}
	u, e := url.Parse(c.GatewayURL)
	if e != nil || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return c, fmt.Errorf("监控网关地址无效")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && !(u.Scheme == "http" && ip != nil && ip.IsLoopback()) {
		return c, fmt.Errorf("监控须使用回环 HTTP 或受信 TLS 网关")
	}
	c.GatewayURL = u.Scheme + "://" + u.Host
	return c, nil
}

type MonitorError struct {
	Code        string
	Message     string
	CanUseCache bool
}

func (e *MonitorError) Error() string { return e.Code + ": " + e.Message }

func FetchInventory(ctxClient *http.Client, c MonitorConfig) (inventory.Report, error) {
	var report inventory.Report
	req, e := http.NewRequest(http.MethodGet, c.GatewayURL+"/storage/v1/monitor", nil)
	if e != nil {
		return report, e
	}
	req.Header.Set("Authorization", "Bearer "+c.MonitorToken)
	res, e := ctxClient.Do(req)
	if e != nil {
		return report, &MonitorError{"DEVICE_UNREACHABLE", "网关或本地设备不可达", true}
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		json.NewDecoder(io.LimitReader(res.Body, 65536)).Decode(&body)
		code := body.Error.Code
		if code == "" {
			code = fmt.Sprintf("HTTP_%d", res.StatusCode)
		}
		message := "监控查询失败；检查配对、网关及设备版本"
		if res.StatusCode == 401 || res.StatusCode == 403 {
			message = "监控凭证无效、已轮换或访问被拒绝"
		}
		return report, &MonitorError{code, message, res.StatusCode == 503 && (code == "DEVICE_OFFLINE" || code == "STORAGE_UNAVAILABLE")}
	}
	if e = json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&report); e != nil || report.SchemaVersion != 1 || report.DeviceID != c.DeviceID || report.SampledAt.IsZero() {
		return report, &MonitorError{"INVALID_MONITOR_RESPONSE", "设备返回了无效或不匹配的监控快照", false}
	}
	return report, nil
}

type monitorCache struct {
	Binding string           `json:"binding"`
	Report  inventory.Report `json:"report"`
}

func monitorBinding(c MonitorConfig) string {
	sum := sha256.Sum256([]byte(c.DeviceID + "\x00" + c.GatewayURL + "\x00" + c.MonitorToken))
	return hex.EncodeToString(sum[:])
}
func monitorCachePath(config string) string {
	return filepath.Join(filepath.Dir(config), "monitor-last.json")
}

func SaveInventory(config string, c MonitorConfig, r inventory.Report) error {
	data, e := json.Marshal(monitorCache{monitorBinding(c), r})
	if e != nil {
		return e
	}
	return Atomic(monitorCachePath(config), data, 0600)
}
func CachedInventory(config string, c MonitorConfig) (inventory.Report, error) {
	var cached monitorCache
	f, e := os.Open(monitorCachePath(config))
	if e != nil {
		return cached.Report, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
		return cached.Report, fmt.Errorf("监控缓存权限无效")
	}
	if e = json.NewDecoder(io.LimitReader(f, 8<<20)).Decode(&cached); e != nil {
		return cached.Report, e
	}
	if cached.Binding != monitorBinding(c) || cached.Report.DeviceID != c.DeviceID || cached.Report.SchemaVersion != 1 || cached.Report.SampledAt.IsZero() {
		return cached.Report, fmt.Errorf("缓存不属于当前配对")
	}
	return cached.Report, nil
}

func MonitorClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, ResponseHeaderTimeout: timeout}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
