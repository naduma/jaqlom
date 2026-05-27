package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var (
	e2eBinOnce sync.Once
	e2eBinPath string
	e2eBinErr  error

	reListenPort = regexp.MustCompile(`http://localhost:(\d+)`)
	reIndexLink  = regexp.MustCompile(`<a href="([^"]+)">([^<]+)</a>`)
)

func e2eBin(t *testing.T) string {
	t.Helper()
	e2eBinOnce.Do(func() {
		e2eBinPath = filepath.Join(os.TempDir(), "jaqlom-e2e-test")
		out, err := exec.Command("go", "build", "-o", e2eBinPath, ".").CombinedOutput()
		if err != nil {
			e2eBinErr = fmt.Errorf("go build failed: %v\n%s", err, out)
		}
	})
	if e2eBinErr != nil {
		t.Fatalf("e2eBin: %v", e2eBinErr)
	}
	return e2eBinPath
}

func e2eStartServer(t *testing.T, rootDir string) string {
	t.Helper()
	bin := e2eBin(t)

	cmd := exec.Command(bin, "-port", "0", rootDir)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("e2eStartServer: StderrPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("e2eStartServer: %v", err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	// Read the actual bound port from the server's startup log line:
	// "listening on http://localhost:PORT"
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if m := reListenPort.FindStringSubmatch(line); m != nil {
			return "http://127.0.0.1:" + m[1]
		}
	}
	t.Fatalf("e2eStartServer: server did not log a ready message")
	return ""
}

// e2eRequireNode fails (not skips) when node is absent.
// All E2E tests that depend on node must call this.
func e2eRequireNode(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatalf("E2E tests require node: %v", err)
	}
}

// browserLink represents a link element extracted by the browser.
type browserLink struct {
	TextContent string `json:"textContent"`
	Href        string `json:"href"`
}

// browserFlowResult holds the results of a playwright browser flow test.
type browserFlowResult struct {
	IndexLinks []browserLink `json:"links"`
	OutputHTML string        `json:"outputHTML"`
	StatusText string        `json:"statusText"`
}

// runPlaywrightBrowserFlow navigates to baseURL via a real Chromium browser,
// collects the top-page file links, clicks through to filePath, waits for
// rendering to complete, and returns the observed state.
func runPlaywrightBrowserFlow(t *testing.T, baseURL, filePath string) browserFlowResult {
	t.Helper()
	e2eRequireNode(t)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("runPlaywrightBrowserFlow: os.Getwd: %v", err)
	}
	playwrightDir := filepath.Join(wd, "node_modules")
	if _, err := os.Stat(filepath.Join(playwrightDir, "playwright-chromium")); err != nil {
		t.Fatalf("runPlaywrightBrowserFlow: playwright-chromium not found in node_modules (run: npm install playwright-chromium): %v", err)
	}

	const script = `
const path = require("node:path");
const { chromium } = require(path.join(process.env.PLAYWRIGHT_DIR, "playwright-chromium"));

async function main() {
  const baseURL = process.env.TEST_BASE_URL;
  const filePath = process.env.TEST_FILE_PATH;

  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();

  await page.goto(baseURL + "/");
  await page.waitForSelector('[data-role="file-list"] a');

  const links = await page.$$eval(
    '[data-role="file-list"] a',
    els => els.map(a => ({ textContent: a.textContent, href: a.getAttribute("href") }))
  );

  // 2. Click the link, wait for navigation and rendering.
  const href = "/file?path=" + encodeURIComponent(filePath);
  await Promise.all([
    page.waitForNavigation({ waitUntil: "load" }),
    page.click('a[href="' + href + '"]'),
  ]);

  await page.waitForFunction(() => typeof window.__jaqlomLoadPromise !== "undefined");
  await page.evaluate(() => window.__jaqlomLoadPromise);

  const outputHTML = await page.$eval('[data-role="output"]', el => el.innerHTML);
  const statusText = await page.$eval('[data-role="status"]', el => el.textContent);

  await browser.close();
  process.stdout.write(JSON.stringify({ links, outputHTML, statusText }));
}

main().catch(e => { console.error(e.message || e); process.exit(1); });
`

	cmd := exec.Command("node", "-e", script)
	cmd.Env = append(
		os.Environ(),
		"PLAYWRIGHT_DIR="+playwrightDir,
		"TEST_BASE_URL="+baseURL,
		"TEST_FILE_PATH="+filePath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("runPlaywrightBrowserFlow: node returned error: %v\n%s", err, output)
	}

	var got browserFlowResult
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatalf("runPlaywrightBrowserFlow: json.Unmarshal: %v\n%s", err, output)
	}
	return got
}

