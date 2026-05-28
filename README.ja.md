# jaqlom

ローカル環境で使うファイルビューア Web サーバ。設定ファイルに拡張子ごとのルールを定義するだけで、新しいファイル形式に対応できる。

[English README](README.md)

[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

## Features

- **シングルバイナリ** — ビルド後は実行ファイル 1 つで動作する
- **設定駆動** — コードを変更せず `jaqlom.json` のルール追加だけで対応形式を増やせる
- **ブラウザレンダリング** — サーバはファイル内容を HTML に埋め込んで返し、表示処理はブラウザ上の JavaScript が行う

## Requirements

- Go 1.21 以上
- CDN 上のスクリプト・CSS を使う場合はインターネット接続

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

[GitHub Releases](https://github.com/naduma/jaqlom/releases) からプラットフォーム別のバイナリをダウンロードできる。

## Usage

```text
jaqlom [flags] [directory]
```

`directory` を省略するとコマンドを実行したカレントディレクトリを走査する。

### Flags

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `-port` | `8080` | 待ち受けポート |
| `-config` | `<directory>/jaqlom.json` | 設定ファイルのパス |
| `-assets` | _(なし)_ | `/assets/` として配信する静的ファイルのディレクトリ |

### Examples

```sh
# カレントディレクトリを走査してポート 8080 で起動
jaqlom

# ディレクトリを指定して起動
jaqlom /path/to/docs

# ポートと設定ファイルを指定
jaqlom -port 3000 -config /path/to/jaqlom.json /path/to/docs

# CDN の代わりにローカルのアセットを使用して起動
jaqlom -assets /path/to/assets /path/to/docs
```

起動後、ターミナルに表示された URL（デフォルト: `http://localhost:8080`）をブラウザで開く。

## Configuration

走査ディレクトリ直下に `jaqlom.json` を置く。`-config` フラグで別パスを指定することもできる。設定変更後はサーバの再起動が必要。

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

| フィールド | 型 | 説明 |
|------------|-----|------|
| `ext` | string | 拡張子。`"md"` と `".md"` のどちらでも可 |
| `url` | string | 読み込むスクリプト URL。`-assets` 起動時はローカルパス（例: `/assets/marked.min.js`）も指定可（省略可） |
| `imports` | object | ES Modules 用の import 定義。キーが import 名、値がローカル変数名。`default` はデフォルト import（省略可） |
| `css` | string[] | `@import url(...)` で埋め込む CSS URL 一覧（省略可） |
| `style` | string | `<style>` として埋め込むインライン CSS（省略可） |
| `render` | string | ブラウザ上で実行される JavaScript スニペット（必須） |

`imports` が 1 つ以上ある場合は `<script type="module">` として出力される。同じ拡張子に複数ルールがある場合は最初に一致したルールが使われる。

### render スニペット

`render` は async 関数の本体として実行されるため、`await` が使える。

**利用できる変数**

| 変数 | 型 | 内容 |
|------|----|------|
| `content` | `string` | 対象ファイルの生テキスト |
| `output` | `HTMLElement` | レンダリング先要素 |
| `status` | `HTMLElement \| null` | ステータスメッセージ表示先 |

`string` を返すと `output.innerHTML` に代入される。`undefined` / `null` の場合は何もしない（スニペット内で `output.innerHTML` を直接操作する想定）。

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
<summary>ローカルアセット（オフライン利用）</summary>

スクリプトや CSS をローカルにダウンロードし、`url` と `css` に `/assets/` パスを指定して `-assets` フラグで起動する。

```sh
# 必要なファイルをダウンロード
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

`example/` ディレクトリには対応形式すべてのルールを含む `jaqlom.local.json` がある。使用前に必要なアセットをダウンロードする:

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

- **ローカル専用ツール** — ネットワーク公開は非推奨
- `GET /file` は解決後の絶対パスが走査ディレクトリ配下であることを検証し、範囲外は `403` で拒否する
- 認証・CSP は実装していない
- `render` はブラウザ内のインラインスクリプトとして実行されるため、信頼できる設定ファイルのみ使用すること

## Development

### Test

```sh
go test ./...
```

E2E テストには Node.js が必要。事前に依存パッケージをインストールすること:

```sh
npm install
go test ./...
```

### Project structure

```text
main.go          CLI フラグ解析、設定読み込み、サーバ起動
config.go        設定ファイル読み込み、拡張子ルール検索
scanner.go       ディレクトリ走査
server.go        HTTP ハンドラ
builder.go       HTML 生成
file_service.go  パストラバーサル防止とファイル読み込み
jaqlom.json      設定例
```

## License

MIT
