# somesoftware v2

Initial reverse-engineered documentation.

## Architecture

The software uses a plugin-based architecture with a central event bus.

## Configuration

Config is stored in `~/.somesoftware/config.toml`.

| Key       | Type   | Default | Description          |
|-----------|--------|---------|----------------------|
| debug     | bool   | false   | Enable debug logging |
| port      | int    | 8080    | HTTP listen port     |
