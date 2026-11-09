package client

import (
	"testing"
	"time"
)

func TestTintFollowsTheDay(t *testing.T) {
	if tintAt("day", 5*time.Minute) != dayTint {
		t.Error("day is not neutral")
	}
	if tintAt("night", time.Minute) != nightTint {
		t.Error("night is not blue")
	}
	// Dawn starts at night and ends in daylight, through a warm moment.
	if tintAt("dawn", 3*time.Minute) != nightTint || tintAt("dawn", 0) != dayTint {
		t.Error("dawn does not go from night to day")
	}
	if mid := tintAt("dusk", 90*time.Second); mid != duskTint {
		t.Errorf("mid-dusk tint %v, want the warm %v", mid, duskTint)
	}
	// Continuity at the turn of the phases.
	if tintAt("dusk", 0) != tintAt("night", 9*time.Minute) {
		t.Error("a jump between dusk and night")
	}
	if tintAt("night", 0) != tintAt("dawn", 3*time.Minute) {
		t.Error("a jump between night and dawn")
	}
}
