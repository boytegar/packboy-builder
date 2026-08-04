package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/boytegar/packboy-builder/internal/app"
	"github.com/boytegar/packboy-builder/internal/log"
	"github.com/boytegar/packboy-builder/internal/setting"

	// Import providers for registration
	_ "github.com/boytegar/packboy-builder/internal/llm/agnesai"
	_ "github.com/boytegar/packboy-builder/internal/llm/alibaba"
	_ "github.com/boytegar/packboy-builder/internal/llm/anthropic"
	_ "github.com/boytegar/packboy-builder/internal/llm/bigmodel"
	_ "github.com/boytegar/packboy-builder/internal/llm/custom"
	_ "github.com/boytegar/packboy-builder/internal/llm/deepseek"
	_ "github.com/boytegar/packboy-builder/internal/llm/google"
	_ "github.com/boytegar/packboy-builder/internal/llm/mimo"
	_ "github.com/boytegar/packboy-builder/internal/llm/minmax"
	_ "github.com/boytegar/packboy-builder/internal/llm/moonshot"
	_ "github.com/boytegar/packboy-builder/internal/llm/ollama"
	_ "github.com/boytegar/packboy-builder/internal/llm/openai"
	_ "github.com/boytegar/packboy-builder/internal/llm/sensenova"
	_ "github.com/boytegar/packboy-builder/internal/llm/volcengine"
)

var version = "1.22.7"

// buildTime and commit are set at build time via -X ldflags.
// When built directly with go build (without ldflags), they remain empty.
var (
	buildTime string
	commit    string
)

// cliOpts holds all CLI flag values in one place.
var cliOpts struct {
	print  string // -p/--print: non-interactive print mode
	cont   bool   // --continue
	resume bool   // --resume

	pluginDir string
	persona   string // --persona: persona name to activate on startup
}

func init() {
	// Load .env file if it exists (silent fail if not found)
	_ = godotenv.Load()
	// Initialize logging (enabled via PCB_DEBUG=1)
	_ = log.Init()

	// Register flags
	rootCmd.Flags().StringVarP(&cliOpts.print, "print", "p", "", "Non-interactive print mode with prompt")
	rootCmd.Flags().BoolVarP(&cliOpts.cont, "continue", "c", false, "Resume the most recent session")
	rootCmd.Flags().BoolVarP(&cliOpts.resume, "resume", "r", false, "Select and resume a previous session")
	rootCmd.PersistentFlags().StringVar(&cliOpts.pluginDir, "plugin-dir", "", "Load plugins from a specific directory")
	rootCmd.Flags().StringVar(&cliOpts.persona, "persona", "", "Activate a persona on startup")

	// Register flags on version subcommand
	versionCmd.Flags().Bool("json", false, "Output version information in JSON format")

	// Register subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(helpCmd)
	rootCmd.SetHelpCommand(helpCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(updateCmd)
}

func main() {
	defer func() { _ = log.Sync() }()

	// Clean up any stale backup file from a previous self-update.
	// On Windows, os.Remove on a running executable's renamed backup
	// fails, so we clean it on the next launch instead.
	cleanupUpdateBackup()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "pcb [message]",
	Short: "Packboy Builder - fast, open agent harness for the terminal",
	Long: `Packboy Builder is a fast, open agent harness for the terminal.
Bring your model, tools, skills, plugins, MCP servers, and subagents.

Non-interactive mode:
  pcb -p "your prompt"     Print response and exit
  echo "msg" | pcb -p ""   Pipe stdin in print mode`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		printPrompt := cliOpts.print
		if printPrompt == "" {
			printPrompt = readStdin()
		}

		// When -r is used with an argument, treat it as a session ID
		var resumeID string
		if cliOpts.resume && len(args) > 0 {
			resumeID = args[0]
			args = args[1:]
		}

		prompt := strings.Join(args, " ")

		opts := setting.RunOptions{
			Print:     printPrompt,
			Prompt:    prompt,
			PluginDir: cliOpts.pluginDir,
			Persona:   cliOpts.persona,
			Continue:  cliOpts.cont,
			Resume:    cliOpts.resume,
			ResumeID:  resumeID,
		}
		if err := app.Run(opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// cleanupUpdateBackup removes any stale .bak file from a previous self-update.
// On Windows, the running process cannot delete the renamed backup of itself,
// so we defer cleanup to the next launch.
func cleanupUpdateBackup() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	backupPath := exe + ".bak"
	if _, err := os.Stat(backupPath); err == nil {
		_ = os.Remove(backupPath)
	}
}

// readStdin returns piped stdin data, or empty string if stdin is a terminal.
func readStdin() string {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		reader := bufio.NewReader(os.Stdin)
		data, err := io.ReadAll(reader)
		if err == nil && len(data) > 0 {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version and build information",
	Run: func(cmd *cobra.Command, args []string) {
		jsonOutput, _ := cmd.Flags().GetBool("json")

		if jsonOutput {
			info := map[string]string{
				"version":    version,
				"build_time": buildTime,
				"go_version": runtime.Version(),
				"commit":     commit,
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(info)
			return
		}

		fmt.Printf("pcb version %s\n", version)
		if buildTime != "" {
			fmt.Printf("build: %s\n", buildTime)
		}
		fmt.Printf("go:    %s\n", runtime.Version())
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for pcb updates and install if available",
	Long: `Check for available pcb version updates and install the latest version.

Checks the latest release on GitHub and upgrades the pcb binary if a newer
version is available.

The current installed version is read from the binary itself. If a newer
release is found, the binary is automatically downloaded and replaced.

Example:
  pcb update`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSelfUpdate(cmd.Context())
	},
}

var helpCmd = &cobra.Command{
	Use:   "help",
	Short: "Show help information",
	Long:  "Display help information about Packboy Builder and its commands.",
	Run: func(cmd *cobra.Command, args []string) {
		printHelp()
	},
}

func printHelp() {
	help := `
Packboy Builder - fast, open agent harness for the terminal

Usage:
  pcb                        Start interactive chat mode
  pcb "message"              Interactive mode with initial prompt
  pcb -p "prompt"            Non-interactive print mode
  pcb [command]              Run a command

Print Mode (non-interactive):
  pcb -p "your prompt"       Print response and exit
  echo "data" | pcb -p "analyze"  Pipe stdin with prompt

Interactive Mode:
  pcb                        Start chat
  pcb "Explain this code"    Start chat with initial prompt

Session:
  pcb -c, --continue         Resume the most recent session
  pcb -r, --resume           Select and resume a previous session
  pcb -r <session-id>        Resume a specific session by ID
  pcb --plugin-dir <path>    Load plugins from a specific directory
  pcb --persona <name>       Activate a persona on startup

Commands:
  version      Print version and build information
  agent run    Run a headless agent
  update       Check for pcb updates and install if available
  help         Show this help message

Keybindings:
  Enter        Send message
  Alt+Enter    Insert newline
  Up/Down      Navigate input history
  Esc          Stop AI response
  Ctrl+T       Toggle task list display
  Ctrl+C       Clear input / Quit

Slash Commands:
  /models      Select model and manage provider connections
  /clear       Clear chat history
  /help        Show help

Examples:
  pcb                        Start interactive chat
  pcb "Explain this code"    Interactive with initial prompt
  pcb -p "Explain this code" Print response and exit
  pcb -c                     Resume previous session
  pcb version                Show version

For more information, visit: https://github.com/boytegar/packboy-builder
`
	fmt.Println(help)
}
