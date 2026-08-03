// Package plural renders a count with its noun: "1 file", "2 files".
//
// Both forms are always passed in: deriving "copies" from "copy" needs an irregular-noun
// table nobody wants to own.
package plural

import "strconv"

func Count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}
