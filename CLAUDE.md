# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

Use `go-safe-build` instead of `go build`:

```sh
go-safe-build --min-coverage 10 -o versioned-docs
```

This runs tests, checks coverage, and builds if coverage threshold is met.

## What This Tool Does

Generates versioned documentation with sparse inheritance:

1. You define all software versions in config
2. The tool scans the content directory to discover which versions have authored docs
3. Undocumented versions inherit docs from the nearest documented version (prefer older, fallback to newer)
4. Outputs HTML with "inherited docs" banners and llms.txt/llms-full.md for LLM consumption

## Usage

```sh
./versioned-docs -config config.yaml -content content/ -out site/
```

## Config Format (config.yaml)

```yaml
project: myproject
repo: https://github.com/user/myproject

software_versions:
  - "1.0.0"
  - "1.1.0"
  - "1.2.0"
```

Documented versions are auto-detected by scanning the content directory for version subdirectories. Versions without a content folder inherit from the nearest documented version.
