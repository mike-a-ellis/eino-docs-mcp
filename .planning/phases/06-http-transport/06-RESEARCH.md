# Phase 6: HTTP Transport - Research

**Researched:** 2026-01-26
**Domain:** MCP Go SDK HTTP Transport (Streamable HTTP)
**Confidence:** HIGH

## Summary

The MCP Go SDK (v1.2.0) provides first-class support for HTTP transport through the **Streamable HTTP** transport mechanism. This is the official, modern transport for production MCP servers and replaces the older HTTP+SSE approach from protocol version 2024-11-05.

The current implementation already has:
- An MCP server built with the Go SDK using stdio transport
- An HTTP health endpoint on port 8080
- Deployment infrastructure on Fly.io with proper health checks
- Server running in SERVER_MODE=true to keep the process alive

To add HTTP transport, we need to:
1. Create a `StreamableHTTPHandler` that wraps the existing MCP server
2. Expose the MCP endpoint on the same HTTP server (port 8080) alongside the health endpoint
3. Support both stateless (for simple tool calls) and stateful (with session management) modes
4. Keep stdio transport working for local development (separate run mode)

**Primary recommendation:** Use `mcp.NewStreamableHTTPHandler` with the existing server instance, enable session management with secure session IDs, and deploy on the existing `/mcp` endpoint at port 8080 alongside the `/health` endpoint.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/modelcontextprotocol/go-sdk | v1.2.0+ | MCP protocol implementation | Official Go SDK from MCP team, includes StreamableHTTPHandler |
| net/http (stdlib) | Go 1.24 | HTTP server | Standard library, no external dependencies needed |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/rs/cors | Latest | CORS middleware | If custom CORS policies needed beyond basic Origin validation |
| github.com/modelcontextprotocol/go-sdk/auth | v1.2.0+ | OAuth 2.1 authentication | For production deployments requiring authenticated access |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| StreamableHTTPHandler | Custom HTTP handler | StreamableHTTPHandler handles SSE, session management, resumability - custom would require reimplementing the entire transport spec |
| Stateful sessions | Stateless mode only | Stateless is simpler but prevents server-to-client requests and doesn't support stream resumption |
| Single /mcp endpoint | Separate SSE and POST endpoints | Old HTTP+SSE pattern (2024-11-05 spec), deprecated in favor of unified endpoint |

**Installation:**
```bash
# Already in go.mod
github.com/modelcontextprotocol/go-sdk v1.2.0
```

## Architecture Patterns

### Recommended Project Structure
```
cmd/mcp-server/
├── main.go              # Entry point with transport selection
internal/mcp/
├── server.go            # MCP server creation and tool registration
├── handlers.go          # Tool handler implementations
├── health.go            # Health check handler
└── transport.go         # NEW: HTTP transport setup (StreamableHTTPHandler)
```

### Pattern 1: Dual Transport Mode

**What:** Server supports both stdio (for local development) and HTTP (for production) via command-line flag or environment variable

**When to use:** When the same MCP server needs to run locally (Claude Desktop) and remotely (production deployment)

**Example:**
```go
// Source: Based on examples/server/everything/main.go
var httpAddr = flag.String("http", "", "if set, use streamable HTTP at this address")

func main() {
    server := mcp.NewServer(&mcp.Implementation{
        Name: "eino-documentation-server",
        Version: "v0.1.0",
    }, nil)

    // Register tools...

    if *httpAddr != "" {
        // HTTP mode - for production
        handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
            return server
        }, nil)
        log.Fatal(http.ListenAndServe(*httpAddr, handler))
    } else {
        // Stdio mode - for local development
        if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
            log.Fatal(err)
        }
    }
}
```

### Pattern 2: Shared MCP Server with Multiple Handlers

**What:** Create one MCP server instance, share it across HTTP and health endpoints

**When to use:** When you need both MCP protocol endpoint and health/monitoring endpoints on same port

