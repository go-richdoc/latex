// Copyright (c) the go-richdoc authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import (
	"strconv"
	"strings"

	"github.com/go-richdoc/richdoc"
)

// Write renders a [richdoc.Document] as a minimal, self-contained and
// compilable LaTeX article. It loads only the packages the document actually
// needs (graphicx, hyperref, amsmath, ulem, listings), maps Meta title/author/
// date onto \title/\author/\date + \maketitle, and escapes LaTeX specials in
// text. RawBlock/RawInline with Format "latex" are emitted verbatim; raw nodes
// for other formats are dropped.
//
// Write is the inverse of [Parse] over the supported subset: parsing Write's
// output reproduces the input tree.
func Write(d *richdoc.Document) ([]byte, error) {
	if d == nil {
		d = &richdoc.Document{}
	}
	n := scanNeeds(d)

	var b strings.Builder
	class := "article"
	if c := metaVal(d, "documentclass"); c != "" {
		class = c
	}
	b.WriteString("\\documentclass{" + class + "}\n")
	b.WriteString("\\usepackage[T1]{fontenc}\n")
	if n.graphics {
		b.WriteString("\\usepackage{graphicx}\n")
	}
	if n.hyperref {
		b.WriteString("\\usepackage{hyperref}\n")
	}
	if n.amsmath {
		b.WriteString("\\usepackage{amsmath}\n")
	}
	if n.ulem {
		b.WriteString("\\usepackage[normalem]{ulem}\n")
	}
	if n.listings {
		b.WriteString("\\usepackage{listings}\n")
	}

	title := metaVal(d, "title")
	if title != "" {
		b.WriteString("\\title{" + title + "}\n")
	}
	if a := metaVal(d, "author"); a != "" {
		b.WriteString("\\author{" + a + "}\n")
	}
	if dt := metaVal(d, "date"); dt != "" {
		b.WriteString("\\date{" + dt + "}\n")
	}

	b.WriteString("\\begin{document}\n")
	if title != "" {
		b.WriteString("\\maketitle\n")
	}

	parts := make([]string, 0, len(d.Blocks))
	for _, blk := range d.Blocks {
		parts = append(parts, writeBlock(blk))
	}
	b.WriteString(strings.Join(parts, "\n\n"))
	if len(parts) > 0 {
		b.WriteString("\n")
	}
	b.WriteString("\\end{document}\n")
	return []byte(b.String()), nil
}

func metaVal(d *richdoc.Document, key string) string {
	if d.Meta == nil {
		return ""
	}
	return d.Meta[key]
}

type needs struct {
	graphics bool
	hyperref bool
	amsmath  bool
	ulem     bool
	listings bool
}

type needsVisitor struct{ n *needs }

func (v needsVisitor) Enter(node any) bool {
	switch b := node.(type) {
	case richdoc.Image:
		v.n.graphics = true
	case richdoc.Link:
		v.n.hyperref = true
	case richdoc.Math:
		v.n.amsmath = true
	case richdoc.MathBlock:
		v.n.amsmath = true
	case richdoc.Strikethrough:
		v.n.ulem = true
	case richdoc.CodeBlock:
		if b.Language != "" {
			v.n.listings = true
		}
	}
	return true
}

func (v needsVisitor) Leave(any) {}

func scanNeeds(d *richdoc.Document) needs {
	var n needs
	richdoc.Walk(d, needsVisitor{n: &n})
	return n
}

var headingCmd = map[int]string{
	1: "section", 2: "subsection", 3: "subsubsection",
	4: "paragraph", 5: "subparagraph",
}

func writeBlock(blk richdoc.Block) string {
	switch b := blk.(type) {
	case richdoc.Heading:
		s := "\\" + headingCmdFor(b.Level) + "{" + writeInlines(b.Inlines) + "}"
		if b.ID != "" {
			// A heading anchor emits the \label right after the section command,
			// which Parse hoists back onto Heading.ID (see parse.go hoistLabel).
			s += "\\label{" + b.ID + "}"
		}
		return s
	case richdoc.Paragraph:
		return writeInlines(b.Inlines)
	case richdoc.List:
		return writeList(b)
	case richdoc.CodeBlock:
		return writeCodeBlock(b)
	case richdoc.BlockQuote:
		return "\\begin{quote}\n" + writeBlocks(b.Blocks) + "\n\\end{quote}"
	case richdoc.Table:
		return writeTable(b)
	case richdoc.MathBlock:
		return "\\[ " + b.TeX + " \\]"
	case richdoc.RawBlock:
		if b.Format == "" || strings.EqualFold(b.Format, "latex") {
			return b.Text
		}
		return ""
	}
	// The block set is closed; the only remaining type is ThematicBreak.
	return "\\hrulefill"
}

