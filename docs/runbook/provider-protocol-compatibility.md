# Provider Protocol Compatibility Runbook

P5 validates customer SDK wire shape at the provider adapter boundary. The
gateway keeps public capability URIs stable, rewrites only the routed upstream
model, preserves provider-specific request fields, and normalizes provider
usage/error classes before billing, reports, and audit consume them.

## Compatibility Matrix

| Provider family | Endpoint | Request fields | Stream | Tools | Multimodal | Usage normalization | Error classes | Tests |
|---|---|---|---|---|---|---|---|---|
| OpenAI compatible | `/v1/chat/completions` | Supported: JSON fields are preserved; routed model is rewritten | Supported, including usage chunks | Supported | Supported via content arrays | prompt/completion/input/output/cache/reasoning/audio | 400/401/403/404/429/5xx/timeout map to stable `provider_*` classes | `internal/provider/openai`, `tests/contract` |
| OpenAI compatible | `/v1/responses` | Supported: JSON fields are preserved; routed model is rewritten | Supported, including `response.completed` usage | Supported | Supported by passthrough | input/output/cache/reasoning/audio | stable `provider_*` classes | `internal/provider/openai` |
| OpenAI compatible | `/v1/embeddings` | Supported: JSON fields are preserved; routed model is rewritten | Not applicable | Not applicable | Text/token input passthrough | prompt/input/total | stable `provider_*` classes | `internal/provider/openai` |
| OpenAI compatible | `/v1/moderations` | Supported: JSON fields are preserved; routed model is rewritten | Not applicable | Not applicable | Text input passthrough | prompt/input/total when provider returns usage | stable `provider_*` classes | `internal/provider/openai` |
| OpenAI compatible | `/v1/images/*`, `/v1/audio/*` | Supported: JSON and multipart model rewrite | Speech/transcription streaming not enabled in current route | Not applicable | Multipart/file fields preserved | token usage when provider returns usage; binary bodies are relayed | stable `provider_*` classes | `internal/provider/openai` |
| Claude Messages | `/v1/messages` | Supported: JSON fields are preserved; routed model is rewritten | Supported, close-time usage merge | Supported | Supported via content blocks | input/output/cache creation/cache read | stable `provider_*` classes | `internal/provider/claude`, `tests/contract` |
| Gemini GenerateContent | `/v1beta/models/{model}:generateContent` | Supported: body fields are preserved; routed model is placed in URL | Not applicable | Supported | Supported via parts and modality details | prompt/candidate/cache/thoughts/audio/image/video | stable `provider_*` classes | `internal/provider/gemini`, `tests/contract` |
| Gemini GenerateContent | `/v1beta/models/{model}:streamGenerateContent` | Supported: body fields are preserved; routed model is placed in URL | Supported, close-time usage merge | Supported | Supported via parts and modality details | prompt/candidate/cache/thoughts/audio/image/video | stable `provider_*` classes | `internal/provider/gemini` |

## Verification

Run the P5 focused gate after provider protocol changes:

```bash
go test ./pkg/tokenusage ./internal/provider/relay ./internal/provider/openai ./internal/provider/claude ./internal/provider/gemini ./internal/dataplane/parser ./internal/dataplane/dispatch ./tests/contract
```

Run the broader package gate before release or merge:

```bash
go test ./...
```

## Error Handling

Adapters must not return raw provider error bodies to clients or logs. They
may retain a safe upstream error code in `ProviderError.ProviderCode`, while
the gateway-facing class remains one of:

```text
provider_request_invalid
provider_auth_failed
provider_not_found
provider_rate_limited
provider_timeout
provider_unavailable
provider_error
```

The dispatcher records these classes on provider attempts and maps them to the
gateway's stable external error envelope.

## Unsupported Boundaries

- P5 does not add provider-branded public routes.
- P5 does not add Realtime, semantic routing/cache, object storage, dynamic
  plugin execution, RBAC, invoice, or portal UI features.
- Provider-private beta features are passed through only when they fit the
  existing public capability route and do not require core lifecycle changes.
