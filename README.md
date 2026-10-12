# Tuilor

**Build your own terminal applications with AI + MCP.**

Tuilor is a planned open-source application builder that turns natural-language
requests and connected Model Context Protocol (MCP) capabilities into reusable
terminal applications. AI will generate declarative JSON definitions; a Go runtime
will execute and render validated definitions. Routine interactions should run
without repeated LLM calls.

## Project status

The repository is at the architecture-review stage. Phase 1 production code has
not been implemented, dependencies have not been installed, and no Go module has
been initialized. The existing empty `main.go` is preserved pending the agreed
entry-point layout.

## Intended experience

These commands describe the product vision and are **not available yet**:

```sh
tuilor connect github
tuilor connect linear
tuilor create
tuilor save dev-dashboard
tuilor run dev-dashboard
tuilor edit dev-dashboard
```

A user could request a dashboard for GitHub pull requests and Linear issues,
customize the generated application, and reopen it later. Eventually, compatible
applications should be portable and shareable.

## Principles and stack

- Build persistent, customizable applications from declarative definitions.
- Start with local filesystem storage and explicit permissions for tool execution.
- Keep generation separate from execution; do not generate arbitrary executable code.
- Use Go and the official `github.com/modelcontextprotocol/go-sdk` for MCP.
- Introduce Bubble Tea, Bubbles, and Lip Gloss when the dynamic TUI milestone begins.
- Add provider-agnostic LLM integration when application generation begins.
- Add infrastructure and abstractions only when requirements justify them.

## Roadmap

| Phase | Deliverable |
| --- | --- |
| 1 | Connect to one trusted local MCP server over stdio and display its tools |
| 2 | Normalize tools into a registry with explicit schema and freshness rules |
| 3 | Generate and validate declarative application definitions with AI |
| 4 | Render definitions dynamically with Bubble Tea |
| 5 | Save, reopen, edit, and version applications on the local filesystem |
| 6 | Add multi-server workflows and permission enforcement |
| 7 | Explore sharing and distribution after the local product is reliable |

The first milestone includes timeouts, cancellation, connection errors, clean
session/subprocess shutdown, and a minimal test MCP server. Tool execution, AI,
persistence, remote transports, and a full TUI are outside that milestone.

## Architecture and learning workflow

Read [the evolving architecture and Phase 1 proposal](docs/architecture.md) and
[ADR 0001: local-first modular Go application](docs/adr/0001-initial-architecture.md).
[AGENTS.md](AGENTS.md) captures repository conventions.

For each milestone: identify requirements, compare designs and tradeoffs, agree
on meaningful architectural choices, implement the smallest useful version,
test behavior and failures, and review what we learned. Architecture explanations
are part of the work: this project is also a way to learn Go and system design.

The repository-local
[system-design-mentor skill](.agents/skills/system-design-mentor/SKILL.md) supports
design reviews. Invoke it as `$system-design-mentor` for requirements, boundaries,
alternatives, failure analysis, and a practical implementation proposal.