func headingCmdFor(level int) string {
	if level < 1 {
		level = 1
	}
	if level > 5 {
		level = 5
	}
	return headingCmd[level]
}

func writeBlocks(blocks []richdoc.Block) string {
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		parts = append(parts, writeBlock(b))
	}
	return strings.Join(parts, "\n\n")
}

func writeList(l richdoc.List) string {
	env := "itemize"
	if l.Ordered {
		env = "enumerate"
	}
	var b strings.Builder
	b.WriteString("\\begin{" + env + "}\n")
	for _, it := range l.Items {
		b.WriteString("\\item " + writeItem(it.Blocks) + "\n")
	}
	b.WriteString("\\end{" + env + "}")
	return b.String()
}

// writeItem renders a list item's blocks: a leading paragraph shares the \item
// line; any further blocks (nested lists, quotes, ...) follow below.
func writeItem(blocks []richdoc.Block) string {
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if p, ok := b.(richdoc.Paragraph); ok {
			parts = append(parts, writeInlines(p.Inlines))
		} else {
			parts = append(parts, writeBlock(b))
		}
	}
	return strings.Join(parts, "\n")
}

func writeCodeBlock(c richdoc.CodeBlock) string {
	if c.Language != "" {
		return "\\begin{lstlisting}[language=" + c.Language + "]\n" + c.Text + "\n\\end{lstlisting}"
	}
	return "\\begin{verbatim}\n" + c.Text + "\n\\end{verbatim}"
}

func writeTable(t richdoc.Table) string {
	ncols := len(t.Align)
	if len(t.Header) > ncols {
		ncols = len(t.Header)
	}
	for _, row := range t.Rows {
		if len(row) > ncols {
			ncols = len(row)
		}
	}
	var b strings.Builder
	b.WriteString("\\begin{tabular}{" + specFromAlign(t.Align, ncols, blockColumns(t, ncols)) + "}\n")
	if len(t.Header) > 0 {
		b.WriteString(writeRow(t.Header) + " \\\\\n\\hline\n")
	}
	for _, row := range t.Rows {
		b.WriteString(writeRow(row) + " \\\\\n")
	}
	b.WriteString("\\end{tabular}")
	tabular := b.String()
	// A CAPTION needs a float: \caption outside one is an error ("\caption outside
	// float"), so a captioned table is wrapped in the "table" environment, which
	// is also what the parse side reads back. The reference writes its caption
	// inside a longtable, which allows one directly; this writer emits tabular.
	if len(t.Caption) > 0 {
		return "\\begin{table}\n\\centering\n" + tabular +
			"\n\\caption{" + writeInlines(t.Caption) + "}\n\\end{table}"
	}
	return tabular
}

// blockColumns reports, per column, whether any cell in it carries block content.
//
// It decides the column SPEC, because that is what makes block content legal: an
// "l" column is a single line, and a paragraph, list or verbatim inside one is a
// LaTeX error ("Something's wrong--perhaps a missing \item"). The reference answers
// the same question the same way -- asked for the LaTeX of a grid table whose cell
// holds a bullet list, docutils emits
//
//	\begin{longtable*}{|p{0.051\DUtablewidth}|p{0.179\DUtablewidth}|}
//	a & \begin{itemize}\item one \item two\end{itemize} \\
//
// a "p" column and the list emitted directly, with no parbox or minipage around
// it. The widths come from the colspecs docutils' own parser computes, which this
// model does not carry, so an equal share of \linewidth is the honest default.
func blockColumns(t richdoc.Table, ncols int) []bool {
	out := make([]bool, ncols)
	mark := func(cells []richdoc.Cell) {
		for i, c := range cells {
			if i < ncols && len(c.Blocks) > 0 {
				out[i] = true
			}
		}
	}
	mark(t.Header)
	for _, row := range t.Rows {
		mark(row)
	}
	return out
}

func writeRow(cells []richdoc.Cell) string {
	parts := make([]string, 0, len(cells))
	for _, c := range cells {
		// Blocks when richdoc v0.5.0's Cell carries them, the flattened Inlines
		// when it does not -- the preference order richdoc.Cell documents. A cell
		// holding a list, a verbatim block or two paragraphs used to arrive here
		// as a run of inlines and lost its structure; the column spec is what
		// makes the real thing legal (see blockColumns).
		if len(c.Blocks) > 0 {
			parts = append(parts, writeBlocks(c.Blocks))
			continue
		}
		parts = append(parts, writeInlines(c.Inlines))
	}
	return strings.Join(parts, " & ")
}

