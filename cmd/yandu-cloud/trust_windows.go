//go:build windows

package main

import "yandu/internal/model"

func trusted(path string) error { return model.Err("UNSUPPORTED_PLATFORM", "云端助手需要 Linux") }
