package bzconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"unicode"
	"unicode/utf8"
)

// printable reports whether s is valid UTF-8 made only of printable runes
// (unicode.IsPrint: letters, marks, numbers, punctuation, symbols and the
// ASCII space).
func printable(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// DisplayPath renders a path for a message. A printable path is shown as
// is. Anything else (control characters such as newline or ESC,
// non-printable runes, invalid UTF-8) is shown Go-quoted (strconv.Quote),
// so an attacker-chosen name cannot inject terminal escape sequences or
// fake lines into bz's output.
func DisplayPath(p string) string {
	if printable(p) {
		return p
	}
	return strconv.Quote(p)
}

// cleanErr re-renders a *fs.PathError, whose message embeds the raw path,
// with DisplayPath. The underlying error stays wrapped for errors.Is.
func cleanErr(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s %s: %w", pe.Op, DisplayPath(pe.Path), pe.Err)
	}
	return err
}
