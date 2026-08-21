// Copyright (c) the go-richdoc authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-richdoc/richdoc"
)

var errUnmatchedBrace = errors.New("latex: unmatched brace")

// Parse converts a practical subset of LaTeX into a [richdoc.Document].
//
// When the source contains a document environment, only its body is parsed and
// the preamble is mined for metadata (\documentclass, \title, \author, \date),
// which is dropped from the body and recorded in Document.Meta. When there is
// no document environment the whole source is treated as the body, so bare
// fragments parse too.
//
// Unknown commands and environments are preserved as [richdoc.RawInline] /
// [richdoc.RawBlock] with Format "latex" rather than discarded. Malformed input
// (an unterminated group, environment or math span) returns an error.
func Parse(src []byte) (*richdoc.Document, error) {
	s := normalizeEOL(string(src))
	preamble, body := splitDocument(s)

	meta := map[string]string{}
	if err := extractPreambleMeta([]rune(preamble), meta); err != nil {
		return nil, err
	}

	p := &parser{src: []rune(body), meta: meta}
	blocks, err := p.parseBlocks()
	if err != nil {
		return nil, err
	}

	d := &richdoc.Document{Blocks: blocks}
	if len(meta) > 0 {
		d.Meta = meta
	}
	return d, nil
}

// normalizeEOL folds CRLF and lone CR into LF so the scanner only deals with \n.
func normalizeEOL(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// splitDocument separates the preamble from the document-environment body. When
// there is no \begin{document}, the whole input is the body and the preamble is
// empty.
func splitDocument(s string) (preamble, body string) {
	const begin = `\begin{document}`
	i := strings.Index(s, begin)
	if i < 0 {
		return "", s
	}
	preamble = s[:i]
	rest := s[i+len(begin):]
	if j := strings.LastIndex(rest, `\end{document}`); j >= 0 {
		return preamble, rest[:j]
	}
	return preamble, rest
}

// parser scans a rune slice with a cursor. Sub-scans (group bodies, environment
// bodies, table cells) run fresh parsers over sub-slices but share the same meta
// map so that metadata found anywhere lands in one place.
type parser struct {
	src  []rune
	meta map[string]string
	pos  int
}

func (p *parser) eof() bool  { return p.pos >= len(p.src) }
func (p *parser) peek() rune { return p.at(0) }
func (p *parser) at(off int) rune {
	i := p.pos + off
	if i < 0 || i >= len(p.src) {
		return 0
	}
	return p.src[i]
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v'
}

// readControl consumes a control sequence, assuming the cursor is on a
// backslash. A word (letters) sets word=true and name; a symbol sets sym.
func (p *parser) readControl() (name string, sym rune, word bool) {
	p.pos++ // consume the backslash
	if p.eof() {
		return "", 0, false
	}
	c := p.src[p.pos]
	if isASCIILetter(c) {
		start := p.pos
		for !p.eof() && isASCIILetter(p.src[p.pos]) {
			p.pos++
		}
		return string(p.src[start:p.pos]), 0, true
	}
	p.pos++
	return "", c, false
}

// blockWords are the control words that begin a block-level construct and so
// terminate an in-progress paragraph.
var blockWords = map[string]bool{
	"begin": true, "section": true, "subsection": true, "subsubsection": true,
	"paragraph": true, "subparagraph": true, "maketitle": true, "title": true,
	"author": true, "date": true, "documentclass": true, "usepackage": true,
	"hrulefill": true, "rule": true,
}

var mathEnvs = map[string]bool{
	"equation": true, "equation*": true, "align": true, "align*": true,
	"alignat": true, "alignat*": true, "gather": true, "gather*": true,
	"multline": true, "multline*": true, "displaymath": true,
	"eqnarray": true, "eqnarray*": true,
}

var headingLevels = map[string]int{
	"section": 1, "subsection": 2, "subsubsection": 3,
	"paragraph": 4, "subparagraph": 5,
}

// atBlockCommand reports, without moving the cursor, whether the cursor sits on
// a block-level command (or the \[ display-math opener).
func (p *parser) atBlockCommand() bool {
	if p.peek() != '\\' {
		return false
	}
	save := p.pos
	name, sym, word := p.readControl()
	p.pos = save
	if !word {
		return sym == '['
	}
	return blockWords[name]
}

// atBlankLine reports, without moving the cursor, whether the whitespace run at
// the cursor spans a blank line (two or more newlines).
func (p *parser) atBlankLine() bool {
	nl := 0
	for i := p.pos; i < len(p.src) && isSpace(p.src[i]); i++ {
		if p.src[i] == '\n' {
			nl++
			if nl >= 2 {
				return true
			}
		}
	}
	return false
}

// skipLeading consumes whitespace, blank lines and comments between blocks.
func (p *parser) skipLeading() {
	for !p.eof() {
		if isSpace(p.peek()) {
			p.pos++
			continue
		}
		if p.peek() == '%' {
			for !p.eof() && p.peek() != '\n' {
				p.pos++
			}
			continue
		}
		return
	}
}

func (p *parser) skipInlineSpaces() {
	for !p.eof() && isSpace(p.peek()) {
		p.pos++
	}
}

// parseBlocks scans the parser's source into a sequence of blocks.
func (p *parser) parseBlocks() ([]richdoc.Block, error) {
	var blocks []richdoc.Block
	for {
		p.skipLeading()
		if p.eof() {
			return blocks, nil
		}
		if p.atBlockCommand() {
			bs, err := p.handleBlockCommand()
			if err != nil {
				return nil, err
			}
			if len(bs) == 1 {
				if h, ok := bs[0].(richdoc.Heading); ok {
					id, hoisted, err := p.hoistLabel()
					if err != nil {
						return nil, err
					}
					if hoisted {
						h.ID = id
						bs[0] = h
					}
				}
			}
			blocks = append(blocks, bs...)
			continue
		}
		nodes, err := p.parseInlineRun(modeBody)
		if err != nil {
			return nil, err
		}
		nodes = trimEdgeSpaces(nodes)
		if len(nodes) > 0 {
			blocks = append(blocks, richdoc.Paragraph{Inlines: nodes})
		}
	}
}

// handleBlockCommand consumes the block-level command at the cursor and returns
// the block(s) it yields (metadata and \maketitle yield none).
func (p *parser) handleBlockCommand() ([]richdoc.Block, error) {
	name, sym, word := p.readControl()
	if !word {
		if sym == '[' { // display math \[ ... \]
			tex, err := p.readDelimMath(']')
			if err != nil {
				return nil, err
			}
			return []richdoc.Block{richdoc.MathBlock{TeX: strings.TrimSpace(tex)}}, nil
		}
		return nil, nil
	}

	if lvl, ok := headingLevels[name]; ok {
		if p.peek() == '*' {
			p.pos++
		}
		if _, err := p.readOptArgRaw(); err != nil {
			return nil, err
		}
		inlines, err := p.readInlineArg()
		if err != nil {
			return nil, err
		}
		return []richdoc.Block{richdoc.Heading{Level: lvl, Inlines: trimEdgeSpaces(inlines)}}, nil
	}

	switch name {
	case "begin":
		env, err := p.readRawArg()
		if err != nil {
			return nil, err
		}
		return p.handleEnv(env)
	case "maketitle":
		return nil, nil
	case "title", "author", "date":
		v, err := p.readRawArg()
		if err != nil {
			return nil, err
		}
		p.meta[name] = v
		return nil, nil
	case "documentclass":
		if _, err := p.readOptArgRaw(); err != nil {
			return nil, err
		}
		v, err := p.readRawArg()
		if err != nil {
			return nil, err
		}
		setDocumentClass(p.meta, v)
		return nil, nil
	case "usepackage":
		if _, err := p.readOptArgRaw(); err != nil {
			return nil, err
		}
		if _, err := p.readRawArg(); err != nil {
			return nil, err
		}
		return nil, nil
	case "hrulefill":
		return []richdoc.Block{richdoc.ThematicBreak{}}, nil
	case "rule":
		if _, err := p.readOptArgRaw(); err != nil {
			return nil, err
		}
		if _, err := p.readRawArg(); err != nil {
			return nil, err
		}
		if _, err := p.readRawArg(); err != nil {
			return nil, err
		}
		return []richdoc.Block{richdoc.ThematicBreak{}}, nil
	}
	return nil, nil
}

// hoistLabel reports whether a \label{id} immediately follows the heading the
// cursor just passed (only whitespace and comments intervening) and, if so,
// consumes it and returns the id so the caller can attach it to Heading.ID. On
// no match the cursor is restored, so a following paragraph's leading content is
// never swallowed.
func (p *parser) hoistLabel() (id string, hoisted bool, err error) {
	save := p.pos
	p.skipLeading()
	if p.peek() != '\\' {
		p.pos = save
		return "", false, nil
	}
	name, _, word := p.readControl()
	if !word || name != "label" {
		p.pos = save
		return "", false, nil
	}
	id, err = p.readRawArg()
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

// handleEnv consumes the body of the environment named env (the cursor sits just
// after \begin{env}) and dispatches to the matching richdoc block.
func (p *parser) handleEnv(env string) ([]richdoc.Block, error) {
	inner, err := p.matchEnvBody(env)
	if err != nil {
		return nil, err
	}
	switch {
	case env == "itemize" || env == "enumerate":
		list, err := p.buildList(env == "enumerate", inner)
		if err != nil {
			return nil, err
		}
		return []richdoc.Block{list}, nil
	case env == "verbatim":
		return []richdoc.Block{richdoc.CodeBlock{Text: stripEnvNewlines(inner)}}, nil
	case env == "lstlisting":
		lang, code := parseListing(inner)
		return []richdoc.Block{richdoc.CodeBlock{Language: lang, Text: code}}, nil
	case env == "quote" || env == "quotation":
		blocks, err := parseBlocksRunes(inner, p.meta)
		if err != nil {
			return nil, err
		}
		return []richdoc.Block{richdoc.BlockQuote{Blocks: blocks}}, nil
	case env == "tabular":
		tbl, err := p.buildTable(inner)
		if err != nil {
			return nil, err
		}
		return []richdoc.Block{tbl}, nil
	case mathEnvs[env]:
		return []richdoc.Block{richdoc.MathBlock{TeX: strings.TrimSpace(string(inner))}}, nil
	default:
		raw := "\\begin{" + env + "}" + string(inner) + "\\end{" + env + "}"
		return []richdoc.Block{richdoc.RawBlock{Format: "latex", Text: raw}}, nil
	}
}

// matchEnvBody returns the raw inner runes of the environment named env and
// advances the cursor past the matching \end{env}, honouring nested
// environments. It errors if the environment is never closed.
func (p *parser) matchEnvBody(env string) ([]rune, error) {
	start := p.pos
	depth := 1
	for !p.eof() {
		c := p.peek()
		switch {
		case c == '%':
			for !p.eof() && p.peek() != '\n' {
				p.pos++
			}
		case c == '\\':
			save := p.pos
			name, _, word := p.readControl()
			switch {
			case word && name == "begin":
				depth++
			case word && name == "end":
				depth--
				if depth == 0 {
					inner := p.src[start:save]
					if _, err := p.readRawArg(); err != nil {
						return nil, err
					}
					return inner, nil
				}
			}
		default:
			p.pos++
		}
	}
	return nil, fmt.Errorf("latex: unclosed environment %q", env)
}

// --- inline scanning ---------------------------------------------------------

type inlineMode int

const (
	modeBody    inlineMode = iota // stops at a blank line or block command
	modeBounded                   // runs to end of the (sub-)source
	modeGroup                     // runs to the matching closing brace
)

// parseInlineRun scans inline content into richdoc inlines. The mode selects the
// stopping rule. Adjacent literal text is coalesced and interior whitespace runs
// collapse to a single space.
func (p *parser) parseInlineRun(mode inlineMode) ([]richdoc.Inline, error) {
	var nodes []richdoc.Inline
	var buf []rune
	flush := func() {
		if len(buf) > 0 {
			nodes = append(nodes, richdoc.Text{Value: string(buf)})
			buf = buf[:0]
		}
	}
	pushSpace := func() {
		if len(buf) == 0 || buf[len(buf)-1] != ' ' {
			buf = append(buf, ' ')
		}
	}

	for {
		if p.eof() {
			if mode == modeGroup {
				return nil, errUnmatchedBrace
			}
			break
		}
		if mode == modeBody && (p.atBlankLine() || p.atBlockCommand()) {
			break
		}
		c := p.peek()
		switch {
		case c == '}':
			if mode == modeGroup {
				p.pos++
				flush()
				return nodes, nil
			}
			return nil, errUnmatchedBrace
		case isSpace(c):
			for !p.eof() && isSpace(p.peek()) {
				p.pos++
			}
			pushSpace()
		case c == '%':
			for !p.eof() && p.peek() != '\n' {
				p.pos++
			}
		case c == '~':
			p.pos++
			pushSpace()
		case c == '$':
			flush()
			p.pos++
			tex, err := p.readDollarMath()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, richdoc.Math{TeX: strings.TrimSpace(tex)})
		case c == '{':
			flush()
			p.pos++
			inner, err := p.parseInlineRun(modeGroup)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, inner...)
		case c == '\\':
			n, lit, err := p.parseInlineControl()
			if err != nil {
				return nil, err
			}
			if lit != 0 {
				buf = append(buf, lit)
				continue
			}
			if n != nil {
				flush()
				nodes = append(nodes, n...)
			}
		default:
			buf = append(buf, c)
			p.pos++
		}
	}
	flush()
	return nodes, nil
}

