# The base build image. Because the server depends on modules that require cgo,
# a stock Go image is not sufficient.
FROM golang:1.14-alpine AS build
RUN apk add --no-cache build-base # for cgo builds (badger)
WORKDIR /build
COPY . .
RUN go build -ldflags "-w -s" -o ./crawldeps ./tools/crawldeps
RUN go build -ldflags "-w -s" -o ./depserver ./tools/depserver

# The crawler.
FROM alpine:latest AS crawler
ENV SERVER= INTERVAL= SAMPLE_RATE=
COPY --from=build /build/crawldeps .
CMD ./crawldeps \
	-address "$SERVER" \
	-interval "$INTERVAL" \
	-sample-rate "$SAMPLE_RATE"

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

