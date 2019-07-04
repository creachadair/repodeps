# repodeps

This repository contains the code from an experiment to analyze, store, and
update package-level dependencies from Git repositories. Currently it works
only for Go, but we plan to try adding support for other languages.

Be warned that this code is not production ready and may change without notice.

## Generating a Database

1. Generate an initial list of repositories. For our experiment, we did this
   using a GitHub search for repositories with a `go.mod` file at the root.
   You could also start with a listing of import paths from godoc.org:

   ```shell
   # See https://github.com/stedolan/jq
   curl -L https://api.godoc.org/packages | jq -r .results[].path > paths.txt
   cat paths.txt | go run ./tools/resolverepo -stdin | jq -r .repo > repos.txt
   ```

   Note if you do this, however, that the results will contain a lot of noise,
   as the godoc.org corpus includes vendored packages, internal packages, code
   that doesn't build, and so forth.


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


3. Extract dependency information:

   ```shell
   find ~/crawl/siva -type f -name '*.siva' -print \
   | go run repodeps.go -stdin -sourcehash -import-comments \
   | xz -cz > deps.json.xz
   ```


4. Load the results into a database:

   ```shell
   export REPODEPS_DB="$HOME/crawl/godeps-db"
   xz -cd deps.json.xz | go run ./tools/writedeps -store "$REPODEPS_DB"
   ```

## Finding Missing Dependencies

1. Find import paths mentioned as dependencies, but not having their own nodes
   in the graph. Resolve each of these to a repository URL:

   ```shell
   go run ./tools/missingdeps -domain-only \
   | go run ./tools/resolverepo -stdin \
   | jq -r .repository > missing.txt
   ```

2. Attempt to fetch and update the contents of the missing repositories into
   the database. This may not succeed for repositories that require
   authentication, or if there are other issues cloning the repository.

   ```shell
   export REPODEPS_POLLDB="$HOME/crawl/poll-db"
   go run ./tools/checkrepo -update -store "$REPODEPS_DB" -stdin < missing.txt \
   | tee capture.json | jq 'select(.needsUpdate or .errors > 1)'
   ```

This process may be iterated to convergence.

## Updating the Graph

To scan all the repositories currently mentioned by the graph to check for
updates:

```shell
go run ./tools/checkrepo -scan -update -store $REPODEPS_DB \
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