// e2eRenderIndexHTML fetches index.html from the running server (not from local
// assets) and evaluates its inline script in a mock DOM environment.
func e2eRenderIndexHTML(t *testing.T, baseURL string) renderedIndexHTML {
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

	matches := reIndexLink.FindAllStringSubmatch(string(body), -1)
	links := make([]renderedLink, 0, len(matches))
	for _, match := range matches {
		links = append(links, renderedLink{TextContent: match[2], Href: match[1]})
	}
	return renderedIndexHTML{Links: links}
}

// e2eRenderFileHTMLState fetches file.html from the running server (not from
// local assets) and evaluates its inline script in a mock DOM environment.
func e2eRenderFileHTMLState(t *testing.T, baseURL, filePath string, options renderFileHTMLOptions) renderedFileState {
	t.Helper()
	return renderFileHTMLState(t, baseURL, filePath, options)
}

func TestE2EBrowserFlow(t *testing.T) {
	t.Parallel()

	// Mock CDN that serves an empty script so the real browser can load it.
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprintln(w, "// mock cdn script")
	}))
	defer cdn.Close()

	rootDir := t.TempDir()
	writeTestFile(t, filepath.Join(rootDir, "docs", "guide.md"), "# guide")
	writeTestFile(t, filepath.Join(rootDir, "jaqlom.json"), fmt.Sprintf(`{
  "rules": [{"ext": "md", "url": "%s/marked.js", "render": "return \"<p>\"+content+\"</p>\""}]
}`, cdn.URL))

	baseURL := e2eStartServer(t, rootDir)
	got := runPlaywrightBrowserFlow(t, baseURL, "docs/guide.md")

	wantLinks := []browserLink{
		{TextContent: "docs/guide.md", Href: "/file?path=docs%2Fguide.md"},
	}
	if diff := cmp.Diff(wantLinks, got.IndexLinks); diff != "" {
		t.Fatalf("browser flow top-page links mismatch (-want +got):\n%s", diff)
	}
	if got.OutputHTML != "<p># guide</p>" {
		t.Fatalf("browser flow detail page outputHTML = %q, want %q", got.OutputHTML, "<p># guide</p>")
	}
	if got.StatusText != "" {
		t.Fatalf("browser flow detail page statusText = %q, want empty", got.StatusText)
	}
}

