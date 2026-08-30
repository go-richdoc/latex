// Copyright (c) the go-richdoc authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package pdf turns a richdoc document into a typeset PDF.
//
// It typesets nothing itself. The document goes out as LaTeX through the
// parent package and is compiled by go-tex/engine, a pure-Go TeX engine that
// runs the genuine LaTeX classes — so what comes back is a page TeX laid out,
// with TeX's line breaking, rather than an approximation of one.
//
// That composition worked before this package existed; what was missing was
// somewhere to put it. Every converter in this organisation reaches richdoc,
// so every one of them now reaches a PDF: Markdown, reStructuredText, LaTeX,
// and whatever is written next.
//
// # Why a package rather than a module, and rather than the parent
//
// The engine is a six-megabyte TeX implementation. The parent package already
// names it, but only from a test — it checks that the LaTeX it emits actually
// typesets — so importing latex today does not link it. Putting this function
// beside Write would link it into every consumer that only wanted LaTeX text.
//
// A separate module would not have that problem and would bring another:
// another repository, another set of shared defaults, another release to keep
// in step, for thirty-five lines that compose two libraries. Go links by
// package, so a package here costs neither.
//
// # What survives
//
// Thirteen Markdown features taken through the whole chain and read back out
// of the finished PDF with pdftotext — headings, emphasis, bold, bulleted and
// numbered lists, inline and block code, a quotation, a table, a link, a rule
// and accented text. All thirteen.
package pdf
