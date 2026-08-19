// Copyright (c) the go-richdoc authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import (
	"reflect"
	"strings"
	"testing"

	"github.com/go-richdoc/richdoc"
)

// --- body-level metadata and commands ---------------------------------------

func TestBodyLevelMetaCommands(t *testing.T) {
	// With no document environment the whole source is the body, so \title,
	// \author, \date, \documentclass, \usepackage and \maketitle are handled by
	// the block command dispatcher rather than the preamble scanner.
	d := mustParse(t, `\title{BodyTitle}
\author{BodyAuthor}
\date{BodyDate}
\documentclass{report}
\usepackage[utf8]{inputenc}
\maketitle
Actual body.`)
	if d.Meta["title"] != "BodyTitle" || d.Meta["author"] != "BodyAuthor" || d.Meta["date"] != "BodyDate" {
		t.Fatalf("meta = %#v", d.Meta)
	}
	if d.Meta["documentclass"] != "report" {
		t.Fatalf("documentclass = %q", d.Meta["documentclass"])
	}
	if len(d.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(d.Blocks))
	}
}

func TestLiteralTildeAndCaretCommands(t *testing.T) {
	d := mustParse(t, `a \textasciitilde b \textasciicircum c \textbackslash d`)
	p := d.Blocks[0].(richdoc.Paragraph)
	got := ""
	for _, in := range p.Inlines {
		if txt, ok := in.(richdoc.Text); ok {
			got += txt.Value
		}
	}
	if !strings.Contains(got, "~") || !strings.Contains(got, "^") || !strings.Contains(got, "\\") {
		t.Fatalf("literals not decoded: %q", got)
	}
}

func TestBackslashSpace(t *testing.T) {
	d := mustParse(t, `a\ b`)
	if txt := richdoc.PlainText(d); txt != "a b" {
		t.Fatalf("plain = %q, want %q", txt, "a b")
	}
}

func TestUnknownCommandAtEOF(t *testing.T) {
	// A raw command whose braced argument ends the source exercises the
	// end-of-input exit of captureRawWord.
	d := mustParse(t, `\customcmd{arg}`)
	p := d.Blocks[0].(richdoc.Paragraph)
	r := p.Inlines[0].(richdoc.RawInline)
	if r.Text != `\customcmd{arg}` {
		t.Fatalf("raw = %q", r.Text)
	}
}

func TestEnvironmentWithComment(t *testing.T) {
	// A comment inside an environment body exercises matchEnvBody's comment skip.
	d := mustParse(t, `\begin{tikzpicture}
\draw (0,0); % a comment inside
\end{tikzpicture}`)
	rb := d.Blocks[0].(richdoc.RawBlock)
	if !strings.Contains(rb.Text, "a comment inside") {
		t.Fatalf("comment not preserved: %q", rb.Text)
	}
}

func TestTableCellWithBracesAndAmpersand(t *testing.T) {
	// A grouped cell keeps its interior ampersand from splitting the row.
	src := `\begin{tabular}{l}
{a & b} \\
\end{tabular}`
	d := mustParse(t, src)
	tbl := d.Blocks[0].(richdoc.Table)
	if len(tbl.Rows) != 1 || len(tbl.Rows[0]) != 1 {
		t.Fatalf("rows = %#v", tbl.Rows)
	}
	out := mustWrite(t, d)
	d2 := mustParse(t, string(out))
	if !reflect.DeepEqual(d, d2) {
		t.Fatalf("round-trip mismatch:\n%s", out)
	}
}

// --- additional Parse error branches ----------------------------------------

