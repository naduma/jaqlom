package main

import (
	"encoding/json"
	"html"
	"sort"
	"strconv"
	"strings"
)

func buildHTMLDocument(title string, headHTML string, bodyHTML string) string {
	var builder strings.Builder

	builder.WriteString("<!doctype html>\n")
	builder.WriteString(`<html lang="">`)
	builder.WriteString("\n<head>\n")
	builder.WriteString(`<meta charset="utf-8">`)
	builder.WriteString("\n")
	builder.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1">`)
	builder.WriteString("\n<title>")
	builder.WriteString(html.EscapeString(title))
	builder.WriteString("</title>\n")
	builder.WriteString(headHTML)
	builder.WriteString("</head>\n<body>\n")
	builder.WriteString(bodyHTML)
	builder.WriteString("\n</body>\n</html>")

	return builder.String()
}

func buildPageHTML(title string, rule Rule, bodyHTML string, content string) string {
	var headBuilder strings.Builder
	var bodyBuilder strings.Builder

	writeInlineCSS(&headBuilder, rule.CSS)

	if rule.Style != "" {
		headBuilder.WriteString("<style>")
		headBuilder.WriteString(rule.Style)
		headBuilder.WriteString("</style>\n")
	}

	bodyBuilder.WriteString(bodyHTML)
	bodyBuilder.WriteString("\n")
	writeContentPayload(&bodyBuilder, content)
	bodyBuilder.WriteString("\n")
	writeInlineScripts(&bodyBuilder, rule)

	return buildHTMLDocument(title, headBuilder.String(), bodyBuilder.String())
}

func writeInlineCSS(builder *strings.Builder, cssURLs []string) {
	for _, href := range cssURLs {
		builder.WriteString(`<style>@import url("`)
		builder.WriteString(html.EscapeString(href))
		builder.WriteString(`");</style>`)
		builder.WriteString("\n")
	}
}

func writeInlineScripts(builder *strings.Builder, rule Rule) {
	if len(rule.Imports) > 0 {
		builder.WriteString(`<script type="module">`)
		builder.WriteString("\n")
		builder.WriteString(buildModuleImports(rule.URL, rule.Imports))
		builder.WriteString(wrapRenderCode(rule.Render))
		builder.WriteString("</script>")
		return
	}

	if rule.URL != "" {
		builder.WriteString(`<script src="`)
		builder.WriteString(html.EscapeString(rule.URL))
		builder.WriteString(`" onerror="window.__jaqlomScriptLoadError='Failed to load script: `)
		builder.WriteString(html.EscapeString(rule.URL))
		builder.WriteString(`'"></script>`)
		builder.WriteString("\n")
		builder.WriteString("<script>")
		builder.WriteString("\n")
		builder.WriteString(wrapRenderCode(rule.Render))
		builder.WriteString("</script>")
		return
	}

	builder.WriteString("<script>\n")
	builder.WriteString(wrapRenderCode(rule.Render))
	builder.WriteString("</script>")
}

func writeContentPayload(builder *strings.Builder, content string) {
	builder.WriteString(`<script id="jaqlom-content" type="application/json">`)
	builder.WriteString(jsQuote(content))
	builder.WriteString(`</script>`)
}

func buildModuleImports(url string, imports map[string]string) string {
	var builder strings.Builder

	if defaultImport, ok := imports["default"]; ok {
		builder.WriteString("import ")
		builder.WriteString(defaultImport)
		builder.WriteString(" from ")
		builder.WriteString(strconv.Quote(url))
		builder.WriteString("\n")
	}

	namedImports := make([]string, 0, len(imports))
	for importedName := range imports {
		if importedName == "default" {
			continue
		}
		namedImports = append(namedImports, importedName)
	}
	sort.Strings(namedImports)

	for _, importedName := range namedImports {
		builder.WriteString("import { ")
		builder.WriteString(importedName)

		localName := imports[importedName]
		if localName != "" && localName != importedName {
			builder.WriteString(" as ")
			builder.WriteString(localName)
		}

		builder.WriteString(" } from ")
		builder.WriteString(strconv.Quote(url))
		builder.WriteString("\n")
	}

	return builder.String()
}

func wrapRenderCode(render string) string {
	var builder strings.Builder

	builder.WriteString("const output = document.querySelector('[data-role=\"output\"]') || document.querySelector(\"main\");\n")
	builder.WriteString("const status = document.querySelector('[data-role=\"status\"]');\n")
	builder.WriteString("window.__jaqlomLoadPromise = (async () => {\n")
	builder.WriteString("if (window.__jaqlomScriptLoadError) {\n")
	builder.WriteString("  throw new Error(window.__jaqlomScriptLoadError);\n")
	builder.WriteString("}\n")
	builder.WriteString("const contentNode = document.getElementById(\"jaqlom-content\");\n")
	builder.WriteString("const content = contentNode ? JSON.parse(contentNode.textContent || \"\\\"\\\"\") : \"\";\n")
	builder.WriteString("if (status) { status.textContent = ''; }\n")
	builder.WriteString("const result = await (async () => {\n")
	builder.WriteString(render)
	builder.WriteString("\n})();\n")
	builder.WriteString("if (typeof result === \"string\") {\n")
	builder.WriteString("  output.innerHTML = result\n")
	builder.WriteString("}\n")
	builder.WriteString("})().catch(function(e) {\n")
	builder.WriteString("  const raw = e.message || e;\n")
	builder.WriteString("  const message = String(raw).startsWith('Failed to load ') ? raw : 'Render error: ' + raw;\n")
	builder.WriteString("  if (status) { status.textContent = message; } else { output.textContent = message; }\n")
	builder.WriteString("});\n")

	return builder.String()
}

func jsQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
