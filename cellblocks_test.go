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
