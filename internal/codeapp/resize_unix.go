//go:build unix

package codeapp

import (
	"os"
	"os/signal"
	"syscall"
)

func forwardResize(process *os.Process) func() {
	changes := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(changes, syscall.SIGWINCH)
	go func() {
		for {
			select {
			case <-changes:
				_ = process.Signal(syscall.SIGWINCH)
			case <-done:
				return
			}
		}
	}()
	return func() { signal.Stop(changes); close(done) }
}
