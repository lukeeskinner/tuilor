# ADR 0010: Declarative local apps as the default product

Status: Superseded for default creation by [ADR 0011](0011-custom-go-default.md),
2026-10-17. Saved declarative compatibility is retained.

Supersedes the Go-only creation/runtime direction in ADRs 0006–0008 for **new**
apps. Retains legacy Go projects and ADR 0009's optional MCP settings/discovery.
This is a new contract, not a migration of the retired JSON formats.

## Context

Source generation permits arbitrary layouts but requires compilation and grants
AI-written code the user's OS privileges. Making MCP setup a prerequisite adds
integration work before a user can create a useful local app. The accepted product
slice is prompt → functional local app → save → reopen, without Go or MCP setup.

## Alternatives

1. Continue source generation: maximum behavioral freedom, but build/dependency
   failures, arbitrary execution and per-app lifecycle complexity remain.
2. Use validated declarations and host-owned operations: bounded behavior and
   predictable interaction/storage, at the cost of a finite component vocabulary.

Choose declarations for new apps. Keep legacy source code and launcher available
with explicit code-execution confirmation; do not invoke source generation or AI
repair from the new-app flow.

## Decision

Use a versioned definition with one collection/page, text/list/form/button
components and text/boolean/choice/date fields. Actions bind form inputs or typed
literals to declared fields, with explicit selected-record targets. The runtime
contains no task-specific names. Completion is an ordinary boolean update.

A concrete catalog describes available capabilities and argument schemas. The
AI consumes this catalog; one small consuming Executor interface separates the
renderer from the concrete local store. Only local list/insert/update/delete
exist now. Native and MCP executors are future work, not speculative providers
implemented alongside the local slice.

Use one strict Responses request returning success/definition or unsupported/
explanation. Validate references/types locally and validate record values again
before mutation. No scripts, shell commands, arbitrary paths, sample-live data,
credentials or executable source belong in definitions. Structured Outputs
constrains shape; it cannot establish intent or authorize capabilities.

Persist bundles as manifest.json, app.json and data.json under host-assigned IDs.
Acquire an application-level OS lock for the runtime. Stage new bundles in sibling
directories and rename once complete; hold a library lock while assigning IDs.
Stage each record change in a temporary file and replace atomically before
updating display state. Preserve corrupt data and previous state on failures.
Do not remove lock files while another process could hold their inode.

CLI create saves without opening, prints the ID/run command, and supports a
non-overwriting --name. Home generation opens a private unsaved candidate;
Ctrl+S publishes definition and trial data together. Save/Discard/Cancel governs
leaving an unsaved app. Saved apps reopen locally without AI or MCP. Optional MCP
management moves under Settings; no native connector/catalog is added now.

## Consequences and limits

- New apps need no installed Go toolchain and never execute AI-generated code.
- Adding field/component types requires host validation and rendering work.
- The task manager and application tracker use the same definitions/runtime.
- Atomic files and OS locks prevent competing Tuilor writers, not arbitrary
  external edits, remote synchronization or every power-loss scenario.
- Natural-language correctness and live model quality still require real checks.
- Meaningful repository tests replace the earlier no-test preference.
- Refinement needs explicit data compatibility rules before schema changes; next
  comes one native integration, then mixed local/external apps. Migrations, mock
  previews, arbitrary layouts and marketplaces are deferred.
