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
	return newCreateWithProvider(func() (generator.Provider, error) { return generator.NewOpenAIFromEnv() })
}

func newCreateWithProvider(factory func() (generator.Provider, error)) *cobra.Command {
	var directory, name string
	var timeout time.Duration
	cmd := &cobra.Command{Use: "create <description>", Short: "Generate, build, and save a custom Go application", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) (runErr error) {
		if strings.TrimSpace(args[0]) == "" || strings.TrimSpace(directory) == "" {
			return fmt.Errorf("description and --output-dir must not be empty")
		}
		if timeout <= 0 {
			return fmt.Errorf("--timeout must be positive")
		}
		if err := codeapp.ValidateID(name); err != nil {
			return err
		}
		if name != "" {
			_, err := os.Lstat(filepath.Join(directory, name))
			if err == nil {
				return fmt.Errorf("app ID %q already exists", name)
			}
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("check app destination: %w", err)
			}
		}
		provider, err := factory()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		tool, err := codeapp.Preflight(ctx)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Generating Go with %s (billable OpenAI request). Saved apps run with your OS permissions, without a sandbox.\n", EscapeTerminalControls(generator.ConfiguredModel())); err != nil {
			return err
		}
		spec, err := generator.GenerateCode(ctx, provider, args[0], nil, "")
		if err != nil {
			return err
		}
		workspace, err := os.MkdirTemp("", "tuilor-create-")
		if err != nil {
			return err
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
		saved, err := project.SaveAs(buildCtx, directory, name)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Saved %s\nApp ID: %s\nRun: tuilor run %s --apps-dir %q\n", EscapeTerminalControls(saved.Dir), EscapeTerminalControls(filepath.Base(saved.Dir)), EscapeTerminalControls(filepath.Base(saved.Dir)), directory)
		return err
	}}
	cmd.Flags().StringVar(&directory, "output-dir", "apps", "Directory for saved applications")
	cmd.Flags().StringVar(&name, "name", "", "Stable app ID (rejects an existing ID)")
	cmd.Flags().DurationVar(&timeout, "timeout", 90*time.Second, "Generation and Go preflight deadline; builds have a separate 120-second deadline")
	return cmd
}
