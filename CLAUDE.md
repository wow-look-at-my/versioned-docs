# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

Use `go-safe-build` instead of `go build`:

```sh
go-safe-build
```

This runs tests, checks coverage (80% threshold), and builds to `build/versioned-docs`.

## What This Tool Does

Generates versioned documentation with sparse inheritance:

1. You define all software versions in config
2. The tool scans the content directory to discover which versions have authored docs
3. Undocumented versions inherit docs from the nearest documented version (prefer older, fallback to newer)
4. Outputs HTML with "inherited docs" banners and llms.txt/llms-full.md for LLM consumption

## Usage

```sh
./build/versioned-docs -config config.yaml -content content/ -out site/
```

## Config Format (config.yaml)

```yaml
project: myproject
repo: https://github.com/user/myproject

version_command: "./versions.sh"
```

The tool runs `version_command` from the config file's directory, reads stdout (one version per line), and uses that as the software versions list. Order is preserved from script output.

Documented versions are auto-detected by scanning the content directory for version subdirectories. All content directories must be in the software versions list (catches typos/misconfigs). Versions without a content folder inherit from the nearest documented version.
