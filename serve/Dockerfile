FROM golang:1.12-alpine AS build

# To build:
#  docker build -t creachadair/repo-depserver -f serve/Dockerfile .

# Build the server.
RUN apk add --no-cache git
ENV GO111MODULE=on
WORKDIR /build
COPY . .
RUN go build ./tools/depserver

FROM alpine:latest AS server

# Run the server.
ENV GRAPH_DB= REPO_DB= ADDRESS=
RUN apk add --no-cache git  # needed for updates
VOLUME /data
WORKDIR /serve
COPY --from=build /build/depserver .
CMD ./depserver \
	-address "$ADDRESS" -graph-db "$GRAPH_DB" -repo-db "$REPO_DB" \
	-rank-iter 40 -rank-scale 6 -sample-rate 0.1

