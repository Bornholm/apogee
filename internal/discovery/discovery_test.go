package discovery

import "testing"

func TestUnescape(t *testing.T) {
	for in, want := range map[string]string{
		`Ciel\ de\ test`: "Ciel de test",
		`plain`:          "plain",
		`a\\b`:           `a\b`,
		`trailing\`:      `trailing\`,
	} {
		if got := unescape(in); got != want {
			t.Errorf("unescape(%q) = %q, want %q", in, got, want)
		}
	}
}
