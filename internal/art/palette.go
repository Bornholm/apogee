package art

import "image/color"

// Tone is one of the three shades of a material ramp.
type Tone int

const (
	Lit Tone = iota
	Mid
	Dark
)

// Material identifies a ramp of three tones in the palette.
type Material int

const (
	Grass Material = iota
	Foliage
	Wood
	Earth
	Underside
	Boulder
	Roof
	Plaster
	Stone
	Aether
	Team
	Cloud
	materialCount
)

// Ramp holds the lit, mid and dark tones of a material.
type Ramp [3]color.RGBA

func hex(v uint32) color.RGBA {
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 0xff}
}

// Palette is the fixed palette shared by the whole game (GDD 12.2): a few
// materials of three tones each. Nature stays desaturated, saturation is kept
// for buildings, teams and the aether.
var Palette = [materialCount]Ramp{
	Grass:     {hex(0x7aa35a), hex(0x5f8a48), hex(0x476b3a)},
	Foliage:   {hex(0x55845a), hex(0x3b6448), hex(0x294837)},
	Wood:      {hex(0xa0714a), hex(0x7d5638), hex(0x593d2a)},
	Earth:     {hex(0xe4b96c), hex(0xc6934f), hex(0x986b3c)},
	Underside: {hex(0x8d92a6), hex(0x676d84), hex(0x484d66)},
	Boulder:   {hex(0xb4bcae), hex(0x87958a), hex(0x5f6d66)},
	Roof:      {hex(0xe27c48), hex(0xb95b37), hex(0x8a412b)},
	Plaster:   {hex(0xeadab4), hex(0xc8b38c), hex(0x9c8868)},
	Stone:     {hex(0xcfc9bc), hex(0xa29c90), hex(0x78736b)},
	Aether:    {hex(0xc2f6ff), hex(0x6ccbf2), hex(0x5d6fd4)},
	Team:      {hex(0x7fa6f0), hex(0x4a78d6), hex(0x2f4f9e)},
	Cloud:     {hex(0xf6f8fa), hex(0xdfe7ef), hex(0xc3cfdd)},
}

// ShadowTint is the colour of cast shadows: tinted blue-violet, never black.
var ShadowTint = color.RGBA{0x2c, 0x2f, 0x5c, 0xff}

// ShadowAlpha is the uniform opacity of the shadow layer (0-255).
const ShadowAlpha = 96

// SkyTop and SkyBottom bound the vertical gradient of the sky background.
var (
	SkyTop    = hex(0x86bdd6)
	SkyBottom = hex(0xd3e9ea)
)
