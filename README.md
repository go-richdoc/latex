# latex

A **LaTeX ⇄ [richdoc](https://github.com/go-richdoc/richdoc)** converter, written
in pure Go (CGO-free, including `GOOS=js`).

`latex` parses a practical subset of LaTeX into the neutral `richdoc` document
model, and emits a minimal, compilable LaTeX article from a `richdoc.Document`.
The two directions are designed as a faithful round-trip.

```go
d, err := latex.Parse(src)   // LaTeX subset -> *richdoc.Document
out, err := latex.Write(d)   // *richdoc.Document -> compilable LaTeX article
```

## API

```go
func Parse(src []byte) (*richdoc.Document, error)
func Write(d *richdoc.Document) ([]byte, error)
```

`Parse` reads only the body of the `document` environment when one is present
(the preamble is mined for metadata); a bare fragment with no `document`
environment is parsed whole. Anything the model has no node for is preserved
verbatim through `RawInline`/`RawBlock` with `Format: "latex"`, so nothing is
lost. `Write` loads only the packages the document needs and escapes LaTeX
specials in text.

## Construct mapping

The supported subset maps to `richdoc` as follows (both directions):

| LaTeX | richdoc |
| --- | --- |
| `\section` / `\subsection` / `\subsubsection` / `\paragraph` / `\subparagraph` | `Heading` (level 1–5) |
| blank-line-separated text | `Paragraph` |
| `\textbf{}` | `Strong` |
| `\textit{}`, `\emph{}` | `Emph` |
| `\texttt{}` | `Code` (inline) |
| `\sout{}` (ulem) | `Strikethrough` |
| `\\`, `\newline` | `LineBreak` |
| `itemize` / `enumerate` (`\item`) | `List` (ordered for `enumerate`) |
| `verbatim` / `lstlisting` | `CodeBlock` (`lstlisting[language=…]` sets the language) |
| `quote` / `quotation` | `BlockQuote` |
| `tabular` | `Table` (`&` cells, `\\` rows, `l\|c\|r` spec → alignment, `\hline` dropped) |
| `\href{url}{text}`, `\url{}` | `Link` |
| `\includegraphics[…]{path}` | `Image` |
| `$…$`, `\(…\)` | `Math` (inline) |
| `\[…\]`, `equation`, `align`, … | `MathBlock` |
| `\hrulefill`, `\rule…` | `ThematicBreak` |
| `\documentclass`, `\title`, `\author`, `\date` | `Document.Meta` |
| `\maketitle` | dropped (title comes from `Meta`) |
| unknown command / environment | `RawInline` / `RawBlock` (`Format: "latex"`) |

`Write` emits `\documentclass{article}` plus only the packages in use
(`fontenc`, `graphicx`, `hyperref`, `amsmath`, `ulem`, `listings`), maps
`Meta` title/author/date onto `\title`/`\author`/`\maketitle`, and renders each
block and inline node back to LaTeX.

## Parsing

`richdoc` is a document model with no parser, and
[`go-tex/engine`](https://github.com/go-tex/engine) is a typesetting engine
whose tokenizer and macro machinery are internal — it exposes a compile API, not
a reusable structured parse tree. So `latex` ships its own focused,
well-tested LaTeX-subset parser (no full TeX macro expander, no non-Go
dependency). The go-tex engine is used to **prove** the round-trip: a test
compiles `Write`'s output with the engine and asserts it typesets.

## License

BSD-3-Clause. Copyright (c) the go-richdoc authors.

## To a PDF

`latex/pdf` typesets a document rather than writing source for one:

```go
data, err := pdf.Write(doc, pdf.Options{})   // *richdoc.Document -> PDF bytes
```

It typesets nothing itself. The document goes out as LaTeX through `Write` above
and is compiled by [go-tex/engine](https://github.com/go-tex/engine), a pure-Go
TeX engine that runs the genuine LaTeX classes — so what comes back is a page
TeX laid out, with TeX's line breaking, rather than an approximation of one.

That composition worked before the package existed; what was missing was
somewhere to put it. Every converter in this organisation reaches `richdoc`, so
every one of them now reaches a PDF.

**It is a package rather than a module, and not part of this one.** The engine
is a six-megabyte TeX implementation. This module already names it, but only
from a test — checking that the LaTeX it emits actually typesets — so importing
`latex` does not link it. Putting `Write` beside the LaTeX writer would link it
into every consumer that only wanted LaTeX text. A separate module would avoid
that and bring another repository, another set of shared defaults and another
release to keep in step, for thirty-five lines composing two libraries. Go links
by package, so a package here costs neither.

What survives the whole way, read back out of the finished PDF with `pdftotext`
rather than trusted: headings, emphasis, bold, bulleted and numbered lists,
inline and block code, a quotation, a table, a link, a rule and accented text.
