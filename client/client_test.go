package client_test

import (
	"context"
	"flag"
	"testing"

	"github.com/creachadair/repodeps/client"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/service"
)

var address = flag.String("address", "", "Service address for manual test")

func TestClient(t *testing.T) {
	if *address == "" {
		t.Skip("No -address provided for manual testing")
	}
	ctx := context.Background()
	cli, err := client.Dial(ctx, *address)
	if err != nil {
		t.Fatalf("Dial %q: %v", *address, err)
	}
	defer cli.Close()

	nr, err := cli.Match(ctx, &service.MatchReq{
		Package: "github.com/creachadair/repodeps/...",
	}, func(row *graph.Row) error {
		t.Logf("Package: %s [%d dependencies]", row.ImportPath, len(row.Directs))
		return nil
	})
	if err != nil {
		t.Errorf("Match failed: %v", err)
	}
	t.Logf("Total %d matching rows", nr)
}