**Example:**
```go
// Source: Current implementation + SDK patterns
func main() {
    // Create shared MCP server
    server := mcpserver.NewServer(&mcpserver.Config{
        Storage:  store,
        Embedder: embedder,
        GitHub:   ghClient,
    })

    // Setup HTTP mux
    mux := http.NewServeMux()

    // Health endpoint (existing)
    healthHandler := mcpserver.NewHealthHandler(store)
    mux.HandleFunc("/health", healthHandler)

    // MCP endpoint (new)
    mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
        return server.server // Return underlying mcp.Server
    }, &mcp.StreamableHTTPOptions{
        // Optional: Enable session management
        // Optional: EventStore for resumability
    })
    mux.Handle("/mcp", mcpHandler)

    // Single HTTP server on port 8080
    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

### Pattern 3: Session Management for Stateful Servers

**What:** Use session IDs to maintain state across HTTP requests, essential for server-to-client requests

**When to use:** When server needs to make requests back to client (sampling, elicitation) or maintain conversation context

**Example:**
```go
// Source: https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/docs/protocol.md
handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
    return server
}, &mcp.StreamableHTTPOptions{
    // Session ID generation - SDK handles this by default with crypto-secure UUIDs
    // EventStore: memoryStore, // Optional: for resumability after disconnection
    // Stateless: false,        // Default: stateful sessions enabled
})

// Server automatically includes Mcp-Session-Id in InitializeResponse
// Clients must include it in subsequent requests
// SDK validates session IDs automatically
```

### Pattern 4: Stateless Mode for Simple Tool Servers

**What:** Disable session management for simple request/response servers that don't need state

**When to use:** When server only responds to tool calls and never initiates client requests

**Example:**
```go
// Source: https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/mcp/streamable_server.go
handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
    return server
}, &mcp.StreamableHTTPOptions{
    Stateless: true, // No session validation, temporary session per request
    JSONResponse: true, // Return single JSON instead of SSE stream (simpler)
})

