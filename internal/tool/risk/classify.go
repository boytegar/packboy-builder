// Package risk classifies shell commands by their plausible real-world
// consequences, so the permission system and the agent can reason about
// the severity, scope, reversibility, and security boundaries of each
// command before executing it.
//
// The classifier uses pattern matching on the command text. It is
// intentionally conservative: commands that are ambiguous or whose
// effects depend on runtime state are classified at a higher risk level
// than a static analysis would suggest.
package risk

import (
	"strings"
)

// Level represents the severity of a command's plausible consequences.
type Level string

const (
	// Low — strictly read-only, no side effects, no disclosure or security impact.
	// Examples: pwd, ls, cat, git status, git log, git diff.
	LevelLow Level = "low"

	// Medium — bounded, understood effects that are straightforward to reverse.
	// Examples: touch, mkdir, mv, cp, npm install (modifies node_modules only).
	LevelMedium Level = "medium"

	// High — credible possibility of severe or hard-to-recover harm.
	// Examples: sudo, rm -rf, git push, git reset --hard, curl | bash.
	LevelHigh Level = "high"
)

// Classification is the result of classifying a command.
type Classification struct {
	Level   Level
	Reason  string
	Summary string
}

// Classify returns a risk classification for the given command.
//
// The rules are ordered by severity: the first matching HIGH rule wins,
// then LOW (for known safe patterns), then MEDIUM. If no rule matches,
// the default is MEDIUM (bounded, understood effects) — not LOW, because
// an unrecognized command could have side effects.
func Classify(command string) Classification {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return Classification{
			Level:   LevelLow,
			Reason:  "Empty command.",
			Summary: "No operation",
		}
	}

	// Check HIGH risk patterns first
	if h, ok := matchHigh(cmd); ok {
		return h
	}

	// Check LOW risk patterns (known safe commands before medium catch-all)
	if l, ok := matchLow(cmd); ok {
		return l
	}

	// Check MEDIUM risk patterns
	if m, ok := matchMedium(cmd); ok {
		return m
	}

	// Default: medium — unknown command, could have side effects
	return Classification{
		Level:   LevelMedium,
		Reason:  "Unrecognized command; effects are not statically determined. Treated as bounded, potentially reversible.",
		Summary: "Unknown command",
	}
}