// parseInlineControl handles a control sequence in inline context. It returns
// either a literal rune to append to the surrounding text (lit != 0) or zero or
// more inline nodes.
func (p *parser) parseInlineControl() (nodes []richdoc.Inline, lit rune, err error) {
	name, sym, word := p.readControl()
	if word {
		switch name {
		case "textbf":
			in, err := p.readInlineArg()
			if err != nil {
				return nil, 0, err
			}
			return []richdoc.Inline{richdoc.Strong{Inlines: in}}, 0, nil
		case "textit", "emph":
			in, err := p.readInlineArg()
			if err != nil {
				return nil, 0, err
			}
			return []richdoc.Inline{richdoc.Emph{Inlines: in}}, 0, nil
		case "texttt":
			in, err := p.readInlineArg()
			if err != nil {
				return nil, 0, err
			}
			return []richdoc.Inline{richdoc.Code{Value: strings.TrimSpace(flattenText(in))}}, 0, nil
		case "sout":
			in, err := p.readInlineArg()
			if err != nil {
				return nil, 0, err
			}
			return []richdoc.Inline{richdoc.Strikethrough{Inlines: in}}, 0, nil
		case "href":
			url, err := p.readRawArg()
			if err != nil {
				return nil, 0, err
			}
			text, err := p.readInlineArg()
			if err != nil {
				return nil, 0, err
			}
			return []richdoc.Inline{richdoc.Link{URL: unescapeURL(url), Inlines: text}}, 0, nil
		case "url":
			url, err := p.readRawArg()
			if err != nil {
				return nil, 0, err
			}
			u := unescapeURL(url)
			return []richdoc.Inline{richdoc.Link{URL: u, Inlines: []richdoc.Inline{richdoc.Text{Value: u}}}}, 0, nil
		case "includegraphics":
			if _, err := p.readOptArgRaw(); err != nil {
				return nil, 0, err
			}
			path, err := p.readRawArg()
			if err != nil {
				return nil, 0, err
			}
			return []richdoc.Inline{richdoc.Image{URL: unescapeURL(path)}}, 0, nil
		case "textbackslash":
			return nil, '\\', nil
		case "textasciitilde":
			return nil, '~', nil
		case "textasciicircum":
			return nil, '^', nil
		case "newline":
			return []richdoc.Inline{richdoc.LineBreak{}}, 0, nil
		case "footnote":
			// A \footnote's argument is inline, but the model's footnote body is
			// block-level, so wrap the parsed inlines in a single Paragraph.
			in, err := p.readInlineArg()
			if err != nil {
				return nil, 0, err
			}
			return []richdoc.Inline{richdoc.Footnote{Blocks: []richdoc.Block{richdoc.Paragraph{Inlines: in}}}}, 0, nil
		case "label":
			// A \label not immediately after a heading is a point Anchor in the
			// inline stream (the heading-hoist case is handled in parseBlocks).
			id, err := p.readRawArg()
			if err != nil {
				return nil, 0, err
			}
			return []richdoc.Inline{richdoc.Anchor{ID: id}}, 0, nil
		case "ref", "eqref":
			// Both \ref and \eqref parse to RefLabel; Write emits \ref, so \eqref
			// round-trips as \ref (a benign normalisation, documented in README).
			id, err := p.readRawArg()
			if err != nil {
				return nil, 0, err
			}
			return []richdoc.Inline{richdoc.CrossRef{Target: id, Kind: richdoc.RefLabel}}, 0, nil
		case "cite":
			id, err := p.readRawArg()
			if err != nil {
				return nil, 0, err
			}
			return []richdoc.Inline{richdoc.CrossRef{Target: id, Kind: richdoc.RefCite}}, 0, nil
		default:
			raw, err := p.captureRawWord(name)
			if err != nil {
				return nil, 0, err
			}
			return []richdoc.Inline{richdoc.RawInline{Format: "latex", Text: raw}}, 0, nil
		}
	}

	switch sym {
	case '\\':
		return []richdoc.Inline{richdoc.LineBreak{}}, 0, nil
	case '(':
		tex, err := p.readDelimMath(')')
		if err != nil {
			return nil, 0, err
		}
		return []richdoc.Inline{richdoc.Math{TeX: strings.TrimSpace(tex)}}, 0, nil
	case '&', '%', '$', '#', '_', '{', '}':
		return nil, sym, nil
	case ' ':
		return nil, ' ', nil
	default:
		return []richdoc.Inline{richdoc.RawInline{Format: "latex", Text: "\\" + string(sym)}}, 0, nil
	}
}

