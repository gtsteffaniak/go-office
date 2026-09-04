# Linux Go toolchain image for go-office development and cross-host builds.
#
#   make docker-dev-image     # build (macOS/Windows Makefile targets)
#   make build-docker-builder # same image, alternate target name
#
# Includes dpkg + file so fetch-assets, asset checks, and x2t validation work.

FROM golang:1.27-bookworm

RUN apt-get update \
 && apt-get install -y --no-install-recommends git ca-certificates dpkg file \
 && rm -rf /var/lib/apt/lists/*

ENV CGO_ENABLED=0
WORKDIR /src
