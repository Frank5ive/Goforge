# The environment every benchmark run uses, for the agent and for scoring.
# Versions are pinned so runs are comparable; bench tags the image with them
# and records its ID in each result.
FROM golang:1.26.0-bookworm AS go

# Claude Code needs Node.js 22
FROM node:22-bookworm

COPY --from=go /usr/local/go /usr/local/go
ENV PATH=/usr/local/go/bin:$PATH

RUN apt-get update \
 && apt-get install -y --no-install-recommends sqlite3 postgresql-client \
 && rm -rf /var/lib/apt/lists/*

ARG CLAUDE_CODE_VERSION
RUN npm install -g "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}"

# GoForge is installed in every image, but on PATH only in the goforge
# condition (see bench-entry)
ARG GOFORGE_VERSION
RUN GOBIN=/opt/goforge/bin go install "github.com/Frank5ive/goforge@${GOFORGE_VERSION}" \
 && go clean -cache -modcache

# Go 1.26 is the only toolchain: no downloads of others mid-run
ENV GOTOOLCHAIN=local

COPY bench-entry.sh /usr/local/bin/bench-entry
ENTRYPOINT ["/usr/local/bin/bench-entry"]
