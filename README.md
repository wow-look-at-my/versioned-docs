# versioned-docs

Static-site generator for documentation that is written sparsely across many
software versions. Only some versions have authored docs; every other version
inherits the nearest documented one. The output is doc-centric: each logical
document shows its newest authored revision by default, with a per-doc version
picker for older revisions and a per-version history view.

Its flagship input is the [claude-docs-gaps](https://github.com/PazerOP/claude-docs-gaps)
corpus (investigation notes about `@anthropic-ai/claude-code`, authored per
package version), built and published by CI to
`https://sites.pazer.build/versioned-docs/branch/master/`.

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

## The claude-docs-gaps profile (`gapsdocs/`)

`gapsdocs/config.yaml` wires the corpus in via the built-in adapter
subcommand:

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

### Building against the real corpus locally

```sh
go-toolchain                                       # test + build to build/versioned-docs
git -C /path/to/claude-docs-gaps fetch origin docs-aggregate
GAPS_REMOTE_URL=/path/to/claude-docs-gaps \
  ./build/versioned-docs -config gapsdocs/config.yaml -content gapsdocs/content -out site
```

The clone needs the `docs-aggregate` ref plus whatever `X.Y.Z` branch refs it
knows (fetch them blobless with
`git fetch --filter=blob:none origin '+refs/heads/*:refs/remotes/origin/*'`
for the full version axis; without them the version axis is just the
documented versions). `gapsdocs/content/`, `gapsdocs/versions.txt`, and
`site/` are gitignored — the corpus is never vendored.

## CI / deploy

- **ci.yml** — `test` (go-toolchain: tests, 80% coverage gate, build) on every
  push, uploading the build as an artifact; `deploy` (master pushes and every
  `workflow_dispatch`) downloads that artifact, fetches the corpus with the
  org-wide `PRIVATE_ORG_REPO_READ` secret (the corpus repo is private and
  cross-owner — the default token cannot read it), builds the site, and
  publishes it to buildhost under `branch/<ref-name>` via
  `buildhost-publish-site` (OIDC).
- **refresh.yml** — cron every 3 days that only re-dispatches ci.yml on
  master. The indirection is load-bearing: buildhost rejects OIDC tokens from
  `schedule`-event runs, so a direct scheduled deploy would 401 (this exact
  failure broke the upstream claude-docs-gaps aggregate cron).
- **preview.yml** — builds the site for each PR and delegates to the org's
  reusable `buildhost-preview` workflow (deploys to `branch/pr-<n>` and posts
  a sticky comment with the URL).

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
