# versioned-docs

Static-site generator for documentation that is written sparsely across many
software versions. Only some versions have authored docs; every other version
inherits the nearest documented one. The output is doc-centric: each logical
document shows its newest authored revision by default, with a per-doc version
picker for older revisions and a per-version history view.

This repository is the generic **tool only**: the Go generator, the
`fetch-aggregate` corpus adapter, and their tests. Sites are built by the
repos that own the content, from their own CI, with their own credentials —
the reference consumer is
[claude-docs-gaps](https://github.com/PazerOP/claude-docs-gaps)
(investigation notes about `@anthropic-ai/claude-code`, authored per package
version), which builds and deploys its docs site itself using the binary this
repo publishes.

## Output model

For a content tree `content/<version>/<doc>.md` and a software version list,
the generator emits:

| Path | What it is |
|---|---|
| `index.html` | Doc-centric index: every live document, linking to its newest revision; removed documents in a separate section |
| `docs/<doc>.html` | Logical doc page: newest authored revision + a version picker listing **only the versions that author this doc** (newest first) |
| `<version>/<doc>.html` | History view: the doc as it applies to that documented version, with page-level inheritance and inherited-content banners |
| `versions.html` | Version-history matrix (documents × documented versions) plus a summary of which docs apply to which software version ranges |
| `llms.txt` | LLM index: newest URL per live doc, removed docs, per-version bundles |
| `llms-full.md` | One bundle of every live doc's newest revision |
| `<version>/llms-full.md` | Per-version bundle of everything that applies to that version |

Only **documented** versions get a `<version>/` directory: an undocumented
version renders byte-identical content to the nearest documented one (that is
what inheritance means), so emitting hundreds of duplicate directories would
add nothing. The version pickers follow the same collapsing: they list only
versions where the content actually changes.

### Inheritance

1. The software version list (from `version_command`) defines release order,
   oldest first.
2. Versions with a `content/<version>/` directory are "documented".
3. Per page: an undocumented version uses the nearest documented version at or
   below it; versions before all docs fall forward to the nearest one above.
   Inherited pages carry a banner naming their source version.

### Termination (tombstones)

Documents about removed features are terminated in `config.yaml`, not deleted
from the corpus:

```yaml
terminated:
  some-doc.md: 1.2.3   # last version this doc applies to
```

Exact boundary semantics for `terminated: {P: T}`:

- **`version == T`** — P is still **visible** (T is the last applicable version).
- **`version > T`** — P is **hidden**: no page in that version directory, no
  entry in that version's page list or `llms-full.md`, an empty cell in the
  matrix.
- **`T < current`** (current = last software version) — P is dropped from the
  default listings (`index.html` documents table, `llms.txt` Documents
  section, root `llms-full.md`) and moves to the **Removed** section, labeled
  "last applies to T".
- **`T == current`** — P still applies; it stays in the default listing with
  no banner.
- Historical pages that still show P (its logical doc page and every
  `<version <= T>/P.html`) carry a **removed banner**: "This document last
  applies to version T. It was removed in later versions…".
- Termination only truncates forward inheritance past T; versions `<= T` are
  untouched.

Validation (all hard build errors, so typos cannot silently drop docs):

- T not in the software version list
- P does not match any known document
- some version newer than T has authored content for P

## Configuration

```yaml
project: myproject
repo: https://github.com/user/myproject

# Optional; runs first, from the config file's directory. Prepares content/
# (e.g. fetches a corpus) and may produce the version list input.
content_command: "./fetch-content.sh"

# Required; runs from the config file's directory; stdout = one version per
# line, oldest -> newest. The last line is the "current" version.
version_command: "./versions.sh"

# Optional tombstones (see above).
terminated: {}
```

Run:

```sh
./build/versioned-docs -config config.yaml -content content/ -out site/ [-base-url /prefix] [-templates templates/]
```

## The `fetch-aggregate` adapter

A built-in subcommand that adapts a docs-aggregate style corpus (a git branch
laid out as `<version>/docs/<file>.md`, as produced by claude-docs-gaps'
aggregate-docs tool) into this tool's input layout. Consumers typically run it
as their config's `content_command`:

```sh
versioned-docs fetch-aggregate -remote <url-or-path> [-branch docs-aggregate] [-content content] [-versions-out versions.txt]
```

`fetch-aggregate`:

1. Lists the remote's `X.Y.Z` version branches (both `refs/heads/` and
   `refs/remotes/*/`, so an offline local clone works) → the software version
   list, ascending semver (numeric compare, so `2.1.98 < 2.1.104`).
2. Shallow-fetches the `docs-aggregate` branch and extracts
   `<version>/docs/<file>.md` → `content/<version>/<file>.md` (the `docs/`
   level is stripped; `INDEX.md`/`summary.json` are skipped).
3. Unions versions found in the corpus into the version list (a deleted
   branch cannot break content validation) and writes `versions.txt`.

Errors sanitize credentials out of URLs before they can reach logs.

### Running against a corpus locally

```sh
go-toolchain                                  # test + build to build/versioned-docs
git -C /path/to/corpus-clone fetch origin docs-aggregate
./build/versioned-docs fetch-aggregate -remote /path/to/corpus-clone
./build/versioned-docs -config config.yaml -content content -out site
```

where `config.yaml` is the consumer's profile (its `version_command` typically
just reads the `versions.txt` that `fetch-aggregate` wrote). The clone needs
the `docs-aggregate` ref plus whatever `X.Y.Z` branch refs it knows (fetch
them blobless with
`git fetch --filter=blob:none origin '+refs/heads/*:refs/remotes/origin/*'`
for the full version axis; without them the version axis is just the
documented versions). Never vendor a fetched corpus or a generated site.

## Consuming from CI

Consumer repos run the tool from their own CI, where their `GITHUB_TOKEN`
can read their own corpus — no cross-repo credentials anywhere. Either
download the prebuilt binary that this repo's CI publishes to
[buildhost](https://pazer.build):

```sh
curl -fL --compressed \
  "https://dl.pazer.build/versioned-docs?branch=master&os=linux&arch=amd64" \
  -o versioned-docs && chmod +x versioned-docs
```

(`linux/arm64` is published too; swap the `arch` param) — or build from
source with [go-toolchain](https://github.com/wow-look-at-my/go-toolchain) as
a fallback. Then run `fetch-aggregate` plus the generator against the
consumer's own corpus and deploy the output wherever that repo publishes.
[claude-docs-gaps](https://github.com/PazerOP/claude-docs-gaps) is the
reference consumer.

## CI (this repo)

`.github/workflows/ci.yml` runs a single `test` job on every push:
[go-toolchain](https://github.com/wow-look-at-my/go-toolchain) tests,
enforces the 80% coverage gate, builds, and — via its autorelease — publishes
the built binaries to the buildhost project `versioned-docs` (GitHub OIDC, no
static secret).

## Development

Build/test/coverage with [go-toolchain](https://github.com/wow-look-at-my/go-toolchain)
— never bare `go` commands:

```sh
go-toolchain
```

It runs the tests, enforces the coverage threshold, and builds to
`build/versioned-docs`. `example/` holds a tiny self-contained profile used by
the link-check test; `templates/` and `static/` are embedded into the binary
as fallbacks and can be overridden per site with `-templates`.
