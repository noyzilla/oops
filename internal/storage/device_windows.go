//go:build windows

package storage

import "fmt"

func deviceOf(path string) (uint64, error) {
	return 0, fmt.Errorf("device lookup unsupported on windows")
}
