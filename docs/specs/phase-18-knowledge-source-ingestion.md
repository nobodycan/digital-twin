# Phase 18 Knowledge Source Ingestion Spec

Date: 2026-07-04

Status: Approved by the user; Stage 2 plan review in progress

Mode: SDD Stage 1 / gstack office-hours

Source context:

- [Phase 12 Knowledge Space Management and Grounded Answering Spec](./phase-12-knowledge-space-management-grounded-answering.md)
- [Phase 14 Knowledge Operations Console Spec](./phase-14-knowledge-operations-console.md)
- [Phase 15 Knowledge-Grounded Answer Loop Spec](./phase-15-knowledge-grounded-answer-loop.md)
- [Phase 16 Knowledge Workbench and Gap Resolution Spec](./phase-16-knowledge-workbench-gap-resolution.md)
- [Phase 17 Knowledge Curation and Source Management Spec](./phase-17-knowledge-curation-source-management.md)
- Current implementation: local knowledge spaces, local document lifecycle,
  retrieval diagnostics, answer-state metadata, knowledge-gap queue,
  workbench note creation, source relationship detail, and in-place curation.

## Context

Phase 17 was the right prerequisite for richer knowledge sources. Before that,
adding PDFs, web pages, or folders would have created more raw material than the
operator could maintain. Now the system can search, filter, edit, inspect source
metadata, and see how documents relate to knowledge gaps.

That changes the product question from:

> Can the digital human remember and repair what it knows?

to:

> Can the operator feed it real source material without losing traceability?

Phase 18 should introduce a small, deterministic ingestion pipeline. The goal is
not "import everything." The goal is to make source intake safe, observable, and
compatible with the curation surfaces that already exist.

## Office-Hours Premise Challenge

The tempting next step is "support PDF, DOCX, website crawling, GitHub sync, and
folder watching." That is the platform-shaped version of the feature. It is also
where most of the risk hides:

- binary parsing failures;
- unbounded crawling;
- duplicate source records;
- prompt-injection text arriving from external pages;
- unclear operator trust around what was imported and when;
- hard-to-debug answers from stale imported material.

The better first move is to build the ingestion spine before adding every
source adapter:

1. a stable import job model;
2. source metadata and content hash tracking;
3. deterministic conversion from source payload to knowledge document;
4. visible import status and failure reasons in `/admin`;
5. safe limits for size, count, and source type;
6. reuse of existing document curation, diagnostics, and gap workflows.

Once that exists, PDF, DOCX, website crawling, and GitHub sync become adapters
instead of separate knowledge systems.

## Product Thesis

A professional digital human needs a governed source intake path, not just an
upload box.

The operator should be able to answer these questions quickly:

- What did I import?
- Where did it come from?
- Did it succeed, partially succeed, or fail?
- Which knowledge space did it land in?
- Was it deduplicated or imported as a new record?
- What source text will the assistant actually retrieve?
- Is this source safe and current enough to trust?

## Goal

Deliver local-first Knowledge Source Ingestion:

1. add a first-class import job model for knowledge intake;
2. support a narrow first set of deterministic source adapters;
3. convert imported source payloads into existing knowledge documents;
4. preserve source metadata, content hash, and import provenance;
5. expose import status and failures in `/admin`;
6. reuse existing curation, diagnostics, retrieval, and answer-state paths;
7. keep CI free of real DeepSeek calls, databases, cloud drives, and network
   dependencies.

## Recommended Narrow Wedge

Ship Phase 18 with two adapters:

1. **Local text/Markdown file import**
   - imports `.txt` and `.md` files selected through the existing browser file
     upload path;
   - stores source metadata such as original filename, extension, size, content
     hash, and import job ID;
   - creates or updates one knowledge document per file.

2. **Manual URL text snapshot**
   - accepts a URL plus pasted page text or extracted text supplied by the
     operator;
   - stores the URL as source metadata;
   - does not crawl links;
   - does not require server-side internet access in CI.

This gives users two useful real-world paths:

- "I have local docs/runbooks."
- "I copied trusted text from a web page and want the digital human to know it."

It deliberately avoids a server-side crawler in the first slice. That keeps the
security and testing surface small while still proving the ingestion model.

## In Scope

### Import Job Model

Add a local import job projection with:

- job ID;
- tenant ID;
- space ID;
- source type;
- source label;
- status: `pending`, `running`, `completed`, `failed`, `partial`;
- created/updated timestamps;
- source count;
- imported document IDs;
- skipped source count;
- failure code and human-readable cause;
- source metadata summary.

The first implementation may run synchronously while still recording job states.
No background worker is required in Phase 18.

### Source Adapters

Support deterministic first adapters:

