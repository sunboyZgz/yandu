package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"yandu/internal/cloud"
	"yandu/internal/secrets"
)

func TestStoragePairDeliversMonitorCredentialAndRefreshesSameOwner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("storage-only cloud pairing targets Unix; local Windows inventory is tested by the native storage contract")
	}
	dir := t.TempDir()
	if e := os.Mkdir(filepath.Join(dir, "tls"), 0700); e != nil {
		t.Fatal(e)
	}
	pub, priv, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Yandu test CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, e := x509.CreateCertificate(rand.Reader, ca, ca, pub, priv)
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(dir, "tls/ca.crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600)
	os.WriteFile(filepath.Join(dir, "tunnel.token"), []byte("unchanged-tunnel-token-01234567890123456789"), 0600)
	passphrase := "monitor-test-passphrase"
	pair := func(out string, refresh bool) error {
		old := os.Stdin
		input, e := os.CreateTemp(dir, "stdin-")
		if e != nil {
			t.Fatal(e)
		}
		defer input.Close()
		input.WriteString(passphrase + "\n")
		input.Seek(0, 0)
		os.Stdin = input
		defer func() { os.Stdin = old }()
		return storagePair(dir, "owner", "127.0.0.1", "127.0.0.1", out, refresh)
	}
	out := filepath.Join(dir, "initial.yandu-profile")
	if e = pair(out, false); e != nil {
		t.Fatal(e)
	}
	c, e := cloud.LoadMonitorConfig(filepath.Join(dir, "monitor.json"))
	if e != nil {
		t.Fatal(e)
	}
	data, _ := os.ReadFile(out)
	p, e := secrets.Decrypt(data, passphrase)
	if e != nil {
		t.Fatal(e)
	}
	if p.Storage.MonitorToken != c.MonitorToken || c.DeviceID != "owner" {
		t.Fatal("device and cloud monitor pairing differ")
	}
	oldToken := c.MonitorToken
	if e = pair(filepath.Join(dir, "duplicate.yandu-profile"), false); e == nil {
		t.Fatal("second device pairing allowed")
	}
	if e = storagePair(dir, "other", "127.0.0.1", "127.0.0.1", filepath.Join(dir, "wrong.yandu-profile"), true); e == nil {
		t.Fatal("refresh can replace owner")
	}
	out = filepath.Join(dir, "refreshed.yandu-profile")
	if e = pair(out, true); e != nil {
		t.Fatal(e)
	}
	c, e = cloud.LoadMonitorConfig(filepath.Join(dir, "monitor.json"))
	if e != nil {
		t.Fatal(e)
	}
	data, _ = os.ReadFile(out)
	next, e := secrets.Decrypt(data, passphrase)
	if e != nil {
		t.Fatal(e)
	}
	if c.MonitorToken == oldToken || c.MonitorToken != next.Storage.MonitorToken || p.TunnelToken != next.TunnelToken || p.TunnelCA != next.TunnelCA {
		t.Fatal("monitor refresh changed tunnel or did not rotate monitor")
	}
}
