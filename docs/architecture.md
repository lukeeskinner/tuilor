# Tuilor architecture

Status: custom Go generation with approved API connections and schema-preserving revisions, 2026-10-17.
[ADR 0012](adr/0012-connected-workspaces.md) extends the source pipeline.
[ADR 0011](adr/0011-custom-go-default.md) supersedes ADR 0010's default-generation
choice. The user chose custom layout and behavior over a fixed component catalog.
Saved declarative bundles remain compatible; no user projects are migrated/deleted.

## Source generation is the default

```text
prompt → credentials + Go preflight → one AI source response
       → host project → source/import validation → compile
       → CLI terminal handoff → app runs → return to Tuilor
       → Save / Run Again / Discard
saved app → trust confirmation → cached build or rebuild → launch without AI
```

AI implements app.go and newModel() (tea.Model, error). Its model owns domain
state, Update/View behavior, layout and keyboard interaction. It is not limited
to the local definition's fields, actions or components. Tuilor owns the starter,
filenames, module setup, persistence helpers, compilation, cache and process
lifecycle. This removes the component ceiling but adds source/dependency/build
failures and execution of code with the user's OS privileges. This is not a sandbox.

The concrete generator reuses Provider, the official OpenAI Go SDK, Responses,
strict Structured Outputs and the existing gpt-6.1-sol/TUILOR_MODEL configuration.
The JSON envelope carries success/name/source/dependencies or unsupported/reason,
not terminal components. New responses also carry integration requirements, documentation URLs, a data-schema JSON string and an empty initial-state JSON string. The source contract requests one final Go file with
one package/import section and newModel factory, not drafts, patches or the host
starter. Three compiling reference compositions demonstrate the pinned widget,
storage and styling APIs while leaving layout/domain choices open. The response budget is 16,384 tokens.
Structured Outputs constrains the envelope; parsing, source/import checks and
compilation establish separate properties. None proves safety or user intent.

Credentials are checked before work. A compatible installed Go 1.25.5+ toolchain
is checked before the paid request; Tuilor does not install or upgrade it. Source
generation/preflight has a 90-second context and compilation a separate 120-second
context. Provider retries are disabled. Cmds own contexts, cancellation functions
and completion channels delimit lifetimes, and only one operation runs at a time.
Esc cancels and waits for cleanup; canceled/failed work never triggers launch.

The default GPT-6.1 Sol model uses its supported low reasoning effort for this
interactive budget; other model families retain their provider defaults. Source
instructions favor concise implementations and targeted API research. Local apps
need no documentation lookup. Documentation entries are HTTPS URLs constrained by
the response schema and checked locally. A generation deadline returns to the
preserved prompt with an explicit timeout message, without another paid request.

The home model preserves the multiline prompt. A fresh generation clears stale
source/review state; successful builds request terminal handoff automatically.
Errors preserve source and diagnostics in review. Explicit Repair with AI makes
one new paid request using source and bounded diagnostics, then builds again.
Existing trial data is copied into the new draft; failure/cancellation keeps the
previous project. Data compatibility is requested but not enforceable for arbitrary
source. There is no implicit paid retry. Schema-preserving refinement is described below; migrations remain unsupported.

## Building, launching and saving

`internal/codeapp` parses Go syntax, checks package/factory/startup ownership,
rejects compiler directives and undeclared imports, and owns module construction.
Provided dependencies stay pinned to Bubble Tea 1.3.10, Bubbles 1.0.0 and Lip Gloss
1.1.0. Additional packages need explicit requests and exact declared versions.
Builds use separate argv values, cgo off, local toolchain, no workspace, and no
inherited build flags/tool-exec overrides. Generated source cannot choose compiler
arguments or executable paths. Build and app environments omit OPENAI_, TUILOR_
and AZURE_OPENAI_ variables. Other OS privileges remain available to the app.

The host starter creates Bubble Tea, restores the terminal, intercepts Esc/Ctrl+C,
forwards resize dimensions with its footer reserved, and clips overflow. Instructions
ask the app to adapt to wide/narrow/small surfaces, scroll overflow and keep focus
visible. The model still must implement those behaviors correctly. Startup and
Create App keep the adaptive palette, reduced-motion setting and multiline editor;
the visible generation notice explicitly authorizes compiling and running code.

