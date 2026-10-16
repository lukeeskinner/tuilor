# ADR 0005: Declarative local collections and saved app bundles

- Date: 2026-10-16
- Status: Accepted by the user before implementation

## Context and alternatives

The greeting server cannot supply todo capabilities. The user chose declarative
apps over generated executable code, then selected a plain local todo app as the
next milestone: add, complete, delete, save, and reopen with data intact.

| Choice | Benefit | Cost |
| --- | --- | --- |
| Todo-specific runtime | Smaller first implementation | Each new domain needs custom behavior |
| Reusable local collections | Lists and record actions also support other personal apps | Requires collection, binding, and record validation |
| Generate executable code | More freedom | Requires a separate build and execution design |

## Decision

Use version 2 definitions with app-owned collections of text and boolean fields,
insert forms, selectable lists, literal updates, and deletion. Keep version 1 MCP
definitions compatible. The same renderer handles both without a component registry.
The definition owns layout and bindings; the local-app package owns records,
runtime-generated IDs, validation, and persistence. It grants no external access.

Without --server, creation uses local generation. With an explicitly selected
server, existing MCP discovery and generation remain. Local Structured Outputs
returns either a candidate or an unsupported reason; validate the envelope and
definition locally. Suitability still depends on the model understanding the
request; schema validation cannot establish natural-language intent.

Use operation-specific action schemas, including empty unused arrays, so the
generation schema enforces the same action-shape rules as runtime validation.
Forms accept insert actions only; lists accept update and delete actions only.

The user selected immediate insertion and completion, confirmed deletion, and
retaining records entered during draft review when saving. Unsaved data remains
in memory. Esc discards it with the candidate; Ctrl+S saves both once.

Publish a new directory containing app.json and data.json by renaming a private
staging directory. This keeps incomplete new apps out of the library. Later
changes write and sync a temporary data file before replacing data.json. Return
a replacement in-memory snapshot only after persistence succeeds. Corrupt or
missing records block opening rather than triggering a reset. Use app-owned
paths assigned by the runtime, never paths chosen by generated JSON.

My Apps displays names with filenames and supports bundles alongside legacy JSON
files. Local apps require neither an MCP server nor an API key when reopened.
Generation remains separate from runtime operations, and existing exact MCP call
confirmation is unchanged.

## Consequences

The first complete app demonstrates reusable data and UI capabilities without
external accounts. Persistence uses only the standard library and local files.
Atomic replacement prevents partial JSON writes; it is not a multi-process
transaction or a guarantee against every power-loss scenario. One editor per app
is supported; concurrent edits are not merged.

Editing task titles, filtering, mixed local/MCP definitions, external integrations,
and arbitrary code are deferred. Verification uses temporary fixtures and terminal
checks, with no repository test files or billable API requests.
