# repair-grpc

Transparent HTTP proxy that repairs OpenAI-compatible tool-call messages before
they reach LLM providers. Fixes [opencode issue #24090](https://github.com/opencode-ai/opencode/issues/24090).

## Why

Opencode's `@ai-sdk/openai-compatible` adapter drops `tool_calls` from assistant
messages during history replay. Tool results referencing those calls become
orphaned, causing two errors from downstream providers:

| Provider | Error | What's broken |
|----------|-------|---------------|
| **MiniMax** | `tool result's tool id() not found (2013)` | Assistant message is missing `tool_calls` array |
| **DeepSeek** | `messages[N]: missing field 'tool_call_id'` | Tool message is missing `tool_call_id` field |

The repair proxy sits between clients and the model gateway, intercepting
`POST /v1/chat/completions` requests and reconstructing missing fields before
forwarding.

## How it works

1. **Request intercept:** proxy receives `POST /v1/chat/completions`, reads
   the full JSON body.
2. **Repair pass:** walks the `messages` array. For each assistant message
   followed by tool results:
   - Adds missing `tool_call_id` to tool messages (DeepSeek case)
   - Grafts matching `tool_calls` entries into the preceding assistant
     message (MiniMax case), using a cache populated from previous responses
     or falling back to synthetic `{name:"unknown", arguments:"{}"}` entries.
3. **Response cache:** on upstream responses, extracts `tool_calls`
   definitions (id, function name, arguments) into an in-memory cache so
   subsequent repairs use real function names.
4. **Forward:** passes the repaired body upstream. All other requests are
   forwarded transparently.

The repair is **idempotent** — already-correct messages pass through unchanged.

## Deployment modes

Set via `MODE` env var:

| Mode | Port | Protocol | Use case |
|------|------|----------|----------|
| `proxy` | 8080 | HTTP | Reverse proxy in front of the ai Gateway. Transparent to clients. |
| `webhook` | 8080 | HTTP | Prompt guard webhook (not used — guardrail API strips tool fields). |
| `extproc` | 4444 | gRPC | Envoy ExternalProcessor (not used — `StreamedBodyResponse` is WIP in Envoy). |
| `both` | both | both | All modes simultaneously. |

### Proxy mode (production)

```
clients → https://gateway.goyangi.io → envoy-internal LB (TLS term)
  → HTTPRoute → repair-grpc:8080
  → repair body
  → https://agentgateway-proxy:443 → ai Gateway → model backends
```

Env: `MODE=proxy`, `UPSTREAM=https://agentgateway-proxy.network.svc:443`,
`PROXY_ADDR=0.0.0.0:8080`, `METRICS_ADDR=0.0.0.0:9090`

TLS between the proxy and ai Gateway is skipped (`InsecureSkipVerify`) because
the internal Service name doesn't match the `*.goyangi.io` certificate SAN.

## Metrics

`GET :9090/metrics` exposes Prometheus-formatted metrics:

| Metric | Description |
|--------|-------------|
| `repair_requests_total` | Intercepted chat completions |
| `repair_tool_calls_grafted{source}` | Grafted `tool_calls` entries (cache/synthetic) |
| `repair_tool_ids_added` | Synthetic `tool_call_id` values added |
| `repair_cache_hits` / `repair_cache_misses` | Cache performance |
| `repair_body_size_bytes{direction}` | Request/response body sizes |
| `repair_duration_seconds{type}` | Repair/cache processing latency |

## Limits

- **MiniMax body size:** returns misleading 2013 error for bodies above ~96KB.
  Not a tool-call issue — harmless for normal requests.
- **DeepSeek tool sequences:** requires that every assistant with `tool_calls`
  is followed by exactly that many tool messages. Ensure message lists are
  not truncated mid-sequence.
- **Stateless cache:** resets on pod restart. First requests after restart
  use synthetic function names until cache warms from upstream responses.

## Rollback

When the upstream bug is fixed (opencode PR #24170 or `@ai-sdk/openai-compatible`
patch), remove from the cluster:

1. Delete `kubernetes/apps/network/agentgateway/repair-proxy/` directory
2. Remove `agentgateway-repair-proxy` Kustomization from `ks.yaml`
3. Switch `gateway.goyangi.io` DNS back to ai Gateway LB `192.168.69.130`
4. Delete the `repair-proxy-auth` AgentgatewayPolicy, HTTPRoute, and
   `agentgateway-proxy` Service (removed automatically by prune)
