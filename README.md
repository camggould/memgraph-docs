# memgraph-docs

Build human-readable documents on top of [memgraph](https://github.com/camggould/memgraph), an agentic knowledge substrate. Exposed as an MCP server so agents can write — and reconstruct — structured documents (headings, paragraphs, lists, code, quotes, media refs) as a tree of memgraph nodes.

> **Status:** Alpha (v0.1.0). See [DESIGN.md](DESIGN.md) for the model.

## What it does

memgraph stores facts as atomic nodes with first-class edges. memgraph-docs adds a vocabulary on top: a `doc.document` node holds a tree of typed content children connected by `doc.contains` edges. An agent can:

- create a document, append headings/paragraphs/lists/code/quotes/media/dividers,
- nest content under headings (or under other nodes — sections become subtrees),
- edit a node (new memgraph version) or move it around,
- reconstruct the whole tree as a CommonMark markdown string.

Because everything lives in memgraph, you also get versioning, lineage, freshness, and graph-shaped queries for free. memgraph-docs doesn't reimplement any of that.

## Install

### One-line install (macOS, Linux)

> Pre-built binaries arrive with the first tagged release. Until then, use `go install` or build from source.

### `go install`

```sh
go install github.com/camggould/memgraph-docs/cmd/memgraph-docs@latest
```

Requires Go 1.25+. Binary lands in `$(go env GOBIN)` (or `$GOPATH/bin`).

### Build from source

```sh
git clone https://github.com/camggould/memgraph-docs.git
cd memgraph-docs
go build -o memgraph-docs ./cmd/memgraph-docs
```

Pure-Go build, no cgo. Cross-compile for any supported target:

```sh
GOOS=linux GOARCH=amd64 go build -o memgraph-docs-linux-amd64 ./cmd/memgraph-docs
```

## Quickstart

memgraph-docs runs as an MCP server over stdio. It opens a memgraph SQLite store and exposes `docs_*` tools.

```sh
memgraph-docs serve --sqlite ~/.memgraph/store.db
```

The store file is the same one memgraph uses. You can run both servers against the same file (just not at the same time — SQLite is single-writer).

### Wire it into an MCP client

```json
{
  "mcpServers": {
    "memgraph-docs": {
      "command": "memgraph-docs",
      "args": ["serve", "--sqlite", "/Users/you/.memgraph/store.db"]
    }
  }
}
```

For Claude Code:

```sh
claude mcp add memgraph-docs -- memgraph-docs serve --sqlite ~/.memgraph/store.db
```

### Sample agent flow

```
> Write a short doc in my notes graph about today's outage.

[agent calls docs_create_document(graph_id="...", title="Outage 2026-05-16")]
[agent calls docs_append_heading(level=1, text="Summary")]
[agent calls docs_append_paragraph(text="...")]
[agent calls docs_append_heading(level=1, text="Timeline")]
[agent calls docs_append_list(ordered=true, items=["09:42 — pager"...])]
[agent calls docs_render_markdown(...)]
```

The render output is a CommonMark string ready to paste into a wiki, doc tool, or email.

## MCP tools

**Read:**

| Tool | Purpose |
|---|---|
| `docs_list_documents` | All root document nodes in a graph |
| `docs_get_outline` | Structural tree (headings + containers, no bodies) |
| `docs_render_markdown` | Full document as a markdown string |

**Write — content:**

| Tool | Purpose |
|---|---|
| `docs_create_document` | New document root with title / author / abstract |
| `docs_append_heading` | Heading 1-6 |
| `docs_append_paragraph` | Paragraph |
| `docs_append_list` | Ordered or unordered list with items in one call |
| `docs_append_code` | Fenced code block with language |
| `docs_append_quote` | Blockquote |
| `docs_append_media` | Image / video / audio / file by URL |
| `docs_append_divider` | Horizontal rule |

**Write — edits:**

| Tool | Purpose |
|---|---|
| `docs_update_text` | New version of an existing node's text |
| `docs_move` | Reparent and/or reorder a node |

All append tools accept an optional `parent_id`. If omitted, the new node is added under the document root. To build a section, append a heading and then append its content with that heading's `lineage_id` as the parent.

## Node kinds

All kinds are namespaced with `doc.`:

| Kind | Notes |
|---|---|
| `doc.document` | Root |
| `doc.heading` | metadata.level = 1..6 |
| `doc.paragraph` | |
| `doc.list` | metadata.ordered = bool; children are list_items |
| `doc.list_item` | Can have nested lists as children |
| `doc.code_block` | metadata.language |
| `doc.quote` | |
| `doc.media` | metadata.kind ∈ {image, video, audio, file}, metadata.src, metadata.alt |
| `doc.divider` | |

Edges: one kind, `doc.contains`, parent → child. Sibling order is the edge's `ordinal`.

Clients can introduce custom kinds; the renderer falls back to a best-effort plain-paragraph rendering.

## Library use

The same builder API is available as a Go package:

```go
import (
    docs "github.com/camggould/memgraph-docs"
    memgraph "github.com/camggould/memgraph"
    "github.com/camggould/memgraph/store/sqlite"
)

store, _ := sqlite.Open("store.db")
defer store.Close()

b := docs.NewBuilder(store)
doc, _ := b.CreateDocument(ctx, docs.CreateDocumentInput{
    GraphID: graphID,
    Title:   "My Doc",
})

heading, _ := b.AppendHeading(ctx, docs.AppendHeadingInput{
    DocumentID: doc.LineageID,
    Level:      1,
    Text:       "Hello",
})

b.AppendParagraph(ctx, docs.AppendParagraphInput{
    DocumentID: doc.LineageID,
    ParentID:   heading.LineageID,
    Text:       "Content under the heading.",
})

md, _ := b.RenderMarkdown(ctx, doc.LineageID)
```

## Development

```sh
git clone https://github.com/camggould/memgraph-docs.git
cd memgraph-docs
go test -race ./...
```

## Relationship to memgraph

memgraph-docs is the L2 reference client described in memgraph's [PRD §10](https://github.com/camggould/memgraph/blob/main/PRD.md). The substrate (graph, lineage, conflicts, MCP transport, storage backends) lives in memgraph; memgraph-docs is a thin layer that adds vocabulary, structural conventions, and a renderer.

If you want raw memgraph tools (search, traverse, create_graph, etc.), run [memgraph](https://github.com/camggould/memgraph) alongside this server.

## License

MIT. See [LICENSE](LICENSE).
