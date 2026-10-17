# Tuilor

**Describe a terminal app. Generate it, build it, and use it.**

Tuilor generates Go applications with custom Bubble Tea layouts and behavior.
A successful build opens automatically. Exit the app to save it, run it again,
or discard it. Saved apps launch from **My Apps** without an AI request.

## Build

Use Go 1.25.5 or later:

```sh
go build -o bin/tuilor ./cmd/tuilor
go build -o bin/testserver ./cmd/testserver
```

Go is required for generation and rebuilding. A compatible cached app can run
without Go. Initial builds may download pinned dependencies through Go tooling.

## Generate and open an app

Set `OPENAI_API_KEY` in the terminal where you run Tuilor. In zsh:

```zsh
read -rs "OPENAI_API_KEY?OpenAI API key: "
export OPENAI_API_KEY
echo
./bin/tuilor
```

1. Choose **Create App** and describe an app, such as **Build a todo list app**.
2. Enter inserts a newline; **Ctrl+G** makes a billable OpenAI request.
3. Tuilor checks the installed Go toolchain, generates source, and compiles it.
4. A successful build opens automatically. **Esc** or **Ctrl+C** returns to Tuilor.
5. Choose **Save**, **Run Again**, or **Discard**. Save retains source and trial data.
6. After saving, Esc opens My Apps with the new app selected.

Generated code runs with your OS permissions. It is **not sandboxed**. Only
request and run apps you trust. Tuilor removes OpenAI credentials and Tuilor AI
configuration from compiler and app environments; this does not prevent programs
from accessing other files or resources available to your account.

The AI writes `app.go` implementing `newModel() (tea.Model, error)`. Tuilor supplies
the entry point, terminal lifecycle, exit keys, and atomic local data helpers.
The AI chooses the app's interface, model, and data structures. Generation asks
for responsive sizing, scrolling, visible focus/help, and small-terminal fallbacks.
Generation includes a small compiling reference for the pinned widget APIs,
persistence, and sizing, plus the host helper contract. The host entry-point
implementation is kept out of the generation context. Compilation establishes buildability; actual behavior
and visual quality still need runtime verification.

The default libraries are Go's standard library, Bubble Tea v1.3.10, Bubbles
v1.0.0, and Lip Gloss v1.1.0. Explicitly requested third-party modules may be
included with pinned versions. Undeclared imports and invalid dependency
declarations are rejected. No new MCP wiring inside generated programs is
implemented in this milestone.

The default model is `gpt-6.1-sol`, selected for stronger code generation with a
balance of capability and cost. It supports Responses and Structured Outputs;
see the [official model documentation](https://developers.openai.com/api/docs/models/gpt-6.1-sol).
It costs more per token than the previous `gpt-4.1-mini` default. `TUILOR_MODEL`
still overrides it; the Create App screen shows the selected model. To use the
new default after previously exporting a model, run `unset TUILOR_MODEL`.
Generation uses the
official OpenAI Go SDK, Responses API, and strict Structured Outputs to transport
source and metadata. It has a 90-second deadline, including preflight. Dependency
preparation and compilation have a separate 120-second deadline.

Esc cancels an active operation and waits for cleanup; canceled builds never
launch automatically. Errors preserve the prompt and candidate. **Repair with AI**
is an explicit, billable action after an error; Tuilor never repairs or retries
AI requests automatically.

## Save and reopen

Run `./bin/tuilor`, choose **My Apps**, and select a name. Use `--apps-dir ./my-apps`
to choose another library. Names include filenames to distinguish duplicates.

Generated app directories contain a host-written manifest, source, starter,
module files, `data/`, and a cached executable in `.bin/`. Keep the directory
together when moving it. Source/module/starter/platform fingerprints and a binary
hash detect stale caches. Missing or stale caches rebuild without AI; build errors
are shown instead of triggering regeneration. These hashes are cache checks, not
code signatures or a security boundary.

Unsaved projects live in a private session directory. Save publishes a complete
private directory and retains trial data. Save failures preserve the candidate;
repeated Save does not create duplicates. Discard removes the candidate and
restores the prompt. Remaining session files are cleaned up when Tuilor exits.
The starter's data helpers use atomic replacement and preserve malformed data;
generated apps are instructed to report storage errors before changing displayed
state. Use one running instance per app; concurrent writes are not merged.

To try the hand-authored Go example without an AI request:

```sh
mkdir -p apps
cp -R examples/code-todo apps/code-todo
./bin/tuilor
```

Open **Todo (Go)**. Type a task and Enter; Tab focuses the list; arrows select,
Space completes, and Delete asks for confirmation. Esc always returns to Tuilor.
The example has its own module so it is excluded from the host's package build;
Tuilor supplies its starter and resolves its dependencies when opening it.

## CLI generation and MCP discovery

```sh
./bin/tuilor create "Build a notes app"
./bin/tuilor tools ./bin/testserver
./bin/tuilor version
```

CLI `create` now generates Go source, compiles it, saves the project, and prints
its directory. It does not launch the app; open it through My Apps. `--output-dir`
chooses its library, and `--timeout` changes the generation/preflight deadline.
Building has a separate 120-second deadline. A failed build never publishes an app.
The CLI prints the model and trust notice to stderr before the API request.

MCP tool and input-schema discovery remains available through `tools`. Start
only trusted local servers: discovery runs them with your OS permissions and is
not a sandbox. The test server exposes `greet`. Generated Go apps currently have
no built-in MCP connection or execution support.

## Retired JSON app format

The declarative JSON generator, schemas, renderer, local collection runtime, and
MCP form execution have been removed. Both Create App and CLI `create` now produce
Go projects. My Apps lists project directories containing `manifest.json`;
old flat JSON definitions and `app.json` collection bundles are ignored. Existing
files in your apps directory are left untouched. Root/creation `--server` flags
are no longer supported; use `tools <server-executable>` for MCP discovery.

JSON remains in use for source-response Structured Outputs, project manifests,
and app data. These describe transport and storage, not a fixed component renderer.

## Development

```sh
go fmt ./...
go build ./...
go vet ./...
```

There are no repository test files, by project choice. Verification uses temporary
HTTP/MCP fixtures, real local compilation, and terminal checks. No paid generation
is part of those checks. A real generation is initiated manually with Ctrl+G.

See [architecture](docs/architecture.md), [ADRs](docs/adr/), and
[repository instructions](AGENTS.md). [ADR 0006](docs/adr/0006-generated-go-apps.md)
records Go generation. [ADR 0007](docs/adr/0007-go-only-apps.md) records removal
of the legacy app runtime and the new model default.
