package pathpolicy

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"yandu/internal/model"
)

var identifier = regexp.MustCompile(`^[a-z][a-z0-9-]{0,47}$`)
var segment = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var domain = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)

func Identifier(s string) bool { return identifier.MatchString(s) }
func Domain(s string) bool {
	return len(s) <= 253 && strings.Contains(s, ".") && domain.MatchString(s) && !strings.Contains(s, "..")
}
func Prefix(p string, trailing bool) error {
	if !strings.HasPrefix(p, "/") || p == "/" || strings.HasSuffix(p, "/") != trailing {
		return model.Err("URL_AMBIGUOUS", "路径必须使用 ASCII 路径段，资源前缀以 / 结尾")
	}
	for _, s := range strings.Split(strings.Trim(p, "/"), "/") {
		if !segment.MatchString(s) {
			return model.Err("URL_AMBIGUOUS", "路径段只允许字母、数字、下划线和连字符")
		}
	}
	return nil
}
func Overlap(a, b string) bool {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}
func ValidateProject(p model.Project, s model.Snapshot) error {
	if !Identifier(p.ID) || strings.TrimSpace(p.DisplayName) == "" || len(p.DisplayName) > 160 {
		return model.Err("INVALID_PROJECT", "项目标识或名称无效")
	}
	site, ok := s.Site(p.SiteID)
	if !ok {
		return model.Err("CLOUD_AUTH_DENIED", "请先导入包含此站点的连接包")
	}
	if err := Prefix(p.ProjectBase, false); err != nil {
		return err
	}
	if p.Publication != "disabled" && p.Publication != "active" {
		return model.Err("INVALID_PROJECT", "发布状态无效")
	}
	if p.AccessPolicy != "public_read" && p.AccessPolicy != "basic_auth" {
		return model.Err("INVALID_PROJECT", "访问策略无效")
	}
	if p.AccessPolicy == "basic_auth" && (p.Username == "" || len(p.Username) > 80 || strings.ContainsAny(p.Username, ":\r\n\x00") || p.PasswordHash == "") {
		return model.Err("INVALID_PROJECT", "密码保护需要独立资源用户名和密码")
	}
	if p.ContentPolicy != "media_only" || (p.CachePolicy != "revalidate" && p.CachePolicy != "immutable") {
		return model.Err("INVALID_PROJECT", "文件类型或缓存策略无效")
	}
	if len(p.ResourcePrefixes) < 1 || len(p.ResourcePrefixes) > 100 {
		return model.Err("INVALID_PROJECT", "每个项目需要 1–100 个资源前缀")
	}
	for i, pre := range p.ResourcePrefixes {
		if err := Prefix(pre, true); err != nil {
			return err
		}
		if !strings.HasPrefix(pre, p.ProjectBase+"/") || pre == p.ProjectBase+"/" {
			return model.Err("PAGE_ROUTE_PROTECTED", "资源前缀必须比页面基础路径更深")
		}
		allowed := false
		for _, a := range site.AllowedPrefixes {
			if strings.HasPrefix(pre, a) {
				allowed = true
			}
		}
		if !allowed {
			return model.Err("CLOUD_AUTH_DENIED", "资源路径超出站点授权范围")
		}
		protected := append([]string{p.ProjectBase + "/api/", p.ProjectBase + "/assets/", p.ProjectBase + "/posts/", p.ProjectBase + "/_next/"}, site.ProtectedPrefixes...)
		for _, v := range protected {
			if Overlap(pre, v) {
				return model.Err("PAGE_ROUTE_PROTECTED", "资源范围与受保护的业务路径重叠")
			}
		}
		for j, q := range p.ResourcePrefixes {
			if i != j && Overlap(pre, q) {
				return model.Err("ROUTE_CONFLICT", "资源前缀重复或相互包含")
			}
		}
		for _, r := range s.Reserved[p.SiteID] {
			if r.ProjectID != p.ID && Overlap(pre, r.Prefix) {
				return model.Err("ROUTE_CONFLICT", "路径尚被已删除的映射保留，请先释放")
			}
		}
		for _, q := range append(append([]model.Project{}, s.Projects...), s.Effective...) {
			if q.ID == p.ID || q.SiteID != p.SiteID {
				continue
			}
			for _, v := range q.ResourcePrefixes {
				if Overlap(pre, v) {
					return model.Err("ROUTE_CONFLICT", "资源前缀已由其他项目保留")
				}
			}
		}
	}
	if !filepath.IsAbs(p.RootPath) || strings.HasPrefix(p.RootPath, `\\`) || strings.HasPrefix(p.RootPath, "//") || strings.ContainsAny(p.RootPath, "\r\n\x00") {
		return model.Err("PATH_OUTSIDE_ROOT", "资源目录必须为绝对路径")
	}
	allowed := false
	for _, r := range s.Roots {
		if Within(r, p.RootPath) {
			allowed = true
		}
	}
	if !allowed {
		return model.Err("PATH_OUTSIDE_ROOT", "请先明确授权此资源根目录")
	}
	return CheckDirectory(p.RootPath)
}
func Within(root, p string) bool {
	r, e := filepath.Rel(filepath.Clean(root), filepath.Clean(p))
	return e == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) && !filepath.IsAbs(r)
}
func CheckDirectory(p string) error {
	if err := NoLinks(p); err != nil {
		return err
	}
	v, e := os.Stat(p)
	if os.IsNotExist(e) {
		return model.Err("PATH_NOT_FOUND", "资源目录不存在")
	}
	if e != nil {
		return model.Err("ACCESS_DENIED", "资源目录不可读")
	}
	if !v.IsDir() {
		return model.Err("PATH_NOT_FOUND", "资源根必须是目录")
	}
	f, e := os.Open(p)
	if e != nil {
		return model.Err("ACCESS_DENIED", "服务账号无法读取目录")
	}
	f.Close()
	return nil
}
func NoLinks(p string) error {
	p = filepath.Clean(p)
	for {
		v, e := os.Lstat(p)
		if e != nil {
			if os.IsNotExist(e) {
				return model.Err("PATH_NOT_FOUND", "文件或目录不存在")
			}
			return model.Err("ACCESS_DENIED", "无法检查目录权限")
		}
		if v.Mode()&os.ModeSymlink != 0 || isReparse(p) {
			return model.Err("REPARSE_POINT_DENIED", "资源路径不允许符号链接、junction 或 reparse point")
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	return nil
}
func Relative(raw, base, prefix string) (string, error) {
	u, e := url.ParseRequestURI(raw)
	if e != nil {
		return "", model.Err("URL_AMBIGUOUS", "无效 URL")
	}
	encoded := u.EscapedPath()
	lower := strings.ToLower(encoded)
	for _, bad := range []string{"%2f", "%5c", "%00", "%25"} {
		if strings.Contains(lower, bad) {
			return "", model.Err("URL_AMBIGUOUS", "拒绝编码斜杠、重复编码或 NUL")
		}
	}
	p := u.Path
	if !strings.HasPrefix(p, prefix) || !strings.HasPrefix(p, base+"/") {
		return "", model.Err("PATH_OUTSIDE_ROOT", "请求未命中资源前缀")
	}
	rel := strings.TrimPrefix(p, base+"/")
	if rel == "" || strings.ContainsAny(rel, "\\\x00:") {
		return "", model.Err("URL_AMBIGUOUS", "不安全文件路径")
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || seg == "." || seg == ".." || strings.HasSuffix(seg, ".") || strings.HasSuffix(seg, " ") {
			return "", model.Err("URL_AMBIGUOUS", "不允许点段、空段或 Windows 尾部别名")
		}
		if strings.ContainsAny(seg, `<>"|?*`) {
			return "", model.Err("URL_AMBIGUOUS", "Windows 文件名含有非法字符")
		}
		name := strings.ToUpper(strings.Split(seg, ".")[0])
		if name == "CON" || name == "PRN" || name == "AUX" || name == "NUL" || name == "CLOCK$" || (len(name) == 4 && (strings.HasPrefix(name, "COM") || strings.HasPrefix(name, "LPT")) && name[3] >= '0' && name[3] <= '9') {
			return "", model.Err("URL_AMBIGUOUS", "不允许 Windows 设备文件名")
		}
		for _, r := range seg {
			if r < 32 || r == 127 {
				return "", model.Err("URL_AMBIGUOUS", "不允许控制字符")
			}
		}
	}
	return filepath.FromSlash(rel), nil
}
func File(root, rel string) (string, error) {
	p := filepath.Join(root, rel)
	if !Within(root, p) {
		return "", model.Err("PATH_OUTSIDE_ROOT", "文件超出资源目录")
	}
	if err := NoLinks(p); err != nil {
		return "", err
	}
	v, e := os.Stat(p)
	if e != nil || !v.Mode().IsRegular() {
		return "", model.Err("PATH_NOT_FOUND", "资源不存在或不是普通文件")
	}
	return p, nil
}
func URL(domain, base, rel string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i, v := range parts {
		parts[i] = url.PathEscape(v)
	}
	return fmt.Sprintf("https://%s%s/%s", domain, base, strings.Join(parts, "/"))
}
