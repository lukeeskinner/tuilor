# Tuilor

**Build your own terminal applications with AI + MCP.**

Tuilor generates declarative JSON apps from natural-language requests and tools
discovered from a trusted local MCP server. You can open saved apps through the
Bubble Tea home screen, view their pages, edit fields, and submit forms to MCP
tools. Generation uses AI; routine app interactions use ordinary Go code and
do not call an AI API.

Every tool call requires confirmation of the server, tool name, and exact
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

Then generate an app (this makes a billable OpenAI request):

```sh
./bin/tuilor create "Build a greeting application" --server ./bin/testserver
```

Tuilor prints the path of a new private JSON file in `apps/`. Use `--output-dir`
for another directory and `--timeout` to change the default 90-second deadline.
`TUILOR_MODEL` overrides the default `gpt-4.1-mini` model. The implementation uses
the official OpenAI Go SDK, Responses API, and strict Structured Outputs, followed
by local format and capability validation.

The initial format supports pages, text, forms, text inputs, and actions bound to
discovered MCP tools with direct top-level string parameters. See
[the example definition](examples/greeting.json).

## Run an app through My Apps

```sh
./bin/tuilor --server ./bin/testserver
```

Choose **My Apps**, select a JSON filename, and press Enter. Tuilor discovers the
chosen server's tools, closes the connection, validates the saved definition,
and displays its pages and forms. No API key is needed. To use another app directory:

```sh
./bin/tuilor --apps-dir ./my-apps --server ./bin/testserver
```

To try an app without generating one first:

```sh
mkdir -p apps
cp examples/greeting.json apps/greeting.json
./bin/tuilor --server ./bin/testserver
```

Open the greeting app, type your name, and press Enter. Review the confirmation
and press `y` to run that exact call once, or Esc to decline. A successful call
displays a result such as `{"message":"Hello, Luke"}` beneath the form. Pressing
Enter on the confirmation screen does not authorize execution.

| Key | Behavior |
| --- | --- |
| Up / Down, Enter | Choose a menu item or saved app |
| Tab / Shift+Tab | Focus the next / previous field or Submit button |
| Enter inside an app | Review the focused form's tool call |
| y on confirmation | Authorize the displayed call once |
| Ctrl+N / Ctrl+P | Switch pages, preserving edits |
| Page Up / Page Down | Scroll the current page |
| Esc | Decline confirmation, cancel a running request, or return to the preceding screen |
| Ctrl+C | Exit from any screen |
| q | Exit from menus; type `q` normally in form fields |

App lists include regular `.json` files directly inside the selected directory,
sorted by filename. Missing directories appear as empty libraries. Invalid apps
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

There are currently no automated test files, by project choice. Terminal and
temporary protocol-fixture checks cover navigation, validation, per-call
confirmation, exact argument binding, server errors, cancellation, timeout, and
direct-child cleanup. Runtime verification requires no OpenAI request.

See [the architecture](docs/architecture.md), [ADRs](docs/adr/), and
[repository instructions](AGENTS.md). [ADR 0003](docs/adr/0003-form-execution.md)
records the permission and connection-lifetime choices for form execution.
