package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lukeeskinner/tuilor/internal/mcpclient"
	"github.com/lukeeskinner/tuilor/internal/tui"
)

func NewRootCommand() *cobra.Command {
	var server, appsDir string
	root := &cobra.Command{
		Use:           "tuilor",
		Short:         "Build terminal applications with AI and MCP",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			model, err := tea.NewProgram(tui.NewModel(appsDir, server),
				tea.WithContext(ctx), tea.WithInput(cmd.InOrStdin()), tea.WithOutput(cmd.OutOrStdout()),
			).Run()
			if final, ok := model.(tui.Model); ok {
				err = errors.Join(err, final.Close())
			}
			return err
		},
	}
	root.Flags().StringVar(&server, "server", "", "Trusted local MCP executable for app validation")
	root.Flags().StringVar(&appsDir, "apps-dir", "apps", "Directory containing saved app JSON files")

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

	return root

}
