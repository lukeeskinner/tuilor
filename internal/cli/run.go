package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/lukeeskinner/tuilor/internal/codeapp"
	"github.com/lukeeskinner/tuilor/internal/localapp"
	"github.com/lukeeskinner/tuilor/internal/tui"
	"github.com/spf13/cobra"
)

func newRunCommand() *cobra.Command {
	var directory string
	cmd := &cobra.Command{Use: "run <app-id>", Short: "Open a saved application without AI", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,99}$`).MatchString(args[0]) {
			return fmt.Errorf("provide an app ID, not a file path")
		}
		dir := filepath.Join(directory, args[0])
		manifest := filepath.Join(dir, "manifest.json")
		info, err := os.Lstat(manifest)
		if err != nil {
			return fmt.Errorf("find saved app: %w", err)
		}
		if !info.Mode().IsRegular() || info.Size() > 1024*1024 {
			return fmt.Errorf("app manifest must be a regular file up to 1 MiB")
		}
		raw, err := os.ReadFile(manifest)
		if err != nil {
			return fmt.Errorf("find saved app: %w", err)
		}
		var header struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(raw, &header); err != nil {
			return fmt.Errorf("read manifest: %w", err)
		}
		if header.Kind == "declarative" {
			bundle, err := localapp.Read(cmd.Context(), dir)
			if err != nil {
				return err
			}
			return tui.RunLocal(cmd.Context(), bundle, cmd.InOrStdin(), cmd.OutOrStdout())
		}
		if header.Kind != "go" {
			return fmt.Errorf("unsupported saved app format")
		}
		project, err := codeapp.Open(cmd.Context(), dir)
		if err != nil {
			return err
		}
		approved, err := tui.ConfirmGoApp(cmd.Context(), project.Spec.Name, cmd.InOrStdin(), cmd.OutOrStdout())
		if err != nil || !approved {
			return err
		}
		if !project.Ready() {
			ctx, cancel := context.WithTimeout(cmd.Context(), 120*time.Second)
			defer cancel()
			tool, err := codeapp.Preflight(ctx)
			if err != nil {
				return err
			}
			if err := project.Build(ctx, tool); err != nil {
				return err
			}
		}
		return project.Run(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
	}}
	cmd.Flags().StringVar(&directory, "apps-dir", "apps", "Directory containing saved applications")
	return cmd
}
