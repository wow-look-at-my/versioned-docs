# somesoftware v3

Updated docs reflecting v3 changes.

## Architecture

Same plugin architecture, but now with async event dispatch.
The event bus was rewritten to use channels internally.

## Configuration

Config moved to `~/.config/somesoftware/config.toml` (XDG compliant).

| Key       | Type   | Default | Description          |
|-----------|--------|---------|----------------------|
| debug     | bool   | false   | Enable debug logging |
| port      | int    | 8080    | HTTP listen port     |
| workers   | int    | 4       | **NEW** Worker pool  |
