//go:build !windows

package main

import (
	"fmt"
	"syscall"
)

const targetOpenFilesLimit uint64 = 8192

func ensureWorkerFileLimit() (string, error) {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return "", err
	}

	previous := limit.Cur
	target := targetOpenFilesLimit
	if limit.Max > 0 && target > limit.Max {
		target = limit.Max
	}
	if target <= limit.Cur {
		return "", nil
	}

	limit.Cur = target
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return "", err
	}
	return fmt.Sprintf("file descriptor limit raised: %d -> %d", previous, target), nil
}
