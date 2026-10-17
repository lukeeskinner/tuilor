# Tuilor architecture

Status: accepted custom Go application generation, 2026-10-17.
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
not terminal components. The source contract requests one final Go file with
one package/import section and newModel factory, not drafts, patches or the host
starter. A small compiling reference demonstrates the pinned widget and storage
APIs while leaving layout/domain choices open. The response budget is 16,384 tokens.
Structured Outputs constrains the envelope; parsing, source/import checks and
compilation establish separate properties. None proves safety or user intent.

Credentials are checked before work. A compatible installed Go 1.25.5+ toolchain
is checked before the paid request; Tuilor does not install or upgrade it. Source
generation/preflight has a 90-second context and compilation a separate 120-second
context. Provider retries are disabled. Cmds own contexts, cancellation functions
and completion channels delimit lifetimes, and only one operation runs at a time.
Esc cancels and waits for cleanup; canceled/failed work never triggers launch.

The home model preserves the multiline prompt. A fresh generation clears stale
source/review state; successful builds request terminal handoff automatically.
Errors preserve source and diagnostics in review. Explicit Repair with AI makes
one new paid request using source and bounded diagnostics, then builds again.
Existing trial data is copied into the new draft; failure/cancellation keeps the
previous project. Data compatibility is requested but not enforceable for arbitrary
source. There is no implicit paid retry or general migration/refinement engine.

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
atomic replacement, but generated code must follow the contract. Unlike the
retained local store, arbitrary Go apps have no host-enforced single-writer lock.

CLI create uses the same source generator/build pipeline, saves without launching,
and prints an assigned ID/run command. --output-dir, --name and --timeout remain.
run <id> --apps-dir opens cached/rebuilt Go apps after confirmation. MCP stays
optional under Settings; native connections and app/MCP execution are not added.

## Saved declarative compatibility

The previous local runtime remains available for kind:declarative manifests.
`internal/app` validates versioned collection/field/component/action definitions.
`internal/capability` describes local list/insert/update/delete, and the consuming
Executor interface in internal/tui separates the renderer from concrete storage.
The renderer supports one collection/page, text/list/form/button, and text/boolean/
choice/date fields. Those limits apply only to saved declarative apps, not new Go
apps. The unused definition generator, AI schema, draft publication and creation
screens have been removed. Runtime/storage tests continue to protect saved apps.

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

Repository tests use fake providers and cache fixtures to verify source contracts,
error/cancellation paths, explicit repair, source rejection, named/idempotent
publication, saved data and credential filtering. Declarative persistence/runtime
tests remain. Temporary mock-HTTP/PTY checks use real Go compilation and generated
executables to verify automatic opening, terminal restoration, save/reopen and
repair, without paid requests. Build/race/vet complement these checks. Paid model
quality, arbitrary domain correctness, and visual quality still need real manual
verification. Next integrations should have explicit credential/permission boundaries;
this change does not automatically implement MCP/native service access.
