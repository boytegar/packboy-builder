package setting

import (
	"testing"
)

func TestNextWithBypass_Disabled(t *testing.T) {
	// Bypass is no longer an OperationMode; NextWithBypass ignores the flag and
	// cycles the three primary modes: Normal → ReadOnly → Swarm → Normal.
	cycle := []OperationMode{ModeNormal, ModeReadOnly, ModeSwarm, ModeNormal}
	for i := 0; i < len(cycle)-1; i++ {
		got := cycle[i].NextWithBypass(false)
		if got != cycle[i+1] {
			t.Errorf("NextWithBypass(false): from %d, got %d, want %d", cycle[i], got, cycle[i+1])
		}
	}
}

func TestNextWithBypass_Enabled(t *testing.T) {
	// Bypass is orthogonal now, so enabled=true no longer adds it to the cycle.
	// The cycle is identical to the disabled path.
	cycle := []OperationMode{ModeNormal, ModeReadOnly, ModeSwarm, ModeNormal}
	for i := 0; i < len(cycle)-1; i++ {
		got := cycle[i].NextWithBypass(true)
		if got != cycle[i+1] {
			t.Errorf("NextWithBypass(true): from %d, got %d, want %d", cycle[i], got, cycle[i+1])
		}
	}
}

func TestNextWithBypass_UnknownMode(t *testing.T) {
	unknown := OperationMode(99)
	if got := unknown.NextWithBypass(false); got != ModeNormal {
		t.Errorf("NextWithBypass(false) from unknown: got %d, want %d", got, ModeNormal)
	}
	if got := unknown.NextWithBypass(true); got != ModeNormal {
		t.Errorf("NextWithBypass(true) from unknown: got %d, want %d", got, ModeNormal)
	}
}

func TestNext_StillWorks(t *testing.T) {
	cycle := []OperationMode{ModeNormal, ModeReadOnly, ModeSwarm, ModeNormal}
	for i := 0; i < len(cycle)-1; i++ {
		got := cycle[i].Next()
		if got != cycle[i+1] {
			t.Errorf("Next(): from %d, got %d, want %d", cycle[i], got, cycle[i+1])
		}
	}
}

func TestNext_BypassReturnsNormal(t *testing.T) {
	got := ModeBypassPermissions.Next()
	if got != ModeNormal {
		t.Errorf("Next() from ModeBypassPermissions: got %d, want %d", got, ModeNormal)
	}
}

func TestNext_AutoAcceptAndAutoPilotReturnNormal(t *testing.T) {
	// AutoAccept and AutoPilot are no longer in the Shift+Tab cycle; calling Next
	// on them returns to Normal (engaged only via /goal / /autopilot).
	if got := ModeAutoAccept.Next(); got != ModeNormal {
		t.Errorf("Next() from ModeAutoAccept: got %d, want %d", got, ModeNormal)
	}
	if got := ModeAutoPilot.Next(); got != ModeNormal {
		t.Errorf("Next() from ModeAutoPilot: got %d, want %d", got, ModeNormal)
	}
}

func TestOperationModePersistenceNames(t *testing.T) {
	tests := []struct {
		mode OperationMode
		name string
	}{
		{ModeNormal, "default"},
		{ModeAutoAccept, "auto-accept"},
		{ModeAutoPilot, "auto-pilot"},
		{ModeBypassPermissions, "bypass"},
		{ModeDontAsk, "dont-ask"},
		{ModeReadOnly, "read-only"},
		{ModeSwarm, "swarm"},
	}
	for _, tt := range tests {
		if got := tt.mode.PersistenceName(); got != tt.name {
			t.Errorf("%v.PersistenceName() = %q, want %q", tt.mode, got, tt.name)
		}
		if got := OperationModeFromString(tt.name); got != tt.mode {
			t.Errorf("OperationModeFromString(%q) = %v, want %v", tt.name, got, tt.mode)
		}
	}
}

func TestAutoPilot_StringAndFromString(t *testing.T) {
	if got := ModeAutoPilot.String(); got != "autopilot" {
		t.Errorf("ModeAutoPilot.String() = %q, want %q", got, "autopilot")
	}
	for _, s := range []string{"autoPilot", "auto-pilot", "autopilot", "pilot"} {
		if got := OperationModeFromString(s); got != ModeAutoPilot {
			t.Errorf("OperationModeFromString(%q) = %d, want %d", s, got, ModeAutoPilot)
		}
	}
}

func TestSwarmMode_StringAndFromString(t *testing.T) {
	if got := ModeSwarm.String(); got != "swarm" {
		t.Errorf("ModeSwarm.String() = %q, want %q", got, "swarm")
	}
	for _, s := range []string{"swarm", "agent"} {
		if got := OperationModeFromString(s); got != ModeSwarm {
			t.Errorf("OperationModeFromString(%q) = %d, want %d", s, got, ModeSwarm)
		}
	}
}

func TestNormalMode_StringIsDefault(t *testing.T) {
	if got := ModeNormal.String(); got != "default" {
		t.Errorf("ModeNormal.String() = %q, want %q", got, "default")
	}
}