func TestMoreParseErrors(t *testing.T) {
	bad := map[string]string{
		"bare group unclosed":  `text {never closed`,
		"list item bad brace":  `\begin{itemize}\item a}b\end{itemize}`,
		"tabular opt unclosed": `\begin{tabular}[x\end{tabular}`,
		"tabular spec bad":     `\begin{tabular}{x\end{tabular}`,
		"heading arg unclosed": `\section{oops`,
		"heading opt unclosed": `\section[oops`,
		"begin name unclosed":  `\begin{oops`,
		"graphics opt bad":     `x \includegraphics[oops`,
		"inline paren eof bs":  `math \(a\`,
	}
	for name, src := range bad {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("%s: expected error for %q", name, src)
		}
	}
}

// --- white-box helper branch coverage ---------------------------------------

func TestAtOutOfRange(t *testing.T) {
	p := &parser{src: []rune("x")}
	if p.at(-1) != 0 || p.at(5) != 0 {
		t.Fatal("at out of range should be 0")
	}
}

func TestReadControlEOF(t *testing.T) {
	p := &parser{src: []rune(`\`)}
	name, sym, word := p.readControl()
	if name != "" || sym != 0 || word {
		t.Fatalf("trailing backslash: name=%q sym=%q word=%v", name, sym, word)
	}
}

func TestHandleBlockCommandErrorBranches(t *testing.T) {
	mk := func(s string) *parser { return &parser{src: []rune(s), meta: map[string]string{}} }
	cases := []string{
		`\rule[x`,    // optional-arg error
		`\rule{x`,    // first mandatory-arg error
		`\rule{a}{b`, // second mandatory-arg error
		`\usepackage[x`,
		`\usepackage{x`,
		`\documentclass[x`,
		`\documentclass{x`,
		`\title{x`,
	}
	for _, s := range cases {
		p := mk(s)
		if _, err := p.handleBlockCommand(); err == nil {
			t.Errorf("%q: expected error", s)
		}
	}
}

func TestStripLeadingOptUnclosed(t *testing.T) {
	got := stripLeadingOpt([]rune("[never closed"))
	if string(got) != "[never closed" {
		t.Fatalf("unclosed opt should be returned as-is: %q", string(got))
	}
	// No optional argument at all.
	got = stripLeadingOpt([]rune("plain"))
	if string(got) != "plain" {
		t.Fatalf("no opt: %q", string(got))
	}
}

func TestNextTokKinds(t *testing.T) {
	rs := []rune(`\word\@ a%c`)
	kind, name, _, ni := nextTok(rs, 0)
	if kind != tkWord || name != "word" {
		t.Fatalf("word: %d %q", kind, name)
	}
	kind, _, sym, ni2 := nextTok(rs, ni)
	if kind != tkSym || sym != '@' {
		t.Fatalf("sym: %d %q", kind, sym)
	}
	kind, _, sym, ni3 := nextTok(rs, ni2)
	if kind != tkChar || sym != ' ' {
		t.Fatalf("char: %d %q", kind, sym)
	}
	kind, _, _, _ = nextTok(rs, ni3)
	if kind != tkChar { // 'a'
		t.Fatalf("char a: %d", kind)
	}
	kind, _, _, _ = nextTok(rs, ni3+1)
	if kind != tkComment {
		t.Fatalf("comment: %d", kind)
	}
}

func TestSplitRowsBraceAndEnv(t *testing.T) {
	// A \\ inside braces or a nested environment must not split the row.
	rows := splitRows([]rune(`a {x \\ y} b \\ \begin{z} p \\ q \end{z} \\ tail`))
	if len(rows) != 3 {
		t.Fatalf("rows = %d (%q)", len(rows), rowsToStrings(rows))
	}
}

func TestSplitCellsBrace(t *testing.T) {
	cells := splitCells([]rune(`{a & b} & c & {d}`))
	if len(cells) != 3 {
		t.Fatalf("cells = %d (%q)", len(cells), rowsToStrings(cells))
	}
}

func TestSplitItemsNesting(t *testing.T) {
	// \item inside braces or a nested environment does not start a new entry.
	items := splitItems([]rune(`\item one {\item still one} \item two \begin{itemize}\item nested\end{itemize}`))
	if len(items) != 2 {
		t.Fatalf("items = %d (%q)", len(items), rowsToStrings(items))
	}
}

func rowsToStrings(rows [][]rune) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = string(r)
	}
	return out
}

// --- Write table column-count sources ---------------------------------------

func TestWriteTableNcolsFromRows(t *testing.T) {
	// No alignment given; column count derives from the widest row.
	d := richdoc.New().Table(nil, nil, [][]richdoc.Cell{
		{richdoc.Td(richdoc.Txt("a")), richdoc.Td(richdoc.Txt("b"))},
	}).Doc()
	if !strings.Contains(string(mustWrite(t, d)), `\begin{tabular}{ll}`) {
		t.Fatal("column count should come from rows")
	}
}

func TestWriteTableNcolsFromHeader(t *testing.T) {
	// One alignment, a two-column header: column count comes from the header.
	d := richdoc.New().Table(
		[]richdoc.Alignment{richdoc.AlignRight},
		[]richdoc.Cell{richdoc.Td(richdoc.Txt("H1")), richdoc.Td(richdoc.Txt("H2"))},
		nil,
	).Doc()
	if !strings.Contains(string(mustWrite(t, d)), `\begin{tabular}{rl}`) {
		t.Fatalf("column count should come from header:\n%s", mustWrite(t, d))
	}
}

// --- preamble scanner branches ----------------------------------------------

func TestPreambleCommentAndSymbol(t *testing.T) {
	// A comment and a control symbol in the preamble are skipped; the class is
	// still captured.
	d := mustParse(t, `% a preamble comment
\documentclass[12pt]{memoir}
\\
\title{PT}
\begin{document}
body
\end{document}`)
	if d.Meta["documentclass"] != "memoir" || d.Meta["title"] != "PT" {
		t.Fatalf("meta = %#v", d.Meta)
	}
}
