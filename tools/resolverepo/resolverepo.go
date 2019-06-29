// Program resolverepo attempts to resolve an import path using a vanity domain
// to the underlying repository address.
package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/creachadair/repodeps/tools"
)

var doReadStdin = flag.Bool("stdin", false, "Read import paths from stdin")

func main() {
	flag.Parse()

	repos := make(map[string]*bundle)

	var numPaths int64
	start := time.Now()
nextPath:
	for ip := range tools.Inputs(*doReadStdin) {
		numPaths++
		if numPaths%100 == 0 {
			log.Printf("[progress] %d repositories, %d import paths", len(repos), numPaths)
		}
		// Check whether we already have a prefix for this import path, and skip
		// a lookup in that case.
		for pfx, b := range repos {
			if strings.HasPrefix(ip, pfx) {
				b.ImportPaths = append(b.ImportPaths, ip)
				continue nextPath
			}
		}

		// Otherwise, we have to make a query.
		imps, err := resolveImportRepo(ip)
		if err != nil {
			log.Printf("Resolving %q: %v [skipped]", ip, err)
			continue
		}
		if len(imps) == 0 {
			log.Printf("Resolving %q: not found", ip)
			continue
		}
		repos[imps[0].Prefix] = &bundle{
			Repo:        imps[0].Repo,
			Prefix:      imps[0].Prefix,
			ImportPaths: []string{ip},
		}
	}
	log.Printf("[done] %d repositories, %d import paths [%v elapsed]",
		len(repos), numPaths, time.Since(start))

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
				Prefix: fields[0],
				Repo:   fields[2],
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
				Prefix: prefix,
				Repo:   "https://" + prefix + ".git",
			}}
		}

	case "gopkg.in":
		// The actual mapping to source code requires version information, but we
		// can construct the repository URL from the import alone.
		parts := strings.SplitN(ip, "/", 4)
		if len(parts) < 3 {
			break
		}
		ext := filepath.Ext(parts[2])
		repo := strings.TrimSuffix(parts[2], ext)
		url := strings.Join([]string{
			"https://github.com",
			parts[1], // username
			repo,
		}, "/")
		return []metaImport{{
			Prefix: strings.Join(parts[:3], "/"),
			Repo:   url,
		}}
	}
	return nil
}

type metaImport struct {
	Prefix, Repo string
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