func TestE2ETopPageNavigation(t *testing.T) {
	t.Parallel()
	e2eRequireNode(t)

	rootDir := t.TempDir()
	writeTestFile(t, filepath.Join(rootDir, "docs", "guide.md"), "# guide")
	writeTestFile(t, filepath.Join(rootDir, "jaqlom.json"), `{
  "rules": [{"ext": "md", "url": "https://cdn.example.test/marked.js", "render": "return content"}]
}`)

	baseURL := e2eStartServer(t, rootDir)

	t.Run("top page", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/")
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET / status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})

	t.Run("detail page", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/file?path=docs/guide.md")
		if err != nil {
			t.Fatalf("GET /file: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /file status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})

	// Uses e2eRenderIndexHTML: HTML is fetched from the running server, not local assets.
	t.Run("index html file links", func(t *testing.T) {
		got := e2eRenderIndexHTML(t, baseURL)
		want := renderedIndexHTML{
			Links: []renderedLink{
				{TextContent: "docs/guide.md", Href: "/file?path=docs%2Fguide.md"},
			},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("e2eRenderIndexHTML mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestE2EMultipleExtensionRendering(t *testing.T) {
	t.Parallel()
	e2eRequireNode(t)

	rootDir := t.TempDir()
	writeTestFile(t, filepath.Join(rootDir, "docs", "guide.md"), "# guide")
	writeTestFile(t, filepath.Join(rootDir, "docs", "diagram.mmd"), "graph TD;")
	writeTestFile(t, filepath.Join(rootDir, "docs", "guide.adoc"), "= Guide")
	writeTestFile(t, filepath.Join(rootDir, "docs", "graph.dot"), "digraph G {}")
	writeTestFile(t, filepath.Join(rootDir, "jaqlom.json"), `{
  "rules": [
    {"ext": "md",   "url": "https://cdn.example.test/marked.js",  "render": "return \"<p>\"+content+\"</p>\""},
    {"ext": "mmd",  "url": "https://cdn.example.test/mermaid.js", "render": "return \"<div class='mermaid'>\"+content+\"</div>\""},
    {"ext": "adoc", "render": "return \"<article>\"+content+\"</article>\""},
    {"ext": "dot",  "render": "return \"<pre class='graphviz'>\"+content+\"</pre>\""}
  ]
}`)

	baseURL := e2eStartServer(t, rootDir)

	tests := []struct {
		name     string
		filePath string
		wantHTML string
	}{
		{name: "markdown", filePath: "docs/guide.md", wantHTML: "<p># guide</p>"},
		{name: "mermaid", filePath: "docs/diagram.mmd", wantHTML: "<div class='mermaid'>graph TD;</div>"},
		{name: "asciidoc", filePath: "docs/guide.adoc", wantHTML: "<article>= Guide</article>"},
		{name: "graphviz", filePath: "docs/graph.dot", wantHTML: "<pre class='graphviz'>digraph G {}</pre>"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Uses e2eRenderFileHTMLState: HTML is fetched from the running server.
			got := e2eRenderFileHTMLState(t, baseURL, tc.filePath, renderFileHTMLOptions{})
			if got.StatusText != "" {
				t.Fatalf("e2eRenderFileHTMLState(%q) statusText = %q, want empty", tc.filePath, got.StatusText)
			}
			if diff := cmp.Diff([]string{}, got.Requests); diff != "" {
				t.Fatalf("e2eRenderFileHTMLState(%q) requests mismatch (-want +got):\n%s", tc.filePath, diff)
			}
			if got.OutputHTML != tc.wantHTML {
				t.Fatalf("e2eRenderFileHTMLState(%q) outputHTML = %q, want %q", tc.filePath, got.OutputHTML, tc.wantHTML)
			}
		})
	}
}

func TestE2EErrorMessages(t *testing.T) {
	t.Parallel()
	e2eRequireNode(t)

	rootDir := t.TempDir()
	writeTestFile(t, filepath.Join(rootDir, "docs", "guide.md"), "# guide")
	writeTestFile(t, filepath.Join(rootDir, "docs", "readme.txt"), "plain text")
	writeTestFile(t, filepath.Join(rootDir, "jaqlom.json"), `{
  "rules": [{"ext": "md", "url": "https://cdn.example.test/marked.js", "render": "return content"}]
}`)

	baseURL := e2eStartServer(t, rootDir)

	t.Run("path missing", func(t *testing.T) {
		got := e2eRenderFileHTMLState(t, baseURL, "", renderFileHTMLOptions{OmitPath: true})
		if got.StatusText != "No path specified." {
			t.Fatalf("e2eRenderFileHTMLState(path missing) statusText = %q, want %q", got.StatusText, "No path specified.")
		}
	})

	t.Run("rule missing", func(t *testing.T) {
		got := e2eRenderFileHTMLState(t, baseURL, "docs/readme.txt", renderFileHTMLOptions{})
		if got.StatusText != "No rule found for extension: txt" {
			t.Fatalf("e2eRenderFileHTMLState(rule missing) statusText = %q, want %q", got.StatusText, "No rule found for extension: txt")
		}
	})

	t.Run("file missing", func(t *testing.T) {
		got := e2eRenderFileHTMLState(t, baseURL, "docs/missing.md", renderFileHTMLOptions{})
		if got.StatusText != "File not found: docs/missing.md" {
			t.Fatalf("e2eRenderFileHTMLState(file missing) statusText = %q, want %q", got.StatusText, "File not found: docs/missing.md")
		}
	})

	t.Run("script load failure", func(t *testing.T) {
		got := e2eRenderFileHTMLState(t, baseURL, "docs/guide.md", renderFileHTMLOptions{FailScriptLoad: true})
		if got.StatusText != "Failed to load script: https://cdn.example.test/marked.js" {
			t.Fatalf("e2eRenderFileHTMLState(script failure) statusText = %q, want %q", got.StatusText, "Failed to load script: https://cdn.example.test/marked.js")
		}
	})
}

func TestE2ERenderError(t *testing.T) {
	t.Parallel()
	e2eRequireNode(t)

	rootDir := t.TempDir()
	writeTestFile(t, filepath.Join(rootDir, "docs", "guide.md"), "# guide")
	writeTestFile(t, filepath.Join(rootDir, "jaqlom.json"), `{
  "rules": [{"ext": "md", "url": "https://cdn.example.test/marked.js", "render": "throw new Error(\"boom\")"}]
}`)

	baseURL := e2eStartServer(t, rootDir)

	got := e2eRenderFileHTMLState(t, baseURL, "docs/guide.md", renderFileHTMLOptions{})
	if got.StatusText != "Render error: boom" {
		t.Fatalf("e2eRenderFileHTMLState(render error) statusText = %q, want %q", got.StatusText, "Render error: boom")
	}
}

// playwrightFilePageResult holds the result of navigating directly to a file page.
type playwrightFilePageResult struct {
	StatusText string   `json:"statusText"`
	OutputHTML string   `json:"outputHTML"`
	Requests   []string `json:"requests"`
}

// runPlaywrightFilePage navigates a real Chromium browser directly to pageURL,
// waits for __jaqlomLoadPromise to resolve, and returns the observed state.
func runPlaywrightFilePage(t *testing.T, pageURL string) playwrightFilePageResult {
	t.Helper()
	e2eRequireNode(t)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("runPlaywrightFilePage: os.Getwd: %v", err)
	}
	playwrightDir := filepath.Join(wd, "node_modules")
	if _, err := os.Stat(filepath.Join(playwrightDir, "playwright-chromium")); err != nil {
		t.Fatalf("runPlaywrightFilePage: playwright-chromium not found in node_modules (run: npm install playwright-chromium): %v", err)
	}

	const script = `
const path = require("node:path");
const { chromium } = require(path.join(process.env.PLAYWRIGHT_DIR, "playwright-chromium"));

async function main() {
  const pageURL = process.env.TEST_PAGE_URL;
  const pageOrigin = new URL(pageURL).origin;
  const requests = [];

  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();

  page.on("request", request => {
    const url = new URL(request.url());
    if (url.origin === pageOrigin) {
      requests.push(url.pathname + url.search);
    }
  });

  await page.goto(pageURL, { waitUntil: "load" });
  await page.waitForFunction(() => typeof window.__jaqlomLoadPromise !== "undefined");
  await page.evaluate(() => window.__jaqlomLoadPromise);

  const outputHTML = await page.$eval('[data-role="output"]', el => el.innerHTML);
  const statusText = await page.$eval('[data-role="status"]', el => el.textContent);

  await browser.close();
  process.stdout.write(JSON.stringify({ statusText, outputHTML, requests }));
}

main().catch(e => { console.error(e.message || e); process.exit(1); });
`

	cmd := exec.Command("node", "-e", script)
	cmd.Env = append(
		os.Environ(),
		"PLAYWRIGHT_DIR="+playwrightDir,
		"TEST_PAGE_URL="+pageURL,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("runPlaywrightFilePage: node returned error: %v\n%s", err, output)
	}

	var got playwrightFilePageResult
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatalf("runPlaywrightFilePage: json.Unmarshal: %v\n%s", err, output)
	}
	return got
}


func TestE2EBrowserMultipleExtensions(t *testing.T) {
	t.Parallel()

	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprintln(w, "// mock")
	}))
	t.Cleanup(cdn.Close)

	rootDir := t.TempDir()
	writeTestFile(t, filepath.Join(rootDir, "docs", "guide.md"), "# guide")
	writeTestFile(t, filepath.Join(rootDir, "docs", "diagram.mmd"), "graph TD;")
	writeTestFile(t, filepath.Join(rootDir, "docs", "guide.adoc"), "= Guide")
	writeTestFile(t, filepath.Join(rootDir, "docs", "graph.dot"), "digraph G {}")
	writeTestFile(t, filepath.Join(rootDir, "jaqlom.json"), fmt.Sprintf(`{
  "rules": [
    {"ext": "md",   "url": "%s/marked.js",  "render": "return \"<p>\"+content+\"</p>\""},
    {"ext": "mmd",  "url": "%s/mermaid.js", "render": "return \"<div class='mermaid'>\"+content+\"</div>\""},
    {"ext": "adoc", "render": "return \"<article>\"+content+\"</article>\""},
    {"ext": "dot",  "render": "return \"<pre class='graphviz'>\"+content+\"</pre>\""}
  ]
}`, cdn.URL, cdn.URL))

	baseURL := e2eStartServer(t, rootDir)

	tests := []struct {
		name     string
		filePath string
		wantHTML string
	}{
		{name: "markdown", filePath: "docs/guide.md", wantHTML: "<p># guide</p>"},
		{name: "mermaid", filePath: "docs/diagram.mmd", wantHTML: "<div class=\"mermaid\">graph TD;</div>"},
		{name: "asciidoc", filePath: "docs/guide.adoc", wantHTML: "<article>= Guide</article>"},
		{name: "graphviz", filePath: "docs/graph.dot", wantHTML: "<pre class=\"graphviz\">digraph G {}</pre>"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := runPlaywrightBrowserFlow(t, baseURL, tc.filePath)
			if got.StatusText != "" {
				t.Fatalf("runPlaywrightBrowserFlow(%q) statusText = %q, want empty", tc.filePath, got.StatusText)
			}
			if got.OutputHTML != tc.wantHTML {
				t.Fatalf("runPlaywrightBrowserFlow(%q) outputHTML = %q, want %q", tc.filePath, got.OutputHTML, tc.wantHTML)
			}
		})
	}
}