Before launch, the parent Bubble Tea session ends and restores its terminal.
`internal/cli` runs the child with terminal stdio, then resumes the retained model
on exit/crash. Launch lifetime is independent of generation/build timeouts.
Shutdown cancels and reaps the direct child with bounded I/O waits; arbitrary
descendant process trees are outside the guarantee. Unix resize signals are
forwarded. Saved-app launches and Run Again ask for explicit code trust first.

Projects contain a kind:go manifest, app.go, host main.go, go.mod/go.sum, data/,
and .bin/. Cache fingerprints cover source, starter, modules, dependencies,
platform and Go version; a binary hash detects changed artifacts. Caches are not
code signatures. Compatible caches run without Go/API credentials. Stale/missing
builds rebuild from source and pinned dependencies without AI.

Drafts live in the CLI's private session workspace. Save stages source, module
files, trial data and binary in a temporary sibling, then renames. On success the
Project changes to saved state, making repeated saves idempotent. Save failures
preserve the candidate. CLI --name optionally selects a validated, non-overwriting
stable ID; otherwise the host assigns a random Go ID. User-controlled IDs, not AI
names or source, determine paths. Discard removes drafts; final workspace cleanup
belongs to the CLI. The starter's JSON helpers preserve corrupt data and use
atomic replacement, but generated code must follow the contract. Tuilor launches/publications now acquire the shared OS file lock. Arbitrary code can still bypass host helpers or spawn unrelated writers.

CLI create uses the same source generator/build pipeline, saves without launching,
and prints an assigned ID/run command. --output-dir, --name and --timeout remain.
run <id> --apps-dir opens cached/rebuilt Go apps after confirmation. MCP stays
optional under Settings → Advanced. Native API connections are separate; app/MCP execution is not added.

## Saved declarative compatibility

The previous local runtime remains available for kind:declarative manifests.
`internal/app` validates versioned collection/field/component/action definitions.
`internal/capability` describes local list/insert/update/delete, and the consuming
Executor interface in internal/tui separates the renderer from concrete storage.
The renderer supports one collection/page, text/list/form/button, and text/boolean/
choice/date fields. Those limits apply only to saved declarative apps, not new Go
apps. The unused definition generator, AI schema, draft publication and creation
screens have been removed. The saved runtime and storage behavior remain supported.

`internal/localapp` owns saved-bundle loading, validated random record IDs and atomic
record replacement. Its per-app OS lock prevents competing Tuilor writers.
It preserves corrupt data and pending forms on errors and accepts displayed
mutations only after successful persistence. It does not claim synchronization
with arbitrary external edits or guaranteed recovery from every power failure.
My Apps distinguishes manifest kinds and loads each with its own runtime. Source
projects are now labeled Go app, not legacy, because source is the default format.
Retired pre-pivot JSON formats are still not silently reinterpreted.

## MCP connection management



[ADR 0009](adr/0009-mcp-connections.md) records connection management and OAuth.
Settings → Advanced connections (MCP) is a concrete Bubble Tea screen, not a catalog. Users configure either
a trusted stdio executable with separate argv values or a Streamable HTTP URL.
The connections package owns a versioned nonsecret document at
os.UserConfigDir()/tuilor/mcp.json, validated before every atomic replacement.
It assigns IDs independently of names; duplicate names are distinguishable by ID.
Corrupt configuration is reported and preserved. Concurrent editors are outside
the initial guarantee; one instance should edit settings at a time.

The add form defaults to Remote service / Browser sign-in and uses plain-language
labels instead of transport/authentication enum names. Only essential fields are
initially visible. Advanced settings reveals local argv/folder/environment or OAuth
client/callback options; hiding it preserves values and does not change storage or
validation. Naming a connection does not create an integration or infer a provider.

The mcpclient package exposes Discover and SignIn but no tool-call entry point.
Discover initializes, iterates paginated tools (including input/output schemas),
and closes the session. Local processes receive a minimal runtime environment
plus explicitly selected variable names, excluding OPENAI_, AZURE_OPENAI_, and
TUILOR_ variables. Server stderr is discarded; remote SDK errors are translated
to fixed messages to avoid leaking response bodies or credentials. Metadata is
bounded to 1,000 tools / 2 MiB and escaped before display. There are no sampling,
elicitation, resource, or execution callbacks.

The TUI owns operation cancellation/completion signals and request IDs; contexts
belong to commands, never model fields. A check has a 10-second deadline, closes
its session, and reaps its direct local subprocess. One operation runs at a time.
Esc cancels and suppresses late screen updates. CLI/model shutdown allows bounded
cleanup, including the HTTP SDK's bounded session-termination request. There is
no automatically running connection after a successful check.