// matchHigh checks for HIGH-risk command patterns.
func matchHigh(cmd string) (Classification, bool) {
	lower := strings.ToLower(cmd)

	// sudo — elevated privileges that can modify system files or configurations
	if hasWord(lower, "sudo") || strings.HasPrefix(lower, "sudo ") {
		return Classification{
			Level:   LevelHigh,
			Reason:  "Uses sudo or elevated privileges that can modify system files or configurations.",
			Summary: "Elevated privileges",
		}, true
	}

	// Broad or uncertain deletion: rm -rf, git clean, find ... -delete
	if matchAny(lower, []string{
		"rm -rf", "rm -fr", "rm -r -f",
	}) || (strings.HasPrefix(lower, "rm ") && strings.Contains(lower, "-r") && strings.Contains(lower, "-f")) {
		return Classification{
			Level:   LevelHigh,
			Reason:  "Broad or uncertain deletion with rm -rf or similar patterns that can permanently delete files.",
			Summary: "Destructive deletion",
		}, true
	}

	if hasWord(lower, "git clean") && (strings.Contains(lower, "-f") || strings.Contains(lower, "-fd") || strings.Contains(lower, "-df")) {
		return Classification{
			Level:   LevelHigh,
			Reason:  "git clean can permanently delete untracked files whose contents and recoverability have not been established.",
			Summary: "Destructive git clean",
		}, true
	}

	if strings.Contains(lower, "find ") && strings.Contains(lower, "-delete") {
		return Classification{
			Level:   LevelHigh,
			Reason:  "find ... -delete may delete files whose recoverability has not been established.",
			Summary: "Destructive find -delete",
		}, true
	}

	// git push — modifies the remote repository
	if hasWord(lower, "git push") {
		if strings.Contains(lower, "--force") || strings.Contains(lower, " -f ") {
			return Classification{
				Level:   LevelHigh,
				Reason:  "git push --force can overwrite remote repository history.",
				Summary: "Force push",
			}, true
		}
		return Classification{
			Level:   LevelHigh,
			Reason:  "git push publishes commits to a remote where sensitive content may be difficult to fully remove.",
			Summary: "Push to remote",
		}, true
	}

	// git reset --hard, git stash drop/clear — discards uncommitted work
	if matchAny(lower, []string{
		"git reset --hard", "git stash drop", "git stash clear",
	}) {
		return Classification{
			Level:   LevelHigh,
			Reason:  "Discards or overwrites uncommitted or untracked changes, which are irreversibly destroyed.",
			Summary: "Destructive git operation",
		}, true
	}

	// git checkout / git switch / git restore over dirty paths — overwrites unstaged edits
	if (hasWord(lower, "git checkout") || hasWord(lower, "git switch") || hasWord(lower, "git restore")) &&
		(strings.Contains(lower, "--theirs") || strings.Contains(lower, "--ours") && false) {
		// Conservative: checkout/switch/restore are only HIGH when they might overwrite
		// uncommitted work. We can't reliably detect this from the command text alone
		// without checking the working tree state, so we don't block here.
	}

	// curl | bash / wget | bash / sh -c of remote content
	if (strings.Contains(lower, "curl ") || strings.Contains(lower, "wget ")) &&
		(strings.Contains(lower, "bash") || strings.Contains(lower, "| sh") || strings.Contains(lower, "|sh")) {
		return Classification{
			Level:   LevelHigh,
			Reason:  "Pipes remote content into a shell, executing untrusted code from the internet.",
			Summary: "Untrusted code execution",
		}, true
	}

	// Database resets / destructive migrations
	if matchAny(lower, []string{
		"drop database", "drop table", "truncate table",
		"db reset", "database reset", "sequelize.sync({force:true})",
	}) {
		return Classification{
			Level:   LevelHigh,
			Reason:  "Irreversible action to production deployments, database migrations, or sensitive operations.",
			Summary: "Destructive database operation",
		}, true
	}

	// Production deployments or environment resets
	if matchAny(lower, []string{
		"deploy ", "publish ", "release ",
	}) && matchAny(lower, []string{
		"--prod", "--production", "production",
	}) {
		return Classification{
			Level:   LevelHigh,
			Reason:  "Irreversible action to production deployments.",
			Summary: "Production deployment",
		}, true
	}

	// dd — disk operations
	if hasWord(lower, "dd") {
		return Classification{
			Level:   LevelHigh,
			Reason:  "dd can perform irreversible low-level disk operations.",
			Summary: "Disk operation",
		}, true
	}

	// mkfs — filesystem format
	if matchAny(lower, []string{"mkfs", "fdisk", "parted"}) {
		return Classification{
			Level:   LevelHigh,
			Reason:  "Filesystem formatting or partitioning can destroy all data on the target.",
			Summary: "Filesystem operation",
		}, true
	}

	// chmod 777 / chmod -R
	if strings.Contains(lower, "chmod ") {
		if strings.Contains(lower, "777") || strings.Contains(lower, "-r") {
			return Classification{
				Level:   LevelHigh,
				Reason:  "chmod with world-writable (777) or recursive flags can weaken file permissions broadly.",
				Summary: "Permission weakening",
			}, true
		}
	}

	// Exposing ports / modifying firewall
	if matchAny(lower, []string{"iptables ", "ufw ", "firewall-cmd "}) {
		return Classification{
			Level:   LevelHigh,
			Reason:  "Modifies firewall rules, potentially allowing external access.",
			Summary: "Firewall modification",
		}, true
	}

	// Docker volume/database resets
	if strings.Contains(lower, "docker volume rm") || strings.Contains(lower, "docker volume prune") {
		return Classification{
			Level:   LevelHigh,
			Reason:  "Docker volume deletion permanently destroys non-ephemeral development data.",
			Summary: "Docker volume deletion",
		}, true
	}

	return Classification{}, false
}

