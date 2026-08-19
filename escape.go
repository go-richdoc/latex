// Copyright (c) the go-richdoc authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import "strings"

// escapeText renders a run of plain text as LaTeX, escaping every character
// that carries a special meaning. It is the inverse of the unescaping the
// parser performs when it reads control symbols such as \& and control words
// such as \textbackslash back into literal runes.
//
// The transformation is a single pass so that the braces introduced by a
// replacement (for example the {} of \textbackslash{}) are never themselves
// re-escaped.
func escapeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\textbackslash{}`)
		case '&':
			b.WriteString(`\&`)
		case '%':
			b.WriteString(`\%`)
		case '$':
			b.WriteString(`\$`)
		case '#':
			b.WriteString(`\#`)
		case '_':
			b.WriteString(`\_`)
		case '{':
			b.WriteString(`\{`)
		case '}':
			b.WriteString(`\}`)
		case '~':
			b.WriteString(`\textasciitilde{}`)
		case '^':
			b.WriteString(`\textasciicircum{}`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// escapeURL escapes the characters that would break tokenization inside a
// braced URL argument (of \href or \includegraphics). It deliberately leaves
// the many URL-legal characters (~, ^, /, :, ., ?, =, ...) untouched, and is
// the exact inverse of unescapeURL over the set it does handle.
func escapeURL(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '#', '%', '&', '_', '{', '}':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// unescapeURL reverses escapeURL: it drops a backslash that precedes any of the
// URL specials escapeURL emits, and leaves every other backslash in place.
func unescapeURL(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '\\' && i+1 < len(rs) {
			switch rs[i+1] {
			case '#', '%', '&', '_', '{', '}':
				b.WriteRune(rs[i+1])
				i++
				continue
			}
		}
		b.WriteRune(rs[i])
	}
	return b.String()
}
