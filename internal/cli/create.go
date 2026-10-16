package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lukeeskinner/tuilor/internal/app"
	"github.com/lukeeskinner/tuilor/internal/generator"
	"github.com/lukeeskinner/tuilor/internal/localapp"
	"github.com/lukeeskinner/tuilor/internal/mcpclient"
	"github.com/spf13/cobra"
)

func newCreateCommand() *cobra.Command {
	var server, directory string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "create <description>",
		Short: "Generate and save a local app, or use an explicitly selected MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(args[0]) == "" || strings.TrimSpace(directory) == "" || (cmd.Flags().Changed("server") && strings.TrimSpace(server) == "") {
				return fmt.Errorf("description, --output-dir, and explicitly supplied --server must not be empty")
			}
			if timeout <= 0 {
				return fmt.Errorf("--timeout must be positive")
			}
			// Reject missing credentials before starting any subprocess.
			provider, err := generator.NewOpenAIFromEnv()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			var definition *app.Definition
			if server == "" {
				definition, err = generator.GenerateLocal(ctx, provider, args[0])
			} else {
				tools, err := mcpclient.ListTools(ctx, server)
				if err != nil {
					return err
				}
				capabilities := make([]app.Capability, 0, len(tools))
				for _, tool := range tools {
					capabilities = append(capabilities, app.Capability{
						Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema,
					})
				}
				definition, err = generator.Generate(ctx, provider, args[0], capabilities)
				if err != nil {
					return err
				}
			}
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("save generated app: %w", err)
			}
			var path string
			if definition.Version == 2 {
				store, newErr := localapp.New(definition)
				if newErr != nil {
					return newErr
				}
				saved, saveErr := store.Save(directory)
				if saveErr != nil {
					return saveErr
				}
				path = saved.Path
			} else {
				path, err = app.Save(definition, directory)
			}
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), EscapeTerminalControls(path))
			return err
		},
	}
	cmd.Flags().StringVar(&server, "server", "", "Trusted local MCP executable (launched directly, without a shell)")
	cmd.Flags().StringVar(&directory, "output-dir", "apps", "Directory for new generated JSON files")
	cmd.Flags().DurationVar(&timeout, "timeout", 90*time.Second, "Total discovery and generation deadline (cleanup may take longer)")
	return cmd
}