func TestE2EBrowserFilePagesUseSingleDocumentRequest(t *testing.T) {
	t.Parallel()

	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprintln(w, "// mock")
	}))
	t.Cleanup(cdn.Close)

	rootDir := t.TempDir()
	writeTestFile(t, filepath.Join(rootDir, "docs", "guide.md"), "# guide")
	writeTestFile(t, filepath.Join(rootDir, "docs", "diagram.mmd"), "graph TD;")
	writeTestFile(t, filepath.Join(rootDir, "docs", "guide.adoc"), "= Guide")
	writeTestFile(t, filepath.Join(rootDir, "docs", "graph.dot"), "digraph G {}")
	writeTestFile(t, filepath.Join(rootDir, "jaqlom.json"), fmt.Sprintf(`{
  "rules": [
    {"ext": "md",   "url": "%s/marked.js",  "render": "return \"<p>\"+content+\"</p>\""},
    {"ext": "mmd",  "url": "%s/mermaid.js", "render": "return \"<div class='mermaid'>\"+content+\"</div>\""},
    {"ext": "adoc", "render": "return \"<article>\"+content+\"</article>\""},
    {"ext": "dot",  "render": "return \"<pre class='graphviz'>\"+content+\"</pre>\""}
  ]
}`, cdn.URL, cdn.URL))

	baseURL := e2eStartServer(t, rootDir)

	for _, tc := range []struct {
		name     string
		filePath string
	}{
		{name: "markdown", filePath: "docs/guide.md"},
		{name: "mermaid", filePath: "docs/diagram.mmd"},
		{name: "asciidoc", filePath: "docs/guide.adoc"},
		{name: "graphviz", filePath: "docs/graph.dot"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := runPlaywrightFilePage(t, baseURL+"/file?path="+url.QueryEscape(tc.filePath))
			want := []string{"/file?path=" + url.QueryEscape(tc.filePath)}
			if diff := cmp.Diff(want, got.Requests); diff != "" {
				t.Fatalf("runPlaywrightFilePage(%q) requests mismatch (-want +got):\n%s", tc.filePath, diff)
			}
		})
	}
}

