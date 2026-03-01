# Provider Verification (Responses-Only Safe)

Use this when onboarding a third-party provider endpoint, especially Codex-like relays that reject legacy `/v1/chat/completions`.

## Quick Check Script

From repository root:

```bash
server/tools/verify_responses_provider.sh <base_url> <api_key> [model]
```

Example:

```bash
server/tools/verify_responses_provider.sh https://example.com sk-xxxx gpt-5.3-codex
```

## Make Target

From `server/`:

```bash
make verify-provider BASE_URL=https://example.com API_KEY=sk-xxxx MODEL=gpt-5.3-codex
```

## End-to-End Provider Pool Check

This runs the full path:
- local Blue API auth (preview token or provided JWT)
- add/update provider
- fetch models
- provider health test
- direct `POST /v1/responses` inference

From repository root:

```bash
server/tools/verify_provider_pool_responses_e2e.sh \
  --server-url http://127.0.0.1:18080 \
  --provider-base-url https://example.com \
  --api-key sk-xxxx \
  --provider-id codex-e2e-script \
  --provider-name "Codex E2E Script" \
  --model gpt-5.3-codex-spark
```

From `server/` (Make target):

```bash
make verify-provider-e2e \
  BASE_URL=https://example.com \
  API_KEY=sk-xxxx \
  SERVER_URL=http://127.0.0.1:18080 \
  PROVIDER_ID=codex-e2e-script \
  PROVIDER_NAME="Codex E2E Script" \
  MODEL=gpt-5.3-codex-spark
```

If the server is not in preview mode, pass JWT:

```bash
make verify-provider-e2e BASE_URL=https://example.com API_KEY=sk-xxxx TOKEN=<jwt>
```

## How to Read Output

- `responses_only=yes`:
  - Endpoint rejects legacy chat protocol.
  - Set provider `api_format=responses`.
  - Use `recommended_base_url` from script output.
- `chat_completions_code=400` and error contains `Unsupported legacy protocol ... use /v1/responses`:
  - This is expected for responses-only providers.
- `responses_v1_code=200`:
  - `/v1/responses` is healthy and should be preferred.

## Recommended Provider Settings

- `api_format`: `responses`
- `base_url`: use script `recommended_base_url`
- Keep model routing on Codex models (`gpt-5.*-codex*`) for best compatibility.

## API Endpoints

- `POST /api/v1/providers/verify`
  - Verify a candidate without creating/updating a provider.
  - Body:
    - `base_url` (required)
    - `api_key` (optional)
    - `skip_tls_verify` (optional)
    - `model` (optional, default `gpt-5.3-codex`)
- `POST /api/v1/providers/:id/verify`
  - Verify an existing provider; can apply recommendation.
  - Body:
    - `apply` (optional, default `false`)
    - `key_id` (optional, verify using specific stored key)
    - `model` (optional)
