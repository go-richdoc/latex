// Copyright (c) the go-richdoc authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import (
	"strings"
	"testing"
)

// nested builds a document whose environments nest n deep.
func nested(n int) []byte {
	var b strings.Builder
	b.WriteString(`\documentclass{article}\begin{document}`)
	for i := 0; i < n; i++ {
		b.WriteString(`\begin{itemize}\item `)
	}
	b.WriteString("x")
	for i := 0; i < n; i++ {
		b.WriteString(`\end{itemize}`)
	}
	b.WriteString(`\end{document}`)
	return []byte(b.String())
}

// TestDeepNestingIsRefused is a denial of service by CPU, and the cost is the
// NESTING rather than the bytes, which is what the control below pins.
//
// An environment's body is found by scanning forward to its matching \end and
// then PARSED, so the innermost text is scanned once per enclosing level.
// Measured before the limit: 2000 levels in 68 KB took 2.9 s, where the same
// 68 KB flat took 2 ms — and the curve is superlinear, so a half-megabyte
// document would have held a CPU for half an hour.
func TestDeepNestingIsRefused(t *testing.T) {
	_, err := Parse(nested(300))
	if err == nil {
		t.Fatal("300 levels parsed; the default limit is 256")
	}
	if !strings.Contains(err.Error(), "256") {
		t.Errorf("the error does not name the limit it enforced: %v", err)
	}
}

// TestRealWorldNestingStillParses is the control that decides whether the
// default is usable. Over the 97,689 real .tex files on the machine this was
// written on, the DEEPEST nests 64 environments — one file does that, the next
// deepest is 26, and 92,729 nest four or fewer. So 64 has to parse, and the
// default of 256 clears it four times over.
func TestRealWorldNestingStillParses(t *testing.T) {
	for _, depth := range []int{1, 4, 26, 64, 256} {
		doc, err := Parse(nested(depth))
		if err != nil {
			t.Fatalf("%d levels: %v", depth, err)
		}
		if doc == nil || len(doc.Blocks) == 0 {
			t.Errorf("%d levels: no blocks", depth)
		}
	}
}

// TestFlatIsNotNesting is the other half of the control: the limit must count
// DEPTH and not size or construct count. The same 2000 environments written as
// siblings is an ordinary document — and it was always fast, which is how the
// cost was attributed to nesting in the first place.
func TestFlatIsNotNesting(t *testing.T) {
	var b strings.Builder
	b.WriteString(`\documentclass{article}\begin{document}`)
	for i := 0; i < 2000; i++ {
		b.WriteString(`\begin{itemize}\item x\end{itemize}`)
	}
	b.WriteString(`\end{document}`)
	doc, err := Parse([]byte(b.String()))
	if err != nil {
		t.Fatalf("2000 sibling environments: %v", err)
	}
	if len(doc.Blocks) != 2000 {
		t.Errorf("got %d blocks, want 2000", len(doc.Blocks))
	}
}

// TestLimitsAreCallerChosen covers the two conventions this struct borrows
// from go-richdoc/rst's LineLengthLimit: zero means the default, and a
// NEGATIVE value means no limit, for a caller who would rather spend the time
// than lose the document.
func TestLimitsAreCallerChosen(t *testing.T) {
	if _, err := ParseWithLimits(nested(40), Limits{MaxDepth: 8}); err == nil {
		t.Error("a caller's own tighter limit was not enforced")
	}
	if _, err := ParseWithLimits(nested(40), Limits{}); err != nil {
		t.Errorf("zero should mean the default of %d: %v", defaultMaxDepth, err)
	}
	if _, err := ParseWithLimits(nested(300), Limits{MaxDepth: -1}); err != nil {
		t.Errorf("a negative limit should mean no limit: %v", err)
	}
}

// TestTheLimitReachesInlineContextToo covers the half of the threading that is
// easy to leave out: depth is checked where a sub-parser is MADE, and one is
// made for inline content as well as for blocks -- a float's caption and a
// table's cells. With the limit at 1 the table environment itself is allowed
// and its caption is one level too deep, which is the only way to reach that
// branch and the reason it showed as the single uncovered one in CI.
func TestTheLimitReachesInlineContextToo(t *testing.T) {
	const src = `\documentclass{article}\begin{document}` +
		`\begin{table}\caption{deep}\begin{tabular}{l}x\end{tabular}\end{table}` +
		`\end{document}`
	_, err := ParseWithLimits([]byte(src), Limits{MaxDepth: 1})
	if err == nil {
		t.Fatal("a caption one level past the limit was accepted")
	}
	if !strings.Contains(err.Error(), "nest more than 1 deep") {
		t.Errorf("the error does not name the limit: %v", err)
	}
	// And the control: the same document with room to spare.
	if _, err := ParseWithLimits([]byte(src), Limits{MaxDepth: 8}); err != nil {
		t.Errorf("the same table with room: %v", err)
	}
}
