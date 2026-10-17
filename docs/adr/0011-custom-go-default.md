# ADR 0011: Custom Go generation as the default

Status: Accepted, 2026-10-17.

Supersedes ADR 0010's choice of a declarative default. Preserves its saved-app
compatibility, tests and local persistence. Reuses the source pipeline from
ADRs 0006–0008 and optional MCP settings from ADR 0009.

## Context and alternatives

The user wants custom terminal applications without a fixed component ceiling.
A declarative runtime offers controlled operations but every new widget, domain
operation or layout needs implementation in the host. Generated Go offers much
broader layout and behavior at the cost of compiler failures, dependency handling
and execution with the user's OS privileges. The user explicitly selected Go.

## Decision

Home Create App and CLI create use GenerateCode again. AI provides one complete
app.go/newModel through the existing strict Responses envelope. Host code owns
startup, persistence helpers, files, module setup, build/cache and process lifecycle.
Keep pinned UI modules and explicitly requested pinned third-party dependencies.
Do not add native connectors, MCP execution or arbitrary server installation.

Preflight credentials and installed Go before a paid request. Build separately,
and automatically open only successful, uncanceled home builds. CLI create saves
without running and supports a non-overwriting --name. A successful save updates
the project so repeated saves do not create duplicates. Saved app launches and
Run Again require explicit trust confirmation. Keep compile/launch arguments
separate and remove AI settings from child environments.

Restore explicit paid Repair with AI after errors. One user action makes one
request containing previous source and diagnostics, then builds again. Never
retry automatically. Preserve the prior project/trial data on repair failures
and cancellation. Generated repairs must preserve data, but no universal migration
or arbitrary-source correctness guarantee is claimed.

Keep saved declarative bundles readable. Do not delete their files, runtime,
validation or tests during this focused switch. New creation does not use their
component catalog. Go apps are labeled Go app rather than legacy.

A subsequent unused-code audit removes the retired declarative AI generator,
AI schema and unsaved-app creation/publication workflow. Saved bundle loading,
record operations, locking, renderer and their regression tests remain active.

## Consequences

New apps are not constrained to host-defined CRUD/forms/layouts. Go is required
for generation/rebuilds, and dependencies may require downloads. Cached apps run
without Go or AI. Compilation is evidence of buildability, not safety, correctness
or visual quality; generated code is unsandboxed. There is no guarantee that every
prompt or external integration can be implemented. Future external access still
needs real credentials, authorization and deliberate execution boundaries.

Verification combines repository tests for source contracts, cancellation,
publication and filtering with temporary mock-HTTP/PTY checks that compile real
source and exercise terminal handoff, persistence, saved reopening and explicit
repair. Paid generation remains an explicitly initiated manual check.
