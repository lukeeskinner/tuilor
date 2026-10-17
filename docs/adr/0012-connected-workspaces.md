# ADR 0012: Connected Go workspaces and schema-preserving revisions

Status: accepted, 2026-10-17.

## Context

Custom source generation alone does not provide reusable authentication, reviewed
API actions, or a dependable way to evolve saved apps. The user chose general Go
interfaces, GitHub plus custom APIs, a trusted in-process helper rather than a
broker, and revisions that preserve a declared persistent-data format.

## Decision

Offer GitHub and Custom API in a small integration picker, with portable badges
and opt-in Nerd Font icons. GitHub uses browser device authorization with a public
client ID or hidden token entry; custom APIs use configurable token authentication.
Store nonsecret configuration atomically and credentials in the OS keyring. MCP
discovery stays separate, without a generated-app runtime bridge.

Extend structured source generation with proposed integration requirements,
documentation references, JSON Schema and empty initial state. Optional public
documentation research shares the generation deadline and has a five-call cap.
Credentials and saved records are excluded from generation. Unsupported capabilities
must be explained rather than simulated as live functionality.

The host supplies asynchronous API requests, origin/base-path restrictions,
bounded responses, cancellation, and exact non-GET confirmation. Only approved
credentials enter the child at launch. These are conventions within unsandboxed,
trusted Go; a broker would provide a stronger boundary but was explicitly deferred.

New saved apps use revision directories with an atomically published active
pointer. Data reads/writes validate against a declared unchanged schema. Modify
trials isolated copied data, then checks the active revision and original data
hash before publication. Restore previous code carries current compatible data
forward. Single-writer OS locking protects Tuilor operations. Existing flat Go
projects remain runnable without automatic conversion.

## Alternatives and consequences

- Hardcoded GitHub actions would simplify validation but restrict generated app
  behavior. The general request helper preserves the selected dynamic source model.
- A separate API broker could keep credentials out of generated processes and
  enforce approvals across that boundary. The chosen helper is simpler but cannot
  stop trusted source from bypassing it or accessing OS resources.
- Backups alone aid recovery but do not validate data shape. The declared schema
  provides a checkable storage contract while leaving layout/domain code general.
  It still cannot prove semantic compatibility or correct behavior.
- In-place revision replacement risks partial projects. Complete directories plus
  an active pointer preserve the previous app on publication failure, at the cost
  of additional disk usage. Migrations, merging and revision pruning are deferred.

Trial and restored apps cannot undo external writes. Reconnect may be necessary
after provider authorization changes. No implementation promises working behavior
for every prompt, automatic repair, sandboxing, or unsupported OAuth providers.

## Verification

Use temporary OAuth/HTTP/keyring/PTY fixtures without repository test files or
paid requests. Exercise authentication failures, exact request confirmation,
credential redaction, storage failures, corrupt state, locks, publication,
rollback, terminal restoration and compatibility. Run formatting, builds, vet,
existing test commands and diff checks. Real account sign-in and billable
generation remain explicitly initiated manual checks.
