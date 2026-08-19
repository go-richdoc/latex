// Copyright (c) the go-richdoc authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import (
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
		return "\\" + headingCmdFor(b.Level) + "{" + writeInlines(b.Inlines) + "}"
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
	b.WriteString("\\begin{tabular}{" + specFromAlign(t.Align, ncols) + "}\n")
	if len(t.Header) > 0 {
		b.WriteString(writeRow(t.Header) + " \\\\\n\\hline\n")
	}
	for _, row := range t.Rows {
		b.WriteString(writeRow(row) + " \\\\\n")
	}
	b.WriteString("\\end{tabular}")
	return b.String()
}

func writeRow(cells []richdoc.Cell) string {
	parts := make([]string, 0, len(cells))
	for _, c := range cells {
		parts = append(parts, writeInlines(c.Inlines))
	}
	return strings.Join(parts, " & ")
}

func specFromAlign(align []richdoc.Alignment, ncols int) string {
	var b strings.Builder
	for i := 0; i < ncols; i++ {
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
		return "\\includegraphics{" + escapeURL(v.URL) + "}"
	case richdoc.Math:
		return "$" + v.TeX + "$"
	case richdoc.RawInline:
		if v.Format == "" || strings.EqualFold(v.Format, "latex") {
			return v.Text
		}
		return ""
	}
	// The inline set is closed; the only remaining type is LineBreak.
	return "\\\\"
}
