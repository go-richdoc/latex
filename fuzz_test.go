// Copyright (c) the go-richdoc authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-richdoc/richdoc"
)

// FuzzParse points this reader at bytes nobody wrote, and it is the member of
// this family with the most to gain from that: the others delegate their
// tokenising (goldmark, x/net/html, encoding/xml, archive/zip) while this one
// is a HAND-WRITTEN recursive parser over runes, with its own cursor, its own
// brace matching and its own environment stack.
//
// Four properties:
//
//   - Parse does not panic. A hand-written cursor is where an off-by-one
//     lives, and "latex: unclosed environment" is the shape of answer this
//     parser is supposed to give instead.
//   - its two results are exclusive: a document OR an error, never neither.
//   - Parse is DETERMINISTIC. The parser threads a meta map through every
//     sub-parser, and a single range over it reaching the output would make a
//     document depend on nothing the source said.
//   - the DEPTH limit holds in the shape of the TREE. Limits.MaxDepth bounds
//     environment nesting, and each level can add only a constant number of
//     richdoc levels (a list adds List -> Item -> Paragraph), so a tree far
//     deeper than the limit allows means the limit is not being counted where
//     the recursion is.
//
// The limit itself was added in the same round and for a measured reason: an
// environment's body is scanned to its matching \end and then parsed, so the
// innermost text is scanned once per enclosing level. 2000 levels in 68 KB
// took 2.9 s where the same 68 KB flat took 2 ms. See Limits.
func FuzzParse(f *testing.F) {
	seeds := []string{
		`\documentclass{article}\begin{document}Hello \textbf{world}.\end{document}`,
		`\begin{itemize}\item one\item two\end{itemize}`,
		`\begin{enumerate}\item \begin{itemize}\item inner\end{itemize}\end{enumerate}`,
		`\begin{tabular}{ll}a & b\\c & d\end{tabular}`,
		`\begin{table}\caption{A caption}\begin{tabular}{l}x\end{tabular}\end{table}`,
		`$x^2 + \frac{1}{2}$ and \[ \sum_{i=0}^n i \]`,
		`\section{One}\subsection{Two}text\label{l}\ref{l}\cite{k}`,
		`\footnote{a note}\emph{e}\texttt{t}\href{http://x/}{link}`,
		// The malformed shapes this parser answers with an error rather than a
		// panic, which is the property under test.
		`\begin{itemize}\item unclosed`,
		`{unbalanced`,
		`$unterminated math`,
		`\end{itemize}`,
		`\begin{}\end{}`,
		// Nesting, for the limit, and a flat run, which must stay cheap.
		strings.Repeat(`\begin{itemize}\item `, 40) + "x" + strings.Repeat(`\end{itemize}`, 40),
		strings.Repeat(`\begin{itemize}\item x\end{itemize}`, 40),
		// A preamble with the metadata it mines, and a document environment
		// found by LastIndex.
		`\documentclass[12pt]{article}\title{T}\author{A}\date{1843}\begin{document}b\end{document}\end{document}`,
		"",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		// A small depth on purpose: a bound is only tested near its edge.
		lim := Limits{MaxDepth: 12}
		doc, err := ParseWithLimits(src, lim)
		if err != nil {
			if doc != nil {
				t.Fatalf("both a document and an error: %v", err)
			}
			return
		}
		if doc == nil {
			t.Fatal("neither a document nor an error")
		}

		again, err2 := ParseWithLimits(src, lim)
		if err2 != nil || again == nil {
			t.Fatalf("the second Parse disagreed with the first: doc=%v err=%v", again != nil, err2)
		}
		if first, second := fmt.Sprintf("%#v", doc), fmt.Sprintf("%#v", again); first != second {
			t.Fatalf("two parses of the same bytes differ\nfirst:\n%.400s\nsecond:\n%.400s", first, second)
		}

		// Each environment level can add a constant number of richdoc levels;
		// the slack is generous on purpose, because the invariant under test is
		// "the limit is counted where the recursion is" and not an exact ratio.
		if d, ceiling := fuzzTreeDepth(doc.Blocks), 4*lim.MaxDepth+8; d > ceiling {
			t.Errorf("the tree nests %d deep with MaxDepth %d, over the %d allowed",
				d, lim.MaxDepth, ceiling)
		}
	})
}

func fuzzTreeDepth(bs []richdoc.Block) int {
	if len(bs) == 0 {
		return 0
	}
	deepest := 0
	for _, b := range bs {
		for _, child := range fuzzChildBlocks(b) {
			if d := fuzzTreeDepth(child); d > deepest {
				deepest = d
			}
		}
	}
	return deepest + 1
}

func fuzzChildBlocks(b richdoc.Block) [][]richdoc.Block {
	switch v := b.(type) {
	case richdoc.BlockQuote:
		return [][]richdoc.Block{v.Blocks}
	case richdoc.List:
		out := make([][]richdoc.Block, 0, len(v.Items))
		for _, it := range v.Items {
			out = append(out, it.Blocks)
		}
		return out
	case richdoc.Table:
		var out [][]richdoc.Block
		for _, row := range append([][]richdoc.Cell{v.Header}, v.Rows...) {
			for _, c := range row {
				if len(c.Blocks) > 0 {
					out = append(out, c.Blocks)
				}
			}
		}
		return out
	}
	return nil
}
