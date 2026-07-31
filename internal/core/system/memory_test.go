package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveImports(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pcb-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mainContent := `# Main File
@imported.md
Some content after import`

	importedContent := `## Imported Content
This was imported from another file.`

	if err := os.WriteFile(filepath.Join(tmpDir, "main.md"), []byte(mainContent), 0o644); err != nil {
		t.Fatalf("Failed to write main.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "imported.md"), []byte(importedContent), 0o644); err != nil {
		t.Fatalf("Failed to write imported.md: %v", err)
	}

	seen := make(map[string]bool)
	result := resolveImports(mainContent, tmpDir, 0, seen)

	if !strings.Contains(result, "<!-- Imported: imported.md -->") {
		t.Errorf("Expected import comment, got: %s", result)
	}
	if !strings.Contains(result, "This was imported from another file.") {
		t.Errorf("Expected imported content, got: %s", result)
	}
	if !strings.Contains(result, "Some content after import") {
		t.Errorf("Expected content after import, got: %s", result)
	}
}

func TestResolveImportsCycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pcb-test-cycle")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	file1Content := `# File 1
@file2.md`

	file2Content := `# File 2
@file1.md`

	if err := os.WriteFile(filepath.Join(tmpDir, "file1.md"), []byte(file1Content), 0o644); err != nil {
		t.Fatalf("Failed to write file1.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "file2.md"), []byte(file2Content), 0o644); err != nil {
		t.Fatalf("Failed to write file2.md: %v", err)
	}

	seen := make(map[string]bool)
	seen[filepath.Join(tmpDir, "file1.md")] = true
	result := resolveImports(file1Content, tmpDir, 0, seen)

	if !strings.Contains(result, "# File 2") {
		t.Errorf("Expected file2 content, got: %s", result)
	}
	if !strings.Contains(result, "Skipped (cycle)") {
		t.Errorf("Expected cycle skip comment, got: %s", result)
	}
}

func TestResolveImportsNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pcb-test-notfound")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := `# Test
@nonexistent.md`

	seen := make(map[string]bool)
	result := resolveImports(content, tmpDir, 0, seen)

	if !strings.Contains(result, "Import not found") {
		t.Errorf("Expected not found comment, got: %s", result)
	}
}

func TestResolveImportsMaxDepth(t *testing.T) {
	content := `@deep.md`

	seen := make(map[string]bool)
	result := resolveImports(content, "/tmp", maxImportDepth, seen)

	if result != content {
		t.Errorf("Expected unchanged content at max depth, got: %s", result)
	}
}

func TestLoadRulesDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pcb-test-rules")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	rulesDir := filepath.Join(tmpDir, "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("Failed to create rules dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(rulesDir, "coding.md"), []byte("# Coding Rules"), 0o644); err != nil {
		t.Fatalf("Failed to write coding.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "security.md"), []byte("# Security Rules"), 0o644); err != nil {
		t.Fatalf("Failed to write security.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "readme.txt"), []byte("Ignore me"), 0o644); err != nil {
		t.Fatalf("Failed to write readme.txt: %v", err)
	}

	seen := make(map[string]bool)
	files := loadRulesDirectory(rulesDir, "project", seen)

	if len(files) != 2 {
		t.Errorf("Expected 2 rule files, got %d", len(files))
	}

	if len(files) > 0 && !strings.Contains(files[0].Path, "coding.md") {
		t.Errorf("Expected coding.md first (alphabetical), got: %s", files[0].Path)
	}
	if len(files) > 1 && !strings.Contains(files[1].Path, "security.md") {
		t.Errorf("Expected security.md second, got: %s", files[1].Path)
	}
}

func TestGetAllMemoryPaths(t *testing.T) {
	cwd := "/test/project"
	paths := GetAllMemoryPaths(cwd)

	// AGENTS.md + CLAUDE.md (+ .claude). SAN.md is not loaded.
	if len(paths.Project) != 3 {
		t.Errorf("Expected 3 project paths, got %d", len(paths.Project))
	}
	if !strings.Contains(paths.Project[0], "AGENTS.md") {
		t.Errorf("Expected AGENTS.md preferred first, got: %s", paths.Project[0])
	}
	if !strings.Contains(paths.Project[1], "CLAUDE.md") {
		t.Errorf("Expected CLAUDE.md second, got: %s", paths.Project[1])
	}
	if len(paths.Local) != 0 {
		t.Errorf("Expected no local paths, got %v", paths.Local)
	}
	if !strings.Contains(paths.ProjectRules, "rules") {
		t.Errorf("Expected rules in project rules path, got: %s", paths.ProjectRules)
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{500, "500B"},
		{1024, "1.0KB"},
		{2048, "2.0KB"},
		{1024 * 1024, "1.0MB"},
		{1536 * 1024, "1.5MB"},
	}

	for _, tc := range tests {
		result := FormatFileSize(tc.size)
		if result != tc.expected {
			t.Errorf("FormatFileSize(%d) = %s, expected %s", tc.size, result, tc.expected)
		}
	}
}

func TestResolveImportsNested(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pcb-test-nested")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	aContent := `# Level A
@b.md
After B import`

	bContent := `## Level B
@c.md
After C import`

	cContent := `### Level C
Deepest content`

	if err := os.WriteFile(filepath.Join(tmpDir, "a.md"), []byte(aContent), 0o644); err != nil {
		t.Fatalf("Failed to write a.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "b.md"), []byte(bContent), 0o644); err != nil {
		t.Fatalf("Failed to write b.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "c.md"), []byte(cContent), 0o644); err != nil {
		t.Fatalf("Failed to write c.md: %v", err)
	}

	seen := make(map[string]bool)
	result := resolveImports(aContent, tmpDir, 0, seen)

	if !strings.Contains(result, "<!-- Imported: b.md -->") {
		t.Errorf("Expected b.md import comment, got: %s", result)
	}
	if !strings.Contains(result, "<!-- Imported: c.md -->") {
		t.Errorf("Expected c.md import comment, got: %s", result)
	}
	if !strings.Contains(result, "Deepest content") {
		t.Errorf("Expected deepest content from c.md, got: %s", result)
	}
	if !strings.Contains(result, "After C import") {
		t.Errorf("Expected content after C import from b.md, got: %s", result)
	}
	if !strings.Contains(result, "After B import") {
		t.Errorf("Expected content after B import from a.md, got: %s", result)
	}
}

func TestResolveImportsRelativePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pcb-test-relative")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	mainContent := `# Main
@./subdir/nested.md`

	nestedContent := `## Nested
Nested content here`

	if err := os.WriteFile(filepath.Join(tmpDir, "main.md"), []byte(mainContent), 0o644); err != nil {
		t.Fatalf("Failed to write main.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.md"), []byte(nestedContent), 0o644); err != nil {
		t.Fatalf("Failed to write nested.md: %v", err)
	}

	seen := make(map[string]bool)
	result := resolveImports(mainContent, tmpDir, 0, seen)

	if !strings.Contains(result, "<!-- Imported: ./subdir/nested.md -->") {
		t.Errorf("Expected nested import comment, got: %s", result)
	}
	if !strings.Contains(result, "Nested content here") {
		t.Errorf("Expected nested content, got: %s", result)
	}
}

func TestLoadMemoryFilesWithImports(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pcb-test-memory-imports")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	agentsContent := `# Project Memory
@extra.md
End of memory`

	extraContent := `## Extra Content
This was imported`

	if err := os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte(agentsContent), 0o644); err != nil {
		t.Fatalf("Failed to write AGENTS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "extra.md"), []byte(extraContent), 0o644); err != nil {
		t.Fatalf("Failed to write extra.md: %v", err)
	}

	files := LoadMemoryFiles(tmpDir)

	var projectFile *MemoryFile
	for i := range files {
		if files[i].Level == "project" && strings.Contains(files[i].Path, "AGENTS.md") {
			projectFile = &files[i]
			break
		}
	}

	if projectFile == nil {
		t.Fatal("Expected to find project AGENTS.md file")
	}

	if !strings.Contains(projectFile.Content, "<!-- Imported: extra.md -->") {
		t.Errorf("Expected import comment in content, got: %s", projectFile.Content)
	}
	if !strings.Contains(projectFile.Content, "This was imported") {
		t.Errorf("Expected imported content, got: %s", projectFile.Content)
	}
}

func TestFindMemoryFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pcb-test-find")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	existingFile := filepath.Join(tmpDir, "exists.md")
	if err := os.WriteFile(existingFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	tests := []struct {
		name     string
		paths    []string
		expected string
	}{
		{
			name:     "first existing file wins",
			paths:    []string{filepath.Join(tmpDir, "notexist.md"), existingFile},
			expected: existingFile,
		},
		{
			name:     "no files exist",
			paths:    []string{filepath.Join(tmpDir, "a.md"), filepath.Join(tmpDir, "b.md")},
			expected: "",
		},
		{
			name:     "empty paths",
			paths:    []string{},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FindMemoryFile(tc.paths)
			if result != tc.expected {
				t.Errorf("FindMemoryFile() = %q, expected %q", result, tc.expected)
			}
		})
	}
}

func TestLoadInstructions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pcb-test-instructions")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("Project instructions here"), 0o644); err != nil {
		t.Fatalf("Failed to write AGENTS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("Claude instructions here"), 0o644); err != nil {
		t.Fatalf("Failed to write CLAUDE.md: %v", err)
	}
	// SAN.md must be ignored even if present.
	if err := os.WriteFile(filepath.Join(tmpDir, "SAN.md"), []byte("SAN should not load"), 0o644); err != nil {
		t.Fatalf("Failed to write SAN.md: %v", err)
	}

	user, project := LoadInstructions(tmpDir)

	if !strings.Contains(project, "Project instructions here") {
		t.Errorf("project instructions should contain AGENTS.md content, got: %s", project)
	}
	if !strings.Contains(project, "Claude instructions here") {
		t.Errorf("project instructions should contain CLAUDE.md content, got: %s", project)
	}
	if strings.Contains(project, "SAN should not load") {
		t.Errorf("project instructions must not contain SAN.md content, got: %s", project)
	}

	_ = user
}

func TestLoadMemoryFiles_LoadsAgentsAndClaudeAndSkipsSan(t *testing.T) {
	tmpHome := t.TempDir()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpHome)

	userSanDir := filepath.Join(tmpHome, ".pcb")
	userClaudeDir := filepath.Join(tmpHome, ".claude")
	projectSanDir := filepath.Join(tmpDir, ".pcb")
	projectClaudeDir := filepath.Join(tmpDir, ".claude")

	for _, dir := range []string{userSanDir, userClaudeDir, projectSanDir, projectClaudeDir, filepath.Join(userSanDir, "rules"), filepath.Join(projectSanDir, "rules")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}
	}

	if err := os.WriteFile(filepath.Join(userSanDir, "AGENTS.md"), []byte("user agents"), 0o644); err != nil {
		t.Fatalf("WriteFile(user AGENTS): %v", err)
	}
	if err := os.WriteFile(filepath.Join(userClaudeDir, "CLAUDE.md"), []byte("user claude"), 0o644); err != nil {
		t.Fatalf("WriteFile(user CLAUDE): %v", err)
	}
	if err := os.WriteFile(filepath.Join(userSanDir, "rules", "01-global.md"), []byte("global rule"), 0o644); err != nil {
		t.Fatalf("WriteFile(global rule): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("project agents"), 0o644); err != nil {
		t.Fatalf("WriteFile(project AGENTS): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("project claude"), 0o644); err != nil {
		t.Fatalf("WriteFile(project CLAUDE): %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectClaudeDir, "CLAUDE.md"), []byte("project claude nested"), 0o644); err != nil {
		t.Fatalf("WriteFile(project .claude CLAUDE): %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectSanDir, "rules", "01-project.md"), []byte("project rule"), 0o644); err != nil {
		t.Fatalf("WriteFile(project rule): %v", err)
	}
	// Stale SAN.md files must not be loaded.
	if err := os.WriteFile(filepath.Join(userSanDir, "SAN.md"), []byte("user pcb ignored"), 0o644); err != nil {
		t.Fatalf("WriteFile(user SAN): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "SAN.md"), []byte("project pcb ignored"), 0o644); err != nil {
		t.Fatalf("WriteFile(project SAN): %v", err)
	}

	files := LoadMemoryFiles(tmpDir)
	// global AGENTS + global CLAUDE + global rule + project AGENTS + project CLAUDE + project .claude CLAUDE + project rule
	if len(files) != 7 {
		t.Fatalf("expected 7 memory files, got %d (%v)", len(files), pathsOf(files))
	}

	if files[0].Level != "global" || !strings.Contains(files[0].Path, "AGENTS.md") {
		t.Fatalf("expected global AGENTS.md first, got level=%q path=%q", files[0].Level, files[0].Path)
	}
	if !strings.Contains(files[0].Content, "user agents") {
		t.Fatal("expected user AGENTS.md content")
	}
	if files[1].Level != "global" || !strings.Contains(files[1].Path, "CLAUDE.md") {
		t.Fatalf("expected global CLAUDE.md second, got level=%q path=%q", files[1].Level, files[1].Path)
	}
	if files[2].Level != "global" {
		t.Fatalf("expected global rules third, got level=%q", files[2].Level)
	}
	if files[3].Level != "project" || !strings.HasSuffix(files[3].Path, "AGENTS.md") {
		t.Fatalf("expected project AGENTS.md fourth, got level=%q path=%q", files[3].Level, files[3].Path)
	}
	if files[4].Level != "project" || !strings.HasSuffix(files[4].Path, "CLAUDE.md") {
		t.Fatalf("expected project CLAUDE.md fifth, got level=%q path=%q", files[4].Level, files[4].Path)
	}
	for _, f := range files {
		if strings.Contains(f.Path, "SAN.md") || strings.Contains(f.Content, "pcb ignored") {
			t.Fatalf("SAN.md must not load: path=%q", f.Path)
		}
		if f.Level == "local" {
			t.Fatalf("local memory must not load: path=%q", f.Path)
		}
	}
}

func pathsOf(files []MemoryFile) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Level + ":" + f.Path
	}
	return out
}

func TestMemory_ImportChain(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pcb-test-import-chain")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	agentsContent := "# Root\n@a.md"
	aMdContent := "## Level A\n@b.md"
	bMdContent := "### Level B\nFinal content from B"

	if err := os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte(agentsContent), 0o644); err != nil {
		t.Fatalf("Failed to write AGENTS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "a.md"), []byte(aMdContent), 0o644); err != nil {
		t.Fatalf("Failed to write a.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "b.md"), []byte(bMdContent), 0o644); err != nil {
		t.Fatalf("Failed to write b.md: %v", err)
	}

	files := LoadMemoryFiles(tmpDir)

	var projectFile *MemoryFile
	for i := range files {
		if files[i].Level == "project" && strings.Contains(files[i].Path, "AGENTS.md") {
			projectFile = &files[i]
			break
		}
	}

	if projectFile == nil {
		t.Fatal("Expected to find project AGENTS.md file")
	}

	if !strings.Contains(projectFile.Content, "Level A") {
		t.Errorf("Expected 'Level A' content (from a.md) in resolved output; got: %s", projectFile.Content)
	}
	if !strings.Contains(projectFile.Content, "Final content from B") {
		t.Errorf("Expected 'Final content from B' (from b.md) in resolved output; got: %s", projectFile.Content)
	}
	if !strings.Contains(projectFile.Content, "<!-- Imported: a.md -->") {
		t.Errorf("Expected import comment for a.md; got: %s", projectFile.Content)
	}
	if !strings.Contains(projectFile.Content, "<!-- Imported: b.md -->") {
		t.Errorf("Expected import comment for b.md; got: %s", projectFile.Content)
	}
}

func TestMemory_MissingFile_NoError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pcb-test-missing-agentsmd")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := LoadMemoryFiles(tmpDir)

	for _, f := range files {
		if f.Level == "project" && strings.Contains(f.Path, tmpDir) {
			t.Errorf("Did not expect a project memory file when AGENTS.md is absent, got: %s", f.Path)
		}
	}

	_, project := LoadInstructions(tmpDir)
	_ = project
}

func TestResolveAutoMemoryDir(t *testing.T) {
	cwd := "/work/project-x"
	// Empty override → the project-partitioned default.
	if got := ResolveAutoMemoryDir(cwd, ""); got != AutoMemoryDir(cwd) {
		t.Fatalf("empty override: got %q, want default %q", got, AutoMemoryDir(cwd))
	}
	// Absolute override is used verbatim.
	if got := ResolveAutoMemoryDir(cwd, "/srv/mem"); got != "/srv/mem" {
		t.Fatalf("absolute override: got %q", got)
	}
	// Relative override resolves against cwd.
	if got := ResolveAutoMemoryDir(cwd, "notes/mem"); got != filepath.Join(cwd, "notes/mem") {
		t.Fatalf("relative override: got %q", got)
	}
	// "~" expands to the home dir.
	if home, err := os.UserHomeDir(); err == nil {
		if got := ResolveAutoMemoryDir(cwd, "~/mem"); got != filepath.Join(home, "mem") {
			t.Fatalf("tilde override: got %q, want %q", got, filepath.Join(home, "mem"))
		}
	}
}
