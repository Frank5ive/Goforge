/*
Goforge generates production-ready Go REST APIs and keeps extending them.

A generated server uses only the standard library's net/http and comes with
graceful shutdown, structured logging, request IDs, panic recovery, timeouts,
health and readiness checks, tests, a Makefile, a golangci-lint config, a
Dockerfile, and a GitHub Actions workflow. GoForge then adds REST resources and
databases to it, so a project can grow without rewriting its infrastructure.
It is built to be run by developers and by AI coding agents.

# Installation

	go install github.com/Frank5ive/goforge@latest

Installing needs Go 1.26 or newer; generated projects need Go 1.24 or newer.
Prebuilt binaries for Linux, macOS, and Windows are on the releases page.

# Usage

	goforge generate      create a server
	goforge add resource  add a REST resource with validation, pagination, and tests
	goforge add db        store data in PostgreSQL, MySQL/MariaDB, SQLite, or MongoDB
	goforge inspect       describe a project: resources, routes, database, settings
	goforge check         run gofmt, go vet, the tests, and golangci-lint, and report
	                      each problem with its file and line

A whole API in one command:

	goforge generate --name shop --module github.com/you/shop --db postgres \
	  --resource "category/categories:name:string,description:string?" \
	  --resource "product:name:string(1..100),price:float(0..),category_id:ref(category)"

Or step by step, inside a generated project:

	goforge add resource product --fields "name:string(1..100),price:float(0..),in_stock:bool"
	goforge add db postgres

Each resource serves a paged list, create, get, PUT, PATCH, and DELETE, with
its validation rules checked and tested. The project describes itself at
GET /openapi.json, an OpenAPI 3.1 document that goforge regenerates for each
resource it adds. Field types are string, int, float,
bool, time, email, and ref(<resource>). A trailing ? makes a string, email, or
ref optional, and (min..max) bounds a number or a string's length.

# Scripts and AI agents

Commands never prompt without a terminal. generate and the add commands accept
--dry-run, and change all files or none. generate, add, inspect, and check
accept --json, which prints the result as JSON; a failure exits with status 1
and reports a stable error code, a message, and a hint.

Run goforge --help, or goforge <command> --help, for every flag. The [README]
covers each command in detail, and [llms.txt] is a compact reference for
language models.

[README]: https://github.com/Frank5ive/goforge#readme
[llms.txt]: https://github.com/Frank5ive/goforge/blob/master/llms.txt
*/
package main
