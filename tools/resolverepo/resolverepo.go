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

// Program resolverepo attempts to resolve an import path using a vanity domain
// to the underlying repository address.
package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/creachadair/repodeps/tools"
	"github.com/creachadair/taskgroup"
)

var (
	doReadStdin = flag.Bool("stdin", false, "Read import paths from stdin")
	concurrency = flag.Int("concurrency", 16, "Number of concurrent workers")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options] <import-path>...

Resolve Go import paths to Git repository URLs for vanity domains.  The
resolution algorithm is borrowed from the "go get" command, which issues an
HTTP query to the hosting site to request import information.

For each resolved repository, the tool prints a JSON text to stdout having the
fields:

   "repo":        repository fetch URL (string)
   "prefix":      import path prefix covered by this repository (string)
   "importPaths": import paths (array of strings)

The non-flag arguments name the import paths to resolve. With -stdin, each line
of stdin will be read as an additional import path to resolve.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	// Input paths described on the command line and/or read from stdin.
	inputs := tools.Inputs(*doReadStdin)

	// Accumulated repository mappings, becomes output.
	var rmu sync.RWMutex // protects repos
	repos := make(map[string]*bundle)

	// Receive results from concurrent resolver tasks.
	results := make(chan metaImport, *concurrency)
	find := func(ip string) bool {
		rmu.RLock()
		defer rmu.RUnlock()
		for pfx, b := range repos {
			if strings.HasPrefix(ip, pfx) {
				b.ImportPaths = append(b.ImportPaths, ip)
				return true
			}
		}
		return false
	}

	start := time.Now()

	// Collect lookup results and update the repository map.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		for imp := range results {
			rmu.Lock()
			b := repos[imp.Prefix]
			if b == nil {
				b = &bundle{
					Repo:        imp.Repo,
					Prefix:      imp.Prefix,
					ImportPaths: []string{imp.ImportPath},
				}
				repos[imp.Prefix] = b
			} else {
				b.ImportPaths = append(b.ImportPaths, imp.ImportPath)
			}
			rmu.Unlock()
		}
	}()

	// Process inputs and send results to the collector.
	g := taskgroup.New(nil)
	for i := 0; i < *concurrency; i++ {
		g.Go(func() error {
			for ip := range inputs {
				if find(ip) {
					continue // already handled
				}
				imps, err := resolveImportRepo(ip)
				if err != nil {
					log.Printf("[skipped] resolving %q: %v", ip, err)
				} else if len(imps) == 0 {
					log.Printf("[skipped] resolving %q: not found", ip)
				} else {
					results <- imps[0]
				}
			}
			return nil
		})
	}

	// Wait for all the workers to complete.
	if err := g.Wait(); err != nil {
		log.Fatalf("Processing failed: %v", err)
	}
	close(results)
	<-ctx.Done() // wait for the collector to complete

	log.Printf("[done] %d repositories found [%v elapsed]", len(repos), time.Since(start))

	// Encode the output.
	enc := json.NewEncoder(os.Stdout)
	for _, b := range repos {
		sort.Strings(b.ImportPaths)
		if err := enc.Encode(b); err != nil {
			log.Fatalf("Encoding failed: %v", err)
		}
	}
}

type bundle struct {
	Repo        string   `json:"repo"`
	Prefix      string   `json:"prefix"`
	ImportPaths []string `json:"importPaths,omitempty"`
}

// resolveImportRepo attempts to resolve the URL of the specified import path
// using the HTTP metadata protocol used by "go get". Unlike "go get", this
// resolver only considers Git targets.
func resolveImportRepo(ipath string) ([]metaImport, error) {
	// Shortcut well-known Git hosting providers, to save network traffic.
	if wk := checkWellKnown(ipath); wk != nil {
		return wk, nil
	}

	// Request package resolution. If the site supports it, we will receive a
	// <meta name="go-import" content="<prefix> <vcs> <url>"> tag.
	url := "https://" + ipath + "?go-get=1"
	rsp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK && rsp.StatusCode != http.StatusNotFound {
		return nil, errors.New(rsp.Status)
	}

	var imp []metaImport

	// Logic cribbed with modificdations from parseMetaGoImports in
	// src/cmd/go/internal/get/discovery.go
	dec := xml.NewDecoder(rsp.Body)
	dec.Strict = false
	var t xml.Token
	for {
		t, err = dec.RawToken()
		if err != nil {
			if err == io.EOF || len(imp) > 0 {
				err = nil
			}
			break
		}
		st, ok := t.(xml.StartElement)
		if ok && strings.EqualFold(st.Name.Local, "body") {
			break // stop scanning when the body begins
		} else if e, ok := t.(xml.EndElement); ok && strings.EqualFold(e.Name.Local, "head") {
			break // stop scanning when the head ends
		}

		if !ok || !strings.EqualFold(st.Name.Local, "meta") {
			continue // skip non-meta tags
		} else if attrValue(st.Attr, "name") != "go-import" {
			continue // skip unrelated meta tags
		}

		fields := strings.Fields(attrValue(st.Attr, "content"))
		if len(fields) == 3 && fields[1] == "git" {
			imp = append(imp, metaImport{
				Prefix:     fields[0],
				Repo:       fields[2],
				ImportPath: ipath,
			})
		}
	}
	if rsp.StatusCode != http.StatusOK && len(imp) == 0 {
		return nil, errors.New(rsp.Status)
	}
	return imp, nil
}

// checkWellKnown checks whether ip is lexically associated with a well-known
// git host. If so, it synthesizes an import location; otherwise returns nil.
func checkWellKnown(ip string) []metaImport {
	pfx, _ := tools.HasDomain(ip)
	switch pfx {
	case "github.com", "bitbucket.org":
		parts := strings.Split(ip, "/")
		if len(parts) >= 3 {
			prefix := strings.Join(parts[:3], "/")
			return []metaImport{{
				Prefix:     prefix,
				Repo:       "https://" + prefix,
				ImportPath: ip,
			}}
		}

	case "gopkg.in":
		// The actual mapping to source code requires version information, but we
		// can construct the repository URL from the import alone.
		parts := strings.SplitN(ip, "/", 4)
		if len(parts) < 2 {
			return nil
		}

		var user, repo, prefix string
		if len(parts) == 2 {
			// Rule 1: gopkg.in/pkg.vN      ⇒ github.com/go-pkg/pkg
			repo = trimExt(parts[1])
			user = "go-" + repo
			prefix = strings.Join(parts[:2], "/")
		} else {
			// Rule 2: gopkg.in/user/pkg.vN ⇒ github.com/user/pkg
			repo = trimExt(parts[2])
			user = parts[1]
			prefix = strings.Join(parts[:3], "/")
		}
		url := strings.Join([]string{
			"https://github.com", user, repo,
		}, "/")
		return []metaImport{{
			Prefix:     prefix,
			Repo:       url,
			ImportPath: ip,
		}}
	}
	return nil
}

type metaImport struct {
	Prefix, Repo string
	ImportPath   string
}

// attrValue returns the value for the named attribute, or "" if the name is
// not found.
func attrValue(attrs []xml.Attr, name string) string {
	for _, attr := range attrs {
		if strings.EqualFold(attr.Name.Local, name) {
			return attr.Value
		}
	}
	return ""
}

// trimExt returns a copy of s with a trailing extension removed.
func trimExt(s string) string { return strings.TrimSuffix(s, filepath.Ext(s)) }
