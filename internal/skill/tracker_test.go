package skill

import (
	"reflect"
	"testing"
)

func TestTracker(t *testing.T) {
	tracker := NewTracker()
	tracker.MarkLoaded("zeta")
	tracker.MarkLoaded("alpha")
	tracker.MarkLoaded("alpha")

	if !tracker.IsLoaded("alpha") || tracker.IsLoaded("missing") {
		t.Fatal("unexpected load state")
	}
	if got, want := tracker.LoadedNames(), []string{"alpha", "zeta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadedNames() = %v, want %v", got, want)
	}
}
