package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "tuilor",
		Short: "Build terminal applications with AI and MCP",
	}

	version := &cobra.Command{
		Use:   "version",
		Short: "Display the Tuilor version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "Tuilor v0.1.0")
		},
	}

	root.AddCommand(version)

	return root

}
