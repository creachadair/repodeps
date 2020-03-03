package graph

import (
	"context"
	"encoding/hex"
	"io"

	"bitbucket.org/creachadair/stringset"
	"github.com/cayleygraph/quad"
	"github.com/cayleygraph/quad/nquads"
	"github.com/cayleygraph/quad/voc/rdf"
)

const (
	typePackage = quad.IRI("dep:Package")
	typeRepo    = quad.IRI("dep:Repo")
	typeFile    = quad.IRI("dep:File")

	relType       = quad.IRI(rdf.Type)
	relDefinedIn  = quad.IRI("dep:defined-in")  // package DefinedIn repo
	relDigest     = quad.IRI("dep:digest")      // file Digest <string>
	relHasFile    = quad.IRI("dep:has-file")    // package HasFile file
	relImports    = quad.IRI("dep:imports")     // package Imports package
	relRanking    = quad.IRI("dep:ranking")     // package Ranking <float>
	relRepoPath   = quad.IRI("dep:repo-path")   // file RepoPath <string>
	relImportPath = quad.IRI("dep:import-path") // package ImportPath <string>
	relRepoURL    = quad.IRI("dep:repo-url")    // repo RepoURL <string>
	relMissing    = quad.IRI("dep:is-missing")  // package Missing <bool>
)

// WriteQuads converts g to RDF 1.1 N-quads and writes them to w.
func (g *Graph) WriteQuads(ctx context.Context, w io.Writer) error {
	qw := nquads.NewWriter(w)
	return g.EncodeToQuads(ctx, qw.WriteQuad)
}

// EncodeToQuads converts g to RDF 1.1 N-quads and calls f for each. If f
// reports an error the conversion is terminated and the error is returned to
// the caller of EncodeToQuads.
func (g *Graph) EncodeToQuads(ctx context.Context, f func(quad.Quad) error) (err error) {
	defer func() {
		if v := recover(); v != nil {
			if e, ok := v.(error); ok {
				err = e
			} else {
				panic(v) // not mine
			}
		}
	}()
	send := func(s, p, o quad.Value) {
		if err := f(quad.Quad{
			Subject:   s,
			Predicate: p,
			Object:    o,
		}); err != nil {
			panic(err)
		}
	}
	var seq quad.Sequence
	assign := func(m map[string]quad.BNode, key string) quad.BNode {
		b, ok := m[key]
		if !ok {
			b = seq.Next()
			m[key] = b
		}
		return b
	}
	pkgs := make(map[string]quad.BNode) // :: import path → BNode
	shas := make(map[string]quad.BNode) // :: digest → BNode
	defn := stringset.New()
	need := stringset.New()

	P := func(pkg string) quad.BNode { return assign(pkgs, pkg) }
	F := func(sha string) quad.BNode { return assign(shas, sha) }
	R := func(url string) quad.IRI { return quad.IRI(url) }

	if err := g.Scan(ctx, "", func(row *Row) error {
		pid := P(row.ImportPath)
		send(pid, relType, typePackage)
		send(pid, relRanking, quad.Float(row.Ranking))
		send(pid, relImportPath, quad.String(row.ImportPath))
		defn.Add(row.ImportPath)
		need.Discard(row.ImportPath)

		send(R(row.Repository), relType, typeRepo)
		send(P(row.ImportPath), relDefinedIn, R(row.Repository))

		for _, pkg := range row.Directs {
			send(P(row.ImportPath), relImports, P(pkg))
			if !defn.Contains(pkg) {
				need.Add(pkg)
			}
		}

		for _, src := range row.SourceFiles {
			hd := hex.EncodeToString(src.Digest)
			send(F(hd), relType, typeFile)
			send(F(hd), relDigest, quad.String(hd))
			send(F(hd), relRepoPath, quad.String(src.RepoPath))
			send(P(row.ImportPath), relHasFile, F(hd))
		}
		return nil
	}); err != nil {
		return err
	}

	// If any packages were depended upon but not mentioned in the graph, emit
	// dummy rows for them.
	for pkg := range need.Diff(defn) {
		if _, ok := pkgs[pkg]; !ok {
			send(P(pkg), relType, typePackage)
			send(P(pkg), relMissing, quad.Bool(true))
		}
	}
	return nil
}
