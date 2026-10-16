# Tuilor

**Build your own terminal applications with AI + MCP.**

Tuilor generates declarative JSON apps from natural-language requests. Local apps
use app-owned collections; MCP apps use tools discovered from a trusted server.
Open saved apps through the Bubble Tea home screen. Generation uses AI; routine
app interactions use ordinary Go code and do not call an AI API.

Local apps support selectable lists, text inputs, and insert/update/delete actions.
Adding and completing records happen immediately; deleting requires confirmation.
Saved local apps persist record changes automatically.

Every MCP tool call requires confirmation of the server, tool name, and exact
arguments. Each approved submission opens a fresh connection, rechecks the tool
and argument values, calls once, and closes. Form edits and results stay in memory
and are discarded when you leave the app. Workflows and sharing are deferred.

## Build

Use Go 1.25.5 or later:

```sh
go build -o bin/tuilor ./cmd/tuilor
go build -o bin/testserver ./cmd/testserver
```

## Generate an app

Set `OPENAI_API_KEY` in the terminal where you run Tuilor. For zsh, enter it at a
hidden prompt:

```zsh
read -rs "OPENAI_API_KEY?OpenAI API key: "
export OPENAI_API_KEY
echo
```

Start the home screen without a server for a local app:

```sh
./bin/tuilor
```

Choose **Create App** and enter **Build a todo list app**. Enter inserts a newline;
**Ctrl+G** starts a billable OpenAI request. Generation and validation share a 90-second
deadline (including discovery when using MCP). Esc cancels, waits for cleanup,
and restores your prompt.

Successful output opens as an **unsaved app**. Add tasks and try the list, then
press **Ctrl+S** to save into `apps/` (or `--apps-dir`). Saving keeps
the app open and shows its path; repeated Ctrl+S does not create duplicates.
Local apps save both the definition and records entered during review. MCP apps
save only their definition, without entered values or results. Saving grants no
MCP execution permission. Esc discards an unsaved app and restores the prompt; after
saving, Esc opens My Apps with the new file selected. A failed save keeps the
candidate so you can retry. Tuilor waits for an active file write before leaving.

You can also generate and save directly from the CLI, as before (also billable):

```sh
./bin/tuilor create "Build a todo list app"
```

Tuilor prints the path of a new private app directory in `apps/`. Use `--output-dir`
for another directory and `--timeout` to change the default 90-second deadline.
`TUILOR_MODEL` overrides the default `gpt-4.1-mini` model. The implementation uses
the official OpenAI Go SDK, Responses API, and strict Structured Outputs, followed
by local format and capability validation.

Version 2 local definitions support collections with text and boolean fields,
text, insert forms, and selectable lists with literal updates and confirmed
deletion. See [the todo example](examples/todo/app.json). Requests requiring
unavailable features, such as Gmail access in local mode, should return an
unsupported explanation rather than an unrelated app. The model makes that
suitability decision; local validation enforces the actual format and capabilities.

To generate an MCP app instead, supply a server explicitly:

```sh
./bin/tuilor --server ./bin/testserver
# Or generate and save directly:
./bin/tuilor create "Build a greeting application" --server ./bin/testserver
```

Version 1 MCP definitions retain text and forms with direct string bindings. The
test server provides only `greet`; it cannot supply todo or email operations.
See [the greeting example](examples/greeting.json).

## Run an app through My Apps

```sh
./bin/tuilor
```

Choose **My Apps**, select an application name, and press Enter. Local apps load
their definition and records without a server or API key. Names include filenames
to distinguish duplicates. MCP apps still require `--server` to discover and
validate their tools before opening. To use another app directory:

```sh
./bin/tuilor --apps-dir ./my-apps
```

To try the local todo app without generating one first:

```sh
mkdir -p apps
cp -R examples/todo apps/todo-example
./bin/tuilor
```

For the MCP greeting example, copy `examples/greeting.json` into `apps/` and
start with `--server ./bin/testserver`. Open it, type your name, and press Enter.
Review the confirmation
and press `y` to run that exact call once, or Esc to decline. A successful call
displays a result such as `{"message":"Hello, Luke"}` beneath the form. Pressing
Enter on the confirmation screen does not authorize execution.

| Key | Behavior |
| --- | --- |
| Up / Down, Enter | Choose a menu item or saved app |
| Enter in Create App | Insert a newline |
| Ctrl+G in Create App | Generate an unsaved app |
| Ctrl+S in an unsaved app | Save once, including local records |
| Tab / Shift+Tab | Focus the next / previous list, field, or button |
| Up / Down in a focused list | Select a record |
| Space in a focused todo list | Complete the selected task |
| Delete in a focused list | Review deletion; `y` confirms, Esc cancels |
| Enter in a form | Add a local record, or review an MCP call |
| y on confirmation | Authorize the displayed call once |
| Ctrl+N / Ctrl+P | Switch pages, preserving edits |
| Page Up / Page Down | Scroll the current page |
| Esc | Decline confirmation, cancel a running request, or return to the preceding screen |
| Ctrl+C | Exit from any screen |
| q | Exit from menus; type `q` normally in prompts and form fields |

My Apps includes regular top-level `.json` files and local app directories, sorted
by filename. Hidden staging directories and symlinks are excluded. Each local app
contains `app.json` and `data.json`; keep them together when moving an app. New
bundles are published by directory rename; data changes use atomic file replacement.
Failed writes retain the previous state. Invalid or missing data produces an error
and is never reset automatically. Use one running editor per app; concurrent edits
are not merged. Esc discards all records in an unsaved candidate.

Missing directories appear as empty libraries. Invalid apps
and connection failures appear as errors in the app list. Discovery and each
complete submission have 10-second deadlines; subprocess cleanup may take
additional time. After sending a call, cancellation or timeout cannot establish
whether the tool already performed its effects. Tuilor reports that uncertainty
and never retries automatically.

The app validates string constraints such as length, pattern, and enum before
confirmation and against the server's current schema before execution. Calls are
serialized. Text and structured JSON results are displayed with terminal controls
escaped; non-text content is summarized without fetching resources. Displayed
output is limited to 64 KiB. Fresh connections do not retain server-side session
state between submissions.

Only launch explicitly trusted local executables. Servers run with your OS
privileges, even during discovery. Generated definitions contain no launch
commands or execution permissions. Untrusted metadata is escaped for display.

## Other commands and development

```sh
./bin/tuilor tools ./bin/testserver
./bin/tuilor version
go fmt ./...
go build ./...
go vet ./...
```

There are currently no automated test files, by project choice. Temporary local
HTTP/MCP fixtures and terminal checks cover generation, invalid output, errors,
cancellation, the 90-second deadline, prompt preservation, review, save failures,
duplicate-save prevention, reopening, local CRUD, failed-write rollback, corrupt
data preservation, and confirmed execution. These checks use
a dummy key and local responses, with no billable OpenAI request.

See [the architecture](docs/architecture.md), [ADRs](docs/adr/), and
[repository instructions](AGENTS.md). [ADR 0003](docs/adr/0003-form-execution.md)
records the permission and connection-lifetime choices for form execution.
[ADR 0004](docs/adr/0004-home-generation.md) records prompt, review, and save behavior.

[ADR 0005](docs/adr/0005-local-apps.md) records local collections and persistence.
