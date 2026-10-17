# ADR 0004: Generate candidates in the home screen and save after review

> Historical decision: legacy JSON compatibility and generation are retired by
> [ADR 0007](0007-go-only-apps.md). Current app generation uses Go projects.

- Date: 2026-10-15
- Status: Accepted by the user before implementation

## Context and alternatives

Generation already works through the CLI. The next milestone brings it into the
home screen while retaining the existing JSON format, renderer, and permission
boundary. The user approved this scoped implementation plan.

| Approach | Benefit | Cost |
| --- | --- | --- |
| Save immediately, as CLI create does | Fewer states and immediate persistence | Unwanted candidates enter the library before review |
| Review an in-memory candidate, then save explicitly | Users can inspect and try an app before accepting it | Requires draft and save-result state |

## Decision

Use a concrete creation model with the existing Bubbles textarea. Enter inserts
newlines; Ctrl+G generates. Validate prompt, server configuration, and credentials
before discovery. Reuse the existing generator with OPENAI_API_KEY and TUILOR_MODEL.
No credential UI, new dependency, schema change, or provider abstraction is needed.

The root owns one asynchronous operation at a time. Discovery, generation, and
validation share a 90-second deadline; close discovery before contacting OpenAI.
Esc cancels, waits for cleanup, ignores the result, and restores the prompt.
Ctrl+C cancels and exits after cleanup. Never retry automatically.

Render successful candidates with the existing app model. Mark them unsaved and
allow tool calls only through the existing exact-call confirmation. Esc discards
the candidate and preserves the prompt. Ctrl+S calls app.Save once, keeping the
app open and displaying its path. Preserve a draft after save failure. Serialize
the local file write and wait for completion before leaving; cancellation cannot
undo a completed write. After success, Esc refreshes My Apps and selects the file.
Saving persists neither entered values nor tool-call permissions. CLI create
continues to save automatically.

## Consequences

Generation and execution remain separate: AI produces validated data, and runtime
actions require user approval. An explicit save is the boundary between a candidate
and a reusable app. Reusing concrete models keeps this milestone small and leaves
future refinement UI possible without building it now.

Verify through temporary local HTTP/MCP fixtures, terminal interaction, formatting,
build, vet, and diff checks. Retain no repository test files, following the user's
preference. A real billable generation remains an explicitly initiated manual run.
