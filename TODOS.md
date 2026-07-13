# Deferred Work

This file records deliberate deferrals that need a new spec and a separate
SDD/TDD pipeline before implementation. It is not an authorization to expand
the active phase.

## Trustworthy Review Attribution

**What:** Add durable reviewer identity, roles, approval semantics, and
revocation or supersession rules for quality review checkpoints.

**Why deferred:** The current local admin boundary proves possession of an
admin credential, not a stable human identity. Adding a free-form reviewer
field would create false attribution.

**Depends on:** An approved authentication and identity model with actor IDs,
role semantics, and an audit-retention policy.

## Tamper-Evident Review Ledger

**What:** Evaluate a signed or remote append-only ledger for environments that
need tamper evidence, independent retention, or compliance-grade audit claims.

**Why deferred:** Phase 28 provides application-immutable local records only.
A hash chain or signature without a threat model and protected key custody
would overstate its guarantees.

**Depends on:** Deployment threat model, key custody, backup, restore, export,
and verification requirements.

## Scheduled Review Workflow

**What:** Add review cadence, reminders, assignment, escalation, and optional
alerts after operators demonstrate a recurring workflow.

**Why deferred:** Phase 28 first validates whether durable checkpoints and
equivalent-window comparisons are useful. Scheduling now would automate an
unproven process.

**Depends on:** Observed usage, notification channels, ownership semantics,
and explicit alert thresholds that do not become a quality score.

## Multi-Process Durable Store

**What:** Replace or protect the single-process JSON ledger when multiple
server processes or hosts may write concurrently.

**Why deferred:** Phase 28 is composed as one local server process. A process
mutex and best-effort whole-file replacement do not coordinate independent
writers.

**Depends on:** A deployment topology that requires multiple writers and a
storage choice with transactions or cross-process locking.

## Checkpoint Retention And Redaction

**What:** Define retention, export, legal hold, and controlled redaction or
supersession for saved rationales and historical gap references.

**Why deferred:** Phase 28 intentionally has no edit or delete API. Introducing
destructive history operations without policy would weaken the review record.

**Depends on:** Data classification, retention policy, backup behavior, and
the trustworthy reviewer model.