// captureRawWord reconstructs the source of an unrecognized command \name plus
// any bracket/brace argument groups that immediately follow, for verbatim
// passthrough.
func (p *parser) captureRawWord(name string) (string, error) {
	raw := "\\" + name
	for !p.eof() {
		switch p.peek() {
		case '{':
			inner, err := p.captureGroupInner('{', '}')
			if err != nil {
				return "", err
			}
			raw += "{" + inner + "}"
		case '[':
			inner, err := p.captureGroupInner('[', ']')
			if err != nil {
				return "", err
			}
			raw += "[" + inner + "]"
		default:
			return raw, nil
		}
	}
	return raw, nil
}

// readDollarMath reads to the next unescaped '$' (already past the opener) and
// returns the raw math source.
func (p *parser) readDollarMath() (string, error) {
	start := p.pos
	for !p.eof() {
		c := p.peek()
		if c == '\\' {
			p.pos += 2
			continue
		}
		if c == '$' {
			tex := string(p.src[start:p.pos])
			p.pos++
			return tex, nil
		}
		p.pos++
	}
	return "", errors.New("latex: unclosed inline math ($)")
}

// readDelimMath reads to the closing \<close> (the cursor is past the opener)
// and returns the raw math source. It serves \( \) with close=')' and \[ \]
// with close=']'.
func (p *parser) readDelimMath(close rune) (string, error) {
	start := p.pos
	for !p.eof() {
		c := p.peek()
		if c == '\\' {
			if p.at(1) == close {
				tex := string(p.src[start:p.pos])
				p.pos += 2
				return tex, nil
			}
			p.pos += 2
			continue
		}
		p.pos++
	}
	if close == ']' {
		return "", errors.New(`latex: unclosed display math (\[ \])`)
	}
	return "", errors.New(`latex: unclosed inline math (\( \))`)
}

