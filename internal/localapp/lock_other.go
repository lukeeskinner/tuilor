//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd && !dragonfly && !windows

package localapp

import (
	"fmt"
	"os"
)

func lockFile(*os.File) error {
	return fmt.Errorf("local app file locking is unsupported on this platform")
}
func unlockFile(*os.File) error { return nil }
