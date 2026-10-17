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

Startup shows a brief animated wordmark, then the colorful home menu. Press any
key to skip the intro. Small terminals get a compact layout; light and dark
terminal backgrounds have matching palettes. Use `TUILOR_REDUCED_MOTION=1` to
skip startup animation, or `NO_COLOR=1` to disable color. The intro makes no AI
requests or server connections and does not replay after exiting an app.

Create App uses a centered multiline brief editor with the current model, billing
notice, execution notice, and shortcuts kept visible. Compact terminals use a
smaller layout. Generation, compilation, cancellation, saving, and review share
the same studio palette; progress labels reflect the actual operation.

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

## MCP connections and browser sign-in

Choose **MCP servers** on the home screen. This works without an OpenAI API key.

1. Press **n** to add a connection. New connections start as **Remote service**
   with **Browser sign-in**. Give it a name and paste the MCP server URL provided
   by your server provider. A name such as Google is a label, not a built-in
   integration, and an ordinary website URL is not an MCP endpoint.
2. Tab/Shift+Tab changes focus; left/right changes Connection and Sign-in.
   Choose **Local program** for an installed MCP server. Enter its program path;
   for the test server, use the absolute path to `bin/testserver`.
3. Optional settings stay under **Advanced settings** (focus it and press Enter).
   Local programs can specify arguments **one per line**, a working folder, and
   environment-variable names. Browser sign-in exposes a client ID and callback
   port only when Advanced is open. Collapsing Advanced preserves entered values.
   Arguments are literal values, never a shell command. **q** types normally.
   Remote services can also use **No sign-in** or **API token**; API token stores
   a variable name such as `MY_MCP_TOKEN`, whose value you set before launching
   Tuilor. HTTPS is required except on loopback; never paste tokens into a URL.
4. **Ctrl+S** saves. Enter opens a connection's actions. OAuth connections offer
   **Sign In** and **Sign Out**. Sign In opens the browser; the displayed URL also
   works if browser launch fails. Check Connection never opens a browser.
5. Choose **Check Connection**, then **View Tools** to inspect descriptions and
   input/output schemas. Page Up/Page Down scrolls tool metadata.

Local checks ask before starting the executable. It runs with your OS permissions,
not in a sandbox. Only a small runtime environment and explicitly selected
variables are forwarded; OpenAI credentials and Tuilor settings are excluded.
Do not put secrets in command arguments. Server stdout is reserved for MCP;
raw stderr and remote error bodies are not displayed or logged.

Connections live in `os.UserConfigDir()/tuilor/mcp.json` (on macOS,
`~/Library/Application Support/tuilor/mcp.json`). This contains nonsecret settings
and host-assigned IDs. Failed saves and corrupt configuration preserve the files.
Use one Tuilor instance when editing connection settings; concurrent edits are
not merged. **Remove** asks for confirmation. Edits that change an OAuth endpoint
or authentication settings clear the old login before saving new settings.

OAuth supports public clients with PKCE and either a pre-registered client ID or
server-provided dynamic registration. The callback is
`http://127.0.0.1:<port>/callback`; port `0` selects a free port. Set a fixed port
when a pre-registered client requires an exact redirect URL. Servers requiring
confidential-client credentials or only hosted client metadata documents are not
supported yet. Signing into an MCP server does not automatically implement its
separate downstream-account setup, if any.

OAuth tokens and refresh settings are stored in the OS credential store using
`github.com/zalando/go-keyring v0.2.8`, never in project source or configuration.
macOS uses Keychain; supported Windows/Linux environments use their native
credential stores. A locked or unavailable store is an error, with no plaintext
fallback. Sign Out removes Tuilor's local login; revoke provider-side access
separately if desired. Expired tokens refresh during a check; invalid refresh
credentials require Sign In again.

Checks have a 10-second deadline; browser sign-in allows five minutes. Esc cancels
and returns; Ctrl+C cancels and exits. Cleanup is bounded, late results do not
reopen screens, and OS credential-store calls can still require an OS prompt.
Each check closes its session, so **Last check passed** describes the last check,
not a continuously running connection. Tools are rediscovered on each check.

This milestone performs initialization and tool discovery only: **no AI requests
or tool calls**. Saved Go apps do not use these connections yet. Selecting them
during generation and adding explicit tool-call approval is the next milestone.

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

Standalone MCP tool and input-schema discovery remains available through `tools`. Start
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
