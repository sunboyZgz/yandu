//go:build !windows

package service

import "context"

func Dispatch(run func(context.Context) error) (bool, error) { return false, nil }
