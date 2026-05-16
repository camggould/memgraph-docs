// Package docs is a thin layer over memgraph that models documents as trees
// of typed nodes (headings, paragraphs, lists, media, etc.) connected by
// ordered doc.contains edges. The package provides a Builder for writing
// document trees into a memgraph.Store and a renderer for emitting them as
// markdown. See DESIGN.md for the conceptual model.
package docs
