# ADR 0003: Confirm each form call and use a fresh MCP connection

- Date: 2026-10-15
- Status: Accepted by the user before implementation
- Extends: ADR 0002's rendering-only milestone

## Context and alternatives

Saved apps render correctly but their forms cannot yet invoke tools. The user
authorized working submissions and selected both recommended options below.

| Choice | Benefit | Cost |
| --- | --- | --- |
| Connect per submission | Reuses bounded one-shot ownership; refreshes capabilities | Startup overhead; loses server-side session state |
| Retain an app-lifetime session | Faster calls; supports session state | Additional session ownership, recovery, and shutdown work |
| Confirm every exact call | Clear permission for one tool and argument snapshot | Requires approval for each submission |
| Grant tools for an app session | Fewer prompts | Broader authority and additional permission state |

## Decision

Connect and close per submission. Confirm every call with its executable, tool
name, and exact bound JSON arguments visible. Only an explicit `y` authorizes
one call. Generated JSON, model output, and tool annotations never grant authority.

Validate values before confirmation using discovered metadata. After approval,
rediscover the tool and validate the same immutable arguments against the current
schema on the connection used for execution. Send at most one tools/call request.
Do not inject defaults, retry, follow resource links, or satisfy additional tool
interaction requests. Keep generation independent and retain the existing JSON
format and direct string bindings.

Serialize calls under a 10-second deadline. Canceling waits for cleanup. Results
and errors appear beneath the originating form and are kept only in memory.
Escape terminal controls and limit displayed output to 64 KiB. Non-text content
is summarized; structured JSON is a fallback when no text result is available.

## Consequences

The first runnable app needs no LLM or API key after generation. Tool calls have
explicit authorization and bounded local lifetimes without a workflow framework.
Timeout or cancellation after dispatch does not prove that effects did not occur;
the UI reports uncertainty and makes no automatic retry. Direct-child cleanup
does not guarantee termination of arbitrary descendant processes.

Keep the user's preference for no repository test files. Verify through build,
vet, real greeting calls, and temporary terminal/protocol fixtures. Revisit this
decision when stateful servers, broader permission scopes, or retry requirements
justify the extra lifecycle and outcome-recovery design.
