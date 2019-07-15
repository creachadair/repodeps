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

package service

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/poll"
)

// Resolve attempts to resolve the URL of the repository containing the
// specified import path, using the HTTP metadata protocol used by "go get".
// Unlike "go get", this resolver only considers Git targets.
func (*Server) Resolve(ctx context.Context, req *ResolveReq) (*ResolveRsp, error) {
	return ResolveRepository(ctx, req)
}

// ResolveRepository attempts to resolve the URL of the repository containing
// the specified import path, using the HTTP metadata protocol used by "go
// get".  Unlike "go get", this resolver only considers Git targets.
func ResolveRepository(ctx context.Context, req *ResolveReq) (*ResolveRsp, error) {
	if req.Package == "" {
		return nil, jrpc2.Errorf(code.InvalidParams, "missing package import path")
	} else if deps.IsLocalPackage(req.Package) {
		return nil, fmt.Errorf("package %q is local or intrinsic", req.Package)
	}
	// Shortcut well-known Git hosting providers, to save network traffic.
	wk, err := checkWellKnown(ctx, req.Package)
	if err != nil {
		return nil, err
	} else if wk != nil {
		return wk, nil
	}

	// Request package resolution. If the site supports it, we will receive a
	// <meta name="go-import" content="<prefix> <vcs> <url>"> tag.
	base := "https://" + req.Package
	url := base + "?go-get=1"
	fail := func(err error) (*ResolveRsp, error) {
		return &ResolveRsp{Repository: base, Prefix: req.Package}, err
	}

	rsp, err := http.Get(url)
	if err != nil {
		return fail(err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK && rsp.StatusCode != http.StatusNotFound {
		return fail(errors.New(rsp.Status))
	}

	// Logic cribbed with modificdations from parseMetaGoImports in
	// src/cmd/go/internal/get/discovery.go
	var imp *ResolveRsp
	dec := xml.NewDecoder(rsp.Body)
	dec.Strict = false
	var t xml.Token
	for {
		t, err = dec.RawToken()
		if err != nil {
			if err == io.EOF || imp != nil {
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
			imp = &ResolveRsp{
				Repository: fields[2],
				Prefix:     fields[0],
				ImportPath: req.Package,
			}
			break
		}
	}
	if rsp.StatusCode != http.StatusOK && imp == nil {
		return fail(errors.New(rsp.Status))
	} else if imp == nil {
		return fail(errors.New("no targets"))
	}
	return imp, nil
}

// ResolveReq is the request parameter to the Resolve method.
type ResolveReq struct {
	Package string `json:"package"`
}

// ResolveRsp is the response value from a successful Resolve call.
type ResolveRsp struct {
	Repository string `json:"repository"`
	Prefix     string `json:"prefix"`
	ImportPath string `json:"importPath"`
}

// checkWellKnown checks whether ip is lexically associated with a well-known
// git host. If so, it synthesizes an import location; otherwise returns nil.
func checkWellKnown(ctx context.Context, ip string) (wk *ResolveRsp, err error) {
	defer func() {
		if err == nil && wk != nil && !poll.RepoExists(ctx, wk.Repository) {
			err = fmt.Errorf("repository %q does not exist", wk.Repository)
		}
	}()
	pfx, _ := deps.HasDomain(ip)
	switch pfx {
	case "github.com", "bitbucket.org":
		// TODO: Because we didn't actually try to resolve these, we don't know
		// whether the predicted repo actually exists.
		parts := strings.Split(ip, "/")
		if len(parts) >= 3 {
			prefix := strings.Join(parts[:3], "/")
			return &ResolveRsp{
				Repository: "https://" + prefix,
				Prefix:     prefix,
				ImportPath: ip,
			}, nil
		}

	case "gopkg.in":
		// The actual mapping to source code requires version information, but we
		// can construct the repository URL from the import alone.
		parts := strings.SplitN(ip, "/", 4)
		if len(parts) < 2 {
			return nil, errors.New("invalid gopkg.in import path")
		}

		var user, repo, prefix string
		if len(parts) == 2 {
			// Rule 1: gopkg.in/pkg.vN ⇒ github.com/go-pkg/pkg
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
		return &ResolveRsp{
			Repository: url,
			Prefix:     prefix,
			ImportPath: ip,
		}, nil
	}
	return nil, nil // not well-known
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
