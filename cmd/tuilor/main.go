package main

import (
	"fmt"
	"github.com/lukeeskinner/tuilor/internal/cli"
	"os"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
