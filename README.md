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
export REPODEPS_ADDR=$TMPDIR/depserver.sock
depserver -address $REPODEPS_ADDR -graph-db path/to/graphdb -repo-db path/to/repodb
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

2. [optional] Fetch the repositories. For our original experiment, we did this
   using the [Borges](https://github.com/src-d/borges) tool:

   ```shell
   export GITHUB_TOKEN=<token-string>  # recommended, because rate limits

   mkdir ~/crawl

   borges pack \
      --workers=0 \
      --log-level=info \
      --root-repositories-dir=$HOME/crawl/siva \
      --temp-dir=$HOME/crawl/tmp \
      repos.txt
   ```

   Depending how big your seed list is, this may take a while. Repositories
   that require authentication will be skipped. If you are starting from a
   smallish set of repositories, you probably don't need to do this step.


3. Extract dependency information into a database:

   ```shell
   find ~/crawl/siva -type f -name '*.siva' -print \
   | repodeps -stdin -sourcehash -import-comments -store "$REPODEPS_ADDR"
   ```


## Finding Missing Dependencies

N.B. This section is partly wrong and needs to be updated.

1. Find import paths mentioned as dependencies, but not having their own nodes
   in the graph. Resolve each of these to a repository URL:

   ```shell
   missingdeps -domain-only | resolverepo -stdin \
   | jq -r .repository > missing.txt
   ```

2. Attempt to fetch and update the contents of the missing repositories into
   the database. This may not succeed for repositories that require
   authentication, or if there are other issues cloning the repository.

   ```shell
   export REPODEPS_POLLDB="$HOME/crawl/poll-db"
   checkrepo -update -store "$REPODEPS_DB" -stdin < missing.txt \
   | tee capture.json | jq 'select(.needsUpdate or .errors > 1)'
   ```

This process may be iterated to convergence.


## Updating the Graph

To scan all the repositories currently mentioned by the graph to check for
updates:

```shell
# One time
(cd /tmp ; go get github.com/creachadair/jrpc2/cmd/jcall)

# Periodically
jcall -c "$REPODEPS_ADDR" Scan '{"logUpdates":true, "logErrors":true}'
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
repodeps -store "$REPODEPS_ADDR" -stdlib -trim-repo -import-comments=0 -sourcehash=0 ./go/src
```

Generally these only need to be reindexed when there is a new release.


## Computing PageRank

To compute or re-compute ranking stats,

```shell
jcall -c "$REPODEPS_ADDR" Rank '{"logProgress": true, "update": true}'
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
