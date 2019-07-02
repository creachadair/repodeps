package graph

import (
	"context"
	"encoding/hex"
	"io"

	"bitbucket.org/creachadair/stringset"
	"github.com/cayleygraph/cayley/quad"
	"github.com/cayleygraph/cayley/quad/nquads"
	"github.com/cayleygraph/cayley/voc/rdf"
)

const (
	typePackage = quad.IRI("dep:Package")
	typeRepo    = quad.IRI("dep:Repo")
	typeFile    = quad.IRI("dep:File")

	relType      = quad.IRI(rdf.Type)
	relImports   = quad.IRI("dep:Imports")
	relDefinedIn = quad.IRI("dep:DefinedIn")
	relRepoPath  = quad.IRI("dep:RepoPath")
	relDigest    = quad.IRI("dep:Digest")
)

// WriteQuads converts g to RDF 1.1 N-quads and writes them to w.
func (g *Graph) WriteQuads(ctx context.Context, w io.Writer) error {
	qw := nquads.NewWriter(w)
	return g.EncodeToQuads(ctx, qw.WriteQuad)
}

// EncodeToQuads converts g to RDF 1.1 N-quads and calls f fo reach. If f
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
	P := func(pkg string) quad.IRI { return quad.IRI("pkg:" + pkg) }
	R := func(url string) quad.IRI { return quad.IRI("repo:" + url) }

	pkgs := stringset.New()
	return g.Scan(ctx, "", func(row *Row) error {
		send(R(row.Repository), relType, typeRepo)
		send(P(row.ImportPath), relDefinedIn, R(row.Repository))
		if !pkgs.Contains(row.ImportPath) {
			send(P(row.ImportPath), relType, typePackage)
			pkgs.Add(row.ImportPath)
		}
		for _, pkg := range row.Directs {
			if !pkgs.Contains(pkg) {
				send(P(pkg), relType, typePackage)
				pkgs.Add(pkg)
			}
			send(P(row.ImportPath), relImports, P(pkg))
		}

		for _, src := range row.SourceFiles {
			hd := hex.EncodeToString(src.Digest)
			fid := quad.IRI("sha256:" + hd)
			send(fid, relType, typeFile)
			send(fid, relDigest, quad.String(hd))
			send(fid, relRepoPath, quad.String(src.RepoPath))
		}
		return nil
	})
}
