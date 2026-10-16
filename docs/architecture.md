# Tuilor architecture

Status: implemented generation, interactive rendering, and authorized form execution, 2026-10-15.
[ADR 0001](adr/0001-initial-architecture.md) records the local-first modular Go
direction. [ADR 0002](adr/0002-app-preview.md) records the preview design;
[ADR 0003](adr/0003-form-execution.md) extends it with form execution.

## Boundaries and data flow

Tuilor is one Go application with local filesystem storage. It preserves the
Cobra CLI, Bubble Tea home screen, and official MCP Go SDK integration. Generated
JSON is an intermediate representation shared by generation and display. Routine
preview interactions require no LLM.

```text
Generation:
request + discovered MCP schemas -> OpenAI -> candidate JSON -> app.Parse -> file

Open an app:
home -> My Apps -> read JSON -> discover tools -> app.Parse -> Bubble Tea preview

Submit:
bound values -> validate -> confirm exact call -> fresh connection -> rediscover
-> validate current arguments -> tools/call once -> close -> display result
```

| Package | Responsibility |
| --- | --- |
| `internal/cli` | Parse commands and flags; configure and run the TUI |
| `internal/mcpclient` | Own one stdio connection, discover tools, validate argument values, call once when authorized, close and reap the direct child |
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

The parent TUI model owns one MCP operation at a time. A Bubble Tea command
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
Inputs and results are scoped by page and form position, retain values across
page switches, and reset on leaving an app. Bubbles textinput handles editing; Bubbles viewport
handles scrolling. Resizing adjusts wrapping and field widths, and focus changes
scroll the active field or Submit button into view. Very small terminals show a resize prompt.

## Form execution and permission boundary

Enter on a field or Submit button constructs an immutable argument snapshot for
that form. The current discovery snapshot validates those values first. The TUI
then shows a scrollable confirmation containing the selected executable, exact
tool name, and JSON arguments. Only a literal `y` on that screen authorizes one
call. Enter does not approve; Esc declines. Inputs cannot change during review.
No permission is persisted or granted by the app definition or tool annotations.

After approval, a new 10-second operation connects to the explicitly chosen
server, rediscovers the tool, and validates the approved argument snapshot against
its current schema on that same session. Unknown parameters, missing tools,
required arguments, and string constraints can block the call. Schema resolution
has no remote loader. No default arguments are inserted. The MCP client sends
one tools/call request and closes its session; it does not reconnect or retry.
This supports one-shot tools rather than persistent server-side session state.

The UI blocks concurrent submissions. Esc during execution requests cancellation
and waits for cleanup; Ctrl+C also waits before quitting. Errors after dispatch
report that effects may already have occurred. A successful result is retained
even if subsequent session cleanup reports an error. Text or structured JSON is
displayed under the originating form. Tool-reported errors are labeled separately
from protocol failures. Non-text content is summarized, with no resource fetching,
and displayed output is capped at 64 KiB. Additional tool interaction is unsupported
and never triggers an automatic retry. The runtime has no AI dependency.

## Trust boundaries and current limits

- The user chooses a trusted executable through a startup flag. It is launched
  directly, with no shell interpolation. Discovery is not an OS sandbox.
- File contents and MCP metadata are untrusted. Saved apps pass structural and
  capability validation before rendering; displayed control characters are escaped.
- Generation and opening an app never call MCP tools. Runtime execution requires
  the user's confirmation of each exact tool and argument snapshot.
- Discovery is a snapshot. Submissions rediscover capabilities and validate actual
  values, but cannot prevent the trusted server changing behavior after validation.
- Edited values are not persisted. There is no app editor, schema migration,
  marketplace, workflow engine, or multi-server runtime.
- Cleanup covers directly launched child processes; arbitrary descendant process
  trees are outside the current guarantee.

## Verification and next milestone

No automated test files are retained, following the user's explicit preference.
Build and vet checks accompany manual terminal verification: saved greeting,
multiple pages and forms, focus, scrolling, resize, edit reset, malformed JSON,
unsupported definitions, missing configuration, startup failure, empty libraries,
cancellation, deadline handling, and direct-child cleanup. Execution checks use a
temporary local protocol fixture to inspect exact calls and arguments: declining
permission, duplicate key presses, fresh approval and connection per call, invalid
values, changed schemas, missing tools, tool/protocol errors, zero-input forms,
structured results, escaped controls, cancellation, timeout, and quitting during
a call. No check needs an OpenAI key or writes edited values to saved apps.

Future extensions such as persistent sessions, session-wide grants, retries,
workflows, and non-text rendering require separate design decisions.