SignIn is a separate, explicit five-minute operation. The SDK's authorization-code
handler owns metadata discovery, registration, PKCE, and state/issuer validation.
Tuilor owns a temporary loopback listener, validates callback state before
accepting a response, opens the system browser without a shell, and exposes a
manual URL fallback. The listener closes on every completion path. HTTP endpoints
require HTTPS except for loopback development; redirects are not automatically
followed, preventing bearer credentials from being forwarded to another endpoint.
Public pre-registered clients and dynamic registration are supported. Hosted
client metadata, confidential clients, and downstream service-account setup are
deferred. Check Connection never starts interactive authorization.

The connections credential adapter uses pinned zalando/go-keyring v0.2.8 and
existing oauth2 types. The narrow SecretStore interface permits temporary fixture
substitution; production has no plaintext fallback. Tokens and OAuth refresh
configuration are bound to a hash of connection ID, endpoint, and authentication
settings. Expired tokens refresh under the discovery operation's context; the
replacement token is persisted before use. Small immutable credential chunks
accommodate native keyring item limits, with a final pointer published only after
all chunks are written. Failed staging preserves the previous login. Sign-out
disables the pointer before removing chunks; cleanup failures are visible.
Credential-store calls are synchronous native operations and may require OS
interaction; cancellation is checked between calls. A timed-out host shutdown
reports cleanup failure rather than claiming that an OS prompt was canceled.

Changing an OAuth binding removes its old credentials before publishing settings.
A subsequent configuration write failure may require signing into the unchanged
connection again, but cannot transfer credentials to the new endpoint. Provider
authorization and local sign-out are distinct. Authentication is not tool-call
approval, and these connections are not yet passed to generation or child apps.

## Package boundaries and verification

- internal/generator: source generation, explicit repair and shared provider/OpenAI
  transport.
- internal/codeapp: starter, source/dependency validation, builds/cache, atomic
  project publication, data copying, process environment and launch.
- internal/tui: prompts, async preparation/build/save/review, library/settings,
  saved-code trust confirmation, and retained declarative runtime.
- internal/cli: Cobra create/run/tools, session workspace, terminal and child ownership.
- internal/app, internal/capability, internal/localapp: saved declarative compatibility.
- internal/connections, internal/mcpclient: optional configuration, OS credentials,
  OAuth and MCP discovery, without a tool-execution bridge.

Repository test files and their exclusive tracker fixture were subsequently removed
at the user's request. There is no automated regression coverage. Formatting,
build and vet checks remain; temporary mock-HTTP/PTY fixtures can check compilation,
opening, terminal restoration, save/reopen and repair without paid requests.
Paid model quality, arbitrary domain correctness and visual quality require manual
verification. Additional integrations must preserve the credential and approval boundaries below. No app/MCP bridge is implemented.

## Connected workspaces (ADR 0012)

The integration picker contains GitHub and Custom API only. Text badges are the
portable default; a persisted opt-in enables Nerd Font icons with names retained.
Presets describe connection setup, not a fixed set of generated screens or actions.
MCP configuration remains isolated and its discovery behavior is unchanged.

`internal/integrations` owns concrete configuration, OS credentials, GitHub device
OAuth/refresh, and runtime credential resolution. Config is nonsecret, versioned,
validated and atomically replaced. Tokens are endpoint/authentication-bound,
chunked keyring entries; staging publishes its pointer last. There is no plaintext
fallback. Native keyring calls may block on OS prompts; context checks cannot
forcibly interrupt the OS and bounded shutdown must report that limitation.

GitHub browser authorization requires an external maintainer registration and its
public client ID. Device polling follows oauth2's expiry/interval/slow-down rules.
Identity is validated after sign-in and refresh; refreshed credentials are saved
before use. Default scopes cover public repositories. Hidden token entry supports
private access when provider permissions permit. No GitHub CLI dependency exists.
Custom APIs support none, bearer, header and query credentials, with no invented
OAuth support or generic live-health endpoint.

Generation receives only nonsecret connection descriptions. Optional public web
research is bounded to five tool calls within the original 90-second request.
Search output is reference material, never authority to execute or grant access.
The source envelope stays strict; local validation checks requirements, schema,
initial state, source and dependencies. Failed/canceled builds never launch. The
host owns binding approval after compilation, without another AI request.