// readInlineArg reads a braced argument as inline nodes, or a single following
// token when the argument is not braced.
func (p *parser) readInlineArg() ([]richdoc.Inline, error) {
	p.skipInlineSpaces()
	if p.eof() {
		return nil, nil
	}
	if p.peek() == '{' {
		p.pos++
		return p.parseInlineRun(modeGroup)
	}
	if p.peek() == '\\' {
		return nil, nil
	}
	c := p.peek()
	p.pos++
	return []richdoc.Inline{richdoc.Text{Value: string(c)}}, nil
}

// readRawArg reads a braced argument verbatim, or a single following token when
// unbraced.
func (p *parser) readRawArg() (string, error) {
	p.skipInlineSpaces()
	if p.eof() {
		return "", nil
	}
	if p.peek() == '{' {
		return p.captureGroupInner('{', '}')
	}
	if p.peek() == '\\' {
		name, sym, word := p.readControl()
		if word {
			return "\\" + name, nil
		}
		return "\\" + string(sym), nil
	}
	c := p.peek()
	p.pos++
	return string(c), nil
}

// readOptArgRaw reads a following [optional] argument verbatim, or "" when none.
func (p *parser) readOptArgRaw() (string, error) {
	p.skipInlineSpaces()
	if !p.eof() && p.peek() == '[' {
		return p.captureGroupInner('[', ']')
	}
	return "", nil
}

