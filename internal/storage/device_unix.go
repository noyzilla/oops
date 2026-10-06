//go:build !windows

package storage

import (
	"fmt"
	"os"
	"syscall"
)

func deviceOf(path string) (uint64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("stat_t unavailable for %s", path)
	}
	return uint64(st.Dev), nil //nolint:unconvert // Dev width differs per platform
}
