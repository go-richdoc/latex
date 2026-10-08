// Copyright (c) the go-richdoc authors.
// SPDX-License-Identifier: BSD-3-Clause

//go:build !js

package pdf_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/go-richdoc/latex/pdf"
	"github.com/go-richdoc/richdoc"
)

// embedded is the set of font names a PDF carries, read off its own /BaseFont
// entries with the six-letter subset tag removed.
//
// ⛔ Read off the FILE. Asserting that the parent package emitted \textbf
// proves the converter, not the page: the LaTeX was always right. What a
// reader gets is what came back, and the only way to see that is to open it.
func embedded(b []byte) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`/BaseFont\s*/([A-Za-z0-9+_-]+)`).FindAllSubmatch(b, -1) {
		name := string(m[1])
		if i := strings.IndexByte(name, '+'); i == 6 {
			name = name[i+1:]
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// TestProseIsSetInATextFaceAndNotAMathsOne is the control over the faces this
// package hands the engine.
//
// ⛔ Before it, every document converted here came back set in
// STIXTwoMath-Regular — a MATHS face, setting running prose, because the
// engine's default is whatever go-tex/math bundles and this package named no
// font at all. Nothing said so: the page count was right, the text was
// readable, and the file opened. A maths face sets prose with the wrong
// rhythm and the wrong figures, and the only witness is the name in the file.
func TestProseIsSetInATextFaceAndNotAMathsOne(t *testing.T) {
	out, err := pdf.Write(para("the quick brown fox jumps over the lazy dog"), pdf.Options{})
	if err != nil {
		t.Fatalf("typesetting a paragraph: %v", err)
	}
	faces := embedded(out)
	if len(faces) == 0 {
		t.Fatal("the PDF names no font at all, so this test can see nothing")
	}
	for _, f := range faces {
		if strings.Contains(strings.ToLower(f), "math") {
			t.Errorf("prose is set in %q, which is a maths face", f)
		}
	}
	if !strings.Contains(strings.ToLower(faces[0]), "lora") {
		t.Errorf("the text face is %q, and this package asks for Lora", faces[0])
	}
}

// TestEmphasisStillComesBackRoman records a defect that is NOT in this
// repository, together with the measurement that found it.
//
// ⛔ go-tex/engine's Options carry BoldFont and ItalicFont, documented as
// "bound to \bf (so \textbf really bolds)". This package supplies all three
// faces. They are INERT: binding a wholly different family to bold and asking
// for it six ways — \textbf{x}, {\bf x}, \bfseries, \textit{x}, {\it x} and
// \emph{x}, in LaTeX mode and in plain TeX — leaves exactly one face in the
// file, the roman, every time.
//
// So this asserts what is TRUE TODAY rather than skipping. A test that skips
// asserts nothing and quietly stops being about anything; this one fails the
// day the engine starts honouring the faces, which is the day to delete it and
// assert the opposite.
func TestEmphasisStillComesBackRoman(t *testing.T) {
	doc := richdoc.New().
		P(richdoc.Text{Value: "plain "},
			richdoc.Strong{Inlines: []richdoc.Inline{richdoc.Text{Value: "bold"}}},
			richdoc.Text{Value: " and "},
			richdoc.Emph{Inlines: []richdoc.Inline{richdoc.Text{Value: "italic"}}}).
		Doc()

	out, err := pdf.Write(doc, pdf.Options{})
	if err != nil {
		t.Fatalf("typesetting a paragraph with bold and italic in it: %v", err)
	}
	if faces := embedded(out); len(faces) != 1 {
		t.Errorf("the engine now embeds %d faces (%v) — if it honours bold and italic, "+
			"this test has served its purpose: delete it and assert that it does",
			len(faces), faces)
	}
}
