// Copyright (c) the go-richdoc authors.
// SPDX-License-Identifier: BSD-3-Clause

//go:build !js

package latex_test

import (
	"bytes"
	"testing"

	"github.com/go-richdoc/latex"
	"github.com/go-richdoc/richdoc"
	"github.com/go-tex/engine"
)

// compiles asserts that LaTeX source typesets to at least one page with the
// go-tex engine and no fatal error. The engine runs in lenient mode so that a
// missing asset (an \includegraphics whose file is not shipped) or a command
// outside the engine's implemented set degrades gracefully instead of aborting
// the compile — the point of the check is that Write emits structurally valid,
// typesettable LaTeX.
func compiles(t *testing.T, src []byte) {
	t.Helper()
	pages, err := engine.CompileToSVGPages(src, engine.Options{Lenient: true})
	if err != nil {
		t.Fatalf("engine did not compile the emitted LaTeX: %v\n---\n%s", err, src)
	}
	if len(pages) == 0 {
		t.Fatalf("engine produced no pages for:\n%s", src)
	}
}

// TestWriteOutputCompiles proves that a document exercising every block and
// inline node Write can emit typesets with the go-tex engine.
func TestWriteOutputCompiles(t *testing.T) {
	doc := richdoc.New().
		Meta("title", "A Converter Demo").
		Meta("author", "The go-richdoc authors").
		H(1, richdoc.Txt("Introduction")).
		P(
			richdoc.Bold(richdoc.Txt("Bold")), richdoc.Txt(", "),
			richdoc.Italic(richdoc.Txt("italic")), richdoc.Txt(", "),
			richdoc.Mono("mono"), richdoc.Txt(", "),
			richdoc.Strike(richdoc.Txt("struck")), richdoc.Txt(", and math "),
			richdoc.InlineMath("a^2 + b^2 = c^2"), richdoc.Txt("."),
		).
		P(richdoc.Txt("A hard break here"), richdoc.Br(), richdoc.Txt("and a second line.")).
		H(2, richdoc.Txt("Lists")).
		UList(false,
			richdoc.Item(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("first bullet")}}),
			richdoc.Item(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("second bullet")}}),
		).
		OList(1, false,
			richdoc.Item(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("step one")}}),
			richdoc.Item(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("step two")}}),
		).
		H(2, richdoc.Txt("A quote and some code")).
		Quote(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("A quoted paragraph.")}}).
		CodeBlock("", "plain := verbatim\nsecond line").
		CodeBlock("Go", "package main\nfunc main() {}").
		H(2, richdoc.Txt("A table and a link")).
		Table(
			[]richdoc.Alignment{richdoc.AlignLeft, richdoc.AlignCenter, richdoc.AlignRight},
			[]richdoc.Cell{richdoc.Td(richdoc.Txt("Name")), richdoc.Td(richdoc.Txt("Kind")), richdoc.Td(richdoc.Txt("N"))},
			[][]richdoc.Cell{
				{richdoc.Td(richdoc.Txt("alpha")), richdoc.Td(richdoc.Txt("x")), richdoc.Td(richdoc.Txt("1"))},
				{richdoc.Td(richdoc.Txt("beta")), richdoc.Td(richdoc.Txt("y")), richdoc.Td(richdoc.Txt("2"))},
			},
		).
		P(richdoc.Txt("See "), richdoc.Href("https://example.com", "", richdoc.Txt("the site")), richdoc.Txt(".")).
		HR().
		MathBlock("\\int_0^1 x \\, dx = \\frac{1}{2}").
		P(richdoc.Txt("Escaped specials: 100% of a & b, #1, x_2.")).
		Doc()

	out, err := latex.Write(doc)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	compiles(t, out)
}

// TestV2NodesCompile proves the richdoc v0.2.0 inline nodes (footnote, label,
// cross-reference and citation) emit core LaTeX that typesets with no extra
// package. \label/\ref/\cite/\footnote are all built into LaTeX.
func TestV2NodesCompile(t *testing.T) {
	doc := richdoc.New().
		Add(richdoc.Heading{Level: 1, ID: "sec:intro", Inlines: []richdoc.Inline{richdoc.Txt("Introduction")}}).
		P(
			richdoc.Txt("A referenced section "), richdoc.Ref("sec:intro"),
			richdoc.Txt(" with a footnote"), richdoc.Note(richdoc.Paragraph{Inlines: []richdoc.Inline{
				richdoc.Txt("the footnote body with "), richdoc.Bold(richdoc.Txt("emphasis")),
			}}),
			richdoc.Txt(" and a point"), richdoc.Mark("pt:here"),
			richdoc.Txt(" and a citation "), richdoc.Cite("knuth1984"), richdoc.Txt("."),
		).
		Doc()
	out, err := latex.Write(doc)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	compiles(t, out)
}

// TestRawInlineGlueDoesNotBreakCompile guards a real bug: writeInlines
// concatenates adjacent inline nodes with no separator, so a RawInline
// ending in a bare control word (no argument, no trailing space in the
// author's own content) run directly into the next inline's text used to
// swallow that text into the same undefined control sequence name and fail
// to compile — caught by actually compiling, not just diffing the emitted
// source. Found via go-richdoc/rst's own v0.16.0 role-registry sync, which
// was the first real producer of an inline richdoc.RawInline this pipeline
// had ever exercised end-to-end.
//
// Deliberately does NOT use compiles() (Options{Lenient: true}): lenient
// mode turns exactly this error class, an undefined control sequence, into
// a non-fatal condition, so it cannot tell a real glue bug from a correct
// fix — this needs a strict, default-options compile to mean anything.
func TestRawInlineGlueDoesNotBreakCompile(t *testing.T) {
	doc := richdoc.New().P(
		richdoc.RawI("latex", `\bfseries`), richdoc.Txt("Regular text glued right after."),
	).Doc()
	out, err := latex.Write(doc)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	var buf bytes.Buffer
	if _, err := engine.CompileToPDF(out, engine.Options{}, &buf); err != nil {
		t.Fatalf("engine did not compile the emitted LaTeX: %v\n---\n%s", err, out)
	}
}

// TestParsedThenWrittenCompiles proves the full pipeline: parse real LaTeX, emit
// it again, and confirm the emitted form typesets.
func TestParsedThenWrittenCompiles(t *testing.T) {
	sources := []string{
		`\documentclass{article}
\title{Round Trip}
\author{Test}
\begin{document}
\maketitle
\section{Hello}
Some \textbf{bold} and \emph{italic} text with inline math $x_i$.

\begin{itemize}
\item one
\item two
\end{itemize}

\begin{tabular}{lc}
a & b \\
c & d \\
\end{tabular}

\[ E = mc^2 \]
\end{document}`,
		`Just a \href{https://ex.io}{link} and a formula \(\alpha + \beta\).`,
		`\begin{verbatim}
raw code &%$#
\end{verbatim}`,
	}
	for _, src := range sources {
		d, err := latex.Parse([]byte(src))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		out, err := latex.Write(d)
		if err != nil {
			t.Fatalf("Write: %v", err)
		}
		compiles(t, out)
	}
}
