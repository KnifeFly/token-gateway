# Media Forwarding Providers Runbook

## Scope

P7 keeps media files out of gateway storage. `/v1/files/*` registers transient input asset metadata only: source, MIME, size, content hash when bytes are inspectable, original URL when supplied, and expiry metadata for idempotency/request protection. The gateway does not persist object bytes, host download URLs, scan files, or manage lifecycle.

Provider results follow the opposite direction: the provider owns the result URL or returns a URL for customer-owned storage. Gateway records and returns normalized `results`, `assets`, `usage`, `provider_task_id`, and non-secret `provider_metadata` in task responses and callbacks.

## Input Asset Semantics

| Input | Gateway behavior | Persisted bytes | Returned URL |
|---|---|---:|---|
| Base64/data URL | Decode only for limit, MIME detection, size, and `sha256:` hash | No | Omitted |
| Multipart | Read request body only for limit, MIME detection, size, and `sha256:` hash | No | Omitted |
| URL | Validate `http`/`https`, record original URL and MIME by filename when possible | No | Original URL as transient reference |

`file_url` and `download_url` are optional compatibility fields. Clients must use the returned `file_id` or their own source URL in model inputs and must not assume gateway-hosted object storage.

## Provider Adapter Contract

Every async media provider adapter maps these lifecycle operations:

| Operation | Required mapping |
|---|---|
| submit | Public model, upstream model/version, task input, callback URL, non-secret metadata |
| poll running | Normalized status/progress and provider task id |
| poll terminal | Normalized status, result URLs/assets, usage, error code/message, provider metadata |
| cancel | Provider cancel endpoint when the task is still non-terminal |
| callback | Same `TaskObject` shape as GET task, including `results`, `assets`, `usage`, and provider metadata |
| settlement | Uses normalized `ProviderTaskResult.Usage` and result JSON, never local file bytes |

The generic HTTP adapter accepts `result_urls`, `assets`, `usage`, `error_code`, `error_message`, and `provider_metadata`. The Replicate adapter maps predictions to `assets` and `results` from prediction `output`, and stores prediction metadata such as `prediction_id`, status, and provider `get_url`.

## Validation

Run focused checks after touching this path:

```bash
go test ./internal/dataplane/parser ./internal/task ./internal/provider/replicate ./internal/worker/jobs ./tests/contract
go test ./...
```

Expected coverage:

- Base64 and multipart inputs produce `sha256:` hashes and no gateway download URL.
- URL input rejects non-HTTP schemes and preserves the source URL.
- Generic provider and Replicate fixture tests normalize result URLs/assets/metadata.
- Task completion writes callback payloads with provider task id, result URL, assets, usage, and provider metadata.
- Worker poller settles from normalized provider results.

## Operational Notes

- If a provider requires an upload-first flow, keep the temporary bytes scoped to the provider request and do not return gateway-hosted storage URLs to customers.
- If provider URLs expire, pass expiry or provider self-link metadata through `assets[].expires_at` or `provider_metadata`; customers should copy results into their own storage when durability is required.
- Do not put secrets, signed URL credentials, API keys, or raw prompt/media bytes into `provider_metadata`, logs, metrics labels, or traces.
