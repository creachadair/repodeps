// Program repodeps scans the contents of a collection of GitHub repositories
// and reports the names and dependencies of any Go packages defined inside.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/local"
	"github.com/creachadair/repodeps/siva"
)

var (
	doSourceHash = flag.Bool("sourcehash", false, "Record the names and digests of source files")
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatalf("Usage: %s <repo-dir> ...", filepath.Base(os.Args[0]))
	}
	ctx := context.Background()
	opts := &deps.Options{
		HashSourceFiles: *doSourceHash,
	}

	// Each argument is either a directory path or a .siva file path.
	// Currently only rooted siva files are supported.
	for _, dir := range flag.Args() {
		path, err := filepath.Abs(dir)
		if err != nil {
			log.Fatalf("Resolving path: %v", err)
		}
		log.Printf("Processing %q...", dir)

		var repos []*deps.Repo
		if filepath.Ext(path) == ".siva" {
			repos, err = siva.Load(ctx, path, opts)
		} else {
			repos, err = local.Load(ctx, path, opts)
		}
		if err != nil {
			log.Printf("Skipped %q:\n  %v", dir, err)
			continue
		}

		for _, repo := range repos {
			if err = json.NewEncoder(os.Stdout).Encode(repo); err != nil {
				log.Fatalf("Writing JSON: %v", err)
			}
		}
	}
}
