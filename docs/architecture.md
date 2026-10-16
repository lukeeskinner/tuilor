# Tuilor architecture

Status: implemented local app generation, persistent collections, and MCP apps, 2026-10-16.
[ADR 0001](adr/0001-initial-architecture.md) records the local-first modular Go
direction. [ADR 0002](adr/0002-app-preview.md) records the preview design;
[ADR 0003](adr/0003-form-execution.md) extends it with form execution;
[ADR 0004](adr/0004-home-generation.md) adds prompt, review, and explicit saving;
[ADR 0005](adr/0005-local-apps.md) adds local collections and persistent app bundles.

## Boundaries and data flow

Tuilor is one Go application with local filesystem storage. It preserves the
Cobra CLI, Bubble Tea home screen, and official MCP Go SDK integration. Generated
JSON is an intermediate representation shared by generation and display. Routine
preview interactions require no LLM.

```text
Local creation (no server):
request -> OpenAI structured outcome -> app.Parse v2 -> unsaved renderer + records
-> Ctrl+S -> publish app directory -> My Apps

MCP creation (--server):
request + discovered schemas -> close MCP -> OpenAI -> app.Parse v1 -> renderer/file

Open:
My Apps -> local bundle -> validate definition + records -> renderer
        -> legacy JSON -> discover tools and close -> app.Parse v1 -> renderer

Local action:
form inputs / selected ID -> validate -> candidate records -> atomic data write
-> replace displayed snapshot (deletion requires confirmation first)

Submit:
bound values -> validate -> confirm exact call -> fresh connection -> rediscover
-> validate current arguments -> tools/call once -> close -> display result
```

| Package | Responsibility |
| --- | --- |
| `internal/cli` | Parse commands and flags; configure and run the TUI |
| `internal/mcpclient` | Own one stdio connection, discover tools, validate argument values, call once when authorized, close and reap the direct child |
| `internal/app` | Define JSON, validate structure and capability bindings, save new files |
| `internal/localapp` | Own local records, IDs, immutable change snapshots, bundle loading, and atomic persistence |
| `internal/generator` | Construct AI input and isolate the OpenAI Responses implementation |
| `internal/tui` | Own menus, creation prompt, operation state, unsaved review, save status, preview inputs, focus, pages, and viewport |

The generator's narrow provider interface supports AI substitution. The preview
uses concrete models; no component registry, storage service, runtime framework,
or speculative interfaces have been added.

## Application definitions

Version 1 supports one or more pages, text, forms, text inputs, and one MCP action
per form. Bindings map tool parameter names to field IDs in the same form. Tool
input schemas must be objects with direct string properties; unsupported schema
features fail closed.

Version 2 adds named collections with text and boolean fields. Local forms insert
records through explicit input bindings; lists bind a text label and optional
boolean indicator. Each list can offer one literal update and one delete action
for the selected record. Text values must be nonblank; booleans default false on
insertion. Record IDs come from the runtime, so duplicate titles stay independent.
Collections, fields, components, actions, and references are validated before
rendering. Mixed local/MCP definitions are deferred.

The local schema has separate insert, update, and delete action shapes. Insert
forms cannot assign literal values; update/delete cannot bind form inputs, and
delete cannot assign values. These rules constrain generation itself as well as
local parsing. Reference and type relationships still require semantic validation.

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
My Apps lists top-level JSON files and local app directories in filename order,
showing app names with filenames. Hidden staging entries and symlinks are excluded.
Local opening validates app.json and data.json without launching a process. MCP
opening requires the explicitly selected server. Running saved apps requires no
API key.

The parent TUI model owns one background operation at a time. A Bubble Tea command
opens a selected local bundle, or reads legacy JSON and discovers MCP tools under
a 10-second context before validation. Discovery closes its MCP connection
before returning.
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

## Prompt, generation, and saving

A concrete creation model owns the Bubbles textarea and inline errors. Enter adds
a newline, `q` is ordinary text, and Ctrl+G starts generation. Empty requests
and missing credentials are rejected before any generation. A server is optional;
its presence selects the existing MCP generation mode. Configuration comes from
existing startup flags and environment variables.

The root runs generation and validation in one asynchronous command with a
90-second context. MCP mode includes discovery; ListTools closes its session
before contacting OpenAI. Local mode sends the request and local definition schema
with no MCP connection. Its strict result envelope contains success with an app or
unsupported with a reason. Validate the envelope, then validate successful app JSON.
The model decides request suitability; semantic validation enforces capabilities,
not an understanding of user intent. Esc cancels and waits for cleanup before
restoring the
original prompt; even a successful result arriving during cancellation is ignored.
Ctrl+C cancels and waits before quitting. There is no automatic retry or concurrent
generation. Prompt content remains in memory when navigating away and back.

A valid candidate uses the existing renderer. A draft flag adds the unsaved
indicator. Esc discards the candidate and its local records and restores the
prompt. Ctrl+S saves once; failure retains the complete draft. Local saves include
records added during review, while MCP saves contain only the definition.
Successful saving keeps the app open, displays the path, and selects that entry
when Esc refreshes My Apps. CLI create saves automatically in either mode.

Local forms insert immediately; Space runs the focused list's update action, and
Delete asks permission for the selected record. Enter cannot authorize deletion.
Up/Down change selection and Tab/Shift+Tab move focus between lists, fields, and
buttons. These shortcuts do not intercept text editing in fields. Results and
record changes need no AI request.

## Local storage and write ownership

The concrete local-app package separates record behavior from UI handling. Its
store keeps the definition, data, and optional saved path; Apply returns a new
snapshot after type/reference validation and successful persistence. The UI only
replaces its current store on success. Failed changes preserve inputs, displayed
records, and previous persisted data.

New apps are private directories containing app.json and data.json. Save writes
both in a hidden staging directory and renames the directory into the library.
The runtime chooses names using random IDs; definitions cannot choose paths.
Subsequent changes write a private temporary data file, sync and close it, then
atomically replace data.json. The root serializes writes and waits before leaving
or quitting; cancellation cannot undo a completed filesystem write.

Opening requires valid definitions and complete, typed records with unique IDs.
Missing, malformed, or unsupported data is reported and left untouched. Keep the
two files together when moving an app. Atomic replacement prevents partial JSON
writes, but is not a multi-process transaction or full power-loss durability
protocol. One running editor per app is supported; concurrent changes are not merged.

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
- Local record changes persist; MCP input values and results do not. There is no
  app editor, schema migration, marketplace, workflow engine, or multi-server runtime.
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
a call. Home-screen creation checks use a temporary HTTP Responses fixture and
MCP executable: multiline editing, literal q, resize, discovery closure before AI,
invalid output, API/startup errors, preflight checks, cancellation, late results,
the actual 90-second deadline, prompt preservation, draft execution, save failure
and retry, duplicate-save prevention, and reopening. They use a dummy key with
localhost responses; no billable request is needed. Local checks additionally cover
unsaved CRUD, text/boolean bindings, duplicate titles, stable IDs, deletion
confirmation, private bundle publication, carrying draft records into saves,
failed-write rollback and retry, corrupted data preservation, CLI local creation,
and reopening after restart without an API key or MCP launch.

Future extensions such as persistent sessions, session-wide grants, retries,
workflows, and non-text rendering require separate design decisions.
