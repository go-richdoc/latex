// Copyright (c) the go-richdoc authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import (
	"reflect"
	"strings"
	"testing"

	"github.com/go-richdoc/richdoc"
)

func mustParse(t *testing.T, src string) *richdoc.Document {
	t.Helper()
	d, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", src, err)
	}
	return d
}

// --- round-trip: Parse(Write(Parse(src))) == Parse(src) ----------------------

func TestRoundTrip(t *testing.T) {
	cases := []string{
		``,
		`Just a paragraph.`,
		`Two words with  collapsed   spaces.`,
		`Hello \textbf{bold}, \emph{it}, \textit{it2}, \texttt{code}, \sout{strike}.`,
		"First paragraph.\n\nSecond paragraph.",
		`\section{One}
Body of one.

\subsection{Two}
\subsubsection{Three}
\paragraph{Four}
\subparagraph{Five}
Deep.`,
		`\section*{Starred}
\section[toc short]{With optional}`,
		`Line one \\ line two \newline line three.`,
		`\begin{itemize}
\item alpha
\item beta \textbf{b}
\end{itemize}`,
		`\begin{enumerate}
\item one
\item two
\end{enumerate}`,
		`\begin{itemize}
\item[*] custom label
\item outer
\begin{enumerate}
\item nested
\end{enumerate}
\end{itemize}`,
		`\begin{verbatim}
line 1
line 2
\end{verbatim}`,
		`\begin{lstlisting}[language=Go]
package main
func main() {}
\end{lstlisting}`,
		`\begin{lstlisting}
no language here
\end{lstlisting}`,
		`\begin{quote}
Quoted paragraph with \emph{stress}.
\end{quote}`,
		`\begin{quotation}
Longer quotation.
\end{quotation}`,
		`\begin{tabular}{l|c|r}
\hline
a & b & c \\
\hline
d & e & f \\
\hline
\end{tabular}`,
		`\begin{tabular}{p{3cm}c}
wide & narrow \\
\end{tabular}`,
		`Inline math $a^2 + b^2 = c^2$ and \(x_i\) here.`,
		`\[ \int_0^1 x \, dx = \frac{1}{2} \]`,
		`\begin{equation}
E = mc^2
\end{equation}`,
		`\begin{align*}
a &= b \\
c &= d
\end{align*}`,
		`A link \href{https://example.com/a\_b}{click here} and \url{https://ex.io/x} plain.`,
		`An image \includegraphics[width=2cm]{figure.pdf} inline.`,
		`Specials: a \& b, 50\% off, \#3, x\_y, \{ braces \}, ~ nbsp, \^{} caret, \textbackslash{} slash.`,
		`\hrulefill`,
		`\rule[2pt]{4cm}{0.4pt}`,
		`Unknown \foobar{arg}{arg2} and \baz stays raw.`,
		`\begin{center}
custom environment kept whole
\end{center}`,
		`\documentclass[12pt]{book}
\title{The Title}
\author{The Author}
\date{2026}
\begin{document}
\maketitle
Body text.
\end{document}`,
		`Text with a comment % this is dropped
continues here.`,
		`Trailing bold then space \textbf{x} `,
	}
	for _, src := range cases {
		d1 := mustParse(t, src)
		out, err := Write(d1)
		if err != nil {
			t.Fatalf("Write error for %q: %v", src, err)
		}
		d2, err := Parse(out)
		if err != nil {
			t.Fatalf("re-Parse error for %q: %v\n---\n%s", src, err, out)
		}
		if !reflect.DeepEqual(d1, d2) {
			t.Errorf("round-trip mismatch for:\n%s\n\nwritten:\n%s\n\nd1=%#v\n\nd2=%#v", src, out, d1, d2)
		}
	}
}

// --- Parse: structural assertions -------------------------------------------

