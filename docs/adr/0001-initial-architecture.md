# ADR 0001: Start with a local-first modular Go application

- Date: 2026-10-12
- Status: Accepted for repository initialization, as specified in the project brief
- Scope: Deployment model and architectural direction; Phase 1 APIs remain proposed

## Context

Tuilor will build reusable terminal applications from MCP capabilities and
AI-generated JSON definitions. Initial requirements are local operation, a small
installation footprint, and a codebase suitable for learning Go and system
design. The first milestone connects to one trusted local MCP server over stdio
and discovers tools. There is no requirement for shared tenancy, independent
service scaling, centralized storage, or a hosted control plane.

## Approaches considered

| Approach | Benefits | Costs |
| --- | --- | --- |
| Modular Go application | Simple installation; in-process calls; one debugger and release; explicit package boundaries | Modules share a process and release; package discipline must prevent coupling |
| Microservices for connection, generation, and runtime | Independent deployment and scaling; potential process isolation | Network failures, service discovery, authentication, deployments, and distributed observability before a demonstrated need |

## Decision

Start with a local-first modular Go application. Use ordinary packages to separate
responsibilities and the official MCP Go SDK for communication. Local MCP servers
are external subprocesses; they are not Tuilor-owned microservices.

Introduce local filesystem persistence when that milestone begins. Plan for
Bubble Tea rendering and provider-agnostic generation without implementing them
now. AI generates declarative definitions, and the runtime validates and executes
them under explicit permission rules.

## Tradeoffs and consequences

- Local development and deployment remain simple, and routine interactions can
  execute without an LLM or a Tuilor cloud service.
- Local-first storage does not mean connected MCP tools work offline: their
  upstream services may still require credentials and network access.
- Package boundaries support later changes but do not provide OS isolation.
  Trusted local server processes inherit the user's permissions.
- Generation or runtime workloads cannot scale independently within one process.
  This is acceptable without a demonstrated requirement for separate services.
- Portable application definitions will need capability binding, versioning, and
  permission rules in later milestones. This ADR does not settle their schemas.

## Reconsideration triggers

Revisit deployment boundaries if real requirements emerge for hosted multi-user
execution, independent scaling, remote collaboration, or stronger execution
isolation. Consider a dedicated worker or sandbox before a broad microservice
split when isolation alone is the problem. Require evidence of the operational
need and document migration costs in a new ADR.
