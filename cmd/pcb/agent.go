package main

import (
	"github.com/spf13/cobra"

	"github.com/boytegar/packboy-builder/internal/app"
)

var agentRunOpts app.AgentRunOptions

func init() {
	agentRunCmd.Flags().StringVar(&agentRunOpts.Name, "name", "", "Optional agent name")
	agentRunCmd.Flags().StringVar(&agentRunOpts.Prompt, "prompt", "", "Task prompt")
	agentRunCmd.Flags().StringVar(&agentRunOpts.Model, "model", "", "Model override")
	agentRunCmd.Flags().IntVar(&agentRunOpts.MaxSteps, "max-steps", 0, "Maximum LLM inference steps (0 = unlimited)")

	agentCmd.AddCommand(agentRunCmd)
	rootCmd.AddCommand(agentCmd)
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent management commands",
}

var agentRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a headless agent",
	Long: `Run an agent in headless mode without TUI.

Example:
  pcb agent run --prompt "find main.go"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return app.RunAgent(agentRunOpts)
	},
}
