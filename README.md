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
2. Enter inserts a newline; **Ctrl+G** starts one billable OpenAI request, which may research public documentation (also billable).
3. Tuilor checks Go before the paid request, generates source, and compiles it.
4. If connections are needed, approve a saved connection or add one and sign in. A successful, connected build opens automatically. **Esc** or **Ctrl+C** returns to Tuilor.
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
GitHub and custom REST APIs are supported through the host request helper. MCP-backed generated apps remain future work.

The AI writes `app.go` with `newModel() (tea.Model, error)`. The host owns main.go,
startup/exit, resize delivery, and loadData/saveData helpers. Generated code owns
the domain state, interactions, and responsive layout. Instructions require
scrolling, focus visibility, narrow layouts and very-small-terminal fallbacks.
The default modules are the standard library, Bubble Tea 1.3.10, Bubbles 1.0.0,
Lip Gloss 1.1.0, and the host JSON Schema validator 0.4.3. Additional modules require an explicit user request and exact
pinned versions. Tuilor does not install servers automatically; build and launch
commands use separate arguments without shell interpolation.

The existing `gpt-6.1-sol` model default and `TUILOR_MODEL` override are retained.
The default model uses low reasoning effort to fit interactive generation's time
budget. Other configured models retain their provider defaults.
The model must support Responses and strict Structured Outputs. JSON transports
the source and metadata; it does not define the generated app's interface.
Generation/preflight has a 90-second deadline. Dependency preparation and
compilation have a separate 120-second deadline. Go toolchain upgrades and cgo
are disabled. Esc cancels and waits for cleanup; canceled/failed builds never open.

Errors retain the prompt and candidate. **Repair with AI (billable)** sends the
previous source and diagnostics in one explicit request, then compiles again.
Tuilor never retries or repairs paid requests automatically. A failed repair keeps
previous trial data. New apps declare a persistent-data schema; revisions must keep
it unchanged. Structural validation does not prove behavioral compatibility.

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

New saved apps use a version 2 host manifest with an active revision:

```text
apps/<app-id>/
  manifest.json
  revisions/<revision-id>/
    manifest.json
    app.go / main.go / runtime.go
    go.mod / go.sum
    data.schema.json
    data/state.json
    .bin/app
```

Keep the directory together when moving it. Nonsecret connection bindings are saved
with each revision; credentials stay in your OS credential store. Cache fingerprints
cover source, host helpers, contract, modules and platform. Missing/stale builds
rebuild without AI. Existing flat Go projects still open without migration.

Unsaved projects and trial data live in a private session workspace. Save publishes
a complete directory atomically; repeated Save does not duplicate. Failed saves
preserve the candidate. The CLI cleans unsaved files on exit. Host storage helpers
validate JSON and atomically replace it, preserving malformed or invalid data.
An app-level writer lock excludes competing Tuilor launches and publications.
Generated Go still has full OS permissions and can bypass helper conventions.

## Connections

Open **Settings → Connections → Add integration**:

- **[GH] GitHub:** browser device sign-in or hidden personal access token entry.
  Browser sign-in requires the maintainer's registered OAuth application with device
  flow enabled: set `TUILOR_GITHUB_CLIENT_ID` to its public client ID. No client
  secret or hosted backend is used. The default browser scopes cover public
  repositories; for private access use a token with the appropriate repository and
  organization permissions. Token entry works without a client ID.
- **[+] Custom API:** enter a name and base URL, then choose none, bearer, named
  header, or named query-parameter authentication. HTTPS is required except for
  loopback development servers. Other services' browser OAuth flows are unsupported.

Settings expose Edit, Check access, Reconnect / Sign in, Sign out and confirmed
Remove. Custom API status means **Configured**, not verified live functionality.
GitHub checks validate account identity. Sign-out removes local credentials;
provider-side authorization may remain. `i` toggles optional Nerd Font icons; text
badges are the default and service names are always visible. No font is installed.

Nonsecret settings are in `os.UserConfigDir()/tuilor/integrations.json`. Tokens use
the OS credential store, without a plaintext fallback. Unavailable or locked stores
produce an error. Damaged configuration is reported and never reset automatically.

Try:

```sh
./bin/tuilor create "Build a GitHub issue triage workspace: select a repository, browse and filter real issues, and let me change labels and assignees" --name issue-triage
./bin/tuilor run issue-triage
```

The app proposes connections; you approve the binding before launch. Saved bindings
are reused and refreshed as needed, without AI. GitHub permission limitations and
API errors remain visible. No MCP subprocess is needed.

The host `apiRequest` helper returns a Bubble Tea command. `APIResultMsg` reports
status, bounded body, pagination/rate-limit metadata and safe errors. Requests stay
within the approved API destination. Every non-GET request requires confirmation
of its exact method, endpoint, headers and JSON body. Credentials are hidden. Esc
cancels without sending; writes are never retried automatically. A timeout may
mean a write succeeded, so refresh before trying again.

Only selected integration credentials reach the app at launch in transient
configuration, removed from its environment on startup. They are never sent to the
model or compiler, or written into a project. This helper is **not a sandbox**:
trusted Go source can access OS resources and bypass its conventions. Reopening
requires neither an OpenAI key nor an AI request; stale builds still require Go.

## Modify and restore an app

After opening a newly generated saved app, exit to its review screen and choose
**Modify with AI (billable)**. For example: “Add a repository filter and group
issues by label.” Enter inserts newlines; Ctrl+G makes one explicit paid request.

The candidate builds separately and trials a copy of current local data. Choose
**Keep Revision**, **Run Again**, or **Discard**. Keep publishes the complete
revision and atomically switches the active pointer. If saved data changed during
the trial, publication fails rather than overwriting newer data.

**Restore Previous Version** asks for confirmation, restores the previous code,
and carries forward current local data under the unchanged schema. It never undoes
GitHub/API changes. All trial API writes are real and still require confirmation.
Saved records and credentials are not sent to AI. Schema changes and migrations
are unsupported. Existing apps without a data contract still run; they are not
silently enrolled in schema-backed modification.

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
or tool calls**. Generated apps do not use these MCP connections. Additional native integrations and
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
go vet ./...
git diff --check
```

Repository test files have been removed by user preference. Build and vet checks
remain, but there is no automated regression coverage. Temporary mock-HTTP/PTY
fixtures can verify compilation, terminal handoff and persistence without a paid
API. Live generation and actual app quality require manual checks.

See [architecture](docs/architecture.md), [ADR 0011](docs/adr/0011-custom-go-default.md),
and [repository instructions](AGENTS.md). ADR 0011 supersedes ADR 0010's choice
of declarative generation as the default, retaining its saved-app compatibility.
