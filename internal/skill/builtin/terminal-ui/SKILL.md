---
name: terminal-ui
description: Terminal/TUI design with Bubble Tea and Lipgloss: model-view-update, component composition, theming, and keyboard navigation. Use when building terminal user interfaces.
origin: builtin
---

# Terminal UI Design

## When to use
- Building a Bubble Tea TUI application
- Designing terminal components with Lipgloss
- Creating themes and adaptive color systems
- Implementing keyboard navigation

## Bubble Tea architecture (Elm-style MVU)
```go
type model struct {
    items []string
    cursor int
    selected map[int]bool
}

func (m model) Init() tea.Cmd { return nil }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "up": m.cursor--
        case "down": m.cursor++
        case "enter": m.selected[m.cursor] = true
        case "q": return m, tea.Quit
        }
    }
    return m, nil
}
func (m model) View() string { /* render with Lipgloss */ }
```

## Lipgloss patterns

### Theme system
```go
var theme = struct {
    Primary lipgloss.Color
    Accent  lipgloss.Color
}{
    Primary: lipgloss.Color("#46E8C0"),
    Accent:  lipgloss.Color("#9DB5D4"),
}
```

### Style functions (lazy, theme-aware)
```go
func FocusedStyle() lipgloss.Style {
    return lipgloss.NewStyle().
        Foreground(theme.Primary).
        Bold(true)
}
```
Use functions so styles reflect theme changes at render time.

### Borders
- `lipgloss.RoundedBorder()` for panels
- `lipgloss.NormalBorder()` for tables
- Border colors from theme tokens

### Layout
- Use `lipgloss.JoinHorizontal` / `JoinVertical`
- Margins and padding for spacing
- `lipgloss.Width()` / `Height()` for sizing

## Component patterns
- **List**: `bubbles/list` for scrollable selections
- **Text input**: `bubbles/textinput` for single-line input
- **Viewport**: `bubbles/viewport` for scrollable content
- **Spinner**: `bubbles/spinner` for async loading
- **Table**: `bubbles/table` for structured data

## Keyboard navigation
- Arrow keys / hjkl for navigation
- Enter for confirm, Esc/q for cancel
- Tab for focus cycling between components
- `/` for search, `:` for commands

## Theme rules
- Dark + light adaptive colors (use `AdaptiveColor`)
- Focus color: saturated teal for "you are here"
- Semantic colors: green/success, red/error, yellow/warning
- Dim text for hints, bright text for emphasis
- Never hardcode hex values in components — use theme tokens

## Common anti-patterns
- Mixed selection glyphs (use one: `▎` focus bar)
- No keyboard navigation (mouse-only TUI)
- Blocking operations in Update (use tea.Cmd for async)
- Not handling terminal resize (`tea.WindowSizeMsg`)
- Hardcoded colors (not theme-adaptive)
- Too much text on screen (use scrolling/viewport)
