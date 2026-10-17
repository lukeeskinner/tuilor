# ADR 0006: Generate Go apps and launch successful builds automatically

> Historical decision: legacy JSON compatibility and generation are retired by
> [ADR 0007](0007-go-only-apps.md). Current app generation uses Go projects.

- Date: 2026-10-16
- Status: Accepted by the user before implementation
- Supersedes: expanding the local declarative renderer for new app capabilities

## Context and alternatives

The user expects a prompt to produce a custom, usable TUI with responsive layouts.
A fixed JSON vocabulary needs a new renderer feature for each unsupported UI or
interaction. The user selected generated Go code and automatic opening after a
successful build, while preserving existing JSON/MCP apps.

| Approach | Benefit | Cost |
| --- | --- | --- |
| Extend declarative JSON | Predictable validation and constrained execution | Ongoing renderer work limits generated behavior |
| Generate Go against a small starter | Custom layout, state, and interactions | Requires Go, compilation, process ownership, and trust in generated code |
| Generate unrestricted multi-file projects | Maximum project freedom | More dependency/path/build configuration to validate |

## Decision

Generate one app.go implementing newModel() (tea.Model, error). Tuilor supplies
main.go with Bubble Tea setup, resize/exit handling, and local data helpers. The
AI owns layout and behavior. Structured JSON is the source transport envelope;
it is not the runtime app representation. Keep the existing OpenAI provider and
model configuration, with a larger output budget for code.

Default to pinned Bubble Tea, Bubbles, and Lip Gloss plus the standard library.
Support declared, explicitly requested additional modules with exact versions.
Validate source syntax, imports, startup ownership, and dependency declarations.
Host code owns project paths, module scaffolding, build commands, and executable
paths. Disable cgo, ambient Go build flags, workspaces, and toolchain downloads.
Generation/preflight has 90 seconds; preparation/build has a separate 120 seconds.
Never launch canceled or failed builds. Only an explicit repair makes another
paid request. Compilation establishes buildability, not runtime correctness.

Use a CLI-owned session workspace and finish the parent Bubble Tea program before
running the child. This makes terminal and process ownership explicit. Resume
the retained TUI model after exit or crash. On Unix, forward parent resize signals.
Bubble Tea ExecProcess was considered, but the outer CLI loop keeps launch
cancellation tied to the CLI context and owns cleanup without a context in models.
Bound direct-child shutdown; descendant process trees are outside this guarantee.

After exit, offer Save, Run Again, or Discard. Save publishes source, starter,
module files, trial data, manifest, and a binary cache by staging and directory
rename. Repeat saves are idempotent. Failed saves retain the candidate. Discard
and session exit clean temporary projects. Cache fingerprints and binary hashes
detect staleness; missing/stale builds compile without AI. Existing flat JSON and
local collection bundles remain selectable through My Apps. CLI subcommands keep
their existing JSON-generation behavior. Home-screen Create App always generates
Go source, even with --server configured; that flag supplies the connection for
opening saved MCP apps rather than silently switching generation formats.

## Consequences and limits

Generated Go executes with the user's OS privileges. The creation UI makes this
explicit, and compiler/app environments omit OpenAI and Tuilor AI configuration.
These measures are not a sandbox or an authenticity check for saved projects.
The legacy renderer's validation and MCP permission policy do not govern Go code.
This milestone adds no MCP wiring inside generated programs.

Generated apps receive helpers for atomic JSON persistence and corruption
preservation, but code must use them correctly. One writer per app is supported.
Source projects are portable; binary caches are platform-specific. Go and module
access are needed for compilation, while compatible cached apps need no Go or AI.

Deliver the hand-authored runner first, then generation/repair integration. Use
temporary HTTP/MCP fixtures and real local builds, with no repository test files.
Update the repository vision to describe code generation while preserving legacy
formats. A real billable generation remains an explicitly initiated manual check.
