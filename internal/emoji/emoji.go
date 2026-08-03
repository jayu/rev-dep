// Package emoji holds the glyphs used in rev-dep's CLI output.
//
// Every glyph is Cells wide, so one space after any of them lines up with any other. Two
// (Warning, Fix) are text-presentation characters that render ONE cell and carry a trailing
// pad space to match the rest. Without that pad every call site compensates by hand, and code
// that measures a glyph to align a column gets it wrong - a rune count is not a cell count,
// and U+FE0F is a rune that occupies nothing.
package emoji

// Cells is the display width of every glyph below, padding included.
const Cells = 2

const (
	Success = "✅" // U+2705 white heavy check mark
	Error   = "❌" // U+274C cross mark
	// U+FE0F forces colour; the trailing space pads one cell to two. Both are load-bearing.
	Warning         = "⚠️ " // U+26A0 U+FE0F warning sign
	Fix             = "✍️ " // U+270D U+FE0F writing hand
	Rule            = "📁"   // U+1F4C1 file folder
	File            = "📄"   // U+1F4C4 page facing up
	Done            = "✨"   // U+2728 sparkles
	Search          = "🔍"   // U+1F50D magnifying glass
	Package         = "📦"   // U+1F4E6 package
	Tip             = "💡"   // U+1F4A1 light bulb
	Standalone      = "🧩"   // U+1F9E9 jigsaw puzzle piece
	Guide           = "📖"   // U+1F4D6 open book
	Troubleshooting = "🛟"   // U+1F6DF ring buoy
)
