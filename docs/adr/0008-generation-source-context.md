# ADR 0008: Give source generation one compiling example and the host contract

- Date: 2026-10-16
- Status: Implemented during the user-requested generation reliability fix

## Evidence and alternatives

Actual model responses contained commentary after the final declaration or a
second package block. Strict JSON validation accepted those source strings, but
the Go parser correctly blocked them. Updating the model alone did not eliminate
the problem. A mocked valid response verifies orchestration, not model quality.

We considered continuing to supply the complete host main.go, supplying only
instructions, or supplying a small compiling app plus the host API contract.
The full entry point exposes implementation the model must not reproduce;
instructions alone leave more library usage to recall. Choose the contract and
one compiling example. Multiple complete Go blocks may have contributed to the
bad output; the small sample does not establish that as the sole cause.

## Decision

Provide loadData/saveData signatures, starter ownership, resize/exit behavior,
and one complete app.go example demonstrating correct pinned widget APIs,
transactional saving, exported fields, stable IDs, and a scrolling layout.
The example is source context; it adds no runtime component abstraction. Keep
full source generation, semantic/source validation, compilation, and automatic
opening of successful uncanceled builds. Reject malformed source rather than
silently trimming declarations or choosing fragments. Repair remains explicit.

## Verification and limits

The user authorized real paid requests for this investigation. Five requests
covered fresh ratings generation, repair of the actual failure, a notes failure,
and two fresh notes generations after context simplification. Both final notes
generations compiled; the home-screen run opened automatically and saved/reopened
with data. The repaired ratings app was exercised for add/edit/delete, deletion
cancellation, duplicate names, persistence, and four terminal sizes, then saved
into My Apps. Temporary fixtures remain outside the repository.

The reference itself is compiled through the normal project builder. Keep
format/build/vet and temporary failure/cancellation checks. This small real
check is evidence of the corrected flow, not a guarantee that every future
prompt yields correct source or behavior. No automatic billable retries.
