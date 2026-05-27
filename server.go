package main

import (
	"errors"
	"html"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

const renderedFileBody = `<p data-role="status"></p><main data-role="output"></main>`

func newServer(rootDir string, files []string, cfg Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", serveRenderedIndex(files))
	mux.HandleFunc("/file", serveRenderedFile(rootDir, cfg))
	return mux
}

func serveRenderedIndex(files []string) http.HandlerFunc {
	indexHTML := buildIndexHTML(files)

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(indexHTML))
	}
}

func buildIndexHTML(files []string) string {
	var bodyBuilder strings.Builder

	bodyBuilder.WriteString(`<p data-role="status"></p>`)
	bodyBuilder.WriteString("\n<ul data-role=\"file-list\">\n")
	for _, file := range files {
		bodyBuilder.WriteString(`<li><a href="/file?path=`)
		bodyBuilder.WriteString(url.QueryEscape(file))
		bodyBuilder.WriteString(`">`)
		bodyBuilder.WriteString(html.EscapeString(file))
		bodyBuilder.WriteString("</a></li>\n")
	}
	bodyBuilder.WriteString("</ul>")

	return buildHTMLDocument("jaqlom", "", bodyBuilder.String())
}

func serveRenderedFile(rootDir string, cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		relPath := r.URL.Query().Get("path")
		if relPath == "" {
			serveRenderedMessage(w, "file", "No path specified.")
			return
		}

		title := filepath.Base(relPath)

		rule, err := cfg.ruleForPath(relPath)
		if err != nil {
			ext := strings.TrimPrefix(filepath.Ext(relPath), ".")
			if ext == "" {
				serveRenderedError(w, http.StatusNotFound, title, "No rule found for file without extension")
				return
			}
			serveRenderedError(w, http.StatusNotFound, title, "No rule found for extension: "+ext)
			return
		}

		data, err := readFile(rootDir, relPath)
		if err != nil {
			switch {
			case errors.Is(err, ErrForbidden):
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			case errors.Is(err, ErrNotFound):
				serveRenderedError(w, http.StatusNotFound, title, "File not found: "+relPath)
			default:
				serveRenderedError(w, http.StatusInternalServerError, title, http.StatusText(http.StatusInternalServerError))
			}
			return
		}
		pageHTML := buildPageHTML(title, rule, renderedFileBody, string(data))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(pageHTML))
	}
}

func serveRenderedMessage(w http.ResponseWriter, title, message string) {
	serveRenderedError(w, http.StatusOK, title, message)
}

func serveRenderedError(w http.ResponseWriter, statusCode int, title, message string) {
	pageHTML := buildPageHTML(title, renderedMessageRule(message), renderedFileBody, "")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(pageHTML))
}

func renderedMessageRule(message string) Rule {
	return Rule{Render: "if (status) { status.textContent = " + jsQuote(message) + " } else { output.textContent = " + jsQuote(message) + " }"}
}
