// Program resolverepo attempts to resolve an import path using a vanity domain
// to the underlying repository address.
package main

import (
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/creachadair/repodeps/tools"
)

var doReadStdin = flag.Bool("stdin", false, "Read import paths from stdin")

func main() {
	flag.Parse()

	pfx := make(map[string]string) // :: prefix → url

nextPath:
	for ip := range tools.Inputs(*doReadStdin) {
		// Check whether we already have a prefix for this import path, and skip
		// a lookup in that case.
		for p, url := range pfx {
			if strings.HasPrefix(ip, p) {
				fmt.Printf("%s\t%s\t%s\n", ip, p, url)
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
		pfx[imps[0].Prefix] = imps[0].Repo // cache this prefix
		fmt.Printf("%s\t%s\t%s\n", ip, imps[0].Prefix, imps[0].Repo)
	}
}

// resolveImportRepo attempts to resolve the URL of the specified import path
// using the HTTP metadata protocol used by "go get". Unlike "go get", this
// resolver only considers Git targets.
func resolveImportRepo(ipath string) ([]metaImport, error) {
	url := "https://" + ipath + "?go-get=1"
	rsp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
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
	return imp, nil
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
