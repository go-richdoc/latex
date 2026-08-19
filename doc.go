// Copyright (c) the go-richdoc authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package latex converts between a practical subset of LaTeX and the neutral
// [github.com/go-richdoc/richdoc] document model.
//
// [Parse] reads LaTeX source (only the body of the document environment, when
// present) into a [richdoc.Document]; [Write] emits a minimal, compilable
// article from a [richdoc.Document]. The two are designed as a faithful
// round-trip: Parse(Write(Parse(src))) is semantically equal to Parse(src) for
// the supported subset.
//
// Constructs the model has no native node for are preserved verbatim through
// [richdoc.RawInline] / [richdoc.RawBlock] with Format "latex", so nothing in
// the source is silently lost.
//
// The package is pure Go and builds with CGO disabled, including for
// GOOS=js/GOARCH=wasm.
package latex
