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

// Package tools implements shared code for command-line tools.
package tools

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/creachadair/badgerstore"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/service"
	"github.com/creachadair/repodeps/storage"
)

// OpenMode controls how OpenGraph accesses storage.
type OpenMode int

// Mode constants for OpenGraph.
const (
	ReadOnly OpenMode = iota
	ReadWrite
)

// OpenService opens a read-only service associated with the graph and
// repository databases named.
func OpenService(graph, repos string) (*service.Server, error) {
	return service.New(service.Options{
		RepoDB:   repos,
		GraphDB:  graph,
		ReadOnly: true,
	})
}

// OpenGraph opens the graph named by path.  The caller must ensure the closer
// is closed when the graph is no longer in use.
func OpenGraph(path string, mode OpenMode) (*graph.Graph, io.Closer, error) {
	s, err := openBadger(path, mode)
	if err != nil {
		return nil, nil, fmt.Errorf("opening storage: %v", err)
	}
	return graph.New(storage.NewBlob(s)), s, nil
}

func openBadger(path string, mode OpenMode) (*badgerstore.Store, error) {
	if path == "" {
		return nil, errors.New("no path was provided")
	}
	if mode == ReadWrite {
		return badgerstore.NewPath(path)
	}
	return badgerstore.NewPathReadOnly(path)
}

// Inputs returns a channel that delivers the paths of inputs and is closed
// when no more are available. The non-flag arguments are read, and if
// readStdin is true each line of stdin is also read. The caller must fully
// drain the channel.
func Inputs(readStdin bool) <-chan string {
	ch := make(chan string, len(flag.Args()))
	for _, arg := range flag.Args() {
		ch <- arg
	}
	if readStdin {
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