// captureGroupInner consumes a balanced open..close group at the cursor and
// returns its interior verbatim, honouring nesting and backslash escapes.
func (p *parser) captureGroupInner(open, close rune) (string, error) {
	p.pos++ // consume open
	start := p.pos
	depth := 1
	for !p.eof() {
		c := p.peek()
		switch c {
		case '\\':
			p.pos += 2
			continue
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				inner := string(p.src[start:p.pos])
				p.pos++
				return inner, nil
			}
		}
		p.pos++
	}
	return "", errUnmatchedBrace
}

// --- environment helpers -----------------------------------------------------

func parseBlocksRunes(rs []rune, meta map[string]string) ([]richdoc.Block, error) {
	q := &parser{src: rs, meta: meta}
	return q.parseBlocks()
}

func parseInlinesRunes(rs []rune, meta map[string]string) ([]richdoc.Inline, error) {
	q := &parser{src: rs, meta: meta}
	return q.parseInlineRun(modeBounded)
}

// buildList splits an itemize/enumerate body into \item entries, each parsed as
// its own block sequence.
func (p *parser) buildList(ordered bool, inner []rune) (richdoc.Block, error) {
	items := splitItems(inner)
	out := make([]richdoc.ListItem, 0, len(items))
	for _, it := range items {
		it = stripLeadingOpt(it)
		blocks, err := parseBlocksRunes(it, p.meta)
		if err != nil {
			return nil, err
		}
		out = append(out, richdoc.ListItem{Blocks: blocks})
	}
	return richdoc.List{Ordered: ordered, Start: 1, Items: out}, nil
}

