package main

import (
	"strings"
	"testing"
)

func TestBuildPageHTMLIncludesDocumentStructure(t *testing.T) {
	t.Parallel()

	got := buildPageHTML("Guide", Rule{}, `<main><article>Hello</article></main>`, "file content")

	wantParts := []string{
		"<!doctype html>",
		`<html lang="">`,
		`<head>`,
		`<meta charset="utf-8">`,
		`<meta name="viewport" content="width=device-width, initial-scale=1">`,
		`<title>Guide</title>`,
		`</head>`,
		`<body>`,
		`<main><article>Hello</article></main>`,
		`id="jaqlom-content"`,
		`</body>`,
		`</html>`,
	}
	for _, want := range wantParts {
		if !strings.Contains(got, want) {
			t.Fatalf("buildPageHTML(...) = %q, want substring %q", got, want)
		}
	}
	if strings.Contains(got, `<script src="`) {
		t.Fatalf("buildPageHTML(...) emitted script src for empty rule")
	}
}

func TestBuildPageHTMLEmbedsCSSAndInlineStyleInHead(t *testing.T) {
	t.Parallel()

	rule := Rule{
		CSS: []string{
			"https://cdn.example.test/markdown.css",
			"https://cdn.example.test/theme.css",
		},
		Style: ".markdown-body { max-width: 48rem; }",
	}

	got := buildPageHTML("Guide", rule, "<main></main>", "file content")

	headStart := strings.Index(got, "<head>")
	headEnd := strings.Index(got, "</head>")
	if headStart == -1 || headEnd == -1 || headEnd <= headStart {
		t.Fatalf("buildPageHTML(...) head section not found: %q", got)
	}
	head := got[headStart:headEnd]

	wantParts := []string{
		`<style>@import url("https://cdn.example.test/markdown.css");</style>`,
		`<style>@import url("https://cdn.example.test/theme.css");</style>`,
		`<style>.markdown-body { max-width: 48rem; }</style>`,
	}
	for _, want := range wantParts {
		if !strings.Contains(head, want) {
			t.Fatalf("buildPageHTML(...) head = %q, want substring %q", head, want)
		}
	}
	if strings.Contains(head, `\n`) {
		t.Fatalf("buildPageHTML(...) head contains literal \\n sequence: %q", head)
	}
}

func TestBuildPageHTMLIncludesUMDScriptAndRenderCode(t *testing.T) {
	t.Parallel()

	rule := Rule{
		URL:    "https://cdn.example.test/marked.js",
		Render: "return marked.parse(content)",
	}

	got := buildPageHTML("Guide", rule, "<main id=\"output\"></main>", "# guide")

	if !strings.Contains(got, `<script src="https://cdn.example.test/marked.js"`) {
		t.Fatalf("buildPageHTML(...) missing <script> tag")
	}
	if !strings.Contains(got, `onerror="window.__jaqlomScriptLoadError='Failed to load script: https://cdn.example.test/marked.js'"`) {
		t.Fatalf("buildPageHTML(...) missing UMD script onerror handler")
	}

	if !strings.Contains(got, "return marked.parse(content)") {
		t.Fatalf("buildPageHTML(...) missing render code")
	}
	if !strings.Contains(got, "window.__jaqlomScriptLoadError") {
		t.Fatalf("buildPageHTML(...) missing error handler")
	}
	if strings.Contains(got, `<script type="module">`) {
		t.Fatalf("buildPageHTML(...) emitted module script for UMD rule")
	}
}

func TestBuildPageHTMLIncludesESMImportsAndRenderCode(t *testing.T) {
	t.Parallel()

	rule := Rule{
		URL: "https://cdn.example.test/graphviz.js",
		Imports: map[string]string{
			"default":  "GraphvizDefault",
			"Graphviz": "Graphviz",
		},
		Render: "const graphviz = await Graphviz.load(); output.innerHTML = graphviz.dot(content)",
	}

	got := buildPageHTML("Graph", rule, "<main id=\"output\"></main>", "file content")

	wantParts := []string{
		`<script type="module">`,
		`import GraphvizDefault from "https://cdn.example.test/graphviz.js"`,
		`import { Graphviz } from "https://cdn.example.test/graphviz.js"`,
		`const graphviz = await Graphviz.load(); output.innerHTML = graphviz.dot(content)`,
	}
	for _, want := range wantParts {
		if !strings.Contains(got, want) {
			t.Fatalf("buildPageHTML(...) = %q, want substring %q", got, want)
		}
	}
	if strings.Contains(got, `<script src="`) {
		t.Fatalf("buildPageHTML(...) emitted script src for ESM rule")
	}
}

