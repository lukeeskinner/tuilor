//go:build !unix

package codeapp

import "os"

func forwardResize(process *os.Process) func() { return func() {} }
