package client

import (
	"testing"

	"golang.org/x/image/font/sfnt"
)

// The fonts must draw every character of the French text.
func TestFontsCoverFrench(t *testing.T) {
	for name, b := range map[string][]byte{"Pixelify Sans": pixelifyTTF, "Silkscreen": silkscreenTTF} {
		f, err := sfnt.Parse(b)
		if err != nil {
			t.Fatal(err)
		}
		var buf sfnt.Buffer
		for _, r := range "àâäçéèêëîïôöùûüÿœæÀÂÇÉÈÊËÎÏÔÙÛŒ«»’…—–·%× " {
			if i, _ := f.GlyphIndex(&buf, r); i == 0 {
				t.Errorf("%s: missing glyph %q", name, r)
			}
		}
	}
}
