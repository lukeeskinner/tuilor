# Tuilor architecture

Status: implemented generation and interactive preview, 2026-10-15.
[ADR 0001](adr/0001-initial-architecture.md) records the local-first modular Go
direction. [ADR 0002](adr/0002-app-preview.md) records the agreed preview design.

## Boundaries and data flow

Tuilor is one Go application with local filesystem storage. It preserves the
Cobra CLI, Bubble Tea home screen, and official MCP Go SDK integration. Generated
JSON is an intermediate representation shared by generation and display. Routine
preview interactions require no LLM.

```text
Generation:
request + discovered MCP schemas -> OpenAI -> candidate JSON -> app.Parse -> file

Preview:
home -> My Apps -> read JSON -> discover tools -> app.Parse -> Bubble Tea preview
```

| Package | Responsibility |
| --- | --- |
| `internal/cli` | Parse commands and flags; configure and run the TUI |
| `internal/mcpclient` | Own one stdio connection, discover tools, close and reap the direct child |
| `internal/app` | Define JSON, validate structure and capability bindings, save new files |
| `internal/generator` | Construct AI input and isolate the OpenAI Responses implementation |
| `internal/tui` | Own menus, loading state, preview inputs, focus, pages, and viewport |

The generator's narrow provider interface supports AI substitution. The preview
uses concrete models; no component registry, storage service, runtime framework,
or speculative interfaces have been added.

## Application definitions

Version 1 supports one or more pages, text, forms, text inputs, and one MCP action
per form. Bindings map tool parameter names to field IDs in the same form. Tool
input schemas must be objects with direct string properties; unsupported schema
features fail closed.

The same JSON Schema constrains Structured Outputs and local structural
validation. Local semantic checks additionally enforce supported components,
unique IDs, actual discovered tool names, binding targets, and required tool
parameters. Matching the JSON shape alone is insufficient to establish these
relationships. The definition grants no execution permissions.

The official OpenAI Go SDK v3.74.0 is pinned for Go 1.25 compatibility. Generation
reads `OPENAI_API_KEY`, supports `TUILOR_MODEL`, and defaults to `gpt-4.1-mini`.
The OpenAI implementation receives metadata as data, with no callable tools. It
rejects failed, incomplete, refused, and invalid outputs before saving.

## Preview ownership and lifecycle

Root flags `--apps-dir` (default `apps`) and `--server` configure the home screen.
My Apps lists regular top-level JSON files in filename order. No server starts
until a file is selected. Preview does not require an API key.

The parent TUI model owns one validation request at a time. A Bubble Tea command
reads the selected file, discovers tools under a 10-second context, and calls the
existing full validator. Discovery closes its MCP connection before returning.
No server session remains open during preview.

Esc cancels loading and returns to the library. Opening another file remains
blocked until the previous request finishes cleanup, so stale results cannot
replace a newer preview. Completion after navigation is ignored. Ctrl+C cancels
an active request and waits for its completion. On external cancellation or a
terminal failure, the CLI calls Close on the returned model, with a five-second
cleanup wait. The SDK owns process reaping; Tuilor never calls Wait itself.
Contexts belong to individual operations and are not stored in model structs.

The concrete preview model owns text inputs separately from the definition.
Inputs are scoped by page and form position, retain values across page switches,
and reset on leaving an app. Bubbles textinput handles editing; Bubbles viewport
handles scrolling. Resizing adjusts wrapping and field widths, and focus changes
scroll the active field into view. Very small terminals show a resize prompt.

## Trust boundaries and current limits

- The user chooses a trusted executable through a startup flag. It is launched
  directly, with no shell interpolation. Discovery is not an OS sandbox.
- File contents and MCP metadata are untrusted. Saved apps pass structural and
  capability validation before rendering; displayed control characters are escaped.
- Generation and preview never call MCP tools. Preview labels actions as disabled.
  Executing a form will require explicit permissions and input-value validation.
- Capability discovery is a snapshot. Preview validates bindings, not whether
  future typed input satisfies constraints such as pattern or enum.
- Edited values are not persisted. There is no app editor, schema migration,
  marketplace, workflow engine, or multi-server runtime.
- Cleanup covers directly launched child processes; arbitrary descendant process
  trees are outside the current guarantee.

## Verification and next milestone

No automated test files are retained, following the user's explicit preference.
Build and vet checks accompany manual terminal verification: saved greeting,
multiple pages and forms, focus, scrolling, resize, edit reset, malformed JSON,
unsupported definitions, missing configuration, startup failure, empty libraries,
cancellation, deadline handling, and direct-child cleanup. A temporary protocol
fixture verifies discovery-only messages without API credentials or tool calls.

Before enabling actions, agree permission behavior, session ownership, per-call
cancellation, result display, and the treatment of uncertain outcomes after a
side-effecting request times out. Record those decisions before implementation.