// Recommended for: Simple tool servers, serverless deployments, horizontal scaling
// Limitation: Cannot make server-to-client requests (sampling, elicitation)
```

### Anti-Patterns to Avoid

- **Using deprecated HTTP+SSE transport:** The 2024-11-05 spec's separate SSE and POST endpoints are deprecated; use unified Streamable HTTP endpoint instead
- **Hardcoding session IDs:** Let SDK generate cryptographically secure session IDs; custom IDs risk security issues
- **Ignoring Origin validation:** Production HTTP servers MUST validate Origin header to prevent DNS rebinding attacks
- **Running HTTP transport on stdio:** Don't try to mix transports; use flag/env to select one
- **Binding to 0.0.0.0 for local development:** For local-only servers, bind to 127.0.0.1 to prevent network exposure

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Session management | Custom session ID tracking with maps | StreamableHTTPHandler with default options | SDK handles secure ID generation, validation, cleanup, timeout, and Mcp-Session-Id header management |
| SSE streaming | Custom Server-Sent Events implementation | StreamableHTTPHandler | Handles SSE formatting, event IDs, reconnection, resumability, and multiple concurrent streams |
| Message routing | Custom request/response correlation | StreamableHTTPHandler | Tracks which stream handles which request, routes responses correctly, handles batching |
| Transport abstraction | Custom interface for stdio vs HTTP | mcp.Transport interface | SDK provides StdioTransport, StreamableServerTransport, and connection management |
| Authentication middleware | Custom bearer token verification | github.com/modelcontextprotocol/go-sdk/auth | Provides RequireBearerToken with scope validation, expiration checks, TokenInfo extraction |
| CORS handling | Manual header management | auth.ProtectedResourceMetadataHandler or rs/cors | Handles preflight, credentials, exposed headers for Mcp-Session-Id |

**Key insight:** The Streamable HTTP transport spec is complex (session management, SSE, resumability, multiple concurrent streams, POST/GET/DELETE methods, JSON vs SSE responses). The SDK implementation is battle-tested and handles edge cases. Custom implementations will be buggy.

## Common Pitfalls

### Pitfall 1: Forgetting to Validate Origin Header

**What goes wrong:** DNS rebinding attacks where malicious websites can make requests to local MCP servers

**Why it happens:** Browsers don't enforce same-origin policy for localhost servers unless Origin header is validated

**How to avoid:**
```go
// In production HTTP handler wrapper
func validateOrigin(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        origin := r.Header.Get("Origin")
        if origin != "" {
            // Validate against allowed origins
            if !isAllowedOrigin(origin) {
                http.Error(w, "Forbidden origin", http.StatusForbidden)
                return
            }
        }
        next.ServeHTTP(w, r)
    })
}
```

**Warning signs:** No Origin header validation in HTTP handler code

### Pitfall 2: Using Stateful Mode Without EventStore

**What goes wrong:** Client disconnections lose in-flight messages, no resumability, poor user experience

**Why it happens:** Default StreamableHTTPHandler doesn't persist events for resumption

**How to avoid:**
- For production: Implement custom EventStore backed by database/Redis
- For testing: Use mcp.MemoryEventStore (but beware unbounded memory growth)
- For simple servers: Use Stateless mode instead

**Warning signs:** Users reporting lost responses after network blips, no Last-Event-ID support

### Pitfall 3: Not Exposing Mcp-Session-Id in CORS

**What goes wrong:** Browser clients can't read the session ID from response headers

**Why it happens:** CORS doesn't expose custom headers by default

**How to avoid:**
```go
// If using custom CORS middleware
c := cors.New(cors.Options{
    AllowedOrigins: []string{"https://example.com"},
    ExposedHeaders: []string{"Mcp-Session-Id"}, // Essential!
    AllowCredentials: true,
})
```

**Warning signs:** Browser console shows session ID header but client code can't access it

### Pitfall 4: Sharing Server Instance Across Concurrent Requests Unsafely

**What goes wrong:** Race conditions if server instance modifies shared state

**Why it happens:** StreamableHTTPHandler's function receives *http.Request and returns *mcp.Server, can be called concurrently

**How to avoid:**
```go
// Safe: Return same immutable server instance
handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
    return server // server doesn't modify internal state during requests
}, nil)

// Unsafe: Modifying server state based on request
handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
    server.someState = r.Header.Get("X-Custom") // RACE CONDITION
    return server
}, nil)
```

**Warning signs:** Intermittent test failures, data races detected by go race detector

### Pitfall 5: Forgetting to Set SERVER_MODE When Deploying

**What goes wrong:** Container exits immediately after starting because stdio server exits when no stdin

**Why it happens:** Default server.Run() blocks on stdin, which doesn't exist in containerized HTTP mode

**How to avoid:**
```go
// Check environment to determine behavior
serverMode := os.Getenv("SERVER_MODE") == "true"

if serverMode {
    // HTTP mode - start HTTP server and block
    log.Fatal(http.ListenAndServe(":8080", handler))
} else {
    // Stdio mode - run MCP on stdin/stdout
    if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
        log.Fatal(err)
    }
}
```

**Warning signs:** Container starts then immediately exits in production, works locally with stdio

## Code Examples

Verified patterns from official sources:

### Basic HTTP Transport Setup

```go
// Source: https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/examples/http/main.go
func runServer(addr string) {
    // Create MCP server with tools
    server := mcp.NewServer(&mcp.Implementation{
        Name:    "time-server",
        Version: "1.0.0",
    }, nil)

    // Register tools
    mcp.AddTool(server, &mcp.Tool{
        Name:        "cityTime",
        Description: "Get the current time in NYC, SF, or Boston",
    }, getTime)

    // Create streamable HTTP handler
    handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
        return server
    }, nil)

    log.Printf("MCP server listening on %s", addr)
    if err := http.ListenAndServe(addr, handler); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}
