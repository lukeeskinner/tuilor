//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd && !dragonfly && !windows

package filelock

import (
	"fmt"
	"os"
)

func Lock(*os.File) error {
	return fmt.Errorf("local app file locking is unsupported on this platform")
}
func Unlock(*os.File) error { return nil }
