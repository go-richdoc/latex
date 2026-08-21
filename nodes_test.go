// Copyright (c) the go-richdoc authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import (
	"reflect"
	"strings"
	"testing"

	"github.com/go-richdoc/richdoc"
)

// --- round-trip: the richdoc v0.2.0 inline nodes -----------------------------

func TestRoundTripV2Nodes(t *testing.T) {
	cases := []string{
		`Body with a \footnote{a simple note} inline.`,
		`See \footnote{a \textbf{bold} and \emph{stressed} note} here.`,
		`A point \label{pt:one} target in text.`,
		`As in \ref{sec:intro} and equation \eqref{eq:main} we see it.`,
		`As shown \cite{knuth1984} and \cite{lamport1994} above.`,
		`\section{Introduction}\label{sec:intro}
Body of the introduction.`,
		`\subsection{Details}\label{sec:details}
More text with a \ref{sec:intro} back-reference and a \footnote{note}.`,
		`\section{No label here}
Plain body, no anchor.`,
		`\section{Heading}
A standalone \label{lbl:body} lives in the paragraph, not the heading.`,
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

// --- Parse: structural assertions --------------------------------------------

func TestParseFootnote(t *testing.T) {
	d := mustParse(t, `Text \footnote{a \textbf{b} note} end.`)
	p := d.Blocks[0].(richdoc.Paragraph)
	var fn richdoc.Footnote
	found := false
	for _, in := range p.Inlines {
		if f, ok := in.(richdoc.Footnote); ok {
			fn = f
			found = true
		}
	}
	if !found {
		t.Fatalf("no Footnote in %#v", p.Inlines)
	}
	if len(fn.Blocks) != 1 {
		t.Fatalf("footnote blocks = %d, want 1 paragraph", len(fn.Blocks))
	}
	para, ok := fn.Blocks[0].(richdoc.Paragraph)
	if !ok {
		t.Fatalf("footnote body block = %#v, want Paragraph", fn.Blocks[0])
	}
	// The braced argument keeps its inline structure (text + strong + text).
	if len(para.Inlines) != 3 {
		t.Fatalf("footnote inlines = %#v", para.Inlines)
	}
}

func TestParseStandaloneLabel(t *testing.T) {
	d := mustParse(t, `A \label{pt:x} point.`)
	p := d.Blocks[0].(richdoc.Paragraph)
	var a richdoc.Anchor
	found := false
	for _, in := range p.Inlines {
		if v, ok := in.(richdoc.Anchor); ok {
			a = v
			found = true
		}
	}
	if !found {
		t.Fatalf("no Anchor in %#v", p.Inlines)
	}
	if a.ID != "pt:x" || len(a.Inlines) != 0 {
		t.Fatalf("anchor = %#v, want point anchor ID=pt:x", a)
	}
}

func TestParseCrossRefKinds(t *testing.T) {
	d := mustParse(t, `\ref{a} \eqref{b} \cite{c}`)
	p := d.Blocks[0].(richdoc.Paragraph)
	var refs []richdoc.CrossRef
	for _, in := range p.Inlines {
		if r, ok := in.(richdoc.CrossRef); ok {
			refs = append(refs, r)
		}
	}
	if len(refs) != 3 {
		t.Fatalf("cross-refs = %#v", refs)
	}
	if refs[0].Target != "a" || refs[0].Kind != richdoc.RefLabel {
		t.Fatalf("ref = %#v, want RefLabel a", refs[0])
	}
	if refs[1].Target != "b" || refs[1].Kind != richdoc.RefLabel {
		t.Fatalf("eqref = %#v, want RefLabel b", refs[1])
	}
	if refs[2].Target != "c" || refs[2].Kind != richdoc.RefCite {
		t.Fatalf("cite = %#v, want RefCite c", refs[2])
	}
}

func TestParseHeadingLabelHoist(t *testing.T) {
	// A \label immediately after a section (whitespace/comments allowed) is
	// hoisted onto Heading.ID and does NOT become a separate block.
	d := mustParse(t, "\\section{Intro} % note\n\\label{sec:intro}\nBody.")
	if len(d.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2 (heading + paragraph)", len(d.Blocks))
	}
	h, ok := d.Blocks[0].(richdoc.Heading)
	if !ok || h.ID != "sec:intro" {
		t.Fatalf("heading = %#v, want ID sec:intro", d.Blocks[0])
	}
	if _, ok := d.Blocks[1].(richdoc.Paragraph); !ok {
		t.Fatalf("block1 = %#v, want Paragraph", d.Blocks[1])
	}
}

func TestParseHeadingNoHoistWhenNotLabel(t *testing.T) {
	// A non-\label control right after a heading is not hoisted: the heading
	// keeps an empty ID and the control begins the following paragraph.
	d := mustParse(t, `\section{Intro}\ref{elsewhere} in the body.`)
	h := d.Blocks[0].(richdoc.Heading)
	if h.ID != "" {
		t.Fatalf("heading ID = %q, want empty", h.ID)
	}
	p := d.Blocks[1].(richdoc.Paragraph)
	if _, ok := p.Inlines[0].(richdoc.CrossRef); !ok {
		t.Fatalf("paragraph should start with CrossRef, got %#v", p.Inlines[0])
	}
}

func TestParseHeadingNoHoistOnControlSymbol(t *testing.T) {
	// A control symbol (not a control word) right after a heading also leaves
	// the heading without an ID, exercising hoistLabel's !word branch.
	d := mustParse(t, `\section{Intro}\, spaced body.`)
	h := d.Blocks[0].(richdoc.Heading)
	if h.ID != "" {
		t.Fatalf("heading ID = %q, want empty", h.ID)
	}
	if len(d.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(d.Blocks))
	}
}

// --- Parse: error branches for the new nodes ---------------------------------

func TestParseV2NodeErrors(t *testing.T) {
	bad := map[string]string{
		"footnote unclosed":    `text \footnote{never closed`,
		"label unclosed":       `point \label{never closed`,
		"ref unclosed":         `see \ref{never closed`,
		"eqref unclosed":       `see \eqref{never closed`,
		"cite unclosed":        `see \cite{never closed`,
		"hoisted label bad":    `\section{X}\label{never closed`,
		"footnote inner error": `text \footnote{a \textbf{oops}`,
	}
	for name, src := range bad {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("%s: expected error for %q", name, src)
		}
	}
}

