# jaqlom

A local file viewer web server. Add extension rules to `jaqlom.json` to support new file formats without changing any code.

[日本語版 README](README.ja.md)

[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

## Features

- **Single binary** — just one executable after building
- **Config-driven** — add support for new formats by editing `jaqlom.json`, no code changes needed
- **Browser rendering** — the server embeds file content into HTML; all rendering is handled by JavaScript in the browser

## Requirements

- Go 1.21 or later
- Internet access when loading scripts/CSS from a CDN

## Installation

### go install

```sh
go install github.com/naduma/jaqlom@latest
```

### Build from source

```sh
git clone https://github.com/naduma/jaqlom.git
cd jaqlom
go build -o jaqlom .
```

### Binary

Download a platform-specific binary from [GitHub Releases](https://github.com/naduma/jaqlom/releases).

## Usage

```text
jaqlom [flags] [directory]
```

If `directory` is omitted, the current working directory is used.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `8080` | Port to listen on |
| `-config` | `<directory>/jaqlom.json` | Path to the config file |
| `-assets` | _(none)_ | Directory to serve as static assets under `/assets/` |

### Examples

```sh
# Scan the current directory and start on port 8080
jaqlom

# Specify a directory
jaqlom /path/to/docs

# Specify port and config file
jaqlom -port 3000 -config /path/to/jaqlom.json /path/to/docs

# Serve local assets instead of loading from a CDN
jaqlom -assets /path/to/assets /path/to/docs
```

Open the URL shown in the terminal (default: `http://localhost:8080`) in your browser.

## Configuration

Place `jaqlom.json` in the root of the scanned directory, or use `-config` to point to another path. Changes require a server restart.

```json
{
  "rules": [
    {
      "ext": "md",
      "url": "https://cdn.jsdelivr.net/npm/marked/marked.min.js",
      "css": [
        "https://cdn.jsdelivr.net/npm/github-markdown-css/github-markdown.min.css"
      ],
      "style": ".markdown-body { max-width: 48rem; margin: 2rem auto; padding: 0 1rem; }",
      "render": "output.className = 'markdown-body'; return marked.parse(content)"
    }
  ]
}
```

### Rule fields

| Field | Type | Description |
|-------|------|-------------|
| `ext` | string | File extension. Both `"md"` and `".md"` are accepted |
| `url` | string | Script URL to load. Use a local path (e.g. `/assets/marked.min.js`) when running with `-assets` (optional) |
| `imports` | object | ES Module import map. Keys are import names, values are local variable names. `default` means a default import (optional) |
| `css` | string[] | CSS URLs to embed via `@import url(...)` (optional) |
| `style` | string | Inline CSS to embed in a `<style>` tag (optional) |
| `render` | string | JavaScript snippet executed in the browser (required) |

When one or more `imports` are defined, the script is output as `<script type="module">`. If multiple rules match the same extension, the first match wins.

### render snippet

`render` runs as the body of an async function, so `await` is available.

**Available variables**

| Variable | Type | Description |
|----------|------|-------------|
| `content` | `string` | Raw text content of the target file |
| `output` | `HTMLElement` | Element to render into |
| `status` | `HTMLElement \| null` | Element for status messages |

Returning a `string` assigns it to `output.innerHTML`. Returning `undefined` or `null` does nothing (assumes the snippet manipulates `output.innerHTML` directly).

### Configuration examples

<details>
<summary>Mermaid</summary>

```json
{
  "rules": [
    {
      "ext": "mmd",
      "url": "https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js",
      "style": "main { display: flex; justify-content: center; padding: 2rem; }",
      "render": "mermaid.initialize({ startOnLoad: false }); const { svg } = await mermaid.render('mermaid-' + Date.now(), content.trim()); output.innerHTML = svg;"
    }
  ]
}
```

</details>

<details>
<summary>AsciiDoc</summary>

```json
{
  "rules": [
    {
      "ext": "adoc",
      "url": "https://cdn.jsdelivr.net/npm/@asciidoctor/core/dist/browser/asciidoctor.js",
      "style": "main { max-width: 48rem; margin: 2rem auto; padding: 0 1rem; }",
      "render": "return Asciidoctor().convert(content)"
    }
  ]
}
```

</details>

<details>
<summary>Graphviz DOT</summary>

```json
{
  "rules": [
    {
      "ext": "dot",
      "url": "https://cdn.jsdelivr.net/npm/@hpcc-js/wasm-graphviz/dist/index.js",
      "imports": { "Graphviz": "Graphviz" },
      "style": "main { display: flex; justify-content: center; padding: 2rem; }",
      "render": "const graphviz = await Graphviz.load(); output.innerHTML = graphviz.dot(content.trim());"
    }
  ]
}
```

</details>

<details>
<summary>JSON</summary>

```json
{
  "rules": [
    {
      "ext": "json",
      "style": "main { max-width: 48rem; margin: 2rem auto; padding: 0 1rem; } pre { background: #f6f8fa; padding: 1rem; border-radius: 6px; overflow: auto; font-size: 0.875rem; }",
      "render": "const obj = JSON.parse(content); const escaped = JSON.stringify(obj, null, 2).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;'); return '<pre>' + escaped + '</pre>';"
    }
  ]
}
```

</details>

<details>
<summary>Local assets (offline use)</summary>

Download scripts and CSS files locally, then point `url` and `css` to `/assets/` paths and start the server with `-assets`.

```sh
# Download the files you need
curl -o assets/marked.min.js https://cdn.jsdelivr.net/npm/marked/marked.min.js
curl -o assets/github-markdown.min.css https://cdn.jsdelivr.net/npm/github-markdown-css/github-markdown.min.css

jaqlom -assets ./assets /path/to/docs
```

```json
{
  "rules": [
    {
      "ext": "md",
      "url": "/assets/marked.min.js",
      "css": [
        "/assets/github-markdown.min.css"
      ],
      "style": ".markdown-body { max-width: 48rem; margin: 2rem auto; padding: 0 1rem; }",
      "render": "output.className = 'markdown-body'; return marked.parse(content)"
    }
  ]
}
```

The `example/` directory includes `jaqlom.local.json` with rules for all supported formats. To use it, download the required assets first:

```sh
mkdir -p example/assets
curl -o example/assets/marked.min.js https://cdn.jsdelivr.net/npm/marked/marked.min.js
curl -o example/assets/github-markdown.min.css https://cdn.jsdelivr.net/npm/github-markdown-css/github-markdown.min.css
curl -o example/assets/mermaid.min.js https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js
curl -o example/assets/asciidoctor.js https://cdn.jsdelivr.net/npm/@asciidoctor/core/dist/browser/asciidoctor.js

jaqlom -assets example/assets -config example/jaqlom.local.json example/
```

</details>

## Security

- **Local use only** — exposing to a network is not recommended
- `GET /file` validates that the resolved absolute path is within the scanned directory; paths outside return `403`
- No authentication or CSP is implemented
- `render` runs as an inline script in the browser — only use trusted config files

## Development

### Test

```sh
go test ./...
```

E2E tests require Node.js. Install dependencies before running:

```sh
npm install
go test ./...
```

### Project structure

```text
main.go          CLI flag parsing, config loading, server startup
config.go        Config file loading, extension rule lookup
scanner.go       Directory scanning
server.go        HTTP handlers
builder.go       HTML generation
file_service.go  Path traversal prevention and file reading
jaqlom.json      Example config
```

## License

MIT