func TestParseHeadingsAndMeta(t *testing.T) {
	d := mustParse(t, `\documentclass{article}
\title{T}\author{A}\date{D}
\begin{document}
\maketitle
\section{S}
\end{document}`)
	if got := d.Meta["title"]; got != "T" {
		t.Errorf("title = %q", got)
	}
	if got := d.Meta["author"]; got != "A" {
		t.Errorf("author = %q", got)
	}
	if got := d.Meta["date"]; got != "D" {
		t.Errorf("date = %q", got)
	}
	if _, ok := d.Meta["documentclass"]; ok {
		t.Errorf("documentclass=article should not be recorded, got %v", d.Meta)
	}
	if len(d.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1 (maketitle dropped)", len(d.Blocks))
	}
	h, ok := d.Blocks[0].(richdoc.Heading)
	if !ok || h.Level != 1 {
		t.Fatalf("block0 = %#v, want Heading level 1", d.Blocks[0])
	}
}

func TestParseNonDefaultClass(t *testing.T) {
	d := mustParse(t, `\documentclass{report}
\begin{document}
x
\end{document}`)
	if got := d.Meta["documentclass"]; got != "report" {
		t.Errorf("documentclass = %q, want report", got)
	}
}

func TestParseInlineTypes(t *testing.T) {
	d := mustParse(t, `\textbf{b} \emph{e} \texttt{c} \sout{s} \href{u}{h} \url{v} \includegraphics{i} $m$ \(n\) x\\y`)
	p := d.Blocks[0].(richdoc.Paragraph)
	var haveStrong, haveEmph, haveCode, haveStrike, haveLink, haveImage, haveMath, haveBreak bool
	for _, in := range p.Inlines {
		switch v := in.(type) {
		case richdoc.Strong:
			haveStrong = true
		case richdoc.Emph:
			haveEmph = true
		case richdoc.Code:
			haveCode = v.Value == "c"
		case richdoc.Strikethrough:
			haveStrike = true
		case richdoc.Link:
			haveLink = true
		case richdoc.Image:
			haveImage = v.URL == "i"
		case richdoc.Math:
			haveMath = true
		case richdoc.LineBreak:
			haveBreak = true
		}
	}
	if !(haveStrong && haveEmph && haveCode && haveStrike && haveLink && haveImage && haveMath && haveBreak) {
		t.Fatalf("missing inline types: %#v", p.Inlines)
	}
}

func TestParseUnknownRaw(t *testing.T) {
	d := mustParse(t, `before \mycmd[o]{a} after`)
	p := d.Blocks[0].(richdoc.Paragraph)
	found := ""
	for _, in := range p.Inlines {
		if r, ok := in.(richdoc.RawInline); ok {
			found = r.Text
		}
	}
	if found != `\mycmd[o]{a}` {
		t.Fatalf("raw inline = %q", found)
	}

	d = mustParse(t, `\begin{tikzpicture}
\draw (0,0) -- (1,1);
\end{tikzpicture}`)
	rb, ok := d.Blocks[0].(richdoc.RawBlock)
	if !ok || rb.Format != "latex" || !strings.Contains(rb.Text, `\draw`) {
		t.Fatalf("raw block = %#v", d.Blocks[0])
	}
}

func TestParseTableAlignment(t *testing.T) {
	d := mustParse(t, `\begin{tabular}{l|c|r}
a & b & c \\
\end{tabular}`)
	tbl := d.Blocks[0].(richdoc.Table)
	want := []richdoc.Alignment{richdoc.AlignLeft, richdoc.AlignCenter, richdoc.AlignRight}
	if !reflect.DeepEqual(tbl.Align, want) {
		t.Fatalf("align = %#v", tbl.Align)
	}
	if len(tbl.Rows) != 1 || len(tbl.Rows[0]) != 3 {
		t.Fatalf("rows = %#v", tbl.Rows)
	}
}

func TestParseNoDocumentEnv(t *testing.T) {
	// A bare fragment (no \begin{document}) is parsed whole.
	d := mustParse(t, `Just \textbf{text}.`)
	if len(d.Blocks) != 1 {
		t.Fatalf("blocks = %d", len(d.Blocks))
	}
}

func TestParseDocumentWithoutEnd(t *testing.T) {
	d := mustParse(t, `\begin{document}
Body without a proper end.`)
	if len(d.Blocks) != 1 {
		t.Fatalf("blocks = %d", len(d.Blocks))
	}
}

