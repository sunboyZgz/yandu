package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"golang.org/x/crypto/scrypt"
	"golang.org/x/crypto/ssh"
	"net"
	"strings"
	"yandu/internal/model"
	"yandu/internal/pathpolicy"
)

type Envelope struct {
	Version    int    `json:"version"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func Encrypt(p model.Profile, password string) ([]byte, error) {
	if len(password) < 12 {
		return nil, model.Err("WEAK_PASSPHRASE", "连接包口令至少 12 个字符")
	}
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key, e := scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
	if e != nil {
		return nil, e
	}
	block, _ := aes.NewCipher(key)
	g, _ := cipher.NewGCM(block)
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	plain, e := json.Marshal(p)
	if e != nil {
		return nil, e
	}
	ciphertext := g.Seal(nil, nonce, plain, []byte("yandu-profile-v1"))
	return json.Marshal(Envelope{1, base64.StdEncoding.EncodeToString(salt), base64.StdEncoding.EncodeToString(nonce), base64.StdEncoding.EncodeToString(ciphertext)})
}
func Decrypt(b []byte, password string) (model.Profile, error) {
	var p model.Profile
	var env Envelope
	if len(b) > 256*1024 || json.Unmarshal(b, &env) != nil || env.Version != 1 {
		return p, model.Err("INVALID_PROFILE", "连接包格式无效")
	}
	salt, e1 := base64.StdEncoding.DecodeString(env.Salt)
	nonce, e2 := base64.StdEncoding.DecodeString(env.Nonce)
	data, e3 := base64.StdEncoding.DecodeString(env.Ciphertext)
	if e1 != nil || e2 != nil || e3 != nil || len(salt) != 32 || len(nonce) != 12 {
		return p, model.Err("INVALID_PROFILE", "连接包格式无效")
	}
	key, e := scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
	if e != nil {
		return p, e
	}
	block, _ := aes.NewCipher(key)
	g, _ := cipher.NewGCM(block)
	plain, e := g.Open(nil, nonce, data, []byte("yandu-profile-v1"))
	if e != nil {
		return p, model.Err("INVALID_PROFILE", "口令错误或连接包已损坏")
	}
	if json.Unmarshal(plain, &p) != nil {
		return p, model.Err("INVALID_PROFILE", "连接数据无效")
	}
	return p, Validate(p)
}
func Validate(p model.Profile) error {
	if p.SchemaVersion != 1 || !pathpolicy.Identifier(p.DeviceID) || p.ServerPort < 1 || p.ServerPort > 65535 || len(p.TunnelToken) < 32 || !pathpolicy.Domain(p.TLSName) || !strings.HasPrefix(p.SSHFingerprint, "SHA256:") {
		return model.Err("INVALID_PROFILE", "连接材料不完整")
	}
	if _, _, e := net.SplitHostPort(p.SSHAddr); e != nil {
		return model.Err("INVALID_PROFILE", "SSH 地址必须包含端口")
	}
	if p.ServerAddr == "" || strings.ContainsAny(p.ServerAddr, "\r\n\x00") || !pathpolicy.Identifier(p.SSHUser) {
		return model.Err("INVALID_PROFILE", "连接地址无效")
	}
	if _, e := ssh.ParsePrivateKey([]byte(p.SSHPrivateKey)); e != nil {
		return model.Err("INVALID_PROFILE", "SSH 设备密钥无效")
	}
	block, _ := pem.Decode([]byte(p.TunnelCA))
	if block == nil {
		return model.Err("INVALID_PROFILE", "缺少隧道 CA")
	}
	cert, e := x509.ParseCertificate(block.Bytes)
	if e != nil || !cert.IsCA {
		return model.Err("INVALID_PROFILE", "隧道 CA 无效")
	}
	if len(p.Sites) < 1 || len(p.Sites) > 20 {
		return model.Err("INVALID_PROFILE", "站点数量无效")
	}
	seen := map[string]bool{}
	for _, s := range p.Sites {
		if !pathpolicy.Identifier(s.ID) || !pathpolicy.Domain(s.Domain) || seen[s.ID] || len(s.AllowedPrefixes) == 0 {
			return model.Err("INVALID_PROFILE", "站点授权无效")
		}
		seen[s.ID] = true
		for _, pre := range append(s.AllowedPrefixes, s.ProtectedPrefixes...) {
			if e := pathpolicy.Prefix(pre, true); e != nil {
				return e
			}
		}
	}
	return nil
}
