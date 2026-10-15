package cli

import (
	"context"
	"fmt"
	"github.com/lukeeskinner/tuilor/internal/mcpclient"
	"time"

	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lukeeskinner/tuilor/internal/tui"
)

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "tuilor",
		Short: "Build terminal applications with AI and MCP",
		SilenceUsage: true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := tea.NewProgram(tui.NewModel()).Run()
			return err
		},
	}

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

			fmt.Fprintln(cmd.OutOrStdout(), "Available MCP Tools:")

			for _, tool := range tools {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n",
					tool.Name, tool.Description)
			}

			return nil
		},
	}

	root.AddCommand(toolsCmd)

	return root

}