func specFromAlign(align []richdoc.Alignment, ncols int, block []bool) string {
	var b strings.Builder
	for i := 0; i < ncols; i++ {
		if i < len(block) && block[i] {
			// An equal share of the line, minus the inter-column padding LaTeX
			// adds on both sides of every cell. \dimexpr is e-TeX, which every
			// engine in use has had for twenty years.
			b.WriteString("p{\\dimexpr\\linewidth/" + strconv.Itoa(ncols) + "-2\\tabcolsep\\relax}")
			continue
		}
		a := richdoc.AlignDefault
		if i < len(align) {
			a = align[i]
		}
		switch a {
		case richdoc.AlignCenter:
			b.WriteByte('c')
		case richdoc.AlignRight:
			b.WriteByte('r')
		default:
			b.WriteByte('l')
		}
	}
	return b.String()
}

func writeInlines(nodes []richdoc.Inline) string {
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(writeInline(n))
	}
	return b.String()
}

// footnoteInlines flattens a footnote body into inline content for \footnote's
// inline argument. Parse always produces exactly one Paragraph, which is the
// exact round-trip case; any non-paragraph block contributes nothing.
func footnoteInlines(blocks []richdoc.Block) []richdoc.Inline {
	var out []richdoc.Inline
	for _, b := range blocks {
		if p, ok := b.(richdoc.Paragraph); ok {
			out = append(out, p.Inlines...)
		}
	}
	return out
}

func writeInline(n richdoc.Inline) string {
	switch v := n.(type) {
	case richdoc.Text:
		return escapeText(v.Value)
	case richdoc.Emph:
		return "\\emph{" + writeInlines(v.Inlines) + "}"
	case richdoc.Strong:
		return "\\textbf{" + writeInlines(v.Inlines) + "}"
	case richdoc.Strikethrough:
		return "\\sout{" + writeInlines(v.Inlines) + "}"
	case richdoc.Code:
		return "\\texttt{" + escapeText(v.Value) + "}"
	case richdoc.Link:
		return "\\href{" + escapeURL(v.URL) + "}{" + writeInlines(v.Inlines) + "}"
	case richdoc.Image:
		return "\\includegraphics" + graphicxKeys(v) + "{" + escapeURL(v.URL) + "}"
	case richdoc.Math:
		return "$" + v.TeX + "$"
	case richdoc.Footnote:
		// Parse always yields a single Paragraph; render its (and any further
		// block's) inline content back into \footnote's inline argument.
		return "\\footnote{" + writeInlines(footnoteInlines(v.Blocks)) + "}"
	case richdoc.Anchor:
		// A label target; Inlines is usually empty (a point anchor). The ID is
		// emitted verbatim so Parse reads back the exact same label.
		return "\\label{" + v.ID + "}" + writeInlines(v.Inlines)
	case richdoc.CrossRef:
		// \eqref normalises to \ref on write: both parse to RefLabel, so an
		// \eqref source round-trips as \ref (a benign normalisation).
		if v.Kind == richdoc.RefCite {
			return "\\cite{" + v.Target + "}"
		}
		return "\\ref{" + v.Target + "}"
	case richdoc.RawInline:
		if v.Format == "" || strings.EqualFold(v.Format, "latex") {
			// A trailing newline guards against gluing: writeInlines
			// concatenates with no separator, so raw content ending in a
			// bare control word (e.g. "\bfseries") run directly into the
			// next inline's text would swallow that text into the same
			// undefined control sequence name and fail to compile. TeX
			// treats a newline here as ordinary whitespace, which a
			// control word silently consumes — no visible artifact.
			return v.Text + "\n"
		}
		return ""
	}
	// The inline set is closed; the only remaining type is LineBreak.
	return "\\\\"
}

// graphicxKeys builds the "[key=value]" list for an \includegraphics, or "" when
// the image carries no size at all.
//
// The key ORDER follows docutils' own latex2e writer (height, then scale, then
// width, read directly), so output from this package and from
// go-docutils/docutils reads the same way. Scale converts back from richdoc's
// PERCENTAGE to graphicx's factor -- "%g" rather than a fixed precision, so 50
// gives "scale=0.5" and not "scale=0.500000".
func graphicxKeys(img richdoc.Image) string {
	var keys []string
	if img.Height != "" {
		keys = append(keys, "height="+img.Height)
	}
	if img.Scale != 0 {
		keys = append(keys, "scale="+strconv.FormatFloat(float64(img.Scale)/100, 'g', -1, 64))
	}
	if img.Width != "" {
		keys = append(keys, "width="+img.Width)
	}
	if len(keys) == 0 {
		return ""
	}
	return "[" + strings.Join(keys, ",") + "]"
}
