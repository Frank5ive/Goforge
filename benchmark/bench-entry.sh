#!/bin/sh
# Entry point of the benchmark image. It puts goforge on PATH only in the
# goforge conditions (goforge, goforge-skill, goforge-mcp, and goforge-required), installs the skill in
# goforge-skill, and gives the run a writable home and Go directories
# (containers run as the host user, who can't write the image's /go).
set -e

case "$BENCH_CONDITION" in
goforge*)
	PATH="/opt/goforge/bin:$PATH"
	export PATH
	;;
esac

export HOME="${BENCH_HOME:-/bench/home}"
export GOPATH="$HOME/go"
mkdir -p "$HOME"

# The skill condition installs the skill that shipped with this GoForge version,
# where Claude Code looks for a user's skills
if [ "$BENCH_CONDITION" = "goforge-skill" ]; then
	mkdir -p "$HOME/.claude/skills"
	cp -r /opt/goforge/skills/. "$HOME/.claude/skills/"
fi

exec "$@"
