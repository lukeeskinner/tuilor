# ADR 0002: Preview saved apps through My Apps after capability validation

> Historical decision: legacy JSON compatibility and generation are retired by
> [ADR 0007](0007-go-only-apps.md). Current app generation uses Go projects.

- Date: 2026-10-15
- Status: Accepted by the user in the interactive-preview implementation plan

## Context

AI generation produces validated JSON definitions. The next milestone must make
those files visible and editable in a small terminal preview while preserving
generation, discovery, and the existing home screen. Tool execution is deferred.

## Alternatives considered

| Choice | Benefits | Costs |
| --- | --- | --- |
| Standalone preview command | Small entry point, direct file selection | Separate from the home-screen library experience |
| My Apps on the home screen | Natural browsing and back navigation | Adds directory listing and screen state |
| Offline structural preview | No server startup or configuration | Cannot verify actual tool names and parameter bindings |
| Discover capabilities before preview | Reuses full validation against the selected server | Requires a trusted executable and handles connection failures |

## Decision

Use My Apps, as selected by the user. Root flags configure the app directory and
trusted local server executable. Selecting a JSON file performs asynchronous
discovery under a 10-second deadline, then uses the existing app parser and full
validation. Close the connection before rendering. Cancellation returns to the
library and prevents stale completion from opening an app.

Keep preview state separate from the definition. Use concrete Bubble Tea models
and Bubbles textinput/viewport, with no generic renderer framework. Inputs retain
values across page switches and reset on exit. Display actions as disabled;
preview has no AI or tool-execution path. Keep the version 1 JSON format intact.

## Consequences

The first renderer can validate real capabilities without keeping a server alive
during interaction. Preview requires server availability but no OpenAI key. It
does not validate future field values or grant permission to execute actions.
Starting the server still grants that trusted executable the user's OS access.

Revisit session lifetime and permission policy when enabling execution. Revisit
offline validation or cached capabilities only if disconnected preview becomes
a requirement. Automated test files remain omitted at the user's request; this
milestone uses build, vet, and manual terminal/protocol checks.