func TestBuildPageHTMLProvidesRenderContext(t *testing.T) {
	t.Parallel()

	rule := Rule{
		URL:    "https://cdn.example.test/marked.js",
		Render: `output.innerHTML = content`,
	}

	got := buildPageHTML("Guide", rule, "<main></main>", "# guide")
	if !strings.Contains(got, `const content = contentNode ? JSON.parse(contentNode.textContent || "\"\"") : "";`) {
		t.Fatalf("buildPageHTML(...) does not declare content binding")
	}
	if !strings.Contains(got, `const output = document.querySelector('[data-role="output"]') || document.querySelector("main");`) {
		t.Fatalf("buildPageHTML(...) does not declare output binding")
	}
}

func TestBuildPageHTMLModuleScriptUsesLexicalBindings(t *testing.T) {
	t.Parallel()

	rule := Rule{
		URL: "https://cdn.example.test/graphviz.js",
		Imports: map[string]string{
			"Graphviz": "Graphviz",
		},
		Render: "const gv = await Graphviz.load(); output.innerHTML = gv.dot(content.trim());",
	}

	got := buildPageHTML("Graph", rule, "<main></main>", "file content")
	if strings.Contains(got, "globalThis.content") || strings.Contains(got, "globalThis.output") {
		t.Fatal("module script uses globalThis bindings")
	}
	if !strings.Contains(got, "const content =") {
		t.Fatal("module script does not declare content lexical binding")
	}
	if !strings.Contains(got, "const output =") {
		t.Fatal("module script does not declare output lexical binding")
	}
}

func TestBuildPageHTMLRenderErrorsWriteToOutput(t *testing.T) {
	t.Parallel()

	rule := Rule{Render: `throw new Error("boom")`}
	got := buildPageHTML("Guide", rule, "<main></main>", "# guide")

	if !strings.Contains(got, `const message = String(raw).startsWith('Failed to load ') ? raw : 'Render error: ' + raw;`) {
		t.Fatal("buildPageHTML(...) does not normalize render errors")
	}
	if !strings.Contains(got, `if (status) { status.textContent = message; } else { output.textContent = message; }`) {
		t.Fatal("buildPageHTML(...) does not write render errors to output/status")
	}
}

func TestBuildPageHTMLEmbedsEscapedContent(t *testing.T) {
	t.Parallel()

	rule := Rule{
		Ext:    "md",
		URL:    "https://example.test/marked.js",
		Render: "return content",
	}

	html := buildPageHTML("test.md", rule, `<main></main>`, `# Hello <world>`)
	if !strings.Contains(html, `id="jaqlom-content"`) {
		t.Fatalf("buildPageHTML() should embed content payload with id=jaqlom-content")
	}
	if !strings.Contains(html, `"# Hello \u003cworld\u003e"`) {
		t.Fatalf("buildPageHTML() should embed content as JSON: got %q", html)
	}

	html = buildPageHTML("test.md", rule, `<main></main>`, `</script>\nlarge content`)
	if !strings.Contains(html, `\u003c/script\u003e`) {
		t.Fatalf("buildPageHTML() should escape closing script tag, got %q", html)
	}

	largeContent := strings.Repeat("x", 10000)
	html = buildPageHTML("test.md", rule, `<main></main>`, largeContent)
	if !strings.Contains(html, largeContent) {
		t.Fatalf("buildPageHTML() should embed large content without truncation")
	}
	if !strings.Contains(html, `id="jaqlom-content"`) {
		t.Fatalf("buildPageHTML() should embed large content in jaqlom-content payload")
	}
}