```

### Multiple Endpoints on Same Port

```go
// Source: Derived from SDK patterns + current implementation
func main() {
    // Initialize dependencies
    store := initStorage()
    server := createMCPServer(store)

    // Create HTTP mux
    mux := http.NewServeMux()

    // Health endpoint
    mux.HandleFunc("/health", healthHandler(store))

    // MCP endpoint
    mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
        return server
    }, nil)
    mux.Handle("/mcp", mcpHandler)

    // Single server on port 8080
    http.ListenAndServe(":8080", mux)
}
```

### Adding Request Logging Middleware

```go
// Source: https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/examples/http/main.go
func createLoggingMiddleware() mcp.ReceivingMiddleware {
    return func(next mcp.HandlerFunc) mcp.HandlerFunc {
        return func(ctx context.Context, req *jsonrpc.JSONRPCRequest) (*jsonrpc.JSONRPCResponse, error) {
            log.Printf("Received: %s", req.Method)
            resp, err := next(ctx, req)
            if err != nil {
                log.Printf("Error: %v", err)
            }
            return resp, err
        }
    }
}

server.AddReceivingMiddleware(createLoggingMiddleware())
```

### Session Management with Security

```go
// Source: https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/docs/protocol.md + auth docs
handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
    return server
}, &mcp.StreamableHTTPOptions{
    // SDK generates cryptographically secure session IDs by default
    // For custom session ID generation (advanced):
    // GetSessionID: func() string {
    //     return crypto/rand.Text(...) // Go 1.24+
    // },
})

// Session IDs are:
// - Globally unique across all sessions
// - Cryptographically secure (prevents guessing)
// - Bound to user ID when using auth middleware (prevents hijacking)
```

### Authentication Middleware

```go
// Source: https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/examples/server/auth-middleware/README.md
import "github.com/modelcontextprotocol/go-sdk/auth"

// Create token verifier
jwtVerifier := func(ctx context.Context, token string) (*auth.TokenInfo, error) {
    // Verify JWT, extract claims
    return &auth.TokenInfo{
        UserID: "user123",
        Scopes: []string{"read", "write"},
        ExpiresAt: time.Now().Add(1 * time.Hour),
    }, nil
}

// Create auth middleware
authMiddleware := auth.RequireBearerToken(jwtVerifier, &auth.RequireBearerTokenOptions{
    Scopes: []string{"read"}, // Required scopes
    ResourceMetadataURL: "https://example.com/.well-known/oauth-protected-resource",
})

// Wrap MCP handler
mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
    return server
}, nil)
authenticatedHandler := authMiddleware(mcpHandler)

