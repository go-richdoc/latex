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

// facesFor typesets one inline run on its own and reports the faces the PDF
// carries.
//
// ⛔ ONE CAUSE PER DOCUMENT. The predecessor of the two tests below put bold
// and italic in the same paragraph and counted the faces, which cannot say
// WHICH markup produced which face — and that is exactly the question that
// mattered the day the engine changed: measured at engine v0.271.0, a mixed
// paragraph yields [Lora-Regular Lora-Italic], a figure equally consistent with
// "italic works", "bold works" and "one of them works". Separate documents
// answered it in one run.
func facesFor(t *testing.T, in ...richdoc.Inline) []string {
	t.Helper()
	out, err := pdf.Write(richdoc.New().P(in...).Doc(), pdf.Options{})
	if err != nil {
		t.Fatalf("typesetting %v: %v", in, err)
	}
	return embedded(out)
}

// TestEmphasisComesBackInAnItalicFace is the assertion its own predecessor
// asked for.
//
// ⛔ THE HISTORY MATTERS, because it is why this is an assertion about the FILE
// and not about the LaTeX. go-tex/engine's Options carry BoldFont and
// ItalicFont, documented as "bound to \bf (so \textbf really bolds)", and this
// package has always supplied all three faces. Up to engine v0.228.0 they were
// INERT: asking six ways — \textbf{x}, {\bf x}, \bfseries, \textit{x}, {\it x}
// and \emph{x}, in LaTeX mode and in plain TeX — left exactly one face in the
// file, the roman, every time. The test standing here asserted that, in order
// to fail the day the engine started honouring them rather than skip and
// quietly stop being about anything.
//
// It fired on engine v0.271.0, which is what replaced it: emphasis now reaches
// the page as a real italic face. Nothing in this repository changed.
func TestEmphasisComesBackInAnItalicFace(t *testing.T) {
	faces := facesFor(t, richdoc.Emph{Inlines: []richdoc.Inline{richdoc.Text{Value: "italic"}}})
	var italic string
	for _, f := range faces {
		if strings.Contains(strings.ToLower(f), "italic") {
			italic = f
		}
	}
	if italic == "" {
		t.Errorf("emphasis embeds %v, with no italic face among them: "+
			"the engine has stopped honouring ItalicFont, which it did at v0.271.0", faces)
	}
}

// TestBoldStillComesBackRoman keeps the tripwire over the half of that defect
// which is still open, in the same shape and for the same reason.
//
// ⛔ Measured at engine v0.271.0, one cause per document:
//
//	roman only   -> [Lora-Regular]
//	bold only    -> [Lora-Regular]              <- this test
//	italic only  -> [Lora-Regular Lora-Italic]  <- the test above
//
// So the engine began honouring ItalicFont and still ignores BoldFont. \textbf
// reaches the page in the roman face: the emphasis is simply not there for a
// reader, and no error is raised anywhere. Reported upstream rather than worked
// around here, because a synthetic bold drawn in this package would hide the
// day the engine fixes it — which is the day this test fails and is deleted.
func TestBoldStillComesBackRoman(t *testing.T) {
	faces := facesFor(t, richdoc.Strong{Inlines: []richdoc.Inline{richdoc.Text{Value: "bold"}}})
	for _, f := range faces {
		if strings.Contains(strings.ToLower(f), "bold") {
			t.Errorf("bold now embeds %q (all faces: %v) — the engine honours BoldFont: "+
				"this test has served its purpose, delete it and assert that it does", f, faces)
		}
	}
}