// buildTable parses a tabular body: an optional [pos], the {column spec}, then
// rows separated by \\ and cells by &.
func (p *parser) buildTable(inner []rune) (richdoc.Block, error) {
	q := &parser{src: inner, meta: p.meta}
	q.skipInlineSpaces()
	if _, err := q.readOptArgRaw(); err != nil {
		return nil, err
	}
	spec, err := q.readRawArg()
	if err != nil {
		return nil, err
	}
	body := stripRules(inner[q.pos:])
	align := specToAlign(spec)

	var rows [][]richdoc.Cell
	for _, row := range splitRows(body) {
		row = stripLeadingOpt(row)
		if isBlankRunes(row) {
			continue
		}
		cells := splitCells(row)
		rowCells := make([]richdoc.Cell, 0, len(cells))
		for _, cell := range cells {
			nodes, err := parseInlinesRunes([]rune(strings.TrimSpace(string(cell))), p.meta)
			if err != nil {
				return nil, err
			}
			rowCells = append(rowCells, richdoc.Cell{Inlines: nodes})
		}
		rows = append(rows, rowCells)
	}
	return richdoc.Table{Align: align, Rows: rows}, nil
}

func isBlankRunes(rs []rune) bool {
	for _, r := range rs {
		if !isSpace(r) {
			return false
		}
	}
	return true
}

// stripLeadingOpt removes a leading [optional] group (row spacing, \item label)
// from rs.
func stripLeadingOpt(rs []rune) []rune {
	i := 0
	for i < len(rs) && isSpace(rs[i]) {
		i++
	}
	if i >= len(rs) || rs[i] != '[' {
		return rs
	}
	depth := 0
	for j := i; j < len(rs); j++ {
		switch rs[j] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return rs[j+1:]
			}
		}
	}
	return rs
}

// specToAlign turns a tabular column spec (l, c, r, p{..}, | ...) into per-column
// alignments.
func specToAlign(spec string) []richdoc.Alignment {
	var out []richdoc.Alignment
	rs := []rune(spec)
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case 'l':
			out = append(out, richdoc.AlignLeft)
		case 'c':
			out = append(out, richdoc.AlignCenter)
		case 'r':
			out = append(out, richdoc.AlignRight)
		case 'p', 'm', 'b':
			out = append(out, richdoc.AlignLeft)
			// skip the following {width} group
			for i+1 < len(rs) && rs[i+1] != '{' {
				i++
			}
			if i+1 < len(rs) && rs[i+1] == '{' {
				i++
				depth := 0
				for i < len(rs) {
					if rs[i] == '{' {
						depth++
					} else if rs[i] == '}' {
						depth--
						if depth == 0 {
							break
						}
					}
					i++
				}
			}
		}
	}
	return out
}