func TestE2EBrowserErrorMessages(t *testing.T) {
	t.Parallel()

	failCDN := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer failCDN.Close()

	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprintln(w, "// mock")
	}))
	defer cdn.Close()

	rootDir := t.TempDir()
	writeTestFile(t, filepath.Join(rootDir, "docs", "guide.md"), "# guide")
	writeTestFile(t, filepath.Join(rootDir, "docs", "readme.txt"), "plain text")
	writeTestFile(t, filepath.Join(rootDir, "docs", "boom.err"), "content")
	writeTestFile(t, filepath.Join(rootDir, "jaqlom.json"), fmt.Sprintf(`{
  "rules": [
    {"ext": "md",  "url": "%s/marked.js", "render": "return content"},
    {"ext": "err", "url": "%s/error.js",  "render": "throw new Error(\"boom\")"}
  ]
}`, failCDN.URL, cdn.URL))

	baseURL := e2eStartServer(t, rootDir)

	t.Run("path missing", func(t *testing.T) {
		got := runPlaywrightFilePage(t, baseURL+"/file")
		if got.StatusText != "No path specified." {
			t.Fatalf("runPlaywrightFilePage(path missing) statusText = %q, want %q", got.StatusText, "No path specified.")
		}
	})

	t.Run("rule missing", func(t *testing.T) {
		got := runPlaywrightFilePage(t, baseURL+"/file?path=docs%2Freadme.txt")
		if got.StatusText != "No rule found for extension: txt" {
			t.Fatalf("runPlaywrightFilePage(rule missing) statusText = %q, want %q", got.StatusText, "No rule found for extension: txt")
		}
	})

	t.Run("file missing", func(t *testing.T) {
		got := runPlaywrightFilePage(t, baseURL+"/file?path=docs%2Fmissing.md")
		if got.StatusText != "File not found: docs/missing.md" {
			t.Fatalf("runPlaywrightFilePage(file missing) statusText = %q, want %q", got.StatusText, "File not found: docs/missing.md")
		}
	})

	t.Run("script load failure", func(t *testing.T) {
		got := runPlaywrightFilePage(t, baseURL+"/file?path=docs%2Fguide.md")
		want := "Failed to load script: " + failCDN.URL + "/marked.js"
		if got.StatusText != want {
			t.Fatalf("runPlaywrightFilePage(script failure) statusText = %q, want %q", got.StatusText, want)
		}
	})

	t.Run("render error", func(t *testing.T) {
		got := runPlaywrightFilePage(t, baseURL+"/file?path=docs%2Fboom.err")
		if got.StatusText != "Render error: boom" {
			t.Fatalf("runPlaywrightFilePage(render error) statusText = %q, want %q", got.StatusText, "Render error: boom")
		}
	})
}

