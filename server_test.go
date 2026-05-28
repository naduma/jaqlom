package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var reServerIndexLink = regexp.MustCompile(`<a href="([^"]+)">([^<]+)</a>`)

func TestNewServerServesIndexHTML(t *testing.T) {
	t.Parallel()

	server := newServer("", []string{"docs/guide.md"}, Config{}, "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("newServer().ServeHTTP(%q) status = %d, want %d", "/", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("newServer().ServeHTTP(%q) content type = %q, want text/html", "/", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, `<a href="/file?path=docs%2Fguide.md">docs/guide.md</a>`) {
		t.Fatalf("newServer().ServeHTTP(%q) body = %q, want server-rendered file link", "/", body)
	}
}

func TestIndexHTMLContainsServerRenderedLinksOnly(t *testing.T) {
	t.Parallel()

	html := buildIndexHTML([]string{"docs/guide.md"})
	if strings.Contains(html, "/api") {
		t.Fatalf("buildIndexHTML(...) should not reference API endpoints: %q", html)
	}
	if strings.Contains(html, "fetch(") {
		t.Fatalf("buildIndexHTML(...) should not contain client-side fetch: %q", html)
	}
	if strings.Contains(html, "<script") {
		t.Fatalf("buildIndexHTML(...) should not require scripts: %q", html)
	}
}

func TestIndexHTMLRendersAllFilesAndBuildsDetailLinks(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(newServer("", []string{"docs/a.md", "nested/b.md"}, Config{}, ""))
	defer server.Close()

	got := renderIndexHTML(t, server.URL)
	want := renderedIndexHTML{
		Links: []renderedLink{
			{TextContent: "docs/a.md", Href: "/file?path=docs%2Fa.md"},
			{TextContent: "nested/b.md", Href: "/file?path=nested%2Fb.md"},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("renderIndexHTML(%q) mismatch (-want +got):\n%s", server.URL, diff)
	}
}

func TestNewServerReturnsNotFoundForUnknownPaths(t *testing.T) {
	t.Parallel()

	server := newServer("", nil, Config{}, "")

	for _, path := range []string{"/favicon.ico", "/robots.txt", "/unknown"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("newServer().ServeHTTP(%q) status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

func TestNewServerServesLocalAssets(t *testing.T) {
	t.Parallel()

	assetsDir := t.TempDir()
	writeTestFile(t, filepath.Join(assetsDir, "marked.min.js"), "console.log(\"marked\");")

	req := httptest.NewRequest(http.MethodGet, "/assets/marked.min.js", nil)
	rec := httptest.NewRecorder()

	newServer("", nil, Config{}, assetsDir).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("newServer().ServeHTTP(%q) status = %d, want %d", "/assets/marked.min.js", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "javascript") {
		t.Fatalf("newServer().ServeHTTP(%q) content type = %q, want javascript", "/assets/marked.min.js", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, "console.log") {
		t.Fatalf("newServer().ServeHTTP(%q) body = %q, want file content", "/assets/marked.min.js", body)
	}
}

func TestNewServerServesLocalAssetsFromSubdirectories(t *testing.T) {
	t.Parallel()

	assetsDir := t.TempDir()
	writeTestFile(t, filepath.Join(assetsDir, "css", "theme.css"), "body { color: black; }")

	req := httptest.NewRequest(http.MethodGet, "/assets/css/theme.css", nil)
	rec := httptest.NewRecorder()

	newServer("", nil, Config{}, assetsDir).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("newServer().ServeHTTP(%q) status = %d, want %d", "/assets/css/theme.css", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); !strings.Contains(body, "color: black") {
		t.Fatalf("newServer().ServeHTTP(%q) body = %q, want file content", "/assets/css/theme.css", body)
	}
}

func TestServeLocalAssetsRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	assetsDir := t.TempDir()
	req := httptest.NewRequest(http.MethodGet, "/assets/../../etc/passwd", nil)
	rec := httptest.NewRecorder()

	serveLocalAssets(assetsDir).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("serveLocalAssets().ServeHTTP(%q) status = %d, want %d", "/assets/../../etc/passwd", rec.Code, http.StatusForbidden)
	}
}

func TestNewServerReturnsNotFoundForMissingLocalAsset(t *testing.T) {
	t.Parallel()

	assetsDir := t.TempDir()
	req := httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil)
	rec := httptest.NewRecorder()

	newServer("", nil, Config{}, assetsDir).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("newServer().ServeHTTP(%q) status = %d, want %d", "/assets/missing.js", rec.Code, http.StatusNotFound)
	}
}

func TestNewServerReturnsNotFoundForLocalAssetsWhenAssetsDirUnset(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/assets/foo.js", nil)
	rec := httptest.NewRecorder()

	newServer("", nil, Config{}, "").ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("newServer().ServeHTTP(%q) status = %d, want %d", "/assets/foo.js", rec.Code, http.StatusNotFound)
	}
}