The compiled host runtime implements apiRequest(APIRequest) tea.Cmd and
APIResultMsg. Mutable arguments are cloned before confirmation. GET executes;
POST/PUT/PATCH/DELETE require host confirmation of the frozen destination,
headers and JSON body. Request destinations stay under the approved base URL,
credential overrides and redirects are rejected, responses are bounded to 1 MiB,
and requests have 20-second deadlines. App exit cancels outstanding work. Writes
never retry; uncertain outcomes tell the user to refresh before another attempt.
Pagination/rate-limit metadata is exposed without unbounded automatic crawling.
API text is untrusted: generation instructs sanitizing it with the host apiText
helper. Credentials are redacted from helper responses and errors.

The launcher strips AI configuration and injects only approved credentials into
transient child configuration. The starter consumes and removes that environment
entry before newModel. Credentials never enter compiler environments, saved
projects, prompts or diagnostics. They are available to trusted generated code:
this design is explicitly not a broker or sandbox. Helper confirmations do not
prevent bypass by arbitrary source. Source generation remains unrestricted in UI
layout, rather than reviving declarative UI generation.

## Data contracts and revision publication

New specs include JSON Schema and an empty initial state. The existing pinned
jsonschema-go v0.4.3 validates both host preflight and generated storage helpers.
External references are rejected without network resolution. Schema validation
protects structure, not semantic understanding, lossless domain behavior or safety.

Version 2 root manifests point to an active project under revisions/<host-id> and
retain a previous pointer. Each revision contains source/starter/runtime, pinned
modules, schema, local data, cached executable, and a version 1 project manifest.
Old flat version 1 projects remain runnable, without inferred schemas or migration.
Fingerprints cover current host helpers and contract metadata as well as files.

Modify with AI sends source, the unchanged schema and requirements; it sends no
saved records or credentials. A draft copies data while holding the source app's
writer lock and captures the active revision and data hash. Trials run independently
from the saved version, but external writes remain real and confirmed. Keep stages
a complete revision, checks that active source/data are unchanged, then atomically
replaces the root pointer. Failure preserves active state and the candidate.

Restore builds previous source with current compatible data in a draft and uses
the same publication path. It cannot undo provider-side actions. The shared
internal/filelock OS implementation is also used by retained local storage.
Tuilor owns locks during launch, snapshot copying and publication; stale trials
are rejected rather than merged. Local data migrations and concurrent merging
are deliberately unsupported.

## Guided connections and portable UI helpers (ADR 0013)

Integration requirements may include setup documentation/credential URLs, a credential
label, permission suggestions and instructions. Strict generation requires these
nonsecret fields; older saved projects without them remain valid. URLs and guidance
are bounded and validated locally, escaped on display and treated as untrusted.
The connection screen pins the actual destination, keeps errors visible, scrolls
instructions and focuses only the credential or Connect action. External setup links
show the parsed host and open only after explicit selection. GitHub uses the existing
host-written device flow; custom OAuth is never generated or simulated.

After explicit connection approval, Store.Connect authenticates and publishes
configuration, cleaning new credential entries on publication failure. Failures keep
inputs and the compiled candidate. User-approved endpoint/auth changes live in the
project's connection_overrides, separately from AI requirements and the build key.
Runtime resolution validates effective requirements and saved connections before
passing credentials to the child. Revision trials retain approvals only when the
original integration proposal is unchanged. No extra AI request is needed to connect.
The OAuth HTTP transport binds operation cancellation even when the pinned SDK's
initial device-code request lacks a context; response close releases that binding.

New source specs declare ui_version:1. Codeapp writes ui.go containing pure formatting
helpers for palette, panels, tabs, compact tables, status and wrapped keyboard hints.
The file is copied into saved/revision projects and included in compilation and cache
fingerprints. The ui prefix followed by an uppercase letter is reserved for these
helpers only in opted-in projects. Existing version-zero projects keep their own
source/helper behavior; repairs/revisions preserve their helper version. A future
incompatible contract must add a new version. Generated code remains free to compose
any layout and owns widget state, focus, scrolling, responsive behavior and data.

Generation receives the exact UI helper contract plus three compiling composition
examples: a review desk, a lane board and an editable document rail. Instructions
favor distinctive palettes, useful density, clear hierarchy and keyboard paths over
an arbitrary source line budget. NO_COLOR/TERM=dumb and small-terminal fallbacks
remain part of the contract. There is no automatic paid visual critique or repair.
