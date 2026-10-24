#!/bin/sh
# Entry point of the benchmark image. It puts goforge on PATH only in the
# goforge conditions (goforge and goforge-required), and gives the run a writable home and Go directories
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

exec "$@"
