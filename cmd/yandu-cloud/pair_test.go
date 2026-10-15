package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"yandu/internal/cloud"
	"yandu/internal/model"
	"yandu/internal/secrets"
)

func storagePairFixture(t *testing.T) string {
	t.Helper()
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
	return dir
}

func withPairInput(t *testing.T, dir, text string, run func() error) error {
	t.Helper()
	old := os.Stdin
	input, e := os.CreateTemp(dir, "stdin-")
	if e != nil {
		t.Fatal(e)
	}
	defer input.Close()
	defer os.Remove(input.Name())
	if _, e = input.WriteString(text); e != nil {
		t.Fatal(e)
	}
	if _, e = input.Seek(0, 0); e != nil {
		t.Fatal(e)
	}
	os.Stdin = input
	defer func() { os.Stdin = old }()
	return run()
}

func TestStoragePairDeliversMonitorCredentialAndRefreshesSameOwner(t *testing.T) {
	dir := storagePairFixture(t)
	passphrase := "monitor-test-passphrase"
	pair := func(out string, refresh bool) error {
		return withPairInput(t, dir, passphrase+"\n", func() error {
			return storagePair(dir, "owner", "127.0.0.1", "127.0.0.1", out, refresh)
		})
	}
	out := filepath.Join(dir, "initial.yandu-profile")
	if e := pair(out, false); e != nil {
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

func TestWeakStoragePassphraseCanBeRetriedWithoutReinstall(t *testing.T) {
	dir := storagePairFixture(t)
	out := filepath.Join(dir, "owner.yandu-profile")
	beforeCA, _ := os.ReadFile(filepath.Join(dir, "tls/ca.crt"))
	beforeToken, _ := os.ReadFile(filepath.Join(dir, "tunnel.token"))
	args := []string{"--scope", "storage-only", "--dir", dir, "--device", "owner", "--server", "127.0.0.1", "--out", out}
	e := withPairInput(t, dir, "short\n", func() error { return pair(args) })
	var me *model.Error
	if !errors.As(e, &me) || me.Code != "WEAK_PASSPHRASE" {
		t.Fatalf("weak piped passphrase not rejected: %v", e)
	}
	for _, name := range []string{"owner.yandu-profile", "storage-owner.json", "monitor.json"} {
		if _, e := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(e) {
			t.Fatalf("weak passphrase wrote pairing state %s: %v", name, e)
		}
	}
	passphrase := "retry-pair-passphrase"
	if e = withPairInput(t, dir, passphrase+"\n", func() error { return pair(args) }); e != nil {
		t.Fatal("could not complete standalone pairing after weak passphrase:", e)
	}
	bundle, _ := os.ReadFile(out)
	p, e := secrets.Decrypt(bundle, passphrase)
	if e != nil || p.DeviceID != "owner" || p.Storage == nil {
		t.Fatalf("recovered profile invalid: %v", e)
	}
	afterCA, _ := os.ReadFile(filepath.Join(dir, "tls/ca.crt"))
	afterToken, _ := os.ReadFile(filepath.Join(dir, "tunnel.token"))
	if string(beforeCA) != string(afterCA) || string(beforeToken) != string(afterToken) {
		t.Fatal("standalone pairing changed existing tunnel material")
	}
}
