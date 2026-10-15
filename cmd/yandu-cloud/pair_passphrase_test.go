package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"yandu/internal/model"
	"yandu/internal/secrets"
)

func TestInteractivePairingRetriesWeakPassphraseAndClearsInput(t *testing.T) {
	inputs := [][]byte{[]byte("short"), []byte("valid-pair-passphrase")}
	count := 0
	var output bytes.Buffer
	bundle, e := encryptWithPassphrase(model.Profile{}, true, func() ([]byte, error) {
		if count >= len(inputs) {
			t.Fatal("unexpected additional prompt")
		}
		pass := inputs[count]
		count++
		return pass, nil
	}, &output)
	if e != nil || len(bundle) == 0 || count != 2 {
		t.Fatalf("interactive pairing failed to retry: %v, reads %d", e, count)
	}
	if !strings.Contains(output.String(), "请重新输入") || strings.Contains(output.String(), "short") || strings.Contains(output.String(), "valid-pair-passphrase") {
		t.Fatal("retry prompt missing or leaked passphrase")
	}
	for _, pass := range inputs {
		if !bytes.Equal(pass, make([]byte, len(pass))) {
			t.Fatal("passphrase input was not cleared")
		}
	}
	// Decryption reaches profile validation only after authenticated decryption.
	_, e = secrets.Decrypt(bundle, "valid-pair-passphrase")
	var me *model.Error
	if !errors.As(e, &me) || me.Code != "INVALID_PROFILE" || me.Message == "口令错误或连接包已损坏" {
		t.Fatalf("encrypted with the wrong passphrase: %v", e)
	}
}

func TestPipedPairingDoesNotRetryWeakInput(t *testing.T) {
	reads := 0
	_, e := encryptWithPassphrase(model.Profile{}, false, func() ([]byte, error) {
		reads++
		if reads > 1 {
			t.Fatal("noninteractive input read more than once")
		}
		return []byte("short"), nil
	}, io.Discard)
	var me *model.Error
	if !errors.As(e, &me) || me.Code != "WEAK_PASSPHRASE" {
		t.Fatalf("weak noninteractive input not rejected: %v", e)
	}
}

func TestPairingReadFailureDoesNotRetry(t *testing.T) {
	reads := 0
	_, e := encryptWithPassphrase(model.Profile{}, true, func() ([]byte, error) {
		reads++
		if reads > 1 {
			t.Fatal("input failure retried")
		}
		return nil, io.EOF
	}, io.Discard)
	if !errors.Is(e, io.EOF) {
		t.Fatalf("input failure hidden: %v", e)
	}
}
