<objective>
Update all references from "EINO documentation" (and variants like "EINO Documentation") to "Eino User Manual Documentation" across the source code, configuration files, and README. This makes the server's self-description more specific about what kind of documentation it serves — it's the Eino User Manual, not API docs or tutorials.
</objective>

<context>
This is an MCP server that serves Eino framework documentation to AI agents. The tool descriptions in the MCP server are how Claude and other AI clients discover what the server does. Making these descriptions more specific improves tool selection accuracy.

Only modify **source code**, **config files**, and **README.md**. Do NOT modify files under `.planning/` or `.prompts/` — those are historical records.
</context>

<requirements>
1. Read each file listed below and update "EINO documentation" / "EINO Documentation" references to use "Eino User Manual Documentation" (or contextually appropriate casing):

**Go source files:**
- `internal/mcp/server.go` — Tool descriptions (lines ~39, 49, 54). These are the most important since they're what AI clients read.
- `internal/mcp/types.go` — Package comment (line 1)
- `cmd/mcp-server/main.go` — Package comment (line 1) and log message (line ~100)
- `cmd/sync/main.go` — Package comment (line 1), cobra command Short/Long descriptions (lines ~24-25), and long description text (line ~36)

**Config files:**
- `fly.toml` — Comment on line 1

**Documentation:**
- `README.md` — Title (line 1), tool description examples (line ~204), API reference text (line ~276), and closing text (line ~503)

2. Preserve the exact surrounding sentence structure. Only change the "EINO documentation" / "EINO Documentation" phrase itself.

3. For the MCP server name in `server.go` (`Implementation.Name`), update from `"eino-documentation-server"` to `"eino-user-manual-server"` to match.

4. Keep casing contextually appropriate:
   - In titles/headings: "Eino User Manual Documentation"
   - In tool descriptions and prose: "Eino User Manual documentation"
   - In code identifiers (like server name): lowercase hyphenated
</requirements>

<verification>
After making changes:
1. Run `go build ./...` to verify no compilation errors
2. Grep for remaining "EINO documentation" (case-insensitive) in `.go`, `.toml`, and `README.md` files to confirm none were missed
3. Grep for the new "Eino User Manual" string to confirm replacements landed
</verification>

<success_criteria>
- All tool Description strings in server.go reference "Eino User Manual documentation"
- Server Implementation.Name updated to "eino-user-manual-server"
- All Go source package comments updated
- README.md references updated
- fly.toml comment updated
- `go build ./...` passes
- No remaining "EINO documentation" references in source/config/README files
</success_criteria>
