# Tuilor

**Describe a terminal app. Generate it, build it, and use it.**

Tuilor generates Go applications with custom Bubble Tea layouts and behavior.
AI designs the app's code rather than choosing from a fixed component catalog.
Tuilor supplies the starter, local persistence helpers, build pipeline, terminal
handoff, and project storage. Successful home-screen builds open automatically.
Saved apps reopen without regeneration or an AI request.

## Build

Use Go 1.25.5 or later:

```sh
go build -o bin/tuilor ./cmd/tuilor
```

Go is required for generation and rebuilding. A compatible cached app can run
without Go. Initial builds may download pinned dependencies through Go tooling.

## Generate and open an app

Set `OPENAI_API_KEY` in the shell that launches Tuilor. Do not put keys in prompts,
source files, or version control. In zsh:

```zsh
read -rs "OPENAI_API_KEY?OpenAI API key: "
export OPENAI_API_KEY
echo
./bin/tuilor
```

1. Choose **Create App** and describe its behavior, layout, and keyboard controls.
2. Enter inserts a newline; **Ctrl+G** starts one billable OpenAI request.
3. Tuilor checks Go before the paid request, generates source, and compiles it.
4. A successful build opens automatically. **Esc** or **Ctrl+C** returns to Tuilor.
5. Choose **Save**, **Run Again**, or **Discard**. Save retains source and trial data.
6. After saving, Esc opens My Apps with the new app selected.

**Generate builds and runs AI-written code with your OS permissions. It is not
sandboxed.** Only request and run apps you trust. Build and app environments omit
OpenAI credentials and Tuilor AI settings, but programs can still access other
files and resources available to your account. Saved-app launches ask for explicit
trust confirmation before executing or rebuilding code.

Try different local apps: a notes editor, a focus timer, a file-backed tracker,
or a calculator with a custom layout. New apps are not restricted to forms, one
page, one collection, or task management. Compilation proves buildability, not
correct behavior, safety, or visual quality. Real API integrations still need real
credentials and supported wiring; Tuilor does not invent working connections.
MCP-backed apps and managed native connectors remain future work.

The AI writes `app.go` with `newModel() (tea.Model, error)`. The host owns main.go,
startup/exit, resize delivery, and loadData/saveData helpers. Generated code owns
the domain state, interactions, and responsive layout. Instructions require
scrolling, focus visibility, narrow layouts and very-small-terminal fallbacks.
The default modules are the standard library, Bubble Tea 1.3.10, Bubbles 1.0.0,
and Lip Gloss 1.1.0. Additional modules require an explicit user request and exact
pinned versions. Tuilor does not install servers automatically; build and launch
commands use separate arguments without shell interpolation.

The existing `gpt-6.1-sol` model default and `TUILOR_MODEL` override are retained.
The model must support Responses and strict Structured Outputs. JSON transports
the source and metadata; it does not define the generated app's interface.
Generation/preflight has a 90-second deadline. Dependency preparation and
compilation have a separate 120-second deadline. Go toolchain upgrades and cgo
are disabled. Esc cancels and waits for cleanup; canceled/failed builds never open.

Errors retain the prompt and candidate. **Repair with AI (billable)** sends the
previous source and diagnostics in one explicit request, then compiles again.
Tuilor never retries or repairs paid requests automatically. A failed repair keeps
previous trial data; repairs are instructed to preserve its format, but there is
no general schema-migration guarantee for arbitrary generated code.

The colorful intro and creation workspace remain. Press any key to skip startup;
`TUILOR_REDUCED_MOTION=1` skips it and `NO_COLOR=1` disables colors. `q` remains
text inside the prompt and generated inputs. The starter reserves Esc/Ctrl+C for
returning to Tuilor.

## CLI creation and saved apps

```sh
./bin/tuilor create "Build a focus timer with keyboard controls" --name focus-timer
./bin/tuilor run focus-timer
```

CLI create generates, validates, compiles and saves **without launching**. It
prints the assigned ID and exact run command. --name requests a stable ID and
rejects existing destinations before a paid request, and again during publication.
Omit it for a host-assigned ID. --output-dir chooses the library; use the same
path through run --apps-dir or the root --apps-dir. --timeout changes only the
generation/preflight deadline, not the separate build deadline.

A saved Go app directory contains manifest.json, app.go, main.go, pinned module
files, data/, and a cached executable in .bin/. Keep it together when moving it.
Source/starter/module/platform fingerprints and binary hashes detect stale caches;
they are not signatures or a sandbox. Missing/stale builds rebuild without AI,
requiring Go and possibly dependency downloads. Build failures are shown rather
than triggering automatic regeneration. Saved Go apps are labeled **Go app**.

Unsaved projects live in a private session workspace. Save stages a complete
sibling directory and renames it into place; repeated Save does not duplicate.
Failed saves preserve the candidate, and Discard restores the prompt. The CLI
cleans session files on exit. Starter data helpers atomically replace JSON and
preserve malformed files. Generated code must use these helpers correctly; they
are conventions, not enforcement against arbitrary code. Use one instance per
Go app: generated programs do not acquire the declarative store's writer lock.

Try a hand-authored source app without a paid request, using an unused destination:

```sh
mkdir -p apps
cp -R examples/code-todo apps/code-todo
./bin/tuilor run code-todo
```

Opening it asks for trust and builds the missing cache. Saved declarative bundles
from the previous milestone remain supported through My Apps and run. Their
local runtime, storage, locks and validation are retained for compatibility;
new Create App and CLI create no longer generate them. The retired JSON generator,
its AI schema and unsaved-JSON creation flow have been removed. No existing app
files are migrated or deleted.

## MCP connections and browser sign-in



Choose **Settings → Advanced connections (MCP)**. This works without an OpenAI API key.

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
or tool calls**. Generated apps do not use these connections yet. Native integrations and
explicitly approved MCP-backed actions remain future work.

## Standalone discovery

```sh
go build -o bin/testserver ./cmd/testserver
./bin/tuilor tools ./bin/testserver
./bin/tuilor version
```

MCP is optional. Discovery initializes and lists tools/schemas, then closes the
session; it does not call tools. Only start trusted executables with separate
arguments. Existing MCP settings and OAuth behavior remain unchanged.

## Development

```sh
go fmt ./...
go build ./...
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

Repository tests cover source-response validation, explicit repair inputs,
preflight failures, invalid-source rejection, cancellation, atomic publication,
credential-environment filtering and saved declarative compatibility. Temporary
mock-HTTP/PTY fixtures exercise real compilation and terminal handoff without a
paid API. Live generation and actual app quality remain separate manual checks.

See [architecture](docs/architecture.md), [ADR 0011](docs/adr/0011-custom-go-default.md),
and [repository instructions](AGENTS.md). ADR 0011 supersedes ADR 0010's choice
of declarative generation as the default, retaining its saved-app compatibility.
