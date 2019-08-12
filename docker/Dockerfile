# The base build image with Go and git, so we can use go get.
FROM golang:1.12-alpine AS build
RUN apk add --no-cache git # so we can use "go get"
ENV GO111MODULE=on GOPROXY=https://proxy.golang.org
WORKDIR /build
COPY . .
RUN go build ./tools/depserver
RUN go build -o ./jcall github.com/creachadair/jrpc2/cmd/jcall

# The crawler.
FROM alpine:latest AS crawler
ENV SERVER= LOGDIR= SLEEPTIME=
VOLUME /logs
WORKDIR /crawl
COPY --from=build /build/jcall .
COPY ./docker/run-crawler.sh .
CMD ./run-crawler.sh

# The server.
FROM alpine:latest AS server
ENV GRAPH_DB= REPO_DB= ADDRESS= WORKDIR=
RUN apk add --no-cache git  # needed to do updates
VOLUME /data
WORKDIR /serve
COPY --from=build /build/depserver .
CMD ./depserver \
	-address "$ADDRESS" \
	-graph-db "$GRAPH_DB" \
	-repo-db "$REPO_DB" \
	-workdir "$WORKDIR" \
	-rank-iter 40 -rank-scale 6 -sample-rate 0.1