// stripEnvNewlines removes a single leading and trailing newline from a verbatim
// body, which are artifacts of the \begin/\end lines.
func stripEnvNewlines(rs []rune) string {
	s := string(rs)
	s = strings.TrimPrefix(s, "\n")
	s = strings.TrimSuffix(s, "\n")
	return s
}

// parseListing extracts an optional [language=..] from an lstlisting body and
// returns the language and the remaining code.
func parseListing(rs []rune) (lang, code string) {
	rest := rs
	i := 0
	for i < len(rest) && isSpace(rest[i]) {
		i++
	}
	if i < len(rest) && rest[i] == '[' {
		depth := 0
		end := -1
		for j := i; j < len(rest); j++ {
			if rest[j] == '[' {
				depth++
			} else if rest[j] == ']' {
				depth--
				if depth == 0 {
					end = j
					break
				}
			}
		}
		if end >= 0 {
			opts := string(rest[i+1 : end])
			lang = listingLanguage(opts)
			rest = rest[end+1:]
		}
	}
	return lang, stripEnvNewlines(rest)
}

func listingLanguage(opts string) string {
	for _, part := range strings.Split(opts, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 && strings.TrimSpace(kv[0]) == "language" {
			return strings.TrimSpace(kv[1])
		}
	}
	return ""
}

// --- top-level splitters (brace/environment aware) ---------------------------

type tokKind int

const (
	tkChar tokKind = iota
	tkWord
	tkSym
	tkComment
)

// nextTok reads one lexical unit for the splitters: a comment, a control word,
// a control symbol, or a single character.
func nextTok(rs []rune, i int) (kind tokKind, name string, sym rune, next int) {
	c := rs[i]
	switch {
	case c == '%':
		j := i
		for j < len(rs) && rs[j] != '\n' {
			j++
		}
		return tkComment, "", 0, j
	case c == '\\':
		if i+1 < len(rs) && isASCIILetter(rs[i+1]) {
			j := i + 1
			for j < len(rs) && isASCIILetter(rs[j]) {
				j++
			}
			return tkWord, string(rs[i+1 : j]), 0, j
		}
		if i+1 < len(rs) {
			return tkSym, "", rs[i+1], i + 2
		}
		return tkSym, "", 0, i + 1
	default:
		return tkChar, "", c, i + 1
	}
}

// splitItems splits an itemize/enumerate body on top-level \item, dropping
// anything before the first \item.
func splitItems(rs []rune) [][]rune {
	var out [][]rune
	brace, env := 0, 0
	started := false
	segStart := 0
	i := 0
	for i < len(rs) {
		kind, name, sym, ni := nextTok(rs, i)
		switch kind {
		case tkChar:
			if sym == '{' {
				brace++
			} else if sym == '}' && brace > 0 {
				brace--
			}
		case tkWord:
			switch name {
			case "begin":
				env++
			case "end":
				if env > 0 {
					env--
				}
			case "item":
				if brace == 0 && env == 0 {
					if started {
						out = append(out, rs[segStart:i])
					}
					started = true
					segStart = ni
				}
			}
		}
		i = ni
	}
	if started {
		out = append(out, rs[segStart:])
	}
	return out
}

// splitRows splits a tabular body on top-level \\ row separators.
func splitRows(rs []rune) [][]rune {
	var out [][]rune
	brace, env := 0, 0
	segStart := 0
	i := 0
	for i < len(rs) {
		kind, name, sym, ni := nextTok(rs, i)
		switch kind {
		case tkChar:
			if sym == '{' {
				brace++
			} else if sym == '}' && brace > 0 {
				brace--
			}
		case tkWord:
			if name == "begin" {
				env++
			} else if name == "end" && env > 0 {
				env--
			}
		case tkSym:
			if sym == '\\' && brace == 0 && env == 0 {
				out = append(out, rs[segStart:i])
				segStart = ni
			}
		}
		i = ni
	}
	out = append(out, rs[segStart:])
	return out
}

// splitCells splits a table row on top-level & cell separators (an escaped \&
// is a symbol token and never separates).
func splitCells(rs []rune) [][]rune {
	var out [][]rune
	brace := 0
	segStart := 0
	i := 0
	for i < len(rs) {
		kind, _, sym, ni := nextTok(rs, i)
		switch kind {
		case tkChar:
			switch sym {
			case '{':
				brace++
			case '}':
				if brace > 0 {
					brace--
				}
			case '&':
				if brace == 0 {
					out = append(out, rs[segStart:i])
					segStart = ni
				}
			}
		}
		i = ni
	}
	out = append(out, rs[segStart:])
	return out
}

