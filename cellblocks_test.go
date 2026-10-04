package latex

import (
	"reflect"
	"strings"
	"testing"

	"github.com/go-richdoc/richdoc"
)

func blockCellTable(caption ...richdoc.Inline) richdoc.Table {
	return richdoc.Table{
		Caption: caption,
		Rows: [][]richdoc.Cell{{
			richdoc.Td(richdoc.Txt("bsddb")),
			{
				Inlines: []richdoc.Inline{richdoc.Txt("one"), richdoc.Txt("\n\n"), richdoc.Txt("two")},
				Blocks: []richdoc.Block{richdoc.List{Items: []richdoc.ListItem{
					richdoc.Item(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("one")}}),
					richdoc.Item(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("two")}}),
				}}},
			},
		}},
	}
}

// TestACellsBlocksNeedAPColumn pins the pair of changes that make block content in
// a cell legal LaTeX, and the second half is the reason for the first.
//
// An "l" column is a single line: a list inside one is not merely ugly, it is an
// ERROR. Compiled with tectonic, the same table with "ll" fails at the itemize --
//
//	! LaTeX Error: Something's wrong--perhaps a missing \item.
//
// -- and with the "p" column it produces a PDF. The reference answers the question
// the same way: asked for the LaTeX of a grid table whose cell holds a bullet list,
// docutils emits p{0.179\DUtablewidth} columns and the itemize directly, with no
// parbox or minipage around it.
func TestACellsBlocksNeedAPColumn(t *testing.T) {
	out, err := Write(richdoc.New().Add(blockCellTable()).Doc())
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, `\begin{tabular}{lp{\dimexpr\linewidth/2-2\tabcolsep\relax}}`) {
		t.Errorf("the column holding blocks is not a p column:\n%s", got)
	}
	if !strings.Contains(got, "\\begin{itemize}\n\\item one\n\\item two\n\\end{itemize}") {
		t.Errorf("the cell's list is not emitted as a list:\n%s", got)
	}
}

// TestAPlainTableKeepsItsLColumns is the CONTROL: a table whose cells hold only
// inlines must keep the spec it always had. A "p" column everywhere would change
// every existing table's layout, which is not what this version is for.
func TestAPlainTableKeepsItsLColumns(t *testing.T) {
	out, err := Write(richdoc.New().Add(richdoc.Table{
		Rows: [][]richdoc.Cell{{richdoc.Td(richdoc.Txt("a")), richdoc.Td(richdoc.Txt("b"))}},
	}).Doc())
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !strings.Contains(string(out), `\begin{tabular}{ll}`) {
		t.Errorf("a plain table's spec changed:\n%s", out)
	}
	if strings.Contains(string(out), "dimexpr") {
		t.Errorf("a plain table gained a p column:\n%s", out)
	}
}

// TestACaptionNeedsAFloat pins where a caption goes. \caption outside a float is an
// error, so a captioned table is wrapped in the "table" environment -- and that is
// what the parse side reads back, which is what makes it a round trip.
func TestACaptionNeedsAFloat(t *testing.T) {
	out, err := Write(richdoc.New().Add(blockCellTable(richdoc.Txt("Should be Table 1"))).Doc())
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := string(out)
	for _, want := range []string{"\\begin{table}", "\\caption{Should be Table 1}", "\\end{table}"} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q in:\n%s", want, got)
		}
	}
	// The caption must come AFTER the tabular, where LaTeX puts a table's own
	// caption by convention, and inside the float either way.
	if strings.Index(got, "\\caption{") < strings.Index(got, "\\end{tabular}") {
		t.Errorf("the caption is inside the tabular:\n%s", got)
	}
}

// TestATableFloatRoundTrips is the parse side of both: the float comes back as a
// Table carrying its caption, and the "p" cell comes back carrying its Blocks.
func TestATableFloatRoundTrips(t *testing.T) {
	doc := richdoc.New().Add(blockCellTable(richdoc.Txt("Cap"))).Doc()
	out, err := Write(doc)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	back, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tbl, ok := back.Blocks[0].(richdoc.Table)
	if !ok {
		t.Fatalf("first block is %T, want a Table -- the float stayed raw", back.Blocks[0])
	}
	if got := richdoc.PlainText(&richdoc.Document{Blocks: []richdoc.Block{richdoc.Paragraph{Inlines: tbl.Caption}}}); got != "Cap" {
		t.Errorf("the caption came back as %q", got)
	}
	cell := tbl.Rows[len(tbl.Rows)-1][1]
	if len(cell.Blocks) == 0 {
		t.Fatalf("the cell came back with no Blocks: %#v", cell)
	}
	if _, ok := cell.Blocks[0].(richdoc.List); !ok {
		t.Errorf("the cell's first block is %T, want a List", cell.Blocks[0])
	}
	if len(cell.Inlines) == 0 {
		t.Errorf("the cell carries Blocks but no Inlines, which breaks richdoc's contract: %#v", cell)
	}
}

