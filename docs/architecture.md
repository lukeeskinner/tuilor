# Tuilor architecture

Status: Go-only apps with automatic launch and standalone MCP discovery, 2026-10-16.
[ADR 0001](adr/0001-initial-architecture.md) records the local-first modular Go
direction. [ADR 0002](adr/0002-app-preview.md) records the preview design;
[ADR 0003](adr/0003-form-execution.md) extends it with form execution;
[ADR 0004](adr/0004-home-generation.md) adds prompt, review, and explicit saving;
[ADR 0005](adr/0005-local-apps.md) adds local collections and persistent app bundles.
[ADR 0006](adr/0006-generated-go-apps.md) supersedes renderer expansion for new local
apps with source generation, compilation, and automatic launch.
[ADR 0007](adr/0007-go-only-apps.md) removes the legacy runtime and upgrades the default model.

## Generated Go app flow

```text
Create App -> credentials + Go preflight -> AI source envelope
-> project source validation -> prepare dependencies -> compile -> CLI launches app
-> child exits -> resume Tuilor -> Save / Run Again / Discard
My Apps -> source project -> valid cached binary OR bounded rebuild -> CLI launch
```

The concrete code-app package owns Spec, Project, toolchain preflight, source and
module validation, compilation, caching, publication, and subprocess launch. The
existing generator provider supplies source through Responses Structured Outputs;
JSON is a transport envelope here, not the runtime UI language. Code output has a
16,384-token budget. The default model is gpt-6.1-sol; TUILOR_MODEL overrides it,
and Create App shows the selected model. The existing Go SDK accepts the model
ID as a string and uses the same Responses/strict JSON Schema request. No model
fallback or billable retry is implicit.

The AI writes app.go and newModel() (tea.Model, error), while the host owns main.go,
module scaffolding, executable paths, and a manifest. The starter wraps the model
to deliver resize messages, reserve an exit footer, clip overflow, and handle Esc
and Ctrl+C. The generated model owns layout and behavior. It receives local JSON
load/save helpers which preserve malformed data and use atomic replacement. This
is a starter convention, not an enforcement boundary against arbitrary Go code.

Generation/preflight has a 90-second context; compilation and dependency resolution
have a fresh 120-second context. No context is stored in a model struct. Only one
operation runs at a time. Cancellation waits for cleanup and suppresses automatic
launch. Failures retain source and prompt. An explicit repair sends the prior
source and bounded diagnostics as data in one new paid request; never retry
implicitly. Go 1.25.5+ must be installed; Tuilor does not install toolchains.

The parent Bubble Tea program finishes before launch, restoring the terminal.
The CLI owns the child lifetime and terminal stdio, then resumes the retained
TUI model after success or failure. On Unix, forward resize signals received by
Tuilor to the child. Cancellation kills and reaps the direct child with bounded
I/O shutdown; arbitrary descendant trees are outside the guarantee. The launch
context is independent of generation/build deadlines. Cleanup of the entire
private session workspace belongs to the CLI, including cancellation and errors.

Pinned UI dependencies are Bubble Tea 1.3.10, Bubbles 1.0.0, and Lip Gloss 1.1.0.
Other dependencies must be explicitly requested and declared with exact versions.
Check module syntax and source imports before invoking Go tooling. Build with
cgo disabled, a local toolchain, no workspace, and no inherited Go build flags.
Commands use separate argv values; source never chooses filenames or executables.

A code manifest has kind go and format version 1, app name/dependencies, toolchain
version, build fingerprint, and binary hash. Source is stored only in app.go.
Fingerprints cover source, starter, module files, declared dependencies, platform,
and build Go version. Cache checks detect stale/changed artifacts, not malicious
projects. Cached apps run without Go or an API key; rebuilds need Go and may need
network access for declared pinned modules. New saves stage and rename a complete
private directory containing source, starter, modules, trial data, and the binary.
Save is idempotent. Failed saves retain the candidate; Discard removes the draft.
My Apps lists only Go project directories containing a regular manifest.json.
Flat JSON apps and old app.json bundles are ignored; their files are not deleted.

Source instructions require one final Go file, the pinned widget API signatures,
and exported JSON data fields. A small complete compiling app is included as
reference context for widget updates, transactional saving, scrolling, and resize
handling. The model receives the host helper signatures and lifecycle contract,
not the full main.go implementation; only one complete Go example is included. It is an example to adapt, not a fixed app or component interpreter. Structured Outputs validates the transport shape,
not Go correctness; malformed or uncompilable source remains blocked, with repair
requiring an explicit action. No source fragments are silently selected or merged.

Generated programs run with full user OS privileges. Creation explicitly states
that Generate compiles and runs AI-written code. Compiler and app environments
omit OpenAI credentials and Tuilor AI settings. This is not a sandbox; static
checks and compilation cannot establish application safety or correctness.
No new MCP integration inside generated programs is implemented in this milestone.

## CLI generation

CLI create uses the same provider and GenerateCode as the home screen. It checks
credentials and Go before a paid request, prepares a private temporary project,
compiles it under a fresh 120-second context, and publishes it through Project.Save.
It prints the saved directory on stdout and the model/trust notice on stderr.
It saves without launching; My Apps opens the project later. --output-dir and
--timeout remain available; --server is removed from both root and create.
Generation failure, build failure, and cancellation never publish an app.

## Package boundaries

- internal/generator: provider request contract, OpenAI Responses implementation,
  structured source generation, and explicit repair input.
- internal/codeapp: host starter, source/import validation, project metadata,
  compilation, cache validation, atomic publication, data copying, child launch.
- internal/tui: prompt editing, asynchronous generation/build/save lifecycle,
  project library, review, and terminal handoff requests.
- internal/cli: Cobra commands, session workspace ownership, terminal handoff,
  child lifetime, and standalone MCP tool discovery.
- internal/mcpclient: explicit local server connection and paginated tool/schema
  discovery using the official SDK. No tool-call entry point is exposed.

The former internal/app and internal/localapp packages, JSON generation prompts,
component renderer, collection mutations, and MCP form execution are removed.
This leaves one app creation/runtime path instead of parallel Go and JSON stacks.
Future UI behavior is generated Go; no generic component framework is needed.

## JSON still used for transport and storage

Responses Structured Outputs transports the name, source, pinned dependencies,
or an unsupported explanation. A manifest identifies a Go project and its cache.
Generated apps use JSON data through the starter's local persistence helpers.
None of these JSON uses defines terminal components or interprets UI behavior.
Saved legacy files are not migrated, opened, or overwritten.

## Verification

Use temporary local HTTP/MCP fixtures and real Go builds without API charges.
Verify model default/override wiring, Go-only listing, generation and explicit
repair, compilation failure, cancellation, save/reopen, data persistence, and
MCP discovery. Keep repository test files absent by user preference. Run go fmt,
go build, go test, go test -race, go vet, and diff checks. Paid generation remains
an explicit user-authorized action; fixture success does not establish model quality.
The 2026-10-16 reliability check used real generation/repair requests after user
authorization. After simplifying context, two fresh notes generations compiled;
one ran through Create App, automatic opening, saving, and reopening. This is a
small smoke check, not a guarantee for arbitrary prompts. See [ADR 0008](adr/0008-generation-source-context.md).
