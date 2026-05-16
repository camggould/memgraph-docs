// Command memgraph-docs is the CLI entrypoint for the memgraph-docs MCP
// server. It opens a memgraph SQLite store and exposes docs_* tools over
// stdio.
//
//	memgraph-docs serve --sqlite path/to/store.db
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	memgraph "github.com/camggould/memgraph"
	"github.com/camggould/memgraph/store/sqlite"

	"github.com/camggould/memgraph-docs/mcp"
)

var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "memgraph-docs",
		Short:         "MCP server that turns a memgraph graph into renderable documents",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.AddCommand(newServeCmd())
	return root
}

func newServeCmd() *cobra.Command {
	var sqlitePath string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the memgraph-docs MCP server over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openSqlite(sqlitePath)
			if err != nil {
				return err
			}
			defer store.Close()

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			srv := mcp.New(store, mcp.WithVersion(version))
			return srv.Serve(ctx)
		},
	}
	cmd.Flags().StringVar(&sqlitePath, "sqlite", "memgraph.db", "Path to memgraph SQLite store")
	return cmd
}

func openSqlite(path string) (memgraph.Store, error) {
	store, err := sqlite.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite store at %s: %w", path, err)
	}
	return store, nil
}
