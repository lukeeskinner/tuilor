package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lukeeskinner/tuilor/internal/mcpclient"
	"github.com/lukeeskinner/tuilor/internal/tui"
)

func NewRootCommand() *cobra.Command {
	var appsDir string
	root := &cobra.Command{
		Use:           "tuilor",
		Short:         "Turn an idea into a reusable terminal application",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) (runErr error) {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			workspace, err := os.MkdirTemp("", "tuilor-session-")
			if err != nil {
				return fmt.Errorf("create project workspace: %w", err)
			}
			defer func() { runErr = errors.Join(runErr, os.RemoveAll(workspace)) }()
			current := tui.NewModel(appsDir).WithWorkspace(workspace)
			for {
				model, err := tea.NewProgram(current,
					tea.WithAltScreen(), tea.WithContext(ctx), tea.WithInput(cmd.InOrStdin()), tea.WithOutput(cmd.OutOrStdout()),
				).Run()
				final, ok := model.(tui.Model)
				if !ok {
					return err
				}
				if err = errors.Join(err, final.Close()); err != nil {
					return err
				}
				project := final.CodeLaunch()
				if project == nil {
					return nil
				}
				err = project.Run(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
				if ctx.Err() != nil {
					return ctx.Err()
				}
				current = final.CodeReturned(err)
			}
		},
	}
	root.Flags().StringVar(&appsDir, "apps-dir", "apps", "Directory containing saved applications")

	version := &cobra.Command{
		Use:   "version",
		Short: "Display the Tuilor version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "Tuilor v0.1.0")
		},
	}

	root.AddCommand(version)

	toolsCmd := &cobra.Command{
		Use:   "tools <server-executable>",
		Short: "List tools available from an MCP server",
		Args:  cobra.ExactArgs(1),

		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			tools, err := mcpclient.ListTools(ctx, args[0])
			if err != nil {
				return err
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Available MCP Tools:"); err != nil {
				return err
			}

			for _, tool := range tools {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n%s\n",
					EscapeTerminalControls(tool.Name), EscapeTerminalControls(tool.Description),
					EscapeTerminalControls(string(tool.InputSchema))); err != nil {
					return err
				}
			}

			return nil
		},
	}

	root.AddCommand(toolsCmd)
	root.AddCommand(newCreateCommand())
	root.AddCommand(newRunCommand())

	return root

}
