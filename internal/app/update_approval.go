// Permission approval flow + gate response. The approval modal lives in
// the input package; here we handle the user's decision (once / for-session
// / persist-as-rule) and forward it through the PermissionGate that gates
// the agent's tool calls.
package app

import (
	tea "charm.land/bubbletea/v2"
	"go.uber.org/zap"

	"github.com/boytegar/packboy-builder/internal/app/conv"
	"github.com/boytegar/packboy-builder/internal/log"
	"github.com/boytegar/packboy-builder/internal/session/transcript"
	"github.com/boytegar/packboy-builder/internal/setting"
	"github.com/boytegar/packboy-builder/internal/tool/perm"
)

type permissionDecision struct {
	Approved bool
	AllowAll bool // option 2: allow for the rest of the session
	Persist  bool // option 3: write a persistent rule
	Request  *perm.PermissionRequest
}

// Scope labels recorded for user-driven permission decisions. These names
// belong to the approval modal — the transcript schema treats them as opaque
// strings, so adding a new modal option (e.g. "this directory only") only
// requires a new label here, not a schema bump.
const (
	permScopeOnce       = "once"
	permScopeSession    = "session"
	permScopePersistent = "persistent"
)

// permDecisionFor maps the user's approve/reject bool to the transcript
// decision enum. Shared by the config-decided fast path (agent.go) and the
// user-decided ask path (this file).
func permDecisionFor(approved bool) string {
	if approved {
		return transcript.PermissionPermit
	}
	return transcript.PermissionReject
}

// permScope encodes which approval-modal option the user picked. Persist
// takes priority over AllowAll because the modal exposes them as
// mutually-exclusive radio-style choices.
func permScope(d permissionDecision) string {
	switch {
	case d.Persist:
		return permScopePersistent
	case d.AllowAll:
		return permScopeSession
	default:
		return permScopeOnce
	}
}

func (m *model) handlePermGateDecision(decision permissionDecision) tea.Cmd {
	if !m.services.Agent.Active() {
		return nil
	}
	req := m.services.Agent.PendingPermission()
	m.services.Agent.SetPendingPermission(nil)
	if req == nil {
		return nil
	}
	reason := "user denied"
	if decision.Approved {
		reason = "user approved"
		if decision.AllowAll && m.env.SessionPermissions != nil && decision.Request != nil {
			m.env.SessionPermissions.AllowTool(decision.Request.ToolName)
		}
		// Always allow: write project allow rules and grant them for this
		// session immediately. Without this, Persist only labeled the
		// transcript and the next matching call re-prompted.
		if decision.Persist {
			m.applyPersistentAllow(decision.Request, req.ToolName, req.Input)
		}
	}
	// Snapshot the request before releasing the permission gate. Agent tools
	// add runtime callbacks to their input map as soon as the response wakes
	// them; serializing that shared map afterward can otherwise race the
	// write and crash the process with concurrent map iteration/write.
	permRecord := permDecisionRecord(req, decision, reason, m.env.SessionMode())
	select {
	case req.Response <- conv.PermGateResponse{Allow: decision.Approved, Reason: reason}:
	default:
	}
	if rec := m.services.Session.Recorder(); rec != nil {
		rec.RecordPermissionDecided(permRecord)
	}
	return conv.PollPermGate(m.services.Agent.PermissionGate())
}

func permDecisionRecord(req *conv.PermGateRequest, decision permissionDecision, reason, mode string) transcript.PermissionRecord {
	return transcript.PermissionRecord{
		RequestID: req.RequestID,
		Tool:      req.ToolName,
		Input:     marshalPermInput(req.Input),
		Detail:    permDetail(decision.Request),
		Decision:  permDecisionFor(decision.Approved),
		Source:    transcript.PermissionSourceUser,
		Scope:     permScope(decision),
		Reason:    reason,
		Mode:      mode,
	}
}

// applyPersistentAllow grants the Always-allow rules for the rest of the
// session and appends them to project settings.json so they survive restarts.
// Session grant first so the next call in this turn does not race a reload.
func (m *model) applyPersistentAllow(uiReq *perm.PermissionRequest, toolName string, input map[string]any) {
	rules := persistentAllowRules(uiReq, toolName, input)
	if len(rules) == 0 {
		return
	}
	if m.env.SessionPermissions != nil {
		for _, rule := range rules {
			m.env.SessionPermissions.AllowPattern(rule)
		}
	}
	for _, rule := range rules {
		if err := setting.AddAllowRuleDirectlyAt(rule, m.env.CWD); err != nil {
			log.Logger().Warn("failed to persist allow rule",
				zap.String("rule", rule), zap.Error(err))
		}
	}
	if m.services.Setting != nil {
		if err := m.services.Setting.Reload(m.env.CWD); err != nil {
			log.Logger().Warn("failed to reload settings after allow rule", zap.Error(err))
		}
	}
}

// persistentAllowRules prefers the modal's suggested patterns (compound bash
// splits into per-subcommand rules). Falls back to BuildRule for tools that
// have no suggestions.
func persistentAllowRules(uiReq *perm.PermissionRequest, toolName string, input map[string]any) []string {
	if uiReq != nil && len(uiReq.SuggestedRules) > 0 {
		return append([]string(nil), uiReq.SuggestedRules...)
	}
	if toolName == "" && uiReq != nil {
		toolName = uiReq.ToolName
	}
	if toolName == "" {
		return nil
	}
	rule := setting.BuildRule(toolName, input)
	if rule == "" {
		return nil
	}
	return []string{rule}
}
