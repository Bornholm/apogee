package client

import (
	"testing"

	"github.com/hajimehoshi/bitmapfont/v4"
)

func TestFontCoversFrench(t *testing.T) {
	for _, r := range "àâäçéèêëîïôöùûüÿœæÀÂÇÉÈÊËÎÏÔÙÛŒ«»’…—–·%×\u00a0" {
		if _, ok := bitmapfont.Face.GlyphAdvance(r); !ok {
			t.Errorf("missing glyph %q", r)
		}
	}
}
