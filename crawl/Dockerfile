FROM golang:1.12-alpine AS build

# Build the tools used by the crawler.
RUN apk add --no-cache git
ENV GO111MODULE=on
WORKDIR /badger
RUN go mod init badger && go get github.com/dgraph-io/badger@v2 2>/dev/null
RUN go build github.com/dgraph-io/badger/badger
WORKDIR /build
COPY . .
RUN go build ./tools/checkrepo
RUN go build ./tools/rankdeps

FROM alpine:latest AS crawler

# The crawler itself.
RUN apk add --no-cache git
VOLUME /data /logs
WORKDIR /crawl
COPY crawl/run-update.sh .
COPY --from=build /badger/badger .
COPY --from=build /build/checkrepo .
COPY --from=build /build/rankdeps .
CMD ./run-update.sh

