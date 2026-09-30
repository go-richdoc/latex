package latex

import (
	"reflect"
	"strings"
	"testing"

	"github.com/go-richdoc/richdoc"
)

// TestGraphicxKeysSurviveTheRoundTrip pins what richdoc v0.4.0 made possible
// here. This package read an \includegraphics option list and THREW IT AWAY --
// richdoc.Image had no field for any of it -- so every width, height and scale a
// LaTeX author wrote was lost on the way in and a bare \includegraphics came out.
//
// SCALE is the one conversion. graphicx's scale is a FACTOR and richdoc.Scale is
// a PERCENTAGE, matching reST's ":scale: 50"; docutils' own latex2e writer makes
// exactly that translation in the other direction.
func TestGraphicxKeysSurviveTheRoundTrip(t *testing.T) {
	cases := []struct {
		name, src string
		want      richdoc.Image
		wantOut   string
	}{
		{
			"width with a unit",
			`\includegraphics[width=5cm]{pic.png}`,
			richdoc.Image{URL: "pic.png", Width: "5cm"},
			`\includegraphics[width=5cm]{pic.png}`,
		},
		{
			"a width relative to the line",
			`\includegraphics[width=0.5\linewidth]{pic.png}`,
			richdoc.Image{URL: "pic.png", Width: `0.5\linewidth`},
			`\includegraphics[width=0.5\linewidth]{pic.png}`,
		},
		{
			"height and width, in latex2e's own key order",
			`\includegraphics[width=5cm,height=2cm]{pic.png}`,
			richdoc.Image{URL: "pic.png", Width: "5cm", Height: "2cm"},
			`\includegraphics[height=2cm,width=5cm]{pic.png}`,
		},
		{
			"scale is a factor here and a percentage in the model",
			`\includegraphics[scale=0.5]{pic.png}`,
			richdoc.Image{URL: "pic.png", Scale: 50},
			`\includegraphics[scale=0.5]{pic.png}`,
		},
		{
			// CONTROL: no options at all, which is what every image in this
			// package used to become. It passes either way.
			"no keys",
			`\includegraphics{pic.png}`,
			richdoc.Image{URL: "pic.png"},
			`\includegraphics{pic.png}`,
		},
	}
	for _, c := range cases {
		doc, err := Parse([]byte(c.src))
		if err != nil {
			t.Fatalf("%s: Parse: %v", c.name, err)
		}
		var got richdoc.Image
		found := false
		for _, b := range doc.Blocks {
			p, ok := b.(richdoc.Paragraph)
			if !ok {
				continue
			}
			for _, in := range p.Inlines {
				if img, ok := in.(richdoc.Image); ok {
					got, found = img, true
				}
			}
		}
		if !found {
			t.Errorf("%s: no image in %#v", c.name, doc.Blocks)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: Image =\n%#v\nwant:\n%#v", c.name, got, c.want)
		}
		out, err := Write(doc)
		if err != nil {
			t.Fatalf("%s: Write: %v", c.name, err)
		}
		if !strings.Contains(string(out), c.wantOut) {
			t.Errorf("%s: want %q in:\n%s", c.name, c.wantOut, out)
		}
	}
}

// TestAScaleThatIsNotANumberIsIgnored pins the parse side's own guard: graphicx
// accepts a length expression where this package expects a factor, and a value it
// cannot read must leave Scale alone rather than become 0 percent — which would
// be indistinguishable from "not given" and is anyway a meaningless image.
func TestAScaleThatIsNotANumberIsIgnored(t *testing.T) {
	doc, err := Parse([]byte(`\includegraphics[scale=\myfactor,width=3cm]{p.png}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	p := doc.Blocks[0].(richdoc.Paragraph)
	for _, in := range p.Inlines {
		if img, ok := in.(richdoc.Image); ok {
			if img.Scale != 0 {
				t.Errorf("Scale = %d, want 0 for a value that is not a number", img.Scale)
			}
			// And the key it COULD read still arrived.
			if img.Width != "3cm" {
				t.Errorf("Width = %q, want 3cm", img.Width)
			}
		}
	}
}