// TestACellWithOneParagraphCarriesNoBlocks is the parse-side control: the common
// case must stay Inlines-only, or every consumer would have to choose between two
// spellings of the same content for every cell in every table.
func TestACellWithOneParagraphCarriesNoBlocks(t *testing.T) {
	back, err := Parse([]byte("\\begin{tabular}{ll}\na & b \\\\\n\\end{tabular}\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tbl := back.Blocks[0].(richdoc.Table)
	for _, row := range tbl.Rows {
		for _, c := range row {
			if len(c.Blocks) != 0 {
				t.Errorf("a one-paragraph cell carries Blocks: %#v", c)
			}
		}
	}
	if !reflect.DeepEqual(tbl.Caption, []richdoc.Inline(nil)) {
		t.Errorf("a plain tabular gained a caption: %#v", tbl.Caption)
	}
}

// TestATableFloatWithoutATabularStaysRaw covers the fallback, and it is a real
// shape: a "table" float can hold anything, and with no tabular in it there is
// nothing to attach a caption to. Keeping the float verbatim loses nothing; turning
// it into an empty Table would lose all of it.
func TestATableFloatWithoutATabularStaysRaw(t *testing.T) {
	back, err := Parse([]byte("\\begin{table}\n\\centering\nJust a note.\n\\caption{Cap}\n\\end{table}\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	raw, ok := back.Blocks[0].(richdoc.RawBlock)
	if !ok {
		t.Fatalf("first block is %T, want a RawBlock", back.Blocks[0])
	}
	if !strings.Contains(raw.Text, "\\caption{Cap}") {
		t.Errorf("the float lost its own content: %q", raw.Text)
	}
}

// TestAMalformedCaptionIsNotACaption covers the two error paths around the caption,
// which are the ones that keep a broken float from taking the whole parse down with
// it: an unterminated \caption{ group, and a caption whose content does not parse.
func TestAMalformedCaptionIsNotACaption(t *testing.T) {
	cases := []string{
		// An unterminated group: extractCaption cannot read an argument, so the
		// float is left exactly as it came.
		"\\begin{table}\n\\begin{tabular}{l}\na \\\\\n\\end{tabular}\n\\caption{unterminated\n\\end{table}\n",
		// A caption whose inline content is malformed -- an unclosed math span.
		"\\begin{table}\n\\begin{tabular}{l}\na \\\\\n\\end{tabular}\n\\caption{$x}\n\\end{table}\n",
	}
	for _, src := range cases {
		back, err := Parse([]byte(src))
		if err != nil {
			// An error is an acceptable answer for malformed input; what is not
			// acceptable is a panic or a silently empty document.
			continue
		}
		if len(back.Blocks) == 0 {
			t.Errorf("%q produced an empty document", src)
		}
	}
}

// TestAnEmptyCellCarriesNothing covers the empty-cell guard: a row ending in "&"
// has a cell with no content at all, and asking the block parser about zero runes
// would answer with an empty paragraph rather than nothing.
func TestAnEmptyCellCarriesNothing(t *testing.T) {
	back, err := Parse([]byte("\\begin{tabular}{ll}\na & \\\\\n\\end{tabular}\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tbl := back.Blocks[0].(richdoc.Table)
	last := tbl.Rows[0][len(tbl.Rows[0])-1]
	if len(last.Blocks) != 0 {
		t.Errorf("an empty cell carries Blocks: %#v", last)
	}
}

// TestACellThatCannotBeReadAsBlocksKeepsItsWords covers the error path in
// cellBlocks, and states why it swallows the error: the inline parse has already
// succeeded on the same runes, so a cell whose block reading fails still has its
// text. An unterminated environment inside a cell is the shape.
func TestACellThatCannotBeReadAsBlocksKeepsItsWords(t *testing.T) {
	back, err := Parse([]byte("\\begin{tabular}{ll}\na & \\begin{itemize}\\item one \\\\\n\\end{tabular}\n"))
	if err != nil {
		// The cell's own inline parse may fail first, which is a legitimate
		// answer; the point of the case is that nothing panics.
		return
	}
	if len(back.Blocks) == 0 {
		t.Error("the document came back empty")
	}
}

// TestATableFloatWithoutACaptionIsStillATable covers the no-caption path through
// the float: a "table" environment is not required to have one, and the table inside
// it has to come out whole either way.
func TestATableFloatWithoutACaptionIsStillATable(t *testing.T) {
	back, err := Parse([]byte("\\begin{table}\n\\centering\n\\begin{tabular}{ll}\na & b \\\\\n\\end{tabular}\n\\end{table}\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tbl, ok := back.Blocks[0].(richdoc.Table)
	if !ok {
		t.Fatalf("first block is %T, want a Table", back.Blocks[0])
	}
	if len(tbl.Caption) != 0 {
		t.Errorf("a float with no caption produced one: %#v", tbl.Caption)
	}
	if got := richdoc.PlainText(back); !strings.Contains(got, "a") || !strings.Contains(got, "b") {
		t.Errorf("the table's cells are gone: %q", got)
	}
}

// TestACellWithProseAndAListKeepsBoth is the two-block shape: a cell may hold a
// paragraph AND a list, and both have to survive. It also covers the "more than one
// block" answer in isOneParagraph, which the single-list case cannot reach.
func TestACellWithProseAndAListKeepsBoth(t *testing.T) {
	back, err := Parse([]byte("\\begin{tabular}{ll}\na & lead in\n\n\\begin{itemize}\n\\item one\n\\end{itemize} \\\\\n\\end{tabular}\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cell := back.Blocks[0].(richdoc.Table).Rows[0][1]
	if len(cell.Blocks) < 2 {
		t.Fatalf("want at least two blocks in the cell, got %#v", cell.Blocks)
	}
	if _, ok := cell.Blocks[0].(richdoc.Paragraph); !ok {
		t.Errorf("the first block is %T, want a Paragraph", cell.Blocks[0])
	}
	if _, ok := cell.Blocks[len(cell.Blocks)-1].(richdoc.List); !ok {
		t.Errorf("the last block is %T, want a List", cell.Blocks[len(cell.Blocks)-1])
	}
}