func TestLegacyHTMLAssetsRemoved(t *testing.T) {
	t.Parallel()

	testCases := []string{
		"assets/index.html",
		"assets/file.html",
	}

	for _, path := range testCases {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("os.Stat(%q) error = %v, want not exist", path, err)
			}
		})
	}
}

func TestNewServerServesFileHTML(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	writeTestFile(t, rootDir+"/docs/guide.md", "# guide")

	cfg := Config{
		Rules: []Rule{
			{
				Ext:    "md",
				URL:    "https://cdn.example.test/marked.js",
				Render: "return content",
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/file?path=docs/guide.md", nil)
	rec := httptest.NewRecorder()

	newServer(rootDir, nil, cfg, "").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("newServer().ServeHTTP(%q) status = %d, want %d", "/file", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("newServer().ServeHTTP(%q) content type = %q, want text/html", "/file", got)
	}
}

func TestFileHTMLLoadsScriptForMatchedRule(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Rules: []Rule{
			{
				Ext:    "md",
				URL:    "https://cdn.example.test/marked.js",
				Render: "return content",
			},
			{
				Ext:    "mmd",
				URL:    "https://cdn.example.test/mermaid.js",
				Render: "return content",
			},
		},
	}

	testCases := []struct {
		name       string
		filePath   string
		fileBody   string
		wantScript string
	}{
		{
			name:       "markdown",
			filePath:   "docs/guide.md",
			fileBody:   "# guide",
			wantScript: "https://cdn.example.test/marked.js",
		},
		{
			name:       "mermaid",
			filePath:   "docs/diagram.mmd",
			fileBody:   "graph TD;",
			wantScript: "https://cdn.example.test/mermaid.js",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDir := t.TempDir()
			writeTestFile(t, rootDir+"/"+tc.filePath, tc.fileBody)

			server := httptest.NewServer(newServer(rootDir, nil, cfg, ""))
			defer server.Close()

			// Verify the inlined script contains the expected import and no external scripts.
			resp, err := http.Get(server.URL + "/file?path=" + tc.filePath)
			if err != nil {
				t.Fatalf("HTTP GET failed: %v", err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			html := string(body)
			if !strings.Contains(html, `<script src="`+tc.wantScript+`"`) {
				t.Fatalf("HTML should have script src for URL: %q", tc.wantScript)
			}
			if !strings.Contains(html, `onerror="window.__jaqlomScriptLoadError='Failed to load script: `+tc.wantScript+`'"`) {
				t.Fatalf("HTML should have script load error handler for URL: %q", tc.wantScript)
			}
			if !strings.Contains(html, "<script>") {
				t.Fatalf("HTML missing inline <script> for render code")
			}
		})
	}
}

func TestFileHTMLMakesNoAPIRequests(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	writeTestFile(t, rootDir+"/docs/guide.md", "# guide")

	cfg := Config{
		Rules: []Rule{
			{
				Ext:    "md",
				URL:    "https://cdn.example.test/marked.js",
				CSS:    []string{"https://cdn.example.test/marked.css"},
				Style:  "body { margin: 0; }",
				Render: "return content",
			},
		},
	}

	server := httptest.NewServer(newServer(rootDir, nil, cfg, ""))
	defer server.Close()

	got := renderFileHTMLState(t, server.URL, "docs/guide.md", renderFileHTMLOptions{})
	if diff := cmp.Diff([]string{}, got.Requests); diff != "" {
		t.Fatalf("renderFileHTMLState(%q) requests mismatch (-want +got):\n%s", "docs/guide.md", diff)
	}
	if got.OutputHTML != "# guide" {
		t.Fatalf("renderFileHTMLState(%q) outputHTML = %q, want %q", "docs/guide.md", got.OutputHTML, "# guide")
	}
}

func TestFileHTMLRendersContentUsingRuleSnippet(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	writeTestFile(t, rootDir+"/docs/guide.md", "# guide")

	cfg := Config{
		Rules: []Rule{
			{
				Ext:    "md",
				URL:    "https://cdn.example.test/marked.js",
				Render: `return "<article>" + marked.parse(content).toUpperCase() + "</article>"`,
			},
		},
	}

	server := httptest.NewServer(newServer(rootDir, nil, cfg, ""))
	defer server.Close()

	got := renderFileHTMLState(t, server.URL, "docs/guide.md", renderFileHTMLOptions{})
	if got.StatusText != "" {
		t.Fatalf("renderFileHTMLState(%q) status = %q, want empty", "docs/guide.md", got.StatusText)
	}
	if got.OutputHTML != "<article># GUIDE</article>" {
		t.Fatalf("renderFileHTMLState(%q) outputHTML = %q, want %q", "docs/guide.md", got.OutputHTML, "<article># GUIDE</article>")
	}
}

func TestFileHTMLShowsMessageWhenPathMissing(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(newServer("", nil, Config{}, ""))
	defer server.Close()

	got := renderFileHTMLState(t, server.URL, "", renderFileHTMLOptions{OmitPath: true})
	if got.StatusText != "No path specified." {
		t.Fatalf("renderFileHTMLState(path missing) status = %q, want %q", got.StatusText, "No path specified.")
	}
	if diff := cmp.Diff([]string{}, got.Requests); diff != "" {
		t.Fatalf("renderFileHTMLState(path missing) requests mismatch (-want +got):\n%s", diff)
	}
}

func TestFileHTMLShowsMessageWhenRuleIsMissing(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	writeTestFile(t, rootDir+"/docs/guide.txt", "plain text")

	cfg := Config{
		Rules: []Rule{
			{
				Ext:    "md",
				URL:    "https://cdn.example.test/marked.js",
				Render: "return content",
			},
		},
	}

	server := httptest.NewServer(newServer(rootDir, nil, cfg, ""))
	defer server.Close()

	got := renderFileHTMLState(t, server.URL, "docs/guide.txt", renderFileHTMLOptions{})
	if got.StatusText != "No rule found for extension: txt" {
		t.Fatalf("renderFileHTMLState(rule missing) status = %q, want %q", got.StatusText, "No rule found for extension: txt")
	}
}

func TestFileHTMLReturnsNotFoundPageWhenFileIsMissing(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	cfg := Config{
		Rules: []Rule{{
			Ext:    "md",
			URL:    "https://cdn.example.test/marked.js",
			Render: "return content",
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/file?path=docs/missing.md", nil)
	rec := httptest.NewRecorder()

	newServer(rootDir, nil, cfg, "").ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("newServer().ServeHTTP(%q) status = %d, want %d", "/file?path=docs/missing.md", rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("newServer().ServeHTTP(%q) content type = %q, want text/html", "/file?path=docs/missing.md", got)
	}
	if !strings.Contains(rec.Body.String(), "File not found: docs/missing.md") {
		t.Fatalf("newServer().ServeHTTP(%q) body = %q, want missing-file message", "/file?path=docs/missing.md", rec.Body.String())
	}
}

func TestFileHTMLReturnsNotFoundPageWhenRuleIsMissing(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	writeTestFile(t, rootDir+"/docs/guide.txt", "plain text")

	cfg := Config{
		Rules: []Rule{{
			Ext:    "md",
			URL:    "https://cdn.example.test/marked.js",
			Render: "return content",
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/file?path=docs/guide.txt", nil)
	rec := httptest.NewRecorder()

	newServer(rootDir, nil, cfg, "").ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("newServer().ServeHTTP(%q) status = %d, want %d", "/file?path=docs/guide.txt", rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("newServer().ServeHTTP(%q) content type = %q, want text/html", "/file?path=docs/guide.txt", got)
	}
	if !strings.Contains(rec.Body.String(), "No rule found for extension: txt") {
		t.Fatalf("newServer().ServeHTTP(%q) body = %q, want rule-missing message", "/file?path=docs/guide.txt", rec.Body.String())
	}
}

func TestFileHTMLShowsMessageWhenScriptLoadFails(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	writeTestFile(t, rootDir+"/docs/guide.md", "# guide")

	cfg := Config{
		Rules: []Rule{
			{
				Ext:    "md",
				URL:    "https://cdn.example.test/marked.js",
				Render: "return content",
			},
		},
	}

	server := httptest.NewServer(newServer(rootDir, nil, cfg, ""))
	defer server.Close()

	got := renderFileHTMLState(t, server.URL, "docs/guide.md", renderFileHTMLOptions{FailScriptLoad: true})
	if got.StatusText != "Failed to load script: https://cdn.example.test/marked.js" {
		t.Fatalf("renderFileHTMLState(script failure) statusText = %q, want %q", got.StatusText, "Failed to load script: https://cdn.example.test/marked.js")
	}
}

func TestFileHTMLShowsMessageWhenRenderFails(t *testing.T) {
	t.Parallel()

	e2eRequireNode(t)

	rootDir := t.TempDir()
	writeTestFile(t, rootDir+"/docs/guide.md", "# guide")

	cfg := Config{
		Rules: []Rule{
			{
				Ext:    "md",
				URL:    "https://cdn.example.test/marked.js",
				Render: `throw new Error("boom")`,
			},
		},
	}

	server := httptest.NewServer(newServer(rootDir, nil, cfg, ""))
	defer server.Close()

	got := e2eRenderFileHTMLState(t, server.URL, "docs/guide.md", renderFileHTMLOptions{})
	if got.StatusText != "Render error: boom" {
		t.Fatalf("rendered page statusText = %q, want %q", got.StatusText, "Render error: boom")
	}
}

type renderedIndexHTML struct {
	Links []renderedLink `json:"links"`
}

type renderedFileState struct {
	LoadedScripts []string `json:"loadedScripts"`
	OutputHTML    string   `json:"outputHTML"`
	OutputText    string   `json:"outputText"`
	Requests      []string `json:"requests"`
	StatusText    string   `json:"statusText"`
}

type renderedLink struct {
	TextContent string `json:"textContent"`
	Href        string `json:"href"`
}

type renderFileHTMLOptions struct {
	FailScriptLoad bool
	OmitPath       bool
}

func renderIndexHTML(t *testing.T, baseURL string) renderedIndexHTML {
	t.Helper()

	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("io.ReadAll(/) returned error: %v", err)
	}

	matches := reServerIndexLink.FindAllStringSubmatch(string(body), -1)
	links := make([]renderedLink, 0, len(matches))
	for _, match := range matches {
		links = append(links, renderedLink{TextContent: match[2], Href: match[1]})
	}

	return renderedIndexHTML{Links: links}
}

func renderFileHTMLState(t *testing.T, baseURL, filePath string, options renderFileHTMLOptions) renderedFileState {
	t.Helper()

	e2eRequireNode(t)

	const script = `
class Element {
  constructor(tagName) {
    this.tagName = tagName;
    this.children = [];
    this._innerHTML = "";
    this._textContent = "";
  }

  appendChild(child) {
    this.children.push(child);
    return child;
  }

  append(child) {
    return this.appendChild(child);
  }

  get innerHTML() {
    return this._innerHTML;
  }

  set innerHTML(value) {
    this._innerHTML = String(value);
    this._textContent = String(value);
  }

  get textContent() {
    return this._textContent;
  }

  set textContent(value) {
    this._innerHTML = "";
    this._textContent = String(value);
  }
}

function extractInlineScript(html) {
  const match = html.match(/<script(?: type="module")?>([\s\S]*?)<\/script>\s*<\/body>/);
  if (!match) {
    throw new Error("rendered file page does not contain an executable inline script");
  }
  return match[1];
}

function extractContent(html) {
  const match = html.match(/<script id="jaqlom-content" type="application\/json">([\s\S]*?)<\/script>/);
  if (!match) {
    return null;
  }
  return match[1];
}

function installGlobals(urls) {
  for (const url of urls) {
    if (url.includes("marked")) {
      globalThis.marked = { parse(value) { return String(value); } };
    }
    if (url.includes("mermaid")) {
      globalThis.mermaid = {
        initialize() {},
        async render(_id, value) { return { svg: "<svg>" + String(value) + "</svg>" }; },
      };
    }
  }
}

async function main() {
  const baseURL = process.env.TEST_BASE_URL;
  const filePath = process.env.TEST_FILE_PATH;
  const omitPath = process.env.TEST_OMIT_PATH === "1";
  const pathSuffix = omitPath ? "" : "?path=" + encodeURIComponent(filePath);
  const realFetch = fetch;
  const html = await (await realFetch(new URL("/file" + pathSuffix, baseURL))).text();
  const inlineScript = extractInlineScript(html);
  const content = extractContent(html);
  const scriptMatches = Array.from(html.matchAll(/<script src="([^"]+)"([^>]*)><\/script>/g));
  const scriptURLs = scriptMatches.map((match) => match[1]);
  const scriptOnError = scriptMatches.length === 0 ? "" : ((scriptMatches[0][2] || "").match(/onerror="([^"]+)"/) || ["", ""])[1];
  const requests = [];
  const status = new Element("p");
  const output = new Element("main");

  globalThis.document = {
    createElement(tagName) {
      return new Element(tagName);
    },
    getElementById(id) {
      if (id === "jaqlom-content" && content !== null) {
        return { textContent: content };
      }
      return null;
    },
    head: new Element("head"),
    querySelector(selector) {
      if (selector === '[data-role="status"]') {
        return status;
      }
      if (selector === '[data-role="output"]' || selector === 'main') {
        return output;
      }
      throw new Error("unexpected selector: " + selector);
    },
  };
  globalThis.window = globalThis;
  globalThis.location = new URL("/file" + pathSuffix, baseURL);
  globalThis.fetch = (path) => {
    const url = new URL(path, baseURL);
    requests.push(url.pathname + url.search);
    return realFetch(url);
  };

  if (process.env.TEST_FAIL_SCRIPT_LOAD === "1") {
    if (scriptOnError) {
      eval(scriptOnError);
    }
  } else {
    installGlobals(scriptURLs);
  }

  eval(inlineScript);
  await Promise.resolve(globalThis.__jaqlomLoadPromise);

  process.stdout.write(JSON.stringify({
    loadedScripts: scriptURLs,
    outputHTML: output.innerHTML,
    outputText: output.textContent,
    requests,
    statusText: status.textContent,
  }));
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
`

	cmd := exec.Command("node", "-e", script)
	cmd.Env = append(
		os.Environ(),
		"TEST_BASE_URL="+baseURL,
		"TEST_FILE_PATH="+filePath,
		boolEnv("TEST_FAIL_SCRIPT_LOAD", options.FailScriptLoad),
		boolEnv("TEST_OMIT_PATH", options.OmitPath),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("renderFileHTMLState: node returned error: %v\n%s", err, output)
	}

	var got renderedFileState
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatalf("renderFileHTMLState: json.Unmarshal: %v\n%s", err, output)
	}
	return got
}

func boolEnv(name string, enabled bool) string {
	if enabled {
		return name + "=1"
	}
	return name + "=0"
}
