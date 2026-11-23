# ADR 0013: Prompt-first connections and portable styling helpers

Status: accepted, 2026-10-17.

## Context

GitHub browser authentication works, but configuring an unfamiliar API still feels
like a protocol editor. Generated apps have custom layouts but their plain reference
and broad styling instructions leave visual quality inconsistent. The user chose
guided API setup from the app prompt and creative, information-rich interfaces
with a clear keyboard path. Arbitrary Go composition remains the application model.

## Decision

Extend integration requirements with bounded, nonsecret setup guidance. One existing
generation request researches public documentation and produces source plus endpoint,
authentication and permission suggestions. Uncertain API facts or unsupported OAuth
receive an explanation, asking for clearer requirements/documentation where helpful.
Only GitHub's host-written OAuth is supported; the AI never supplies OAuth code,
credential values, registration endpoints or execution permission.

After compilation, offer matching connections or a focused Connect screen. Browser
sign-in starts directly; custom authentication asks for one hidden credential.
Advanced exposes the full settings. Host-approved destination/auth overrides are
stored separately from generated requirements with connection bindings. They do not
change source or require regeneration/recompilation. Changed integration proposals
in a revision require fresh approval. Credentials remain in the existing OS store.
Setup links display their actual host and open only on explicit selection; they
are untrusted suggestions, not verified vendor identity.

New source projects declare UI helper version 1. Tuilor copies a small ui.go file
with pure formatting functions; generated code owns state, layout and interaction.
The generator receives the exact helper contract and three compiling compositions.
Helpers participate in publication, cache fingerprints and source ownership checks.
Existing projects with no UI version receive no helpers, including during repair.
Future incompatible helper contracts require a new version, not silent upgrades.
User styling requests take precedence. There is no new component runtime or design
request after generation; paid refinement remains explicit.

## Alternatives and consequences

- More service presets would simplify only listed services. Guided REST setup reuses
  supported auth mechanisms without claiming universal browser authorization.
- Stronger prompts alone are smaller but cannot supply consistent formatting APIs.
  Portable helpers reduce repeated styling work without limiting arbitrary layouts.
- A fixed renderer would offer stronger visual guarantees but restore the component
  ceiling the user rejected. Compiling custom Go still cannot prove aesthetics,
  behavior or safety. Review actual terminal output and interaction.

The pinned oauth2 SDK omits the operation context on its initial device-code request.
The concrete auth HTTP transport now binds cancellation through response-body close,
while retaining the HTTP client's timeout. This avoids an unresponsive Connect flow.

## Verification

Use temporary HTTP/OAuth/keyring/terminal fixtures, without repository tests. Cover
saved matches, all supported auth modes, hidden input, unsafe/misleading links,
credential/configuration failures, cancellation, overrides and unchanged write
confirmation. Compile and render all reference compositions at 120x40, 80x24,
40x12 and 20x6, including long content, monochrome output, save/reopen and cache
rebuilds. Real generation uses the user's explicit API authorization; no real
external-account authorization or writes are part of verification.
