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
| `table` float | the `Table` inside it, with `\caption` as `Table.Caption` |
| `\href{url}{text}`, `\url{}` | `Link` |
| `\includegraphics[…]{path}` | `Image`, with graphicx's `width`, `height` and `scale` keys (see below) |
| `\footnote{…}` | `Footnote` (inline arg wrapped in one `Paragraph`) |
| `\label{id}` | `Anchor` (point target); hoisted onto `Heading.ID` right after a `\section…` |
| `\ref{id}`, `\eqref{id}` | `CrossRef` (`RefLabel`) |
| `\cite{key}` | `CrossRef` (`RefCite`) |
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

Two mappings normalise on the way back to LaTeX. A `\section…` (any level)
immediately followed by `\label{id}` (only whitespace or comments between) is
folded into a single `Heading` with that `ID`, and `Write` re-emits the
`\label` right after the section command — so the pair round-trips as one
`Heading`. Both `\ref` and `\eqref` parse to a `CrossRef` of kind `RefLabel`,
and `Write` always emits `\ref`; an `\eqref` source therefore round-trips as
`\ref` (a benign normalisation, since both denote the same labelled reference).
`\footnote`, `\label`, `\ref` and `\cite` are all core LaTeX, so the emitted
output typesets with no extra package. A `\label` that does not immediately
follow a heading stays a point `Anchor` in the inline stream. Only genuinely
unrecognised commands and environments still fall back to `RawInline` /
`RawBlock`.

An inline `RawInline` with `Format: "latex"` is written with a trailing
newline, since adjacent inline nodes are concatenated with no separator of
their own: raw content ending in a bare control word (e.g. `\bfseries`, no
argument or trailing space) run directly into the next inline's text would
otherwise be swallowed into the same, now-undefined, control sequence name
and fail to compile — a real, previously-latent bug, since nothing produced
an inline `RawInline` end-to-end until `go-richdoc/rst` gained a role
registry (v0.16.0). TeX treats the inserted newline as ordinary whitespace,
which a control word silently consumes, so this is invisible in the typeset
output; the one cosmetic cost is that re-`Parse`-ing such output can pick up
one extra leading space on whatever inline immediately followed the raw
content in source that had none of its own — round-trip-exact whenever the
author's own markup already had a separating space there, which is the
common case. `RawBlock` needs no equivalent since `Write` always joins
blocks (and list items) with at least one newline already.

## Parsing

`richdoc` is a document model with no parser, and
[`go-tex/engine`](https://github.com/go-tex/engine) is a typesetting engine
whose tokenizer and macro machinery are internal — it exposes a compile API, not
a reusable structured parse tree. So `latex` ships its own focused,
well-tested LaTeX-subset parser (no full TeX macro expander, no non-Go
dependency). The go-tex engine is used to **prove** the round-trip: a test
compiles `Write`'s output with the engine and asserts it typesets.

## A cell that holds blocks (richdoc v0.5.0)

A LaTeX cell can hold a list, a verbatim block or several paragraphs, and
`richdoc.Cell` can now carry that: `Blocks` holds the real content and `Inlines`
the flattened view a consumer that predates the field still reads. This converter
writes and reads both.

What makes it legal is the COLUMN SPEC, not the cell. An `l` column is a single
line, and a list inside one is an error — compiled with tectonic, the same table
with `ll` fails at the itemize:

```
! LaTeX Error: Something's wrong--perhaps a missing \item.
```

and with a `p` column it produces a PDF. So a column holding block content is
written as `p{\dimexpr\linewidth/<ncols>-2\tabcolsep\relax}` — an equal share of
the line, minus the padding LaTeX puts on both sides of every cell.

The reference answers the same question the same way. Asked for the LaTeX of a grid
table whose cell holds a bullet list, docutils emits

```latex
\begin{longtable*}{|p{0.051\DUtablewidth}|p{0.179\DUtablewidth}|}
a & \begin{itemize}\item one \item two\end{itemize} \\
```

a `p` column with the itemize **directly inside it**, no parbox and no minipage.
Its widths come from the colspecs docutils' own parser computes, which this model
does not carry, so an equal share is the honest default.

A table with only inline cells keeps the `ll…` spec it always had, which
`TestAPlainTableKeepsItsLColumns` holds.

### Where a caption goes

`\caption` outside a float is an error, so a captioned table is wrapped in the
`table` environment with the caption after the tabular, and the parse side reads
that float back into `Table.Caption`. A table without a caption gets no float.

## The graphicx keys

`\includegraphics`'s option list used to be read and **discarded**: `richdoc.Image`
had no field for any of it, so every `width`, `height` and `scale` a LaTeX author
wrote was lost on the way in, and a bare `\includegraphics` came out. Since
richdoc v0.4.0 the three keys richdoc can hold survive the trip.

The **unit travels with the length**, because that is what a length is in TeX:
`5cm` and `0.5\linewidth` are both legal and mean different things, and a
converter that dropped the unit would have to invent one.

`scale` is the one conversion. graphicx's `scale` is a **factor** (`scale=0.5`)
and `richdoc.Scale` is a **percentage**, matching reST's `:scale: 50` — docutils'
own latex2e writer makes exactly that translation in the other direction. Keeping
the model on the percentage means one converter does the arithmetic instead of
every reader of the model guessing which convention it holds.

The keys are written back in latex2e's own order (height, scale, width), so output
from this package and from [go-docutils/docutils](https://github.com/go-docutils/docutils)
reads the same way. A `scale` this package cannot read as a number — graphicx
accepts an expression where a factor is expected — leaves `Scale` alone rather than
setting it to 0 percent, which would be indistinguishable from "not given".

Not carried: `angle`, `trim`, `clip`, `viewport` and the rest of graphicx's keys,
and the `\noindent\makebox` wrappers docutils emits for `:align:` — richdoc has
`Image.Align`, but recognising those wrappers on the way in is a separate piece of
work from reading a key list.

## Toolchain

Requires **Go 1.27.2**, which is the version the CI workflow pins. The two were out of
step — the module asked for 1.26.4 while CI already ran 1.27.1 — and that difference
decides two things a reader of this repository should not have to guess: 1.27 counts
statements more finely, so a coverage figure from an older toolchain is an upper bound
rather than a measurement, and its `gofmt` reindents a composite literal inside a
multi-value return, which an older local `gofmt` reports as clean. Both were checked
with 1.27.1 here: coverage is unchanged at 100%, and the tree needs no reformatting.

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
