---
name: memgraph-docs
description: Use this skill whenever interacting with a memgraph-docs MCP server — the document-shaped layer over memgraph. Covers how to create a document, build it up with headings/paragraphs/lists/code/quotes/media/dividers, structure sections hierarchically with parent_id, edit text in place via versioning, reorder via move, and render the final document as CommonMark markdown. Invoke any time tools named docs_* are available and you're being asked to write, edit, organize, or render a document.
---

# memgraph-docs — agent guide

memgraph-docs turns a memgraph graph into structured documents. You build a document by appending content nodes (headings, paragraphs, lists, code, quotes, media, dividers) under a document root. The renderer walks the tree and emits CommonMark markdown.

This guide tells you how to use it well. The first principle: **think of a document as a tree, not a stream of text**. Headings can be parents of their sections. Sections become subtrees you can fetch, move, or restructure independently.

## The mental model in 60 seconds

- Every document is rooted at a `doc.document` node. Its `lineage_id` is the **document_id** you'll pass everywhere.
- Content lives in **typed child nodes**: headings, paragraphs, lists, code blocks, quotes, media, dividers.
- Children are connected to parents via `doc.contains` edges with an integer `ordinal` for sibling order.
- Hierarchical sectioning is opt-in: every append accepts an optional `parent_id`. Omit it and the new node goes under the document root. Pass a heading's lineage_id and the new node becomes part of that section's subtree.
- Text edits create new memgraph versions automatically. The document tree references lineages, so links don't break.

Available tools: `docs_create_document`, `docs_list_documents`, `docs_get_outline`, `docs_render_markdown`, `docs_append_heading`, `docs_append_paragraph`, `docs_append_list`, `docs_append_code`, `docs_append_quote`, `docs_append_media`, `docs_append_divider`, `docs_update_text`, `docs_move`.

---

## A typical authoring flow

```
1. docs_list_documents(graph_id)
   — does the document already exist?

2. docs_create_document(graph_id, title, author?, abstract?)
   — returns document_id (= lineage_id of the root)

3. For each major section:
     docs_append_heading(document_id, level=1, text="Summary")  → returns heading_id
     docs_append_paragraph(document_id, parent_id=heading_id, text="...")
     docs_append_paragraph(document_id, parent_id=heading_id, text="...")

4. For lists / code / media interleaved:
     docs_append_list(document_id, parent_id=heading_id, ordered=false, items=[...])
     docs_append_code(document_id, parent_id=heading_id, code="...", language="go")

5. docs_render_markdown(document_id)
   — returns the assembled document as a markdown string
```

The key habit: **capture the section heading's lineage_id when you create it, then pass it as `parent_id` to subsequent content**. That's what makes the structure hierarchical instead of flat.

---

## Structuring documents well

### Decide flat vs hierarchical upfront

- **Flat** (no parent_id): the document root holds all content in order. Headings are visual markers, not containers. Simpler; fine for short notes.
- **Hierarchical** (parent_id = heading_id): each heading owns its content as children. Lets you fetch, move, or render whole sections atomically. Use this for anything with sections you might want to restructure later.

The renderer produces the same visual markdown either way. The difference is structural — and structure pays off when you later call `docs_move` or `docs_get_outline`.

### Heading levels

- `level: 1` — top-level document sections (Summary, Background, Conclusion)
- `level: 2` — major subsections inside a section
- `level: 3` — minor sub-subsections, when really needed
- `level: 4+` — usually a smell. Reconsider whether the structure is too deep.

Heading levels are independent of tree depth. A `level: 2` heading can live at any tree position — the renderer emits `##` regardless. Levels are for *visual* hierarchy; tree position is for *structural* hierarchy.

The renderer auto-emits the document title as an H1 *unless* the first child is already a level-1 heading. If you want full control, lead with your own H1 and skip relying on the title.

### Paragraphs

One paragraph per node. Don't pack multiple paragraphs into a single `docs_append_paragraph` call — the renderer treats `\n\n` literally and the result is one ugly multi-paragraph node. Make multiple calls instead, one per paragraph.

Markdown formatting inside paragraph text (emphasis, links, inline code) renders through verbatim. memgraph-docs v0.1 doesn't model inline marks as separate nodes; whatever you write in `text` is what readers see.

### Lists

- `docs_append_list` takes `items` as an array of strings — it creates the list node and all list_items in one call.
- Set `ordered: true` for numbered lists, `false` for bullets.
- **Nested lists**: append a list with `parent_id` set to a list_item's lineage_id. The renderer indents two spaces per nesting level.

If you want list items with rich content (e.g. a paragraph + a code block inside one bullet), build the list_item manually: create a list, then for each item create a list_item via the lower-level mechanism. v0.1's `docs_append_list` is shorthand for simple flat lists.

### Code blocks

Always pass `language` when known. The renderer emits a fenced block with the language hint so syntax highlighters work.

```
docs_append_code(document_id, parent_id=heading_id, code="...", language="go")
```

Code with no language is fine — emit empty string or omit.

### Quotes

`docs_append_quote` produces a `>` block. Multi-line quotes split on `\n` and each line gets prefixed. Useful for embedded notes, references, or callout-style content.

### Media

