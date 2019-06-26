// Copyright 2019 Michael J. Fromberger. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Program repodeps scans the contents of a collection of GitHub repositories
// and reports the names and dependencies of any Go packages defined inside.
package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/creachadair/atomicfile"
	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/local"
	"github.com/creachadair/repodeps/siva"
	"github.com/creachadair/taskgroup"
	"github.com/golang/protobuf/proto"
)

var (
	doReadInputs  = flag.Bool("stdin", false, "Read input filenames from stdin")
	doSourceHash  = flag.Bool("sourcehash", false, "Record the names and digests of source files")
	doSeparateOut = flag.Bool("separate", false, "Write output to a file per input")
	doBinary      = flag.Bool("binary", false, "Write output as binary rather than JSON")
	concurrency   = flag.Int("concurrency", 32, "Maximum concurrent workers")

	out = &struct {
		sync.Mutex
		io.Writer
	}{Writer: os.Stdout}
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 && !*doReadInputs {
		log.Fatalf("Usage: %s <repo-dir> ...", filepath.Base(os.Args[0]))
	}
	ctx, cancel := context.WithCancel(context.Background())
	opts := &deps.Options{
		HashSourceFiles: *doSourceHash,
	}
	defer cancel()

	g, run := taskgroup.New(taskgroup.Trigger(cancel)).Limit(*concurrency)

	// Each argument is either a directory path or a .siva file path.
	// Currently only rooted siva files are supported.
	var numRepos int
	start := time.Now()
	for dir := range inputs() {
		dir := dir
		path, err := filepath.Abs(dir)
		if err != nil {
			log.Fatalf("Resolving path: %v", err)
		}
		numRepos++
		run(func() error {
			log.Printf("Processing %q...", dir)

			var repos []*deps.Repo
			if filepath.Ext(path) == ".siva" {
				repos, err = siva.Load(ctx, path, opts)
			} else {
				repos, err = local.Load(ctx, path, opts)
			}
			if err != nil {
				log.Printf("Skipped %q:\n  %v", dir, err)
				return nil
			}

			return writeRepos(ctx, path, repos)
		})
	}
	if err := g.Wait(); err != nil {
		log.Fatalf("Analysis failed: %v", err)
	}
	log.Printf("Analysis complete for %d inputs [%v elapsed]", numRepos, time.Since(start))
}

func writeRepos(ctx context.Context, path string, repos []*deps.Repo) error {
	bits, err := encodeRepos(repos)
	if err != nil {
		return err
	}
	if *doSeparateOut {
		return atomicfile.WriteData(path+".deps", bits, 0644)
	}

	if *doBinary {
		var buf [10]byte
		n := binary.PutUvarint(buf[:], uint64(len(bits)))
		bits = append(buf[:n], bits...)
	} else {
		bits = append(bits, '\n')
	}
	out.Lock()
	defer out.Unlock()
	_, err = out.Write(bits)
	return err
}

func encodeRepos(repos []*deps.Repo) ([]byte, error) {
	if *doBinary {
		return proto.Marshal(&deps.Deps{Repositories: repos})
	}
	return json.Marshal(repos)
}

// inputs returns a channel that delivers the paths of inputs and is closed
// when no more are available.
func inputs() <-chan string {
	ch := make(chan string, len(flag.Args()))
	for _, arg := range flag.Args() {
		ch <- arg
	}
	if *doReadInputs {
		s := bufio.NewScanner(os.Stdin)
		go func() {
			defer close(ch)
			for s.Scan() {
				ch <- s.Text()
			}
		}()
	} else {
		close(ch)
	}
	return ch
}