// stripRules removes horizontal-rule commands (\hline and booktabs rules) from a
// tabular body; \cline also drops its {span} argument.
func stripRules(rs []rune) []rune {
	drop := map[string]bool{
		"hline": true, "toprule": true, "midrule": true, "bottomrule": true,
	}
	var out []rune
	i := 0
	for i < len(rs) {
		kind, name, _, ni := nextTok(rs, i)
		if kind == tkWord && drop[name] {
			i = ni
			continue
		}
		if kind == tkWord && name == "cline" {
			i = ni
			for i < len(rs) && isSpace(rs[i]) {
				i++
			}
			if i < len(rs) && rs[i] == '{' {
				depth := 0
				for i < len(rs) {
					if rs[i] == '{' {
						depth++
					} else if rs[i] == '}' {
						depth--
						i++
						if depth == 0 {
							break
						}
						continue
					}
					i++
				}
			}
			continue
		}
		out = append(out, rs[i:ni]...)
		i = ni
	}
	return out
}

// --- small inline utilities --------------------------------------------------

// trimEdgeSpaces trims a leading space from the first text node and a trailing
// space from the last, dropping either node if it becomes empty. Interior
// spacing is preserved.
func trimEdgeSpaces(nodes []richdoc.Inline) []richdoc.Inline {
	if len(nodes) == 0 {
		return nodes
	}
	if t, ok := nodes[0].(richdoc.Text); ok {
		t.Value = strings.TrimLeft(t.Value, " ")
		if t.Value == "" {
			nodes = nodes[1:]
		} else {
			nodes[0] = t
		}
	}
	if len(nodes) == 0 {
		return nodes
	}
	last := len(nodes) - 1
	if t, ok := nodes[last].(richdoc.Text); ok {
		t.Value = strings.TrimRight(t.Value, " ")
		if t.Value == "" {
			nodes = nodes[:last]
		} else {
			nodes[last] = t
		}
	}
	return nodes
}

// flattenText concatenates the literal text carried by a slice of inlines,
// used to reduce a \texttt argument to a plain code string.
func flattenText(nodes []richdoc.Inline) string {
	var b strings.Builder
	for _, n := range nodes {
		switch v := n.(type) {
		case richdoc.Text:
			b.WriteString(v.Value)
		case richdoc.Code:
			b.WriteString(v.Value)
		case richdoc.Emph:
			b.WriteString(flattenText(v.Inlines))
		case richdoc.Strong:
			b.WriteString(flattenText(v.Inlines))
		case richdoc.Strikethrough:
			b.WriteString(flattenText(v.Inlines))
		case richdoc.Link:
			b.WriteString(flattenText(v.Inlines))
		}
	}
	return b.String()
}

// setDocumentClass records a document class, except "article", which is the
// implicit default [Write] emits. Skipping it keeps the parse-first round-trip
// exact (a source with no class and Write's injected \documentclass{article}
// both yield no metadata) while still capturing any non-default class.
func setDocumentClass(meta map[string]string, v string) {
	v = strings.TrimSpace(v)
	if v != "" && v != "article" {
		meta["documentclass"] = v
	}
}

// extractPreambleMeta mines \documentclass, \title, \author and \date from the
// preamble, ignoring everything else.
func extractPreambleMeta(rs []rune, meta map[string]string) error {
	q := &parser{src: rs, meta: meta}
	for !q.eof() {
		c := q.peek()
		if c == '%' {
			for !q.eof() && q.peek() != '\n' {
				q.pos++
			}
			continue
		}
		if c != '\\' {
			q.pos++
			continue
		}
		name, _, word := q.readControl()
		if !word {
			continue
		}
		switch name {
		case "documentclass":
			if _, err := q.readOptArgRaw(); err != nil {
				return err
			}
			v, err := q.readRawArg()
			if err != nil {
				return err
			}
			setDocumentClass(meta, v)
		case "title", "author", "date":
			v, err := q.readRawArg()
			if err != nil {
				return err
			}
			meta[name] = v
		}
	}
	return nil
}
