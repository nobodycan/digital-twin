# Phase 18 Knowledge Source Ingestion Design

Date: 2026-07-04

Status: Spec approved by the user; Stage 2 plan review in progress

Source spec: [Phase 18 Knowledge Source Ingestion Spec](../specs/phase-18-knowledge-source-ingestion.md)

## Office-Hours Summary

Phase 18 should start importing real source material, but it should not jump
straight into a general crawler or binary document parser. The system finally
has the curation primitives that make ingestion safe enough to begin:

1. selected knowledge spaces;
2. local document lifecycle;
3. retrieval diagnostics;
4. knowledge-gap capture;
5. workbench notes;
6. document editing and source relationship detail.

The next step is an ingestion spine:

```text
source input -> import job -> normalized text -> knowledge document
             -> curation/detail -> diagnostics -> grounded answer
```

The first slice should make the pathway boring, visible, and hard to corrupt.
That is more valuable than supporting many formats at once.

## Forcing Questions

### 1. Who desperately needs this?

The first user is still the single local operator building a professional
digital human from their own materials. They likely have runbooks, notes,
product docs, copied web content, or internal guidance that should become
queryable without hand-creating every knowledge record.

### 2. What is the status quo?

The operator can upload simple knowledge, create workbench notes from gaps, and
edit curated documents. But "getting source material in" is still treated as a
document lifecycle action rather than an observable import workflow. Failures,
duplicates, and source provenance are not first-class enough.

### 3. What is the narrowest painful wedge?

Import a local Markdown runbook into a selected knowledge space, see an import
job complete, click the created document, run diagnostics, and ask `/app` a
question grounded in that source.

### 4. What should we avoid building too early?

Avoid PDF/DOCX extraction, website crawling, folder watching, scheduled sync,
and LLM-authored source maintenance. Those are adapter and governance problems.
The import model should exist before those features arrive.

### 5. What can we observe after shipping?

We can observe whether operators import documents, inspect import failures,
click into imported document detail, and use diagnostics after import. We can
also see whether exact dedupe prevents repeated local source clutter.

### 6. How does this fit the future?

Future PDF, DOCX, web, GitHub, and cloud-drive adapters can all feed the same
import job model. Future review workflows can add draft/approval states on top
of import results. Future source trust scoring can attach to the same metadata.

## Recommended Product Shape

Build a **Knowledge Import Panel** inside the existing `/admin` knowledge
section.

It should add:

- source type selection;
- text/Markdown file import;
- URL text snapshot import;
- recent import jobs;
- visible duplicate/failure outcomes;
- links from import results to existing document detail.

It should not create a separate ingestion app. It should reuse existing
knowledge spaces, documents, curation, diagnostics, and retrieval.

## Architecture

```text
web/admin.html
  import controls
  recent import job table
  result links into selected document detail

web/admin.js
  collect selected space and source inputs
  post import request
  render job status/results/failures
  refresh knowledge list/detail after import

internal/server
  POST /admin/knowledge/import
  GET /admin/knowledge/imports?space_id=

internal/admin
  KnowledgeImportService or KnowledgeService import methods
  validate sources
  compute content hash
  exact dedupe inside selected space
  create KnowledgeDocument through existing upload/reindex path
  persist KnowledgeImportJob records

local file store
  existing knowledge documents
  import job persistence behind local store boundary
```

Stage 2 should decide whether import operations live on the existing
`KnowledgeService` or a small `KnowledgeImportService` that wraps it. The key
constraint is that document creation must still go through existing knowledge
service validation and chunking.

## Component Design

### Import Service

Recommended service inputs:

```go
type KnowledgeImportRequest struct {
    SpaceID     string
    SourceType  string
    SourceLabel string
    Sources     []KnowledgeImportSource
}

type KnowledgeImportSource struct {
    Name      string
    URI       string
    Content   string
    SizeBytes int64
    Metadata  map[string]string
}
```

Recommended service output:

```go
type KnowledgeImportJob struct {
    ID                  string
    TenantID            string
    SpaceID             string
    SourceType          string
    SourceLabel         string
    Status              string
    SourceCount         int
    ImportedDocumentIDs []string
    SkippedSources      []KnowledgeImportSkippedSource
    FailureCode         string
    FailureCause        string
    Metadata            map[string]string
    CreatedAt           time.Time
    UpdatedAt           time.Time
}
```

The implementation should start synchronous:

1. create job as `running`;
2. validate each source;
3. import or skip deterministically;
4. persist final `completed`, `partial`, or `failed` state.

The status field still matters even without a worker because it gives future
background ingestion a compatible model.

### Source Validation

Validation should be centralized in the import service:

- source type is allowlisted;
- local file names must end in `.txt` or `.md`;
- URL snapshots require `http` or `https` URL;
- content must be non-empty after trimming;
- content size must stay under the Stage 2 limit;
- metadata keys are allowlisted;
- prompt-injection warning patterns may set `source_warning`, but should not
  block import by default.

### Deduplication

Exact dedupe should use existing content hash behavior:

```text
same tenant + same space + same content_hash -> duplicate
```

The recommended Phase 18 behavior:

