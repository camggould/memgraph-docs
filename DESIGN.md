# memgraph-docs — Design

**Status:** Draft v0.1
**Author:** Cam Gould
**Last updated:** 2026-05-16

memgraph-docs is the L2 reference client from [memgraph](https://github.com/camggould/memgraph) PRD §10 — an MCP server that turns a memgraph graph into a tree of structured document nodes and reconstructs it back into a renderable document.

It is a thin layer on top of memgraph. The schema lives in memgraph nodes and edges; memgraph-docs adds a vocabulary of `kind` values, a builder API, and a markdown renderer.

---

## Architecture

memgraph-docs embeds memgraph as a Go library and exposes its own MCP server with `docs_*` tools. It does **not** wrap or proxy memgraph's raw tools — those are a separate concern. If users want raw access, they run memgraph alongside.

```
agent ──MCP──▶ memgraph-docs serve ──Go library──▶ memgraph.Store ──▶ SQLite / Postgres
                  (docs_* tools)
```

One process, one MCP connection, one stdio. Internally memgraph-docs holds an open `memgraph.Store` and writes/reads from it.

---

## Node kinds

All document content lives in memgraph nodes. The `kind` field carries the document semantic. Every kind is namespaced with `doc.` to keep it separable from other clients.

| Kind | Content | Metadata fields | Notes |
|------|---------|-----------------|-------|
| `doc.document` | title | `author`, `abstract` | Root node of a document. |
| `doc.heading` | heading text | `level` (1–6) | Can be a parent of subsequent content (hierarchical sectioning). |
| `doc.paragraph` | paragraph text | — | Inline rich text is rendered raw (markdown passes through). |
| `doc.list` | — | `ordered` (bool) | Container; children are `doc.list_item`s. |
| `doc.list_item` | item text | — | Can have nested `doc.list` children. |
| `doc.code_block` | code | `language` | Rendered with fenced code block. |
| `doc.quote` | quote text | — | Rendered as `>` block. |
| `doc.media` | caption | `kind` (`image`/`video`/`audio`/`file`), `src` (URL), `alt`, `width`, `height` | URL-based refs for v0.1. |
| `doc.divider` | — | — | Horizontal rule. |

Clients can introduce their own kinds (`doc.callout`, `doc.table`, etc.) and the renderer will fall back to a best-effort representation. memgraph-docs is permissive: unknown kinds render as their `content`.

## Edges

One edge kind only:

- **`doc.contains`** — directional from parent → child. Sibling order is the edge's `ordinal` field (memgraph already provides this).

The document tree is the union of `doc.contains` edges rooted at the `doc.document` node.

## Ordinals

Children of a node are ordered by their incoming edge's `ordinal`. Append assigns `max(siblings) + 1` (or 1 if no siblings). Move can rewrite ordinals; the package compacts to `1..N` to keep numbers small.

## Versioning

Every text edit creates a new version of the underlying node via memgraph's lineage system. The document tree references lineages (via `to_lineage` on edges), so edits don't break parent/child links. History is preserved automatically.

## Rendering

`docs_render_markdown(document_id)` walks the tree depth-first by ordinal and emits CommonMark. Each kind has a renderer; unknown kinds emit their content as a plain paragraph.

Hierarchical headings render at their declared `level`. A heading's children appear after it; the renderer doesn't try to enforce that heading levels match document depth.

## MCP tool surface

**Read:**

- `docs_list_documents(graph_id)` — all root document nodes in a graph
- `docs_get_outline(document_id, max_depth?)` — hierarchical structure with summaries (headings + structural nodes; no body text)
- `docs_render_markdown(document_id)` — full document as a markdown string

**Write — content:**

- `docs_create_document(graph_id, title, author?, abstract?, created_by?)`
- `docs_append_heading(document_id, level, text, parent_id?, created_by?)`
- `docs_append_paragraph(document_id, text, parent_id?, created_by?)`
- `docs_append_list(document_id, items[], ordered, parent_id?, created_by?)` — atomically creates the list + items
- `docs_append_code(document_id, code, language, parent_id?, created_by?)`
- `docs_append_quote(document_id, text, parent_id?, created_by?)`
- `docs_append_media(document_id, media_kind, src, alt?, caption?, width?, height?, parent_id?, created_by?)`
- `docs_append_divider(document_id, parent_id?, created_by?)`

All `parent_id` arguments default to the document root.

**Write — edits:**

- `docs_update_text(node_id, text, created_by?)` — creates a new version of an existing node with the supplied text
- `docs_move(node_id, new_parent_id, position?, created_by?)` — reparents and/or reorders; `position` is 0-indexed ordinal target

## Out of scope (v0.1)

- HTML / JSON rendering — markdown only for v0.1.
- Inline marks (bold, italic, link). Rich text inside paragraphs is whatever the agent writes; rendered raw. Adding inline marks as separate nodes is a v0.2+ concern.
- Tables. Likely a custom kind in a later version.
- Content-addressed blob storage. URLs only.
- Multi-document references (transclusion).
- Concurrent editing CRDT semantics — memgraph's lineage versioning is the floor; richer merge UX comes later.

## Glossary

- **Document:** a tree rooted at a `doc.document` node, connected by `doc.contains` edges.
- **Document ID:** the `LineageID` of the root node (stable across versions).
- **Node ID (in MCP):** the `LineageID` of a content node. memgraph's underlying versioned ID is hidden from this layer.
