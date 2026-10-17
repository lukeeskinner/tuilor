//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package filelock

import (
	"golang.org/x/sys/unix"
	"os"
)

func Lock(f *os.File) error   { return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) }
func Unlock(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_UN) }
