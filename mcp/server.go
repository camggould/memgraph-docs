// Package mcp exposes a memgraph-docs Builder as a Model Context Protocol
// server. The tool surface mirrors DESIGN.md.
package mcp

import (
	"context"
	"errors"

	memgraph "github.com/camggould/memgraph"
	docs "github.com/camggould/memgraph-docs"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps a docs.Builder and a memgraph.Store for serving docs_* tools.
// The Builder is constructed lazily from the Store.
type Server struct {
	store   memgraph.Store
	builder *docs.Builder
	name    string
	ver     string
}

type Option func(*Server)

func WithName(name string) Option       { return func(s *Server) { s.name = name } }
func WithVersion(version string) Option { return func(s *Server) { s.ver = version } }

func New(store memgraph.Store, opts ...Option) *Server {
	s := &Server{
		store:   store,
		builder: docs.NewBuilder(store),
		name:    "memgraph-docs",
		ver:     "0.1.0",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Serve runs the MCP server on stdio. Blocks until ctx is cancelled or the
// transport closes.
func (s *Server) Serve(ctx context.Context) error {
	srv := s.build()
	err := srv.Run(ctx, &sdkmcp.StdioTransport{})
	if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return nil
	}
	return err
}

// build constructs the SDK server with all tools registered. Split out so
// tests can connect via an in-memory transport.
func (s *Server) build() *sdkmcp.Server {
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: s.name, Version: s.ver}, nil)
	s.registerTools(srv)
	return srv
}
