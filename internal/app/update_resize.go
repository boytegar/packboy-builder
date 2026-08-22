// Window resize handling. handleWindowResize runs the first time we get a
// window size (the deferred initial paint), where it commits any resumed
// conversation. On later resizes the width reflow happens in the viewport
// cache: committed blocks are re-rendered at the new width (their glamour
// wrapping is width-dependent) and the viewport re-slices on the next frame.
package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/boytegar/packboy-builder/internal/app/conv"
)

// reflowCommitted re-renders every committed message at the current width and
// rebuilds the viewport render cache, so a resize reflows the whole
// conversation (native-scrollback-style reflow, but in place).
func (m *model) reflowCommitted() {
	if m.chat == nil {
		return
	}
	params := m.messageRenderParams()
	var blocks []string
	for i := 0; i < m.conv.CommittedCount; i++ {
		if rendered := conv.RenderSingleMessage(params, i); rendered != "" {
			blocks = append(blocks, rendered)
		}
	}
	m.chat.rebuildCache(blocks)
}

// A resize mid-render is safe: a flushResult from an earlier geometry still
// lands in the cache; appendBlock + next-frame re-sync handles it, and the
// reflow above rebuilds from message state (correct width) on the next frame.
func (m *model) handleWindowResize(msg tea.WindowSizeMsg) tea.Cmd {
	m.env.Width = msg.Width
	m.env.Height = msg.Height
	m.userInput.SetTerminalHeight(msg.Height)
	if ov, ok := m.activeOverlay(); ok {
		if resizable, ok := ov.(resizableOverlay); ok {
			resizable.Resize(msg.Width, msg.Height)
		}
	}

	m.conv.ResizeMDRenderer(msg.Width)
	// The width changed: committed blocks must be re-rendered at the new width.
	if msg.Width > 0 && m.conv.CommittedCount > 0 {
		m.reflowCommitted()
	}

	if !m.env.Ready {
		m.env.Ready = true

		// Welcome banner is drawn before tea.NewProgram (see run.go); here
		// we only need to commit any resumed conversation.
		var cmds []tea.Cmd
		if len(m.conv.Messages) > 0 {
			cmds = append(cmds, m.commitAllMessages()...)
		}

		if m.userInput.Session.PendingSelector {
			m.userInput.Session.PendingSelector = false
			if m.services.Session.GetStore() != nil {
				_ = m.userInput.Session.Selector.EnterSelect(m.env.Width, m.env.Height, m.services.Session.GetStore(), m.env.CWD)
			}
		}

		m.userInput.Textarea.SetWidth(msg.Width - 4 - 2)
		if len(cmds) > 0 {
			return tea.Batch(cmds...)
		}
		return nil
	}

	m.userInput.Textarea.SetWidth(msg.Width - 4 - 2)
	return nil
}
