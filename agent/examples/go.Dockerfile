FROM ghcr.io/joshjon/verve:base

USER root

# Install Go from official image
COPY --from=golang:1.25-alpine /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

# Install make (used by Makefile targets)
RUN apk add --no-cache make

USER agent

# Set GOPATH for agent user
ENV GOPATH="/home/agent/go"
ENV PATH="${GOPATH}/bin:${PATH}"