- `local_text_file`
  - `.txt`;
  - `.md`;
  - UTF-8 text only;
  - bounded file size.
- `url_text_snapshot`
  - URL string plus pasted text;
  - no recursive crawl;
  - no automatic browser automation;
  - no server-side fetch requirement.

Stage 2 should lock exact size limits and accepted extensions after inspecting
existing upload handler constraints.

### Knowledge Document Creation

Imported sources should become normal knowledge documents:

- assigned to the selected knowledge space;
- chunked by the existing deterministic chunker;
- indexed by the existing reindex path;
- listed in existing document tables;
- editable through Phase 17 curation when safe;
- usable by existing diagnostics and answer grounding.

Metadata should include:

- `source_type`;
- `created_from=knowledge_ingestion`;
- `import_job_id`;
- `source_uri` or original filename;
- `source_label`;
- `content_hash`;
- `source_size_bytes` when known.

### Deduplication Baseline

Phase 18 should provide deterministic exact dedupe by content hash inside a
space:

- same content hash in the same space should not silently create duplicate
  active documents;
- operator-visible result should say whether the source was imported, skipped,
  or mapped to an existing document;
- semantic dedupe remains out of scope.

Stage 2 should decide whether duplicate imports update metadata on the existing
document or only record the skipped source on the import job.

### Admin UX

Extend `/admin` knowledge operations with an import area:

- selected space stays visible;
- source type selector;
- local text/Markdown file picker;
- URL and pasted-text inputs for snapshot import;
- import button;
- import job result/status list;
- imported document links into existing document detail;
- clear failure messages for unsupported type, empty content, size limit, and
  duplicate content.

The interface should remain an operator console. No landing page, no wizard, and
no decorative cards.

### Security and Trust Boundaries

Treat imported text as untrusted source material:

- do not execute or render imported HTML;
- display imported text with `textContent`-style escaping in the browser;
- store source metadata through an allowlist;
- enforce size limits;
- block unsupported file extensions;
- preserve prompt-injection-looking text as data, but surface a warning flag if
  deterministic checks find obvious instruction patterns.

Phase 18 does not need a full malicious-content classifier. A small deterministic
warning is enough for the first slice.

## Out of Scope

- PDF parsing.
- DOCX parsing.
- Website crawling.
- Automatic server-side URL fetching.
- GitHub repository sync.
- Cloud drive sync.
- Folder watching or scheduled imports.
- Background workers.
- Queue persistence outside the local file store.
- Semantic deduplication or clustering.
- LLM-generated summaries or automatic rewrites.
- Review/approval workflow before activation.
- RBAC or production multi-tenant hardening beyond existing local admin guard.

## Alternatives Considered

### Alternative A: Rich Document Import First

Support PDF, DOCX, and HTML extraction immediately.

Pros:

- more impressive first demo;
- matches many real-world document collections;
- creates immediate demand for source management.

Cons:

- parser dependencies and edge cases dominate the phase;
- binary extraction can produce noisy text that hurts retrieval;
- CI fixtures become heavier;
- security review becomes broader before the import model is proven.

Verdict: defer. Add rich formats after the import job model and source metadata
path are stable.

### Alternative B: Server-Side Web Crawler

Accept a URL, fetch the page, extract text, crawl links, and import a site.

Pros:

- strong "knowledge from the web" story;
- reduces manual operator work;
- useful for documentation sites.

Cons:

- network behavior is hard to test deterministically;
- robots, auth, redirects, JavaScript rendering, and rate limits expand scope;
- unbounded crawling can import low-trust text quickly;
- prompt-injection and stale-source risks increase.

Verdict: defer crawling. Use manual URL text snapshots first.

### Alternative C: Import Spine With Text/Markdown and URL Snapshots

Build the import job model and two deterministic adapters.

Pros:

- useful with existing local runbooks and copied source text;
- keeps CI deterministic;
- reuses existing knowledge document, curation, and diagnostics paths;
- gives future adapters a clean contract.

Cons:

- less flashy than PDF/web crawl;
- requires operators to provide extracted text for URL sources;
- still touches admin service, server routes, store persistence, and UI.

Verdict: recommended.

## Proposed Architecture

```text
/admin knowledge ingestion
  -> create import job
  -> validate selected source adapter
  -> normalize source payload
  -> dedupe by content hash inside selected space
  -> create/update existing KnowledgeDocument
  -> persist import job result locally
  -> render imported documents and failures in /admin
  -> reuse curation/detail/diagnostics/retrieval
```

## Data Model Draft

### KnowledgeImportJob

