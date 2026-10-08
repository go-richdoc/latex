// Copyright (c) the go-richdoc authors.
// SPDX-License-Identifier: BSD-3-Clause

package pdf

import (
	"bytes"
	"fmt"
	"io"

	"github.com/go-opentype/fonts/lora"
	"github.com/go-richdoc/latex"
	"github.com/go-richdoc/richdoc"
	"github.com/go-tex/engine"
)

// Options say how to typeset.
type Options struct {
	// Strict aborts on the first thing the engine cannot do, as TeX does.
	//
	// The default is lenient, which is what a document arriving from another
	// format wants: a converter emits ordinary LaTeX, and a gap in the engine
	// should cost the reader a paragraph rather than the whole document. A
	// caller writing its own LaTeX and wanting to be told when it is wrong
	// asks for strict.
	Strict bool
}

// Write typesets a document and returns the PDF.
func Write(doc *richdoc.Document, opt Options) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := WriteTo(&buf, doc, opt); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteTo typesets a document into w and says how many pages it came to.
//
// The page count is worth having. A document that typesets to nothing is not
// an error anywhere in the chain — LaTeX with no body compiles — so a caller
// about to hand somebody an empty file would otherwise have no way to know.
func WriteTo(w io.Writer, doc *richdoc.Document, opt Options) (pages int, err error) {
	if doc == nil {
		return 0, fmt.Errorf("pdf: there is no document to typeset")
	}
	src, err := writeLaTeX(doc)
	if err != nil {
		// The parent package cannot fail today — it writes into a strings
		// Builder and returns no error anywhere. The check stays because that
		// is a fact about its present shape rather than a promise, and a
		// silent discard here would turn a future failure into a PDF of
		// nothing. It is reachable from a test through the seam below.
		return 0, fmt.Errorf("pdf: writing the document as LaTeX: %w", err)
	}
	pages, err = compile(src, engine.Options{
		Lenient: !opt.Strict,
		// ⛔ All three faces, and the reason is a defect this had for as long
		// as it existed. The engine's own default is a MATHS face used as
		// text, with no bold and no italic to switch to — so \textbf and \emph
		// compiled, laid out, and came back ROMAN. Nothing said so: the LaTeX
		// carried \textbf, the engine did not complain, the page count was
		// right, and every emphasis in every converted document was quietly
		// flattened. An ODT that loses every bold word is not a converted
		// document, it is a flattened one.
		//
		// Lora because it is a text face carrying a real bold and a real
		// italic of the same family, which is the whole requirement. The
		// engine binds them to \bf and \it itself.
		Font:       lora.TTF,
		BoldFont:   lora.BoldTTF,
		ItalicFont: lora.ItalicTTF,
	}, w)
	if err != nil {
		return 0, fmt.Errorf("pdf: typesetting it: %w", err)
	}
	return pages, nil
}

// The two seams. Each is a variable so a test can watch what happens when the
// step behind it refuses — which is the difference between a caller being
// handed half a file and being told.
var (
	writeLaTeX = latex.Write
	compile    = engine.CompileToPDF
)
