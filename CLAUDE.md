# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

Use `go-toolchain` (no arguments, repo root) instead of bare `go` commands:

```sh
go-toolchain
```

This runs tests, checks coverage (80% threshold — enforced, currently near the
limit, so new code must arrive with tests), and builds to
`build/versioned-docs`. It also auto-rewrites the tree (formatting, import
order, dependency tidying) — accept and commit those changes, never revert
them.

## What This Tool Does

Generates versioned documentation with sparse inheritance and a doc-centric
default view:

1. `content_command` (optional) runs first and may prepare the content dir and
   version list (e.g. the `fetch-aggregate` subcommand).
2. `version_command` provides all software versions, one per line, oldest
   first. The last one is the "current" version.
3. The content directory is scanned for documented versions (subdirectories);
   every content subdir must be in the version list.
4. Undocumented versions inherit docs from the nearest documented version
   (prefer older, fallback to newer), page by page.
5. Output: doc-centric `index.html` + `docs/<doc>.html` (newest revision, per-
   doc picker over authored versions only), per-version history dirs for
   documented versions, `versions.html` matrix, `llms.txt` / `llms-full.md`.

## Termination

`config.yaml` may declare tombstones:

```yaml
terminated:
  some-doc.md: 1.2.3   # last version the doc applies to
```

Boundary semantics (tested in `terminate_test.go` / `docview_test.go`):
visible at `version == T`, hidden for `version > T`, dropped from default
listings into a "Removed" section when `T < current`, removed banner on the
still-visible historical pages. Unknown T, unknown doc path, or authored
content newer than T are build errors. Implementation: `terminate.go`.

## Layout

- `main.go` — CLI; dispatches the `fetch-aggregate` subcommand, otherwise
  generates: `-config`, `-content`, `-out`, `-templates`, `-base-url`.
- `config.go` — config load, `version_command`/`content_command` execution,
  documented-version discovery.
- `resolve.go` — positional inheritance resolution (order = version_command
  output order; no semver parsing in the core).
- `generate.go` — site generation (per-version history pages, llms outputs).
- `docview.go` — doc-centric layer: chains, doc pages, index/matrix data.
- `terminate.go` — tombstone validation + visibility rules.
- `fetch.go` — `fetch-aggregate`: adapts a docs-aggregate corpus branch
  (`<version>/docs/<file>.md`) into the content layout and emits the
  ascending-semver version list from the remote's version branches.
- `templates/` + `static/` — site assets, embedded via `assets.go` as
  fallbacks; a `-templates` dir overrides them.
- `example/` — tiny profile used by the link-check test.

## Config Format (config.yaml)

```yaml
project: myproject
repo: https://github.com/user/myproject

content_command: "./fetch-content.sh"   # optional, runs first
version_command: "./versions.sh"        # required
terminated: {}                          # optional tombstones
```

Both commands run via `sh -c` from the config file's directory.

## CI / publish

This repo is the generic tool only — no site is built or deployed here.
Consumer repos (reference: `PazerOP/claude-docs-gaps`) run the tool from
their own CI against their own corpus, with their own credentials.

`.github/workflows/ci.yml` is the only workflow: a single `test` job runs
go-toolchain on every push (tests, coverage, build), and go-toolchain's
autorelease publishes the built binaries to the buildhost project
`versioned-docs` via GitHub OIDC; consumers download
`https://dl.pazer.build/versioned-docs?branch=master&os=linux&arch=amd64`.
Autorelease failing a push whose built binary is unchanged ("Release
exists") is its design — leave it alone; do not add change-detection or
skip logic around the publishing. Keep the `test` job's name — the org's
`all-builds` merge gate aggregates automatically.
