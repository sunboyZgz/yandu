package cloud

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStorageTLSInitSANAndNoOverwrite(t *testing.T) {
	for _, server := range []string{"127.0.0.1", "storage.example.com"} {
		t.Run(server, func(t *testing.T) {
			dir := t.TempDir()
			if e := InitializeStorage(dir, server); e != nil {
				t.Fatal(e)
			}
			caData, _ := os.ReadFile(filepath.Join(dir, "tls/ca.crt"))
			block, _ := pem.Decode(caData)
			if block == nil {
				t.Fatal("missing CA PEM")
			}
			ca, e := x509.ParseCertificate(block.Bytes)
			if e != nil {
				t.Fatal(e)
			}
			pair, e := tls.LoadX509KeyPair(filepath.Join(dir, "tls/server.crt"), filepath.Join(dir, "tls/server.key"))
			if e != nil {
				t.Fatal(e)
			}
			leaf, e := x509.ParseCertificate(pair.Certificate[0])
			if e != nil {
				t.Fatal(e)
			}
			roots := x509.NewCertPool()
			roots.AddCert(ca)
			if _, e = leaf.Verify(x509.VerifyOptions{DNSName: server, Roots: roots}); e != nil {
				t.Fatal("SAN/chain validation", e)
			}
			if e = leaf.VerifyHostname("wrong.example.com"); e == nil {
				t.Fatal("wrong server accepted")
			}
			before, _ := os.ReadFile(filepath.Join(dir, "tunnel.token"))
			if e = InitializeStorage(dir, server); e == nil {
				t.Fatal("existing TLS overwritten")
			}
			after, _ := os.ReadFile(filepath.Join(dir, "tunnel.token"))
			if string(before) != string(after) {
				t.Fatal("token changed on repeat init")
			}
			if runtime.GOOS != "windows" {
				for _, name := range []string{"tunnel.token", "tls/ca.key", "tls/server.key"} {
					s, e := os.Stat(filepath.Join(dir, name))
					if e != nil || s.Mode().Perm() != 0600 {
						t.Fatal("private key/token mode incorrect")
					}
				}
			}
		})
	}
}