`docs_append_media` references external resources by URL.

- `media_kind: image` (default) — rendered as `![alt](src)` plus optional italic caption underneath
- `media_kind: video|audio|file` — rendered as a link `[alt](src)` plus optional italic caption
- `src` is required. Use http/https URLs or `file://` paths for local refs.
- `alt` is for accessibility; agents should always pass it for images.
- `caption` becomes italic text beneath the media.

memgraph-docs v0.1 does not store blob bytes — only URLs. Host the media elsewhere.

### Dividers

`docs_append_divider` emits a horizontal rule (`---`). Useful for separating loosely connected sections without using a heading.

---

## Editing existing documents

### Update text in place

`docs_update_text(node_id, text)` creates a new version of the node with the new text. The document tree still points at the same lineage, so the edit appears immediately on next render.

History is preserved. Old versions are reachable via memgraph's `memgraph_history` tool (if available) but invisible to the document renderer.

### Move and reorder

`docs_move(node_id, new_parent_id, position?)`:

- Reparents a node to a new parent
- Optional `position` is a 0-indexed insertion point among the new parent's children; omit to append
- Same-parent move with a different `position` just reorders
- Ordinals get compacted to 1..N on both old and new parents

Use this to:

- Promote a paragraph into a new section (move it under a different heading)
- Reorder list items
- Restructure when an outline evolves

### Avoid: appending then re-appending

If you got the structure wrong, **use `docs_move` rather than appending duplicates**. The graph keeps every node forever in history, so duplicates are real cost.

---

## Reading and rendering

### Get an outline first

Before editing a doc you didn't write, call `docs_get_outline(document_id)`. It returns a tree of just the structural nodes (headings, lists, etc.) with their lineage IDs but without paragraph bodies. Cheap and informative.

Use `max_depth: 2` (or 3) to limit traversal on large documents.

### Render markdown for output

`docs_render_markdown(document_id)` walks the whole tree and returns a CommonMark string. Use it to:

- Show the user a finished document
- Hand off to another tool (paste into Slack, save to file, etc.)
- Verify your structure looks right after a series of appends

---

## Common workflows

### Capturing a new document from scratch

```
doc = docs_create_document(graph_id, title="Outage 2026-05-16", author="ops")

summary = docs_append_heading(doc.lineage_id, level=1, text="Summary")
docs_append_paragraph(doc.lineage_id, parent_id=summary.lineage_id, text="...")

timeline = docs_append_heading(doc.lineage_id, level=1, text="Timeline")
docs_append_list(doc.lineage_id, parent_id=timeline.lineage_id, ordered=true,
                 items=["09:42 — first alert", "09:46 — pager", "10:01 — mitigated"])

actions = docs_append_heading(doc.lineage_id, level=1, text="Followups")
docs_append_list(doc.lineage_id, parent_id=actions.lineage_id, ordered=false,
                 items=["Add health-check timeout", "Document runbook"])

print(docs_render_markdown(doc.lineage_id))
```

### Ingesting prose from the user

When a user dumps a long markdown blob and says "save this as a doc":

1. Parse the markdown structure yourself (recognize `#`, paragraphs, lists, code fences).
2. `docs_create_document` for the root.
3. For each block in the parsed structure, call the matching `docs_append_*` tool with `parent_id` set appropriately.
4. `docs_render_markdown` to confirm the roundtrip.

Don't try to stuff the whole blob into a single `docs_append_paragraph`. The point of memgraph-docs is the structure.

### Continuing a document later

```
docs_list_documents(graph_id)           → find the doc, get lineage_id
docs_get_outline(document_id, max_depth=2)  → understand the structure
docs_append_heading(document_id, level=1, text="New section")
... continue building
docs_render_markdown(document_id)
```

### Restructuring an outline

```
outline = docs_get_outline(document_id)
— identify the heading you want to promote/demote/move
docs_move(node_id=heading_id, new_parent_id=document_id, position=0)
docs_render_markdown(document_id)
```

---

## Anti-patterns to avoid

- **Stuffing multi-paragraph prose into one `docs_append_paragraph` call.** Make multiple calls. The structure is the value.
- **Using flat mode and skipping headings.** Without headings the document has no outline; renames and moves get painful.
- **Appending duplicates instead of `docs_move`.** Use the move tool. History keeps everything forever.
- **Using level: 4+ headings.** Almost always a smell. Restructure with subsections instead.
- **Omitting `alt` on images.** Always pass alt text for accessibility.
- **Trying to embed blob bytes.** v0.1 takes URLs only. Host the file elsewhere.
- **Calling `docs_render_markdown` mid-build to "see what it looks like" many times.** It traverses the whole tree each time. Render at the end, not after every append.

---

## Relationship to memgraph

memgraph-docs and memgraph share the same SQLite (or Postgres) store. If both MCP servers are connected:

- Use **`docs_*` tools** for document authoring and reading.
- Use **`memgraph_*` tools** for raw graph queries: searching across documents, finding all docs that mention an entity, linking documents into a larger knowledge graph, etc.

When in doubt for a document-related task: prefer `docs_*` tools. They keep the structure consistent. Drop down to `memgraph_*` only when you need graph-shaped operations the docs layer doesn't provide.