// In tool handlers, access token info
func myTool(ctx context.Context, req *mcp.CallToolRequest, args Args) (*mcp.CallToolResult, any, error) {
    userInfo := req.Extra.TokenInfo
    log.Printf("User %s called tool", userInfo.UserID)
    // ...
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| HTTP+SSE transport (separate endpoints) | Streamable HTTP (unified endpoint) | 2025-03-26 spec | Single /mcp endpoint handles POST and GET, simpler client implementation |
| Custom session management | Built-in Mcp-Session-Id header | 2025-03-26 spec | Standardized session tracking, automatic validation |
| SSE only for server-to-client | SSE for all responses (optional) | 2025-03-26 spec | Supports streaming multiple responses, server-initiated messages during request handling |
| Stateful by default | Stateless mode available | Recent SDK addition | Better horizontal scaling for simple tool servers |
| Basic auth or API keys | OAuth 2.1 standardization | 2026 best practices | Better security, token expiration, scope-based access control |

**Deprecated/outdated:**
- **HTTP+SSE transport (2024-11-05 spec):** Replaced by Streamable HTTP; clients should POST to single endpoint, not separate /sse and /message endpoints
- **Legacy SSE transport:** Old event format; use Streamable transport events instead
- **No session management:** Modern servers should use session IDs for multi-request conversations
- **Embedding credentials in URL:** Use Authorization header with bearer tokens instead

## Open Questions

Things that couldn't be fully resolved:

1. **EventStore implementation for production**
   - What we know: SDK provides MemoryEventStore (unbounded growth, single-machine), production needs persistent store
   - What's unclear: Best practices for EventStore backed by Redis/PostgreSQL, schema design, cleanup strategy
   - Recommendation: Start with stateless mode or MemoryEventStore, implement custom EventStore if resumability becomes critical

2. **CORS policy for Claude Desktop and web clients**
   - What we know: Must validate Origin header, expose Mcp-Session-Id, handle preflight
   - What's unclear: Exact origins Claude Desktop uses, whether to allow credentials, specific allowed methods
   - Recommendation: Start with permissive policy (all origins for public MCP server), tighten after testing with real clients

3. **Monitoring and observability for HTTP transport**
   - What we know: Should track session count, request latency, tool call success rate
   - What's unclear: Whether SDK provides metrics hooks, best way to instrument StreamableHTTPHandler
   - Recommendation: Use HTTP middleware for request logging, add custom metrics in tool handlers

4. **Session timeout and cleanup**
   - What we know: Server MAY terminate session at any time, responds with 404
   - What's unclear: Recommended timeout duration, cleanup strategy for abandoned sessions
   - Recommendation: SDK handles this internally; rely on default behavior unless specific requirements emerge

## Sources

### Primary (HIGH confidence)
- [MCP Go SDK v1.2.0](https://github.com/modelcontextprotocol/go-sdk) - Official SDK implementation
- [MCP Specification 2025-03-26 - Transports](https://modelcontextprotocol.io/specification/2025-03-26/basic/transports) - Protocol specification
- [SDK Protocol Documentation](https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/docs/protocol.md) - Streamable HTTP transport details
- [HTTP Example](https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/examples/http/main.go) - Official example code
- [Auth Middleware Example](https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/examples/server/auth-middleware/README.md) - Authentication patterns
- [Streamable Server Source](https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/mcp/streamable_server.go) - Implementation details

### Secondary (MEDIUM confidence)
- [Building Custom Connectors via Remote MCP Servers](https://support.claude.com/en/articles/11503834-building-custom-connectors-via-remote-mcp-servers) - Claude Desktop integration
- [MCP Streaming: Running MCP Servers Over the Network](https://medium.com/@sureshddm/mcp-streaming-running-mcp-servers-over-the-network-657b2f9c89a9) - Production deployment patterns
- [How to build secure and scalable remote MCP servers](https://github.blog/ai-and-ml/generative-ai/how-to-build-secure-and-scalable-remote-mcp-servers/) - GitHub blog on security
- [MCP Server Security Best Practices](https://www.truefoundry.com/blog/mcp-server-security-best-practices) - Security checklist
- [MCP Server Best Practices for 2026](https://www.cdata.com/blog/mcp-server-best-practices-2026) - Current best practices

### Tertiary (LOW confidence)
- [SEP-1442: Make MCP Stateless](https://github.com/modelcontextprotocol/modelcontextprotocol/issues/1442) - Future direction, not yet finalized
- [Exploring the Future of MCP Transports](http://blog.modelcontextprotocol.io/posts/2025-12-19-mcp-transport-future/) - Proposed changes, not current spec

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Official SDK with clear StreamableHTTPHandler API, well-documented examples
- Architecture: HIGH - Multiple official examples demonstrating patterns, spec defines exact behavior
- Pitfalls: MEDIUM - Common issues derived from spec requirements and SDK design, verified with examples

**Research date:** 2026-01-26
**Valid until:** 2026-03-01 (30 days - stable SDK, but rapid ecosystem development around MCP)

**Key decision factors for planning:**
1. Use StreamableHTTPHandler - it's the official, complete implementation
2. Enable session management (stateful mode) - needed for future server-to-client requests
3. Keep stdio transport for local development - dual-mode server is standard pattern
4. Start without authentication - add OAuth later as separate enhancement
5. Use MemoryEventStore or no EventStore initially - custom persistent store is complex
