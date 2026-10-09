//go:build !windows

package pathpolicy

func isReparse(p string) bool { return false }