```text
KnowledgeImportJob
  id
  tenant_id
  space_id
  source_type
  source_label
  status
  source_count
  imported_document_ids[]
  skipped_sources[]
  failure_code
  failure_cause
  metadata
  created_at
  updated_at
```

### KnowledgeImportSource

```text
KnowledgeImportSource
  source_type
  name
  uri
  content
  size_bytes
  metadata
```

### Knowledge Document Metadata

Recommended metadata keys:

- `source_type=local_text_file|url_text_snapshot`;
- `created_from=knowledge_ingestion`;
- `import_job_id`;
- `source_uri`;
- `source_label`;
- `source_size_bytes`;
- `source_warning`;
- `content_hash`.

Stage 2 should verify whether `content_hash` should remain only as the existing
document field or also be duplicated into metadata for easier UI filtering.

## UX Requirements

### `/admin`

- Show import controls in the knowledge section.
- Preserve selected knowledge space when importing.
- Let the operator choose source type.
- For local file import, accept one or more `.txt`/`.md` files if the existing
  handler shape supports it; otherwise start with one file.
- For URL snapshot import, require both URL and text content.
- Show the most recent import jobs with status, source count, imported docs,
  skipped count, and failure cause.
- Let the operator click an imported document and land in existing Phase 17
  document detail.
- Do not hide duplicate or failed imports; make them inspectable.

### `/app`

- No required UI changes.
- Existing answer-state and citation surfaces should work once imported
  documents are active in the selected knowledge space.

## Acceptance Criteria

1. Import jobs are persisted locally and listable after process restart.
2. Text/Markdown file import creates normal knowledge documents in the selected
   space.
3. URL text snapshot import creates normal knowledge documents with source URL
   metadata.
4. Imported documents appear in the existing `/admin` document list and detail.
5. Imported documents can be used by retrieval diagnostics and grounded answers.
6. Exact duplicate content in the same space is handled deterministically and is
   visible in the import result.
7. Unsupported extension, empty content, invalid URL, and size-limit failures
   return stable errors and record failed import status when a job exists.
8. Imported source metadata is allowlisted and cannot be arbitrarily written by
   the browser.
9. Imported text is rendered safely in `/admin`; no raw HTML injection.
10. Phase 17 document update, filtering, and relationship behavior does not
    regress.
11. CI remains local and deterministic; no real DeepSeek, external database, or
    network fetch is required.

## Test Matrix Seed

| Area | Scenario | Expected Result |
| --- | --- | --- |
| Admin service | Import `.md` text source | Job completed, document created, chunks indexed |
| Admin service | Import URL snapshot | Document has URL/source metadata |
| Admin service | Duplicate content same space | Duplicate is skipped or linked deterministically |
| Admin service | Same content different space | Import allowed or explicitly documented |
| Admin service | Unsupported extension | Stable validation error and failed job |
| Admin service | Empty source content | Stable validation error and failed job |
| Admin service | Oversized source | Stable validation error and failed job |
| Store | Persist import jobs | Reload lists previous job |
| Store | Backward compatibility | Existing knowledge data loads without import jobs |
| Server | Import endpoint validates payload | Bad requests return JSON error/cause |
| Server | Import job list endpoint | Returns scoped jobs for selected space |
| Web admin | Import controls render | Source type, file/text inputs, import action exist |
| Web admin | Import result renders | Status, imported docs, skipped/failure cause visible |
| Web admin | Imported doc navigation | Selecting result loads existing detail panel |
| Security | Imported HTML-like text | Rendered as text, not executed |
| Regression | Phase 17 curation | Existing edit/filter/detail tests still pass |
| Regression | Phase 15 answer state | Imported docs can ground a selected-space answer |

## Open Questions For Stage 2

1. Should Phase 18 support multiple file upload in the first slice, or one file
   per import job?
2. Should duplicate exact-content imports update source metadata on the existing
   document, or only record a skipped source in the job?
3. What file size limit should be enforced locally?
4. Should import jobs live in the existing knowledge store file, or in a separate
   local `knowledge_import_jobs` file behind the same store boundary?
5. Should URL validation allow only `http`/`https`, or also local documentation
   schemes later?
6. Should imported documents default to active immediately, or should a future
   Phase 19 introduce draft/review before activation?

## Recommended Assignment Before Stage 2

Approve or revise the Phase 18 scope:

> Build Knowledge Source Ingestion: a local import job model, deterministic
> text/Markdown file import, manual URL text snapshot import, exact content-hash
> dedupe, source metadata preservation, import status visibility in `/admin`,
> and reuse of existing curation/diagnostics/retrieval. Defer PDF, DOCX,
> crawling, GitHub/cloud sync, scheduled imports, semantic dedupe, and
> LLM-authored source maintenance.
