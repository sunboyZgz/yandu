//go:build !windows

package main

import (
	"os"
	"syscall"
	"yandu/internal/model"
)

func trusted(path string) error {
	v, e := os.Stat(path)
	if e != nil {
		return e
	}
	st, ok := v.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != 0 || v.Mode().Perm()&0022 != 0 {
		return model.Err("CLOUD_AUTH_DENIED", "助手及注册表必须归 root 所有且不可被普通账号写入")
	}
	return nil
}