// --- Write: assertions for the new nodes -------------------------------------

func TestWriteHeadingLabel(t *testing.T) {
	d := richdoc.New().Add(richdoc.Heading{Level: 1, ID: "sec:x", Inlines: []richdoc.Inline{richdoc.Txt("Title")}}).Doc()
	out := string(mustWrite(t, d))
	if !strings.Contains(out, `\section{Title}\label{sec:x}`) {
		t.Fatalf("heading label output:\n%s", out)
	}
}

func TestWriteEqrefNormalisesToRef(t *testing.T) {
	// A RefLabel cross-reference (from \ref or \eqref) always writes as \ref.
	d := richdoc.New().P(richdoc.Ref("eq:1")).Doc()
	out := string(mustWrite(t, d))
	if !strings.Contains(out, `\ref{eq:1}`) || strings.Contains(out, `\eqref`) {
		t.Fatalf("eqref should normalise to \\ref:\n%s", out)
	}
}

func TestWriteCite(t *testing.T) {
	d := richdoc.New().P(richdoc.Cite("key")).Doc()
	out := string(mustWrite(t, d))
	if !strings.Contains(out, `\cite{key}`) {
		t.Fatalf("cite output:\n%s", out)
	}
}

func TestWriteAnchorWithVisibleInlines(t *testing.T) {
	// An anchor carrying visible content emits the label then its inlines. This
	// is a Write-only shape (Parse only produces point anchors).
	d := richdoc.New().P(richdoc.Mark("id", richdoc.Txt("visible"))).Doc()
	out := string(mustWrite(t, d))
	if !strings.Contains(out, `\label{id}visible`) {
		t.Fatalf("anchor-with-inlines output:\n%s", out)
	}
}

func TestWriteFootnoteNonParagraphBody(t *testing.T) {
	// A footnote whose body is not a lone paragraph: the paragraph's inlines are
	// rendered and any non-paragraph block contributes nothing.
	fn := richdoc.Footnote{Blocks: []richdoc.Block{
		richdoc.ThematicBreak{},
		richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("kept")}},
	}}
	d := richdoc.New().P(richdoc.Txt("x"), fn).Doc()
	out := string(mustWrite(t, d))
	if !strings.Contains(out, `\footnote{kept}`) {
		t.Fatalf("footnote non-paragraph body output:\n%s", out)
	}
}
