# ADR 0007: Use Go projects only and upgrade the generation model

- Date: 2026-10-16
- Status: Accepted: user requested a better model and removal of JSON app logic
- Supersedes: the JSON compatibility runtime retained by ADR 0006; app-generation
  and execution choices in ADRs 0002–0005

## Context and alternatives

The active product generates complete Go terminal applications. Keeping the
older declarative schemas, generator, collection store, renderer, and MCP forms
maintains a second application stack and creates confusing generation routes.
Keeping compatibility would preserve older apps; removing it simplifies both
creation and execution while retiring those formats. The user chose removal.

## Decision

Delete the legacy JSON app packages, prompts, component rendering, collection
runtime, MCP form submission, and examples. Both home Create App and CLI create
use GenerateCode, source compilation, and the Go project store. CLI create saves
without launching; home generation opens successful builds automatically.

My Apps recognizes Go project directories containing manifest.json. Leave old
user files on disk, but do not open or migrate them. Drop unused --server flags.
Keep the standalone tools command and official SDK discovery client; there is
no generated-program MCP integration or host tool-execution path.

Keep JSON for strict source-response transport, project manifests, and app data.
Removing a declarative UI interpreter does not require replacing useful storage
formats or weakening the generation output contract.

Change the provider default from gpt-4.1-mini to gpt-6.1-sol. The official model
catalog recommends Sol for balancing intelligence and cost; Astra is another
viable choice at a higher listed per-token price. Sol supports Responses and
Structured Outputs with the existing request shape and SDK string model ID.
Use its default reasoning behavior; preserve TUILOR_MODEL overrides and show the
selected model in Create App. No automatic fallback, retries, or paid evaluation.

References: [model catalog](https://developers.openai.com/api/docs/models),
[GPT-6.1 Sol](https://developers.openai.com/api/docs/models/gpt-6.1-sol).

## Consequences and validation

The host has one application runtime. Legacy JSON apps no longer open; regenerating
an app produces Go, with no automatic data migration. Saved Go source and trial
data remain compatible. Future MCP integration needs a separate agreed design.

The stronger default has higher per-token pricing than the old mini model; a
better model does not guarantee correct code. Keep compiler validation, explicit
repair, 90-second generation and 120-second build deadlines, and all process/data
safety boundaries. Verify with fake HTTP and local MCP fixtures, build/vet/race
checks, and existing saved Go apps; real paid model behavior is a manual check.
