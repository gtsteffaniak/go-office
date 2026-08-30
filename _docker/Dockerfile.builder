# Reusable Alpine + Go builder for static go-office binaries.
#
#   make build-docker-builder
#
# Euro-Office assets and x2t still come from the Debian assets stage in
# _docker/Dockerfile — the .deb unpack needs dpkg on glibc Linux.

FROM golang:1.25-alpine
RUN apk add --no-cache git ca-certificates
ENV CGO_ENABLED=0
WORKDIR /src