func TestParseCRLF(t *testing.T) {
	d := mustParse(t, "line one\r\n\r\nline two\r")
	if len(d.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(d.Blocks))
	}
}

// --- Parse: error branches ---------------------------------------------------

func TestParseErrors(t *testing.T) {
	bad := map[string]string{
		"unmatched brace":       `text with an unmatched } brace`,
		"unclosed group":        `\textbf{never closed`,
		"unclosed env":          `\begin{itemize}\item x`,
		"unclosed inline $":     `math $x+y that never ends`,
		"unclosed inline paren": `math \(x+y that never ends`,
		"unclosed display":      `\[ x + y with no close`,
		"unclosed href url":     `\href{http://x`,
		"unclosed href text":    `\href{u}{text`,
		"unclosed emph":         `\emph{oops`,
		"unclosed texttt":       `\texttt{oops`,
		"unclosed sout":         `\sout{oops`,
		"unclosed url":          `\url{oops`,
		"unclosed graphics":     `\includegraphics{oops`,
		"unclosed raw cmd":      `\unknown{oops`,
		"bad env end brace":     `\begin{foo}body\end{foo`,
		"bad table cell":        `\begin{tabular}{l} a}b \\ \end{tabular}`,
		"bad quote body":        `\begin{quote} bad } brace \end{quote}`,
		"preamble bad title":    "\\title{oops\n\\begin{document}\nx\n\\end{document}",
		"preamble docclass opt": "\\documentclass[oops\n\\begin{document}\nx\n\\end{document}",
		"preamble docclass arg": "\\documentclass{oops\n\\begin{document}\nx\n\\end{document}",
	}
	for name, src := range bad {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("%s: expected error, got nil for %q", name, src)
		}
	}
}

func TestParseEmptyAndWhitespace(t *testing.T) {
	for _, src := range []string{``, `   `, "\n\n\n", `% only a comment`} {
		d := mustParse(t, src)
		if len(d.Blocks) != 0 {
			t.Errorf("%q: blocks = %d, want 0", src, len(d.Blocks))
		}
		if d.Meta != nil {
			t.Errorf("%q: meta = %v, want nil", src, d.Meta)
		}
	}
}

func TestParseHeadingEmptyArg(t *testing.T) {
	// A heading whose argument is only spaces yields empty inlines.
	d := mustParse(t, `\section{ }`)
	h := d.Blocks[0].(richdoc.Heading)
	if len(h.Inlines) != 0 {
		t.Fatalf("inlines = %#v, want empty", h.Inlines)
	}
}

// --- Write: assertions -------------------------------------------------------

