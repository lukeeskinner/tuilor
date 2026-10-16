package cli

import (
	"fmt"
	"strings"
	"unicode"
)

// EscapeTerminalControls keeps untrusted metadata and error messages from
// emitting terminal commands. Callers add their own formatting newlines.
func EscapeTerminalControls(text string) string {
	var output strings.Builder
	for _, r := range text {
		if unicode.IsControl(r) {
			fmt.Fprintf(&output, "\\u%04x", r)
		} else {
			output.WriteRune(r)
		}
	}
	return output.String()
}
