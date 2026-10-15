package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
	"yandu/internal/model"
	"yandu/internal/secrets"
)

func encryptPairProfile(p model.Profile) ([]byte, error) {
	fd := int(os.Stdin.Fd())
	interactive := term.IsTerminal(fd)
	reader := bufio.NewReader(os.Stdin)
	read := func() ([]byte, error) {
		if interactive {
			pass, e := term.ReadPassword(fd)
			fmt.Fprintln(os.Stderr)
			return pass, e
		}
		line, e := reader.ReadString('\n')
		if e == io.EOF {
			e = nil
		}
		return []byte(strings.TrimSpace(line)), e
	}
	return encryptWithPassphrase(p, interactive, read, os.Stderr)
}

func encryptWithPassphrase(p model.Profile, interactive bool, read func() ([]byte, error), out io.Writer) ([]byte, error) {
	for {
		fmt.Fprint(out, "连接包口令（至少 12 字符，不回显）：")
		pass, e := read()
		if e != nil {
			clear(pass)
			return nil, e
		}
		bundle, e := secrets.Encrypt(p, string(pass))
		clear(pass)
		var me *model.Error
		if interactive && errors.As(e, &me) && me.Code == "WEAK_PASSPHRASE" {
			fmt.Fprintln(out, "口令不足 12 个字符，请重新输入。")
			continue
		}
		// Piped input fails once instead of looping forever at EOF.
		return bundle, e
	}
}
