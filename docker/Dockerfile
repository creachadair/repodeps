# The base build image with Go and git, so we can use go get.
FROM golang:1.14-alpine AS build
WORKDIR /build
COPY . .
RUN go build -ldflags "-w -s" \
	-o ./jcall github.com/creachadair/jrpc2/cmd/jcall
RUN go build -ldflags "-w -s" \
	-o ./depserver ./tools/depserver

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
VOLUME /data/graph-db /data/repo-db /data/tmp
WORKDIR /serve
COPY --from=build /build/depserver .
CMD ./depserver \
	-address "$ADDRESS" \
	-graph-db "$GRAPH_DB" \
	-repo-db "$REPO_DB" \
	-workdir "$WORKDIR" \
	-rank-iter 40 -rank-scale 6 -sample-rate 0.1

