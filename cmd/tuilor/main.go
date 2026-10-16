package main

import (
	"context"
	"fmt"
	"github.com/lukeeskinner/tuilor/internal/cli"
	"os"
	"os/signal"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := cli.NewRootCommand().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, cli.EscapeTerminalControls(err.Error()))
		os.Exit(1)
	}
}
