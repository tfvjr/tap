package cmd

import (
	"github.com/spf13/cobra"
	tapmcp "github.com/tfvjr/tap/internal/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server over stdio",
	Long:  "Launch a Model Context Protocol server that exposes tap's data to AI tools like Claude Code.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tapmcp.Serve(appStore)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