func TestWriteNil(t *testing.T) {
	out, err := Write(nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `\documentclass{article}`) || !strings.Contains(s, `\begin{document}`) {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestWritePackages(t *testing.T) {
	d := richdoc.New().
		P(richdoc.Href("u", "", richdoc.Txt("l")), richdoc.InlineMath("x"), richdoc.Strike(richdoc.Txt("s")), richdoc.Img("i.png", "", "")).
		CodeBlock("Go", "code").
		MathBlock("y").
		Doc()
	out, _ := Write(d)
	s := string(out)
	for _, pkg := range []string{"graphicx", "hyperref", "amsmath", "ulem", "listings"} {
		if !strings.Contains(s, "{"+pkg+"}") && !strings.Contains(s, "]{"+pkg+"}") {
			t.Errorf("missing package %s in:\n%s", pkg, s)
		}
	}
}

func TestWriteMetaAndClass(t *testing.T) {
	d := richdoc.New().Meta("documentclass", "book").Meta("title", "T").Meta("author", "A").Meta("date", "D").
		P(richdoc.Txt("x")).Doc()
	out, _ := Write(d)
	s := string(out)
	for _, want := range []string{`\documentclass{book}`, `\title{T}`, `\author{A}`, `\date{D}`, `\maketitle`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}

func TestWriteTableHeader(t *testing.T) {
	d := richdoc.New().Table(
		[]richdoc.Alignment{richdoc.AlignLeft, richdoc.AlignRight},
		[]richdoc.Cell{richdoc.Td(richdoc.Txt("H1")), richdoc.Td(richdoc.Txt("H2"))},
		[][]richdoc.Cell{{richdoc.Td(richdoc.Txt("a")), richdoc.Td(richdoc.Txt("b"))}},
	).Doc()
	out, _ := Write(d)
	s := string(out)
	if !strings.Contains(s, `\begin{tabular}{lr}`) || !strings.Contains(s, `\hline`) || !strings.Contains(s, "H1 & H2") {
		t.Fatalf("table header output:\n%s", s)
	}
}

func TestWriteTableAlignDefaultAndCenter(t *testing.T) {
	d := richdoc.New().Table(
		[]richdoc.Alignment{richdoc.AlignCenter}, // one align, two columns -> second defaults to l
		nil,
		[][]richdoc.Cell{{richdoc.Td(richdoc.Txt("a")), richdoc.Td(richdoc.Txt("b"))}},
	).Doc()
	out, _ := Write(d)
	if !strings.Contains(string(out), `\begin{tabular}{cl}`) {
		t.Fatalf("spec output:\n%s", out)
	}
}

func TestWriteHeadingClamp(t *testing.T) {
	d := richdoc.New().H(9, richdoc.Txt("deep")).H(0, richdoc.Txt("shallow")).Doc()
	out := string(mustWrite(t, d))
	if !strings.Contains(out, `\subparagraph{deep}`) {
		t.Errorf("level>5 should clamp to subparagraph:\n%s", out)
	}
	if !strings.Contains(out, `\section{shallow}`) {
		t.Errorf("level<1 should clamp to section:\n%s", out)
	}
}

func TestWriteThematicBreakAndLineBreak(t *testing.T) {
	d := richdoc.New().HR().P(richdoc.Txt("a"), richdoc.Br(), richdoc.Txt("b")).Doc()
	out := string(mustWrite(t, d))
	if !strings.Contains(out, `\hrulefill`) || !strings.Contains(out, `a\\b`) {
		t.Fatalf("output:\n%s", out)
	}
}

func TestWriteNestedItem(t *testing.T) {
	d := richdoc.New().UList(false,
		richdoc.Item(
			richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("lead")}},
			richdoc.BlockQuote{Blocks: []richdoc.Block{richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("q")}}}},
		),
	).Doc()
	out := string(mustWrite(t, d))
	if !strings.Contains(out, `\item lead`) || !strings.Contains(out, `\begin{quote}`) {
		t.Fatalf("nested item output:\n%s", out)
	}
}

func TestWriteRawBlockFormats(t *testing.T) {
	d := richdoc.New().
		RawBlock("latex", `\customlatex`).
		RawBlock("html", `<p>dropped</p>`).
		Add(richdoc.Paragraph{Inlines: []richdoc.Inline{
			richdoc.RawI("latex", `\keepinline`),
			richdoc.RawI("html", `<b>drop</b>`),
		}}).
		Doc()
	out := string(mustWrite(t, d))
	if !strings.Contains(out, `\customlatex`) || strings.Contains(out, "dropped") {
		t.Errorf("raw block format handling:\n%s", out)
	}
	if !strings.Contains(out, `\keepinline`) || strings.Contains(out, "<b>drop</b>") {
		t.Errorf("raw inline format handling:\n%s", out)
	}
}

func TestWriteEmptyFormatRaw(t *testing.T) {
	d := richdoc.New().RawBlock("", `\emptyfmt`).
		Add(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.RawI("", `\emptyinline`)}}).Doc()
	out := string(mustWrite(t, d))
	if !strings.Contains(out, `\emptyfmt`) || !strings.Contains(out, `\emptyinline`) {
		t.Fatalf("empty-format raw should be emitted:\n%s", out)
	}
}

func mustWrite(t *testing.T, d *richdoc.Document) []byte {
	t.Helper()
	out, err := Write(d)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	return out
}

// --- escaping ----------------------------------------------------------------

