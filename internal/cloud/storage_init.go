package cloud

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"yandu/internal/pathpolicy"
)

// InitializeStorage uses Go's standard TLS/X.509 implementation so the cloud
// storage installer does not need a global Python or OpenSSL installation.
func InitializeStorage(dir, server string) error {
	if net.ParseIP(server) == nil && !pathpolicy.Domain(server) {
		return fmt.Errorf("invalid TLS server IP/DNS name")
	}
	for _, p := range []string{"tunnel.token", "frps.toml", "gateway.json", "tls/ca.key", "tls/ca.crt", "tls/server.key", "tls/server.crt"} {
		if _, e := os.Lstat(filepath.Join(dir, p)); e == nil || !os.IsNotExist(e) {
			return fmt.Errorf("storage TLS/config already exists; use maintenance without overwriting")
		}
	}
	if e := os.MkdirAll(filepath.Join(dir, "tls"), 0750); e != nil {
		return e
	}
	randomSerial := func() (*big.Int, error) { return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128)) }
	caKey, e := rsa.GenerateKey(rand.Reader, 3072)
	if e != nil {
		return e
	}
	serverKey, e := rsa.GenerateKey(rand.Reader, 3072)
	if e != nil {
		return e
	}
	serial, e := randomSerial()
	if e != nil {
		return e
	}
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Yandu Tunnel CA"}, NotBefore: now.Add(-5 * time.Minute), NotAfter: now.AddDate(10, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	caDER, e := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if e != nil {
		return e
	}
	serial, e = randomSerial()
	if e != nil {
		return e
	}
	leaf := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: server}, NotBefore: now.Add(-5 * time.Minute), NotAfter: now.Add(365 * 24 * time.Hour), BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	if ip := net.ParseIP(server); ip != nil {
		leaf.IPAddresses = []net.IP{ip}
	} else {
		leaf.DNSNames = []string{server}
	}
	leafDER, e := x509.CreateCertificate(rand.Reader, leaf, ca, &serverKey.PublicKey, caKey)
	if e != nil {
		return e
	}
	for _, item := range []struct {
		name string
		key  *rsa.PrivateKey
		cert []byte
	}{{"ca", caKey, caDER}, {"server", serverKey, leafDER}} {
		key, e := x509.MarshalPKCS8PrivateKey(item.key)
		if e != nil {
			return e
		}
		if e = Atomic(filepath.Join(dir, "tls", item.name+".key"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600); e != nil {
			return e
		}
		if e = Atomic(filepath.Join(dir, "tls", item.name+".crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: item.cert}), 0644); e != nil {
			return e
		}
	}
	bytes := make([]byte, 32)
	if _, e = rand.Read(bytes); e != nil {
		return e
	}
	token := hex.EncodeToString(bytes)
	if e = Atomic(filepath.Join(dir, "tunnel.token"), []byte(token+"\n"), 0600); e != nil {
		return e
	}
	config := `bindAddr = "0.0.0.0"
bindPort = 7000
proxyBindAddr = "127.0.0.1"
allowPorts = [{ single = 19090 }]
auth.method = "token"
auth.token = ` + strconv.Quote(token) + `
transport.tls.force = true
transport.tls.certFile = ` + strconv.Quote(filepath.Join(dir, "tls/server.crt")) + `
transport.tls.keyFile = ` + strconv.Quote(filepath.Join(dir, "tls/server.key")) + `
log.to = "console"
log.level = "warn"
`
	if e = Atomic(filepath.Join(dir, "frps.toml"), []byte(config), 0600); e != nil {
		return e
	}
	gateway, _ := json.Marshal(GatewayConfig{Listen: "127.0.0.1:19091", Upstream: "http://127.0.0.1:19090", AllowedCIDRs: []string{"127.0.0.0/8", "::1/128"}})
	return Atomic(filepath.Join(dir, "gateway.json"), gateway, 0644)
}
