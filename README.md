# Tuilor

**Build your own terminal applications with AI + MCP.**

Tuilor generates declarative JSON apps from natural-language requests and tools
discovered from a trusted local MCP server. You can open saved apps through the
Bubble Tea home screen, view their pages, and edit form fields in a preview.
Generation uses AI; preview uses ordinary Go code and does not call an AI API.

Preview validates tool references but does not execute tools. Form edits stay in
memory and are discarded when you leave the app. Tool execution, permissions,
workflows, and sharing are future milestones.

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

## Preview through My Apps

```sh
./bin/tuilor --server ./bin/testserver
```

Choose **My Apps**, select a JSON filename, and press Enter. Tuilor discovers the
chosen server's tools, closes the connection, validates the saved definition,
and displays its preview. No API key is needed. To use another app directory:

```sh
./bin/tuilor --apps-dir ./my-apps --server ./bin/testserver
```

To try preview without generating an app first:

```sh
mkdir -p apps
cp examples/greeting.json apps/greeting.json
./bin/tuilor --server ./bin/testserver
```

| Key | Behavior |
| --- | --- |
| Up / Down, Enter | Choose a menu item or saved app |
| Tab / Shift+Tab | Focus the next / previous form field |
| Ctrl+N / Ctrl+P | Switch pages, preserving edits |
| Page Up / Page Down | Scroll the current page |
| Esc | Cancel validation or return to the preceding screen |
| Ctrl+C | Exit from any screen |
| q | Exit from menus; type `q` normally in form fields |

App lists include regular `.json` files directly inside the selected directory,
sorted by filename. Missing directories appear as empty libraries. Invalid apps
and connection failures appear as errors in the app list. Discovery has a
10-second deadline; subprocess cleanup may take additional time.

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

There are currently no automated test files, by project choice. Preview was
checked through terminal sessions, including editing, navigation, resize, invalid
files, server errors, cancellation, timeout, and direct-child cleanup. No preview
verification requires an OpenAI request.

See [the architecture](docs/architecture.md), [ADRs](docs/adr/), and
[repository instructions](AGENTS.md). The next milestone is explicit permission
and execution design before enabling form actions.
