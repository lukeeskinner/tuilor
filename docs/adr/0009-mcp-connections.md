# ADR 0009: Saved MCP connections and explicit browser sign-in

Status: Accepted, 2026-10-16.

## Context

Tuilor generates full Go apps, but its only MCP integration was standalone local
tool discovery. Users need reusable connections before generated apps can use
services. Connection management must work without an LLM or automatic tool calls.

## Decision

Provide a user-managed connection list for local stdio commands and remote
Streamable HTTP URLs. Keep versioned nonsecret configuration in the user config
directory; keep OAuth credentials in the OS credential store. Use the existing
official MCP Go SDK and pinned zalando/go-keyring v0.2.8.

Keep three concrete responsibilities: connections stores configuration/credentials;
mcpclient owns protocol/authentication sessions; tui owns interaction and request
cancellation. No provider framework or application schema is introduced.

Discovery is a short-lived ten-second operation, not a persistent connection.
Browser sign-in is a separate explicit five-minute operation using authorization
code + PKCE, state/issuer validation, and a temporary loopback callback. Support
public pre-registration and dynamic registration. Check never opens a browser.
Expired credentials refresh without interactive authorization; unrecoverable
credentials require an explicit new sign-in. No plaintext credential fallback.

Configuration uses atomic replacement after validation. Credential chunks are
staged under a fresh revision before publishing their index. Bind credentials to
the connection identity and authentication settings; invalidate them on edits.
Raw remote errors and credentials are never copied into diagnostics.

## Alternatives and consequences

Putting protocol/storage code in the TUI would reduce initial wiring but prevent
reuse by future generation/runtime flows. Separate concrete packages add a small
amount of coordination while preserving clear resource ownership.

Private credential files would work on more headless systems and simplify the
dependency graph. OS credential storage follows the user's chosen security model
and keeps tokens out of ordinary config files; unavailable stores are visible
errors, and native calls may require OS interaction.

Persistent MCP sessions could reduce repeated startup work. Short-lived discovery
avoids idle subprocesses, stale capability catalogs, and additional reconnection
state. A later app-runtime milestone can choose an execution lifetime separately.

A curated catalog is deferred in favor of arbitrary user-provided servers.
The form defaults to Remote service / Browser sign-in, uses plain-language labels,
and hides optional settings under Advanced. This reduces setup choices without
guessing provider endpoints or removing validation; hidden values are retained.
Remote OAuth is not universally plug-and-play: some providers require registration
or downstream account setup beyond this public-client milestone. No hosted client
metadata infrastructure, server installation, or generated-app MCP bridge is added.

Local servers and generated Go still run with user OS permissions. Authentication
does not grant permission to execute tools. This milestone exposes no call entry
point; generated-app integration and exact-call approval remain separate work.

## Verification

Use temporary storage, HTTP/OAuth, stdio, and terminal fixtures without repository
test files or paid API requests. Cover successful discovery/authentication and
failure/cancellation paths, credential refresh/sign-out, corrupt-file preservation,
environment isolation, literal argv, process reaping, and responsive keyboard UI.
Run formatting, build, tests, race checks, vet, and diff checks. Real provider
interoperability and native credential-store prompts require a manual smoke check.