// matchMedium checks for MEDIUM-risk command patterns.
func matchMedium(cmd string) (Classification, bool) {
	lower := strings.ToLower(cmd)

	// Package installs from trusted sources
	if matchAny(lower, []string{"npm install", "npm ci", "npm i ", "pip install", "pip3 install", "yarn install", "yarn add", "pnpm install", "bun install", "go mod download", "go get "}) {
		return Classification{
			Level:   LevelMedium,
			Reason:  "Installing packages from a trusted source; modifies the project directory.",
			Summary: "Package install",
		}, true
	}

	// File creation/modification in non-system directories
	if matchAny(lower, []string{"touch ", "mkdir ", "mv ", "cp ", "ln -s"}) {
		return Classification{
			Level:   LevelMedium,
			Reason:  "File or directory creation/modification; bounded, straightforward to reverse.",
			Summary: "File operation",
		}, true
	}

	// Git operations that modify local state
	if matchAny(lower, []string{"git add", "git commit", "git merge", "git rebase", "git cherry-pick", "git tag", "git stash"}) {
		return Classification{
			Level:   LevelMedium,
			Reason:  "Git operation that modifies local repository state; reversible with subsequent git commands.",
			Summary: "Git operation",
		}, true
	}

	// Trusted local scripts / builds
	if matchAny(lower, []string{"make ", "npm run build", "npm run ", "yarn build", "go build", "go test", "go vet", "gofmt -w", "goimports -w", "tsc ", "webpack "}) {
		return Classification{
			Level:   LevelMedium,
			Reason:  "Running trusted local or repository scripts and building code; concrete effects do not meet a HIGH risk criterion.",
			Summary: "Build/script",
		}, true
	}

	// Authenticated network requests to trusted endpoints
	if matchAny(lower, []string{"curl ", "wget ", "gh pr ", "gh issue "}) && !strings.Contains(lower, "bash") {
		return Classification{
			Level:   LevelMedium,
			Reason:  "Authenticated network request to a known endpoint; uses an existing credential as intended without printing or changing it.",
			Summary: "Network request",
		}, true
	}

	// Docker run / compose
	if matchAny(lower, []string{"docker run ", "docker-compose ", "docker compose "}) {
		return Classification{
			Level:   LevelMedium,
			Reason:  "Running trusted local or repository scripts and building code; concrete effects do not meet a HIGH risk criterion.",
			Summary: "Docker run",
		}, true
	}

	// File writes via redirect (not to system dirs)
	if strings.Contains(lower, ">") && !strings.Contains(lower, "/etc/") && !strings.Contains(lower, "/usr/") && !strings.Contains(lower, "/var/") {
		return Classification{
			Level:   LevelMedium,
			Reason:  "File write via redirect; bounded, straightforward to reverse.",
			Summary: "File redirect",
		}, true
	}

	return Classification{}, false
}