- skip creating a new document;
- record skipped source with reason `duplicate_content`;
- include the existing document ID if it can be resolved deterministically;
- do not mutate the existing document unless Stage 2 explicitly chooses that.

This avoids surprising the operator by changing metadata on a trusted existing
document.

### Document Creation

Imported documents should call the existing upload/reindex path with metadata:

```text
source_type=<adapter>
created_from=knowledge_ingestion
import_job_id=<job-id>
source_uri=<filename-or-url>
source_label=<operator-label>
source_size_bytes=<bytes>
source_warning=<optional-warning>
```

Names should be stable and human-readable:

- local file import: original filename;
- URL snapshot: URL hostname/path or provided source label.

Document IDs should remain deterministic enough for tests but not collide across
spaces or repeated distinct imports. Stage 2 should inspect the current ID
generation style before locking this.

### Server Routes

Candidate routes:

```text
POST /admin/knowledge/import
GET /admin/knowledge/imports?space_id=
```

`POST /admin/knowledge/import` should accept JSON for URL snapshots and
multipart or JSON for local text files depending on existing upload handler
patterns. Stage 2 should choose the smallest testable route shape.

Errors should reuse the current JSON shape:

```json
{"error":"knowledge import failed","cause":"unsupported source type"}
```

### Admin UI

Add a compact import section near the current knowledge upload/curation area:

- selected space controls remain the source of truth;
- source type selector controls visible fields;
- local file input for `.txt`/`.md`;
- URL and pasted text inputs for snapshot imports;
- source label input;
- import action;
- recent jobs table.

The jobs table should show:

- status;
- source type;
- source label or source name;
- imported document count;
- skipped count;
- failure cause;
- action to inspect the first imported document.

The UI should refresh:

- knowledge list;
- selected document detail if the imported document is selected;
- health summary;
- recent import jobs.

## Data Flow

### Local Text/Markdown Import

1. Operator selects a knowledge space in `/admin`.
2. Operator chooses `local_text_file`.
3. Operator selects a `.md` or `.txt` file.
4. Browser sends file content and source metadata to the import route.
5. Server validates extension, content, and size.
6. Import service creates a job and computes content hash.
7. Service dedupes inside the selected space.
8. Non-duplicate source becomes a normal knowledge document.
9. Job records imported and skipped outcomes.
10. UI renders job result and refreshes document list.

### URL Text Snapshot Import

1. Operator chooses `url_text_snapshot`.
2. Operator enters a URL and pasted trusted text.
3. Server validates URL scheme and text content.
4. Import service stores URL as source metadata.
5. Source becomes a normal knowledge document.
6. Existing diagnostics can be run against the imported text.

### Grounded Answer After Import

1. Operator imports source into a selected space.
2. Operator opens `/app` and selects the same knowledge space.
3. User asks a question covered by the imported document.
4. Existing retrieval pipeline ranks imported chunks.
5. Existing answer-state metadata shows grounded or partial support.

## Scope Guardrails

- No parser dependency for PDF/DOCX in Phase 18.
- No server-side crawler or network fetch.
- No background worker.
- No scheduled sync.
- No external database.
- No provider call required in CI.
- No automatic activation review workflow yet.
- No LLM-authored source changes.

## Stage 2 Decisions

Stage 2 autoplan should lock:

1. whether import logic lives in `KnowledgeService` or a new
   `KnowledgeImportService`;
2. import job persistence file shape;
3. exact route payload shape for file import;
4. accepted file size and source count limits;
5. duplicate behavior;
6. document ID generation for imported sources;
7. prompt-injection warning patterns;
8. browser QA flow for importing and grounding.

## Risk Register

| Risk | Severity | Mitigation |
| --- | --- | --- |
| Import creates duplicate clutter | Medium | Exact content-hash dedupe inside selected space |
| Imported malicious text is rendered as HTML | High | Render via text APIs and add static tests |
| Source adapter scope balloons | High | Limit Phase 18 to text/Markdown and manual URL snapshots |
| Synchronous import blocks large requests | Medium | Enforce size limits and keep worker model deferred |
| Failed imports are invisible | Medium | Persist job status and failure cause |
| Existing curation regresses | Medium | Include Phase 17 edit/filter/detail regression tests |
| External network flakiness enters CI | High | No server-side URL fetch in Phase 18 |

## Success Signal

Phase 18 succeeds when a user can:

1. open `/admin`;
2. select a knowledge space;
3. import a Markdown runbook;
4. see a completed import job;
5. click the imported document in existing detail;
6. run diagnostics and see the imported chunk;
7. ask `/app` a question in that space and get a grounded answer;
8. repeat the same import and see a deterministic duplicate/skip result.

## Assignment For Stage 2

Run `$gstack-autoplan` against this spec and design. The plan should produce a
TDD matrix for:

- import job persistence;
- text/Markdown source validation;
- URL snapshot validation;
- exact content-hash dedupe;
- source metadata preservation;
- server route behavior;
- admin import controls and job rendering;
- imported document diagnostics;
- Phase 17 curation regressions;
- Phase 15 grounded-answer regressions.