func TestEscapeText(t *testing.T) {
	in := `\ & % $ # _ { } ~ ^ plain`
	got := escapeText(in)
	want := `\textbackslash{} \& \% \$ \# \_ \{ \} \textasciitilde{} \textasciicircum{} plain`
	if got != want {
		t.Fatalf("escapeText = %q, want %q", got, want)
	}
}

func TestEscapeURLRoundTrip(t *testing.T) {
	for _, u := range []string{
		`http://example.com/a_b`,
		`http://x/%20#frag&q=1_2{}`,
		`http://plain.example/path?a=1`,
		`weird\backslash`,
		`trailing\`,
	} {
		if got := unescapeURL(escapeURL(u)); got != u {
			t.Errorf("URL round-trip: %q -> %q", u, got)
		}
	}
}

// --- white-box helper coverage ----------------------------------------------

func TestReadRawArgVariants(t *testing.T) {
	// braced
	p := &parser{src: []rune(`{hello}`)}
	if v, err := p.readRawArg(); err != nil || v != "hello" {
		t.Fatalf("braced: %q %v", v, err)
	}
	// unbraced control word
	p = &parser{src: []rune(`\foo rest`)}
	if v, err := p.readRawArg(); err != nil || v != `\foo` {
		t.Fatalf("control word: %q %v", v, err)
	}
	// unbraced control symbol
	p = &parser{src: []rune(`\@x`)}
	if v, err := p.readRawArg(); err != nil || v != `\@` {
		t.Fatalf("control symbol: %q %v", v, err)
	}
	// unbraced single char
	p = &parser{src: []rune(`Z more`)}
	if v, err := p.readRawArg(); err != nil || v != "Z" {
		t.Fatalf("single char: %q %v", v, err)
	}
	// eof
	p = &parser{src: []rune(``)}
	if v, err := p.readRawArg(); err != nil || v != "" {
		t.Fatalf("eof: %q %v", v, err)
	}
}

func TestReadInlineArgVariants(t *testing.T) {
	// unbraced single char
	p := &parser{src: []rune(`x`)}
	nodes, err := p.readInlineArg()
	if err != nil || len(nodes) != 1 {
		t.Fatalf("single char: %#v %v", nodes, err)
	}
	// backslash after spaces -> nil (no arg consumed)
	p = &parser{src: []rune(`  \emph{y}`)}
	nodes, err = p.readInlineArg()
	if err != nil || nodes != nil {
		t.Fatalf("backslash arg: %#v %v", nodes, err)
	}
	// eof
	p = &parser{src: []rune(``)}
	nodes, err = p.readInlineArg()
	if err != nil || nodes != nil {
		t.Fatalf("eof: %#v %v", nodes, err)
	}
}

func TestReadOptArgError(t *testing.T) {
	p := &parser{src: []rune(`[never closed`)}
	if _, err := p.readOptArgRaw(); err == nil {
		t.Fatal("expected error for unclosed optional arg")
	}
}

func TestHandleBlockCommandFallThrough(t *testing.T) {
	// A non-block control word reaches the trailing no-op return.
	p := &parser{src: []rune(`\notacommand`), meta: map[string]string{}}
	blocks, err := p.handleBlockCommand()
	if err != nil || blocks != nil {
		t.Fatalf("word fall-through: %#v %v", blocks, err)
	}
	// A non-'[' control symbol also yields no block.
	p = &parser{src: []rune(`\,`), meta: map[string]string{}}
	blocks, err = p.handleBlockCommand()
	if err != nil || blocks != nil {
		t.Fatalf("symbol fall-through: %#v %v", blocks, err)
	}
}

func TestCaptureRawWordErrors(t *testing.T) {
	p := &parser{src: []rune(`\cmd{unclosed`)}
	if _, _, err := p.parseInlineControl(); err == nil {
		t.Fatal("expected error for unclosed brace arg of raw command")
	}
	p = &parser{src: []rune(`\cmd[unclosed`)}
	if _, _, err := p.parseInlineControl(); err == nil {
		t.Fatal("expected error for unclosed bracket arg of raw command")
	}
}

func TestReadDelimMathParenMessage(t *testing.T) {
	p := &parser{src: []rune(`x + y`)}
	_, err := p.readDelimMath(')')
	if err == nil || !strings.Contains(err.Error(), `\(`) {
		t.Fatalf("paren math error = %v", err)
	}
}

func TestListingHelpers(t *testing.T) {
	if got := listingLanguage("frame=single,language=Python,label=x"); got != "Python" {
		t.Errorf("language = %q", got)
	}
	if got := listingLanguage("frame=single"); got != "" {
		t.Errorf("no language: %q", got)
	}
	if got := listingLanguage("bareword"); got != "" {
		t.Errorf("bareword: %q", got)
	}
	// options block with no closing bracket: language is not extracted.
	lang, code := parseListing([]rune("[language=Go without close\ncode"))
	if lang != "" || !strings.Contains(code, "language=Go") {
		t.Errorf("unterminated opts: lang=%q code=%q", lang, code)
	}
}

func TestStripRulesCline(t *testing.T) {
	got := string(stripRules([]rune(`\hline a \cline{1-2} b \midrule c \cline{{n}}`)))
	if strings.Contains(got, "hline") || strings.Contains(got, "cline") || strings.Contains(got, "midrule") {
		t.Fatalf("rules not stripped: %q", got)
	}
	if !strings.Contains(got, "a") || !strings.Contains(got, "b") || !strings.Contains(got, "c") {
		t.Fatalf("content lost: %q", got)
	}
	// \cline with no following brace must not hang or eat content.
	got = string(stripRules([]rune(`\cline x`)))
	if !strings.Contains(got, "x") {
		t.Fatalf("cline no-brace: %q", got)
	}
}

func TestNextTokSymbolAtEOF(t *testing.T) {
	rs := []rune(`\`)
	kind, _, sym, next := nextTok(rs, 0)
	if kind != tkSym || sym != 0 || next != 1 {
		t.Fatalf("trailing backslash: kind=%d sym=%q next=%d", kind, sym, next)
	}
}

func TestSpecToAlignEdges(t *testing.T) {
	got := specToAlign(`l c r | p{2cm} m{1cm} b x`)
	want := []richdoc.Alignment{
		richdoc.AlignLeft, richdoc.AlignCenter, richdoc.AlignRight,
		richdoc.AlignLeft, richdoc.AlignLeft, richdoc.AlignLeft,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("specToAlign = %#v", got)
	}
}

func TestTrimEdgeSpacesLastNode(t *testing.T) {
	// Leading non-text node, trailing text node reduced to empty -> dropped.
	nodes := []richdoc.Inline{richdoc.Strong{Inlines: []richdoc.Inline{richdoc.Txt("x")}}, richdoc.Txt("   ")}
	got := trimEdgeSpaces(nodes)
	if len(got) != 1 {
		t.Fatalf("trailing space node not dropped: %#v", got)
	}
	// Empty input.
	if got := trimEdgeSpaces(nil); got != nil {
		t.Fatalf("nil: %#v", got)
	}
}

func TestFlattenText(t *testing.T) {
	nodes := []richdoc.Inline{
		richdoc.Txt("a"),
		richdoc.Bold(richdoc.Txt("b")),
		richdoc.Italic(richdoc.Txt("c")),
		richdoc.Strike(richdoc.Txt("d")),
		richdoc.Href("u", "", richdoc.Txt("e")),
		richdoc.Mono("f"),
		richdoc.InlineMath("ignored"),
		richdoc.Img("i", "", ""),
	}
	if got := flattenText(nodes); got != "abcdef" {
		t.Fatalf("flattenText = %q", got)
	}
}

func TestDollarMathEscapedDollar(t *testing.T) {
	d := mustParse(t, `price $\$5$ end`)
	p := d.Blocks[0].(richdoc.Paragraph)
	var tex string
	for _, in := range p.Inlines {
		if m, ok := in.(richdoc.Math); ok {
			tex = m.TeX
		}
	}
	if tex != `\$5` {
		t.Fatalf("math tex = %q, want %q", tex, `\$5`)
	}
}
