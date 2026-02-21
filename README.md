# init

A lightweight MCP server and CLI tool that bootstraps new projects with boilerplate files. It embeds template files (LICENSE, CONTRIBUTING.md) at compile time and writes them to a target directory, refusing to overwrite any existing files.

## Installation

```bash
go build -o init .
cp init /usr/local/bin/init
```

Or with just:

```bash
just install
```

## Usage

### MCP Server

Register as an MCP server in your Claude configuration:

```bash
claude mcp add --transport stdio init -- /usr/local/bin/init
```

The server exposes a single `init` tool that accepts a `directory` parameter and writes the embedded template files there.

### CLI

```bash
init --cli --directory /path/to/new/project
```

Returns JSON with the list of files created:

```json
{"directory": "/path/to/new/project", "files_created": ["/path/to/new/project/LICENSE", "/path/to/new/project/CONTRIBUTING.md"]}
```

### Customizing Templates

Edit the files in `files/` and rebuild. The `go:embed` directives in `main.go` bundle them into the binary. To add new templates, add a new embedded file variable and append it to the `embeddedFiles` slice with the desired destination filename.
