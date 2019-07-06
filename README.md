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


## Generating a Database

1. Generate an initial list of repositories. For our experiment, we did this
   using a GitHub search for repositories with a `go.mod` file at the root.
   You could also start with a listing of import paths from godoc.org:

   ```shell
   curl -L https://api.godoc.org/packages | jq -r .results[].path > paths.txt
   cat paths.txt | resolverepo -stdin | jq -r .repository > repos.txt
   ```

   Note if you do this, however, that the results will contain a lot of noise,
   as the `godoc.org` corpus includes vendored packages, internal packages,
   code that doesn't build, and so forth. For our experiment we had better
   results from the search query.


2. Fetch the repositories using [Borges](https://github.com/src-d/borges).

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
   that require authentication will be skipped.


3. Extract dependency information into a database:

   ```shell
   export REPODEPS_DB="$HOME/crawl/godeps-db"

   find ~/crawl/siva -type f -name '*.siva' -print \
   | repodeps -stdin -sourcehash -import-comments -store "$REPODEPS_DB"
   ```


## Finding Missing Dependencies

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
checkrepo -scan -update -store $REPODEPS_DB \
| tee capture.json \
| jq 'select(.needsUpdate or .errors > 1)'
```

The `jq` part of the pipeline is optional; it just serves as a less verbose
progress indicator than watching the entire log.

This may be rerun as often as desired; the `checkrepo` tool maintains a log of
which repository digests it has seen most recently, and will only update those
that change. Use the `-interval` flag to control how often this may occur.  It
doesn't make sense to choose an interval shorter than the time it takes to
fully run the update (which depends on the current database size).
