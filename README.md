# repodeps

This repository contains the code from an experiment to analyze, store, and
update package-level dependencies from Git repositories. Currently it works
only for Go, but we plan to try adding support for other languages.

Be warned that this code is not production ready and may change without notice.


## Installation

```shell
git clone github.com/creachadair/repodeps
cd repodeps
./install.sh  # copies binaries to $GOBIN or $GOPATH/bin
go install github.com/creachadair/jrpc2/cmd/jcall
```

The rest of these instructions assume the installed binaries are somewhere in
your `$PATH`. You will also need [`jq`](https://stedolan.github.io/jq/).  On
macOS you can get `jq` via `brew install jq`.


## Set Up a Database

The `depserver` program maintains a database of dependency graph information,
and a database of repository state. Most of the other programs work by sending
JSON-RPC requests to `depserver`. To set up a server, choose an address (either
a host:port pair or a Unix-domain socket), and run:

```
export DEPSERVER_ADDR=$TMPDIR/depserver.sock
depserver -address $DEPSERVER_ADDR -graph-db path/to/graphdb -repo-db path/to/repodb
```

This will create empty databases if they do not already exist, or re-open
existing ones.

## Initializing the Database

1. Generate an initial list of repositories. For our experiment, we did this
   using a GitHub search for repositories with a `go.mod` file at the root.
   You could also start with a listing of import paths from godoc.org:

   ```shell
   curl -L https://api.godoc.org/packages | jq -r .results[].path > paths.txt
   resolvedeps -filter-known -stdin < paths.txt > repos.txt
   ```

   Note that doing this for the complete list of `godoc.org` packages will take
   a long time, and the results will contain a lot of noise, as that corpus
   includes vendored packages, internal packages, code that doesn't build, and
   so forth. For our experiment we had better results from the search query.
   For local experimentation, you can use your own repositories:

   ```shell
   # If you get rate limited, set GITHUB_TOKEN.
   hub api --paginate users/creachadair/repos \
   | jq -r '.[]|select(.fork|not).html_url' > repos.txt
   ```

2. Extract dependency information into a database.  Given a list of repository
   URLs as in step (1), use:

   ```shell
   xargs -I@ jcall -c "$DEPSERVER_ADDR" Update '{"repository":"@"}' < repos.txt
   ```

   This will be slow if the list is very long, but fine for a few dozen.


## Finding Missing Dependencies

1. Find import paths mentioned as dependencies, but not having their own nodes
   in the graph. Resolve each of these to a repository URL:

   ```shell
   missingdeps -domain-only | resolvedeps -stdin | sort -u > missing.txt
   ```

2. Attempt to fetch and update the contents of the missing repositories into
   the database. This may not succeed for repositories that require
   authentication, or if there are other issues cloning the repository.

   ```shell
   xargs -I@ jcall -c "$DEPSERVER_ADDR" Update '{"repository":"@"}' < missing.txt
   ```

   **Be warned**, however, that programs in the wild often have very weird
   dependencies.  For example, there are packages that depend on non-standard
   forks of the standard library, and you may wind up pulling those forks into
   your database.

This process may be iterated to convergence, in theory, but in practice it will
never fully converge because there are a lot of custom build hacks, dead code,
deleted repositories, and so forth. Call it, rather, iterated improvement.


## Updating the Graph

To scan all the repositories currently mentioned by the graph to check for
updates:

```shell
jcall -c "$DEPSERVER_ADDR" Scan '{"logUpdates":true, "logErrors":true}'
```

This may be rerun as often as desired; the repository database keeps track of
which repository digests it has seen most recently, and will only update those
that change. Use the `-interval` flag on the `depserver` tool to control how
often this may occur.  It doesn't make sense to choose an interval shorter than
the time it takes to fully run the update (which depends on the current
database size).


## Indexing the Standard Library

The standard library packages follow different rules. To index them:

```shell
git clone https://github.com/golang/go
repodeps -store "$DEPSERVER_ADDR" -stdlib -trim-repo -import-comments=0 -sourcehash=0 ./go/src
```

Generally these only need to be reindexed when there is a new release.


## Computing PageRank

To compute or re-compute ranking stats,

```shell
jcall -c "$DEPSERVER_ADDR" Rank '{"logProgress": true, "update": true}'
```

## Converting to Other Formats

These tools work directly on the database, so you have to stop `depserver` if
you want to use them. TODO(creachadair): Fix that.

- To convert to CSV for [Gephi](https://gephi.org):

    ```shell
	# With everything inline.
	csvdeps -store path/to/graphdb > output.csv

	# With a separate vocabulary file.
	csvdeps -store path/to/graphdb -ids vocab.txt > output.csv
	```

- To convert to RDF N-quads for a graph database like [Cayley](https://cayley.io/):

	```shell
	# To write RDF quads to stdout.
	quaddeps -store path/to/graphdb

	# To write to a Bolt database for Cayley.
	# N.B. If your database is very large, Bolt may choke.
	quaddeps -store path/to/graphdb -output cayley.bolt
	```