// matchLow checks for LOW-risk command patterns.
func matchLow(cmd string) (Classification, bool) {
	lower := strings.ToLower(cmd)

	// Read-only display commands (echo/printf without redirect, pwd)
	if (hasWord(lower, "echo") || hasWord(lower, "printf")) && !strings.Contains(lower, ">") && !strings.Contains(lower, ">>") {
		return Classification{
			Level:   LevelLow,
			Reason:  "Display command only; no meaningful side effects.",
			Summary: "Display",
		}, true
	}

	if hasWord(lower, "pwd") {
		return Classification{
			Level:   LevelLow,
			Reason:  "Display command only; no meaningful side effects.",
			Summary: "Display",
		}, true
	}

	// Read-only directory listing
	if hasWord(lower, "ls") {
		return Classification{
			Level:   LevelLow,
			Reason:  "Read-only directory listing; no modifications.",
			Summary: "Directory listing",
		}, true
	}

	if hasWord(lower, "find") && !strings.Contains(lower, "-delete") && !strings.Contains(lower, "-exec") {
		return Classification{
			Level:   LevelLow,
			Reason:  "Read-only file search (find without -delete/-exec); no modifications.",
			Summary: "File search",
		}, true
	}

	// Read-only file reading
	if hasWord(lower, "cat") || hasWord(lower, "head") || hasWord(lower, "tail") ||
		hasWord(lower, "less") || hasWord(lower, "more") || hasWord(lower, "wc") ||
		hasWord(lower, "file") || hasWord(lower, "stat") || hasWord(lower, "du") || hasWord(lower, "df") {
		return Classification{
			Level:   LevelLow,
			Reason:  "Read-only file inspection; no modifications.",
			Summary: "File inspection",
		}, true
	}

	// Git read operations
	if matchAny(lower, []string{"git status", "git log", "git diff", "git show", "git branch", "git remote -v", "git blame", "git ls-files", "git rev-parse"}) {
		return Classification{
			Level:   LevelLow,
			Reason:  "Read-only git operation; no modifications.",
			Summary: "Git read",
		}, true
	}

	// Process/system information
	if hasWord(lower, "whoami") || hasWord(lower, "date") || hasWord(lower, "uname") ||
		hasWord(lower, "ps") || hasWord(lower, "top") || hasWord(lower, "hostname") ||
		hasWord(lower, "env") || hasWord(lower, "printenv") ||
		(strings.Contains(lower, "systemctl") && strings.Contains(lower, "status")) {
		return Classification{
			Level:   LevelLow,
			Reason:  "Information gathering; no meaningful side effects or disclosure.",
			Summary: "System info",
		}, true
	}

	// Grep/ripgrep/awk/sed without -i
	if hasWord(lower, "grep") || hasWord(lower, "rg") || hasWord(lower, "ag") || hasWord(lower, "ack") {
		return Classification{
			Level:   LevelLow,
			Reason:  "Read-only text search; no modifications.",
			Summary: "Text search",
		}, true
	}

	// sed without -i (in-place) is read-only
	if hasWord(lower, "sed") && !strings.Contains(lower, " -i") && !strings.Contains(lower, "--in-place") {
		return Classification{
			Level:   LevelLow,
			Reason:  "Read-only text processing (sed without -i); no modifications.",
			Summary: "Text processing",
		}, true
	}

	// awk is read-only by default
	if hasWord(lower, "awk") {
		return Classification{
			Level:   LevelLow,
			Reason:  "Read-only text processing (awk); no modifications.",
			Summary: "Text processing",
		}, true
	}

	// Tree
	if hasWord(lower, "tree") {
		return Classification{
			Level:   LevelLow,
			Reason:  "Read-only directory tree; no modifications.",
			Summary: "Directory tree",
		}, true
	}

	// which / type / command -v
	if hasWord(lower, "which") || hasWord(lower, "type") || strings.Contains(lower, "command -v ") {
		return Classification{
			Level:   LevelLow,
			Reason:  "Read-only command lookup; no modifications.",
			Summary: "Command lookup",
		}, true
	}

	// go test (read-only — running tests)
	if hasWord(lower, "go test") {
		return Classification{
			Level:   LevelLow,
			Reason:  "Go test is a read-only test execution, no file modifications.",
			Summary: "Test execution",
		}, true
	}

	return Classification{}, false
}

// hasWord checks if a word appears as a whole word in the command.
func hasWord(s, word string) bool {
	if s == "" || word == "" {
		return false
	}
	idx := strings.Index(s, word)
	if idx < 0 {
		return false
	}
	// Check word boundary before
	if idx > 0 {
		c := s[idx-1]
		if !isDelimiter(c) {
			return false
		}
	}
	// Check word boundary after
	afterIdx := idx + len(word)
	if afterIdx < len(s) {
		c := s[afterIdx]
		if !isDelimiter(c) {
			return false
		}
	}
	return true
}

func isDelimiter(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ';' || c == '|' || c == '&' || c == '(' || c == ')' || c == '<' || c == '>'
}

func matchAny(s string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
