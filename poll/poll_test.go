package poll_test

import (
	"context"
	"testing"

	"github.com/creachadair/ffs/blob/memstore"
	"github.com/creachadair/repodeps/poll"
	"github.com/creachadair/repodeps/storage"
	"github.com/golang/protobuf/proto"
)

func TestCheck(t *testing.T) {
	st := storage.NewBlob(memstore.New())
	db := poll.New(st)

	const url = "." // this repository
	ctx := context.Background()

	// The first time we check the database is empty, so this should report that
	// the repository needs an update.
	res, err := db.Check(ctx, url)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	t.Logf("Found ref %q at digest %q, needs update: %v", res.Name, res.Digest, res.NeedsUpdate())
	if !res.NeedsUpdate() {
		t.Error("NeedsUpdate: got false, want true")
	}

	// Now that we have checked, checking the same URL again without a change in
	// between should report the repository is up-to-date.
	//
	// This could in principle fail if the repository is updated while the test
	// is running. Don't do that.
	cmp, err := db.Check(ctx, url)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	t.Logf("Found ref %q at digest %q, needs update: %v", cmp.Name, cmp.Digest, cmp.NeedsUpdate())
	if cmp.NeedsUpdate() {
		t.Errorf("NeedsUpdate: got true, want false")
	}

	// Log the recorded state for debugging purposes.
	var stat poll.Status
	if err := st.Load(ctx, url, &stat); err != nil {
		t.Fatalf("Loading status: %v", err)
	}
	t.Logf("Status message:\n%s", proto.MarshalTextString(&stat))
}
