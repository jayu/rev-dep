// Package emoji centralizes the emoji glyphs used in rev-dep's CLI output so they stay
// consistent and can be changed in one place.
//
// Constants hold the bare glyph only — no surrounding spaces. Spacing is intentionally
// inconsistent across call sites (wide glyphs are often followed by two spaces), so keep
// the spaces in the format strings, not here.
//
// Glyphs that default to text presentation carry the U+FE0F variation selector so they
// render in color (Warning ⚠️, Fix ✍️); dropping it makes them monochrome.
package emoji

const (
	Success         = "✅"  // U+2705 white heavy check mark
	Error           = "❌"  // U+274C cross mark
	Warning         = "⚠️" // U+26A0 U+FE0F warning sign (keep the variation selector!)
	Fix             = "✍️" // U+270D U+FE0F writing hand (keep the variation selector!)
	Rule            = "📁"  // U+1F4C1 file folder
	File            = "📄"  // U+1F4C4 page facing up
	Done            = "✨"  // U+2728 sparkles
	Search          = "🔍"  // U+1F50D magnifying glass
	Package         = "📦"  // U+1F4E6 package
	Tip             = "💡"  // U+1F4A1 light bulb
	Standalone      = "🧩"  // U+1F9E9 jigsaw puzzle piece
	Guide           = "📖"  // U+1F4D6 open book
	Troubleshooting = "🛟"  // U+1F6DF ring buoy
)
