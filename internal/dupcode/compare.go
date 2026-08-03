package dupcode

import "rev-dep-go/internal/parser"

// Blinding is what a comparison ignores. See parser.Blinding.
type Blinding = parser.Blinding

// Compare walks two chunks in lockstep, one canonical byte at a time.
//
// Neither chunk is copied or rewritten: whitespace and comments are stepped over as the
// cursors advance, so `if(a){b()}` matches the same code spread over four indented lines
// without either side being materialised first.
//
// Blinding is applied HERE, during canonicalisation, not to some already-canonical form - see
// parser.Blinding. Whitespace is skipped but not ignored: a run separating two identifier
// characters collapses to one space both sides must have, so `const x` never matches `constx`.
func Compare(a, b []byte, blinding Blinding) bool {
	ia := parser.NewNormIterator(a, blinding)
	ib := parser.NewNormIterator(b, blinding)

	for {
		ca, _, oka := ia.Next()
		cb, _, okb := ib.Next()
		if !oka || !okb {
			return oka == okb // equal only if both ran out together
		}
		if ca != cb {
			return false
		}
	}
}

// Normalize returns the byte sequence Compare matches on, exposed for tests and for debugging
// a surprising match.
func Normalize(code []byte, blinding Blinding) []byte {
	it := parser.NewNormIterator(code, blinding)
	out := make([]byte, 0, len(code))
	for {
		c, _, ok := it.Next()
		if !ok {
			return out
		}
		out = append(out, c)
	}
}
