package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lukeeskinner/tuilor/internal/codeapp"
	"github.com/lukeeskinner/tuilor/internal/generator"
	"github.com/spf13/cobra"
)

func newCreateCommand() *cobra.Command {
	var directory string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "create <description>",
		Short: "Generate, build, and save a Go terminal app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (runErr error) {
			if strings.TrimSpace(args[0]) == "" || strings.TrimSpace(directory) == "" {
				return fmt.Errorf("description and --output-dir must not be empty")
			}
			if timeout <= 0 {
				return fmt.Errorf("--timeout must be positive")
			}
			provider, err := generator.NewOpenAIFromEnv()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			tool, err := codeapp.Preflight(ctx)
			if err != nil {
				return err
			}
			// Explain the trust boundary before the paid generation request. Opening the
			// saved app later runs its Go code with the user's OS permissions.
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Generating Go with %s (billable OpenAI request). Generated apps run with your OS permissions, without a sandbox.\n", EscapeTerminalControls(generator.ConfiguredModel())); err != nil {
				return err
			}
			spec, err := generator.GenerateCode(ctx, provider, args[0], nil, "")
			if err != nil {
				return err
			}
			workspace, err := os.MkdirTemp("", "tuilor-create-")
			if err != nil {
				return fmt.Errorf("create project workspace: %w", err)
			}
			defer func() { runErr = errors.Join(runErr, os.RemoveAll(workspace)) }()
			project, err := codeapp.Create(ctx, filepath.Join(workspace, "project"), spec)
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			cancel()
			buildCtx, cancelBuild := context.WithTimeout(cmd.Context(), 120*time.Second)
			defer cancelBuild()
			if err := project.Build(buildCtx, tool); err != nil {
				return err
			}
			saved, err := project.Save(buildCtx, directory)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), EscapeTerminalControls(saved.Dir))
			return err
		},
	}
	cmd.Flags().StringVar(&directory, "output-dir", "apps", "Directory for saved Go app projects")
	cmd.Flags().DurationVar(&timeout, "timeout", 90*time.Second, "Generation and Go preflight deadline; building has a separate 120-second deadline")
	return cmd
}
