package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/jctx"
	"github.com/creachadair/repodeps/updater"
)

var (
	opts updater.Options
)

func init() {
	flag.StringVar(&opts.RepoDB, "repo-db", "", "Repository database (required)")
	flag.StringVar(&opts.GraphDB, "graph-db", "", "Graph database (required)")
	flag.StringVar(&opts.WorkDir, "workdir", "", "Working directory for updates")
	flag.DurationVar(&opts.MinPollInterval, "interval", 1*time.Hour, "Minimum scan interval")
	flag.IntVar(&opts.ErrorLimit, "error-limit", 10, "Maximum repository update failures")
	flag.Float64Var(&opts.SampleRate, "sample-rate", 1, "Sample fraction of eligible updates (0..1)")
	flag.IntVar(&opts.Concurrency, "concurrency", 16, "Maximum concurrent updates")

	flag.BoolVar(&opts.Options.HashSourceFiles, "hash-source-files", true,
		"Record source file digests")
	flag.BoolVar(&opts.Options.UseImportComments, "use-import-comments", true,
		"Parse import comments to name packages")
}

func main() {
	flag.Parse()

	u, err := updater.New(opts)
	if err != nil {
		log.Fatalf("Creating updater: %v", err)
	}
	srv := jrpc2.NewServer(handler.Map{
		"Update": handler.New(u.Update),
		"Scan":   handler.New(u.Scan),
	}, &jrpc2.ServerOptions{
		AllowPush:     true,
		DecodeContext: jctx.Decode,
	})
	srv.Start(channel.Line(os.Stdin, os.Stdout))
	if err := srv.Wait(); err != nil {
		log.Printf("Server failed: %v", err)
	}
	if err := u.Close(); err != nil {
		log.Fatalf("Close updater: %v", err)
	}
}
