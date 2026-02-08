package mcp

import "net/http"

const landingHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Eino Docs MCP Server</title>
<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: #0f172a; color: #e2e8f0; min-height: 100vh; display: flex; align-items: center; justify-content: center; }
  .card { max-width: 600px; width: 90%; background: #1e293b; border-radius: 12px; padding: 2.5rem; box-shadow: 0 25px 50px rgba(0,0,0,0.4); }
  h1 { font-size: 1.75rem; margin-bottom: 0.5rem; color: #f8fafc; }
  .subtitle { color: #94a3b8; margin-bottom: 1.75rem; }
  .section { margin-bottom: 1.5rem; }
  .section-title { font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.1em; color: #64748b; margin-bottom: 0.5rem; }
  a { color: #38bdf8; text-decoration: none; }
  a:hover { text-decoration: underline; }
  .links { display: flex; gap: 1.5rem; flex-wrap: wrap; }
  pre { background: #0f172a; border: 1px solid #334155; border-radius: 8px; padding: 1rem; overflow-x: auto; font-size: 0.85rem; line-height: 1.5; color: #e2e8f0; }
  code { font-family: "SF Mono", "Fira Code", "Fira Mono", Menlo, monospace; }
  .status { display: inline-block; width: 8px; height: 8px; background: #22c55e; border-radius: 50%; margin-right: 0.5rem; }
  .endpoint { font-family: "SF Mono", monospace; font-size: 0.9rem; color: #a5b4fc; }
</style>
</head>
<body>
<div class="card">
  <h1>Eino Docs MCP Server</h1>
  <p class="subtitle">Semantic search over <a href="https://www.cloudwego.io/docs/eino/">EINO framework</a> documentation via the Model Context Protocol.</p>

  <div class="section">
    <div class="section-title">Add to Claude Code</div>
    <pre><code>claude mcp add eino-docs --transport streamable-http https://eino-docs-mcp.fly.dev/mcp</code></pre>
  </div>

  <div class="section">
    <div class="section-title">Endpoints</div>
    <p><span class="status"></span><a href="/mcp" class="endpoint">/mcp</a> &mdash; MCP Streamable HTTP</p>
    <p><span class="status"></span><a href="/health" class="endpoint">/health</a> &mdash; Health check</p>
  </div>

  <div class="section">
    <div class="section-title">Links</div>
    <div class="links">
      <a href="https://github.com/mike-a-ellis/eino-docs-mcp">GitHub</a>
      <a href="https://www.linkedin.com/in/mike-a-ellis/">Mike Ellis on LinkedIn</a>
    </div>
  </div>
</div>
</body>
</html>`

// NewLandingHandler returns an HTTP handler that serves the landing page at /.
func NewLandingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(landingHTML))
	}
}
