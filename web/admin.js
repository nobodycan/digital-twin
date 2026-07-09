const personaDraft = document.querySelector("#persona-draft");
const personaStatus = document.querySelector("#persona-status");
const saveDraftButton = document.querySelector("#persona-save-draft");
const publishButton = document.querySelector("#persona-publish");
const rollbackButton = document.querySelector("#persona-rollback");
const memoryTableBody = document.querySelector("#memory-table-body");
const knowledgeSpaceSelect = document.querySelector("#knowledge-space-select");
const knowledgeSpaceCreate = document.querySelector("#knowledge-space-create");
const knowledgeSpaceCreateButton = document.querySelector("#knowledge-space-create-button");
const knowledgeImportPanel = document.querySelector("#knowledge-import-panel");
const knowledgeImportSourceType = document.querySelector("#knowledge-import-source-type");
const knowledgeImportSourceLabel = document.querySelector("#knowledge-import-source-label");
const knowledgeImportFileFields = document.querySelector("#knowledge-import-file-fields");
const knowledgeImportURLFields = document.querySelector("#knowledge-import-url-fields");
const knowledgeUpload = document.querySelector("#knowledge-upload");
const knowledgeImportURL = document.querySelector("#knowledge-import-url");
const knowledgeImportContent = document.querySelector("#knowledge-import-content");
const knowledgeImportRun = document.querySelector("#knowledge-import-run");
const knowledgeUploadMock = document.querySelector("#knowledge-upload-mock");
const knowledgeQuery = document.querySelector("#knowledge-query");
const knowledgeQueryMode = document.querySelector("#knowledge-query-mode");
const knowledgeQueryRun = document.querySelector("#knowledge-query-run");
const knowledgeStatus = document.querySelector("#knowledge-status");
const knowledgeHealthSummary = document.querySelector("#knowledge-health-summary");
const knowledgeHealthStatus = document.querySelector("#knowledge-health-status");
const knowledgeHealthMetrics = document.querySelector("#knowledge-health-metrics");
const knowledgeAttentionReasons = document.querySelector("#knowledge-attention-reasons");
const knowledgeTableBody = document.querySelector("#knowledge-table-body");
const knowledgeDetail = document.querySelector("#knowledge-detail");
const knowledgeDetailTitle = document.querySelector("#knowledge-detail-title");
const knowledgeDetailMeta = document.querySelector("#knowledge-detail-meta");
const knowledgeDetailRelations = document.querySelector("#knowledge-detail-relations");
const knowledgeDetailFlags = document.querySelector("#knowledge-detail-flags");
const knowledgeDetailBody = document.querySelector("#knowledge-detail-body");
const knowledgeReviewActions = document.querySelector("#knowledge-review-actions");
const knowledgeFilterQuery = document.querySelector("#knowledge-filter-query");
const knowledgeFilterStatus = document.querySelector("#knowledge-filter-status");
const knowledgeFilterReviewStatus = document.querySelector("#knowledge-filter-review-status");
const knowledgeFilterSourceType = document.querySelector("#knowledge-filter-source-type");
const knowledgeFilterGapLinked = document.querySelector("#knowledge-filter-gap-linked");
const knowledgeFilterApply = document.querySelector("#knowledge-filter-apply");
const knowledgeFilterReset = document.querySelector("#knowledge-filter-reset");
const knowledgeEditToggle = document.querySelector("#knowledge-edit-toggle");
const knowledgeEditForm = document.querySelector("#knowledge-edit-form");
const knowledgeEditName = document.querySelector("#knowledge-edit-name");
const knowledgeEditSourceLabel = document.querySelector("#knowledge-edit-source-label");
const knowledgeEditContent = document.querySelector("#knowledge-edit-content");
const knowledgeEditSave = document.querySelector("#knowledge-edit-save");
const knowledgeEditCancel = document.querySelector("#knowledge-edit-cancel");
const knowledgeDebugResults = document.querySelector("#knowledge-debug-results");
const knowledgeGapQueue = document.querySelector("#knowledge-gap-queue");
const knowledgeImportJobs = document.querySelector("#knowledge-import-jobs");
const knowledgeReviewQueue = document.querySelector("#knowledge-review-queue");
const knowledgeNoteTitle = document.querySelector("#knowledge-note-title");
const knowledgeNoteBody = document.querySelector("#knowledge-note-body");
const knowledgeNoteCreate = document.querySelector("#knowledge-note-create");
const knowledgeNoteGapContext = document.querySelector("#knowledge-note-gap-context");
const toolKnowledgeSearch = document.querySelector("#tool-knowledge-search");
const toolSavePolicy = document.querySelector("#tool-save-policy");
const toolStatus = document.querySelector("#tool-status");
const auditTimelineRefresh = document.querySelector("#audit-timeline-refresh");
const auditTimelineBody = document.querySelector("#audit-timeline-body");
const auditTimelineState = document.querySelector("#audit-timeline-state");
const auditTimelineWeakOnly = document.querySelector("#audit-timeline-weak-only");
const auditTimelineDocumentID = document.querySelector("#audit-timeline-document-id");
const auditTimelineConversationID = document.querySelector("#audit-timeline-conversation-id");
const auditTimelineLimit = document.querySelector("#audit-timeline-limit");
const auditRefresh = document.querySelector("#audit-refresh");
const auditTableBody = document.querySelector("#audit-table-body");
const knowledgeDetailPathPrefix = "/admin/knowledge/";
const knowledgeListPath = "/admin/knowledge";
const knowledgeHealthPath = "/admin/knowledge/health";
const knowledgeGapListPath = "/admin/knowledge/gaps";
const knowledgeGapUpdatePath = "/admin/knowledge/gaps/update";
const knowledgeImportPath = "/admin/knowledge/import";
const knowledgeImportListPath = "/admin/knowledge/imports";
const knowledgeReviewPath = "/admin/knowledge/review";
const knowledgeNoteCreatePath = "/admin/knowledge/notes/create";
const knowledgeUpdatePath = "/admin/knowledge/update";
const knowledgeReviewGatedStage = "review_gated_documents";
const knowledgeNoReviewActiveReason = "no_review_active_documents";

let currentDraftId = "";
let activeVersionId = "";
let selectedKnowledgeSpaceId = "default";
let selectedKnowledgeDocumentId = "";
let selectedKnowledgeDocument = null;
let selectedKnowledgeGapId = "";
let knowledgeEditDraft = null;
const knowledgeGapDiagnosticsState = new Map();

function setPersonaStatus(text) {
  personaStatus.textContent = text;
}

function setKnowledgeStatus(text) {
  knowledgeStatus.textContent = text;
}

function clearElement(element) {
  if (element) {
    element.textContent = "";
  }
}

function renderKnowledgeMetric(label, value) {
  const item = document.createElement("span");
  item.className = "knowledge-metric";
  item.textContent = `${label} ${value}`;
  return item;
}

function renderKnowledgeFlag(flag) {
  const item = document.createElement("span");
  item.className = "knowledge-flag";
  item.textContent = flag.replaceAll("_", " ");
  return item;
}

function renderAuditTrust(record) {
  return record.knowledge_answer_state || record.knowledge_evidence?.answer_state || "unknown";
}

function renderAuditEvidence(record) {
  const evidence = record.knowledge_evidence || {};
  const citations = Array.isArray(evidence.citations) ? evidence.citations : [];
  const summary = evidence.summary || "No supporting evidence recorded";
  if (citations.length === 0) {
    return summary;
  }
  const topSources = citations
    .slice(0, 2)
    .map((citation) => citation.title || citation.document_id || "Untitled source")
    .join(", ");
  return `${summary}\n${topSources}`;
}

function appendAuditEvidence(cell, record) {
  const evidence = record.knowledge_evidence || {};
  const citations = Array.isArray(evidence.citations) ? evidence.citations.slice(0, 2) : [];
  const summary = document.createElement("div");
  summary.textContent = renderAuditEvidence(record);
  cell.append(summary);
  for (const citation of citations) {
    if (!citation.document_id) {
      continue;
    }
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = citation.title || citation.document_id;
    button.addEventListener("click", async () => {
      await inspectKnowledgeDocument(citation.document_id);
      setKnowledgeStatus(`Inspecting evidence source ${citation.document_id}`);
    });
    cell.append(button);
  }
}

function auditTimelineQuery() {
  const params = new URLSearchParams();
  if (auditTimelineState?.value) {
    params.set("state", auditTimelineState.value);
  }
  if (auditTimelineWeakOnly?.checked) {
    params.set("weak_only", "true");
  }
  if (auditTimelineDocumentID?.value.trim()) {
    params.set("document_id", auditTimelineDocumentID.value.trim());
  }
  if (auditTimelineConversationID?.value.trim()) {
    params.set("conversation_id", auditTimelineConversationID.value.trim());
  }
  if (auditTimelineLimit?.value.trim()) {
    params.set("limit", auditTimelineLimit.value.trim());
  }
  const query = params.toString();
  return query ? `/admin/audit/timeline?${query}` : "/admin/audit/timeline";
}

function appendAuditTimelineSourceActions(container, item) {
  const sources = Array.isArray(item.top_sources) ? item.top_sources.slice(0, 2) : [];
  for (const source of sources) {
    if (!source.document_id) {
      continue;
    }
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = source.title || source.document_id;
    button.addEventListener("click", async () => {
      await inspectKnowledgeDocument(source.document_id);
      setKnowledgeStatus(`Inspecting evidence source ${source.document_id}`);
    });
    container.append(button);
  }
}

function renderAuditTimelineItem(item) {
  const row = document.createElement("div");
  row.className = "audit-timeline-item";

  const summary = document.createElement("div");
  summary.className = "audit-timeline-summary";
  const summaryLines = [
    `[${item.answer_state || "unknown"}] ${item.created_at || ""} ${item.conversation_id || ""}`.trim(),
    item.question_summary || item.summary || "No supporting evidence recorded",
    `${item.source_count || 0} sources${item.diagnostics?.no_source_reason ? ` | ${item.diagnostics.no_source_reason}` : ""}`,
  ].filter(Boolean);
  summary.textContent = summaryLines.join("\n");
  row.append(summary);

  const meta = document.createElement("div");
  meta.className = "audit-timeline-meta";
  meta.textContent = `Agent ${item.agent_name || "unknown"} | ${item.status || "unknown"} | ${item.latency_ms || 0}ms`;
  row.append(meta);

  const sources = document.createElement("div");
  sources.className = "audit-timeline-sources";
  const topSources = Array.isArray(item.top_sources) ? item.top_sources.slice(0, 2) : [];
  if (topSources.length === 0) {
    const empty = document.createElement("div");
    empty.className = "audit-timeline-snippet";
    empty.textContent = item.summary || "No supporting evidence recorded";
    sources.append(empty);
  } else {
    for (const source of topSources) {
      const snippet = document.createElement("div");
      snippet.className = "audit-timeline-snippet";
      snippet.textContent = [
        source.title || source.document_id || "Untitled source",
        source.review_status ? `review ${source.review_status}` : "",
        source.snippet || "",
      ].filter(Boolean).join(" | ");
      sources.append(snippet);
    }
  }
  row.append(sources);

  const actions = document.createElement("div");
  actions.className = "audit-timeline-actions";
  appendAuditTimelineSourceActions(actions, item);

  if (item.conversation_id) {
    const filterConversation = document.createElement("button");
    filterConversation.type = "button";
    filterConversation.textContent = "Filter conversation";
    filterConversation.addEventListener("click", async () => {
      if (auditTimelineConversationID) {
        auditTimelineConversationID.value = item.conversation_id;
      }
      await loadAuditTimeline();
    });
    actions.append(filterConversation);
  }

  if (item.gap?.gap_id) {
    const gapButton = document.createElement("button");
    gapButton.type = "button";
    gapButton.textContent = `Open gap ${item.gap.gap_id}`;
    gapButton.addEventListener("click", async () => {
      await loadKnowledgeGaps();
      setKnowledgeStatus(`Open gap queue for ${item.gap.gap_id}`);
    });
    actions.append(gapButton);
  } else if (item.answer_state === "unsupported" || item.answer_state === "partially_supported" || item.answer_state === "review_gated") {
    const gapButton = document.createElement("button");
    gapButton.type = "button";
    gapButton.textContent = "Open gap queue";
    gapButton.addEventListener("click", async () => {
      await loadKnowledgeGaps();
      setKnowledgeStatus("Open gap queue");
    });
    actions.append(gapButton);
  }

  if (!item.gap && item.diagnostics?.no_source_reason) {
    const note = document.createElement("span");
    note.textContent = "gap unknown";
    actions.append(note);
  }

  row.append(actions);
  return row;
}

function effectiveReviewStatus(documentRecord) {
  return documentRecord?.review_status || "active";
}

function renderKnowledgeReviewRow(documentRecord) {
  const row = document.createElement("div");
  row.className = "knowledge-import-job-row";

  const summary = document.createElement("div");
  summary.className = "knowledge-import-job-summary";
  const warning = documentRecord.metadata?.source_warning ? `warning ${documentRecord.metadata.source_warning}` : "";
  summary.textContent = [
    `${documentRecord.name || documentRecord.id} | ${effectiveReviewStatus(documentRecord)}`,
    `status ${documentRecord.status || "unknown"} | source ${documentRecord.metadata?.source_type || "upload"}`,
    warning,
  ].filter(Boolean).join("\n");
  row.append(summary);

  const actions = document.createElement("div");
  actions.className = "knowledge-import-job-actions";
  const inspectButton = document.createElement("button");
  inspectButton.type = "button";
  inspectButton.textContent = "Inspect";
  inspectButton.addEventListener("click", async () => {
    await inspectKnowledgeDocument(documentRecord.id);
    setKnowledgeStatus(`Inspecting review candidate ${documentRecord.id}`);
  });
  actions.append(inspectButton);
  row.append(actions);
  return row;
}

function renderKnowledgeReviewQueue(documents) {
  if (!knowledgeReviewQueue) {
    return;
  }
  clearElement(knowledgeReviewQueue);
  if (!documents || documents.length === 0) {
    knowledgeReviewQueue.textContent = "Review queue";
    return;
  }
  const header = document.createElement("strong");
  header.textContent = `Review queue (${documents.length})`;
  knowledgeReviewQueue.append(header);
  for (const documentRecord of documents) {
    knowledgeReviewQueue.append(renderKnowledgeReviewRow(documentRecord));
  }
}

async function loadKnowledgeReviewQueue() {
  if (!knowledgeReviewQueue) {
    return;
  }
  const params = new URLSearchParams({
    space_id: selectedKnowledgeSpaceId,
    review_status: "pending_review",
  });
  const response = await fetch(`${knowledgeListPath}?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`knowledge review queue failed (${response.status})`);
  }
  renderKnowledgeReviewQueue(await response.json());
}

async function updateKnowledgeReview(reviewStatus) {
  if (!selectedKnowledgeDocumentId) {
    setKnowledgeStatus("Select a document before changing review status");
    return;
  }
  let reason = "";
  if (reviewStatus === "rejected") {
    reason = window.prompt("Rejection reason (optional)", "") || "";
  }
  await postJSON(knowledgeReviewPath, {
    document_id: selectedKnowledgeDocumentId,
    review_status: reviewStatus,
    reason: reason.trim(),
    reviewed_by: "operator",
  });
  setKnowledgeStatus(`Updated review status to ${reviewStatus}`);
  await refreshKnowledgeWorkspace();
  await inspectKnowledgeDocument(selectedKnowledgeDocumentId);
}

function renderKnowledgeReviewActions(documentRecord) {
  if (!knowledgeReviewActions) {
    return;
  }
  clearElement(knowledgeReviewActions);
  if (!documentRecord?.id) {
    return;
  }
  const actions = [];
  switch (effectiveReviewStatus(documentRecord)) {
    case "pending_review":
      actions.push(["Approve", "active"], ["Reject", "rejected"], ["Archive", "archived"]);
      break;
    case "active":
      actions.push(["Reject", "rejected"], ["Archive", "archived"]);
      break;
    case "rejected":
      actions.push(["Send to review", "pending_review"], ["Archive", "archived"]);
      break;
    case "archived":
      actions.push(["Reactivate", "active"], ["Send to review", "pending_review"]);
      break;
    default:
      actions.push(["Approve", "active"]);
      break;
  }
  for (const [label, value] of actions) {
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = label;
    button.addEventListener("click", async () => {
      await updateKnowledgeReview(value);
    });
    knowledgeReviewActions.append(button);
  }
}

function renderKnowledgeHealth(summary) {
  if (!summary) {
    knowledgeHealthStatus.textContent = "Health summary unavailable";
    clearElement(knowledgeHealthMetrics);
    knowledgeAttentionReasons.textContent = "No attention reasons";
    return;
  }
  knowledgeHealthStatus.textContent = `${summary.space_name || "Knowledge"}: ${summary.status || "unknown"}`;
  clearElement(knowledgeHealthMetrics);
  for (const [label, value] of [
    ["active", summary.active_document_count ?? 0],
    ["disabled", summary.disabled_document_count ?? 0],
    ["failed", summary.failed_document_count ?? 0],
    ["chunks", summary.chunk_count ?? 0]
  ]) {
    knowledgeHealthMetrics.append(renderKnowledgeMetric(label, value));
  }
  const reasons = summary.attention_reasons || [];
  knowledgeAttentionReasons.textContent = reasons.length > 0 ? reasons.join(" | ") : "No attention reasons";
}

async function loadKnowledgeHealth() {
  const response = await fetch(`${knowledgeHealthPath}?space_id=${encodeURIComponent(selectedKnowledgeSpaceId)}`);
  if (!response.ok) throw new Error(`health failed (${response.status})`);
  const summary = await response.json();
  renderKnowledgeHealth(summary);
}

function formatStageSummary(result) {
  const ran = (result.stages_run || []).join(", ") || "none";
  const skipped = (result.stages_skipped || []).join(", ") || "none";
  return `Stages run: ${ran}\nStages skipped: ${skipped}`;
}

function formatExplanation(explanation) {
  const matchedTerms = (explanation.matched_terms || []).join(", ") || "none";
  return [
    `chunk_id: ${explanation.chunk_id}`,
    `document_id: ${explanation.document_id}`,
    `lexical_score: ${explanation.lexical_score ?? 0}`,
    `vector_score: ${explanation.vector_score ?? 0}`,
    `final_score: ${explanation.final_score ?? 0}`,
    `rank_reason: ${explanation.rank_reason || "n/a"}`,
    `matched_terms: ${matchedTerms}`,
    `index_status: ${explanation.index_status || "n/a"}`
  ].join("\n");
}

function renderKnowledgeDiagnostics(result) {
  const sections = [];
  sections.push(`mode: ${result.mode || "unknown"}`);
  if (result.no_source_reason) {
    sections.push(`no_source_reason: ${result.no_source_reason}`);
  }
  sections.push(formatStageSummary(result));

  const explanations = result.explanations || [];
  if (explanations.length === 0) {
    sections.push("No ranked chunks");
  } else {
    sections.push(explanations.map((explanation, index) => `#${index + 1}\n${formatExplanation(explanation)}`).join("\n\n"));
  }

  return sections.join("\n\n");
}

function renderKnowledgeDebugResults(result) {
  clearElement(knowledgeDebugResults);
  const header = document.createElement("div");
  header.className = "knowledge-debug-header";
  header.textContent = `mode ${result.mode || "unknown"}`;
  knowledgeDebugResults.append(header);

  if (result.no_source_reason) {
    const empty = document.createElement("div");
    empty.className = "knowledge-debug-empty";
    empty.textContent = `no source: ${result.no_source_reason}`;
    knowledgeDebugResults.append(empty);
  }

  const stageSummary = document.createElement("div");
  stageSummary.className = "knowledge-debug-stages";
  stageSummary.textContent = formatStageSummary(result);
  knowledgeDebugResults.append(stageSummary);

  const explanations = result.explanations || [];
  if (explanations.length === 0) {
    const empty = document.createElement("div");
    empty.className = "knowledge-debug-empty";
    empty.textContent = "No ranked chunks";
    knowledgeDebugResults.append(empty);
    return;
  }

  for (const explanation of explanations) {
    knowledgeDebugResults.append(renderKnowledgeDebugRow(explanation));
  }
}

function renderKnowledgeDetail(detail) {
  const documentRecord = detail.document || {};
  const qualityFlags = detail.quality_flags || [];
  const chunks = (documentRecord.chunks || []).map((chunk) => chunk.text).join("\n\n") || "Chunk preview";
  const indexState = documentRecord.metadata?.vector_status || "unknown";
  const lastErrorCode = documentRecord.metadata?.last_error_code || "none";
  selectedKnowledgeDocument = detail;
  selectedKnowledgeDocumentId = documentRecord.id || "";
  if (knowledgeDetailTitle) {
    knowledgeDetailTitle.textContent = documentRecord.name || documentRecord.id || "Document detail";
  }
  if (knowledgeDetailMeta) {
    knowledgeDetailMeta.textContent = renderKnowledgeSourceMeta(documentRecord);
  }
  clearElement(knowledgeDetailFlags);
  knowledgeDetailFlags.append(renderKnowledgeFlag(`review_${effectiveReviewStatus(documentRecord)}`));
  if (qualityFlags.length === 0) {
    knowledgeDetailFlags.append(renderKnowledgeFlag("healthy"));
  } else {
    for (const flag of qualityFlags) {
      knowledgeDetailFlags.append(renderKnowledgeFlag(flag));
    }
  }
  renderKnowledgeReviewActions(documentRecord);
  renderKnowledgeRelations(detail.relations || {});
  knowledgeDetailBody.textContent = [
    `document: ${documentRecord.name || documentRecord.id || "unknown"}`,
    `status: ${documentRecord.status || "unknown"}`,
    `review_status: ${effectiveReviewStatus(documentRecord)}`,
    `space_id: ${documentRecord.space_id || selectedKnowledgeSpaceId}`,
    `index_state: ${indexState}`,
    `last_error_code: ${lastErrorCode}`,
    "",
    chunks
  ].join("\n");
  if (knowledgeEditDraft === null) {
    cancelKnowledgeEdit();
  }
}

function renderKnowledgeSourceMeta(documentRecord) {
  const segments = [];
  segments.push(`space ${documentRecord.space_id || selectedKnowledgeSpaceId}`);
  if (documentRecord.metadata?.source_type) {
    segments.push(`source ${documentRecord.metadata.source_type}`);
  }
  if (documentRecord.metadata?.source_label) {
    segments.push(`label ${documentRecord.metadata.source_label}`);
  }
  if (documentRecord.metadata?.source_gap_id) {
    segments.push(`gap ${documentRecord.metadata.source_gap_id}`);
  }
  if (documentRecord.review_reason) {
    segments.push(`review ${documentRecord.review_reason}`);
  }
  return segments.join(" | ") || "No source metadata";
}

function renderKnowledgeRelations(relations) {
  if (!knowledgeDetailRelations) {
    return;
  }
  const lines = ["Source relationships"];
  if (relations.source_gap?.id) {
    lines.push(`Created from gap: ${relations.source_gap.id} | ${relations.source_gap.question || "no question"}`);
  }
  const resolvedGaps = relations.resolved_gaps || [];
  if (resolvedGaps.length > 0) {
    lines.push(`Resolved gaps: ${resolvedGaps.map((gap) => `${gap.id} | ${gap.question || "no question"}`).join(" || ")}`);
  }
  if (!relations.source_gap?.id && resolvedGaps.length === 0) {
    lines.push("No linked gaps");
  }
  knowledgeDetailRelations.textContent = lines.join("\n");
}

async function inspectKnowledgeDocument(documentID) {
  if (!documentID) {
    setKnowledgeStatus("Knowledge error: missing document id");
    return;
  }
  const detailURL = `${knowledgeDetailPathPrefix}${documentID}/detail`;
  const detail = await fetch(detailURL);
  if (!detail.ok) {
    throw new Error(`${detailURL} failed (${detail.status})`);
  }
  renderKnowledgeDetail(await detail.json());
}

function renderKnowledgeDebugRow(explanation) {
  const row = document.createElement("div");
  row.className = "knowledge-debug-row";

  const title = document.createElement("strong");
  title.textContent = `${explanation.document_id} / ${explanation.chunk_id}`;
  row.append(title);

  const body = document.createElement("div");
  body.className = "knowledge-debug-body";
  body.textContent = formatExplanation(explanation);
  row.append(body);
  return row;
}

function renderKnowledgeGapRow(gap) {
  const row = document.createElement("div");
  row.className = "knowledge-gap-row";
  const summary = document.createElement("div");
  summary.className = "knowledge-gap-summary";
  const evidence = [];
  if (gap.resolved_by_document_id) {
    evidence.push(`document ${gap.resolved_by_document_id}`);
  }
  if (gap.resolution_note) {
    evidence.push(gap.resolution_note);
  }
  summary.textContent = `${gap.status}: ${gap.question} (${gap.no_source_reason})${evidence.length > 0 ? ` | ${evidence.join(" | ")}` : ""}`;
  row.append(summary);

  const actions = document.createElement("div");
  actions.className = "knowledge-gap-actions";

  if (gap.status === "open") {
    const investigateButton = document.createElement("button");
    investigateButton.type = "button";
    investigateButton.textContent = "Investigate";
    investigateButton.addEventListener("click", async () => {
      await knowledgeGapInvestigate(gap);
    });
    actions.append(investigateButton);
  }

  const diagnosticsButton = document.createElement("button");
  diagnosticsButton.type = "button";
  diagnosticsButton.textContent = "Run diagnostics";
  diagnosticsButton.addEventListener("click", async () => {
    await runKnowledgeGapDiagnostics(gap);
  });
  actions.append(diagnosticsButton);

  const createNoteButton = document.createElement("button");
  createNoteButton.type = "button";
  createNoteButton.textContent = "Create note";
  createNoteButton.addEventListener("click", () => {
    createKnowledgeNoteFromGap(gap);
  });
  actions.append(createNoteButton);

  if (gap.status !== "resolved") {
    const resolveButton = document.createElement("button");
    resolveButton.type = "button";
    resolveButton.textContent = "Resolve";
    resolveButton.addEventListener("click", async () => {
      await resolveKnowledgeGap(gap);
    });
    actions.append(resolveButton);
  }

  if (gap.status === "open") {
    const ignoreButton = document.createElement("button");
    ignoreButton.type = "button";
    ignoreButton.textContent = "Ignore";
    ignoreButton.addEventListener("click", async () => {
      await postJSON(knowledgeGapUpdatePath, {
        gap_id: gap.id,
        status: "ignored"
      });
      await refreshKnowledgeWorkspace();
    });
    actions.append(ignoreButton);
  }

  if (actions.childNodes.length > 0) {
    row.append(actions);
  }

  return row;
}

function renderKnowledgeImportJob(job) {
  const row = document.createElement("div");
  row.className = "knowledge-import-job-row";

  const summary = document.createElement("div");
  summary.className = "knowledge-import-job-summary";
  const imported = job.imported_document_ids || [];
  const skipped = job.skipped_sources || [];
  const failed = job.failed_sources || [];
  const warnings = [];
  for (const documentID of imported) {
    const documentRecord = (selectedKnowledgeDocument?.document?.id === documentID ? selectedKnowledgeDocument.document : null);
    if (documentRecord?.metadata?.source_warning) {
      warnings.push(documentRecord.metadata.source_warning);
    }
  }
  summary.textContent = [
    `${job.status || "unknown"} | ${job.source_type || "unknown"} | ${job.source_label || "unlabeled"}`,
    `sources ${job.source_count ?? 0} | imported ${imported.length} | skipped ${skipped.length} | failed ${failed.length}`,
    skipped.map((item) => `${item.name || item.uri || "source"} skipped ${item.reason || "duplicate_content"}`).join(" | "),
    failed.map((item) => `${item.name || item.uri || "source"} failed ${item.reason || "unsupported_extension"}`).join(" | "),
    warnings.length > 0 ? `source_warning ${warnings.join(" | ")}` : "",
    job.error_code ? `error ${job.error_code}` : "",
  ].filter(Boolean).join("\n");
  row.append(summary);

  const actions = document.createElement("div");
  actions.className = "knowledge-import-job-actions";

  for (const documentID of imported) {
    const inspectButton = document.createElement("button");
    inspectButton.type = "button";
    inspectButton.textContent = `Inspect ${documentID}`;
    inspectButton.addEventListener("click", async () => {
      await inspectKnowledgeDocument(documentID);
      setKnowledgeStatus(`Inspecting imported document ${documentID}`);
    });
    actions.append(inspectButton);
  }

  if (actions.childNodes.length > 0) {
    row.append(actions);
  }

  return row;
}

async function loadKnowledgeGaps() {
  const response = await fetch(`${knowledgeGapListPath}?space_id=${encodeURIComponent(selectedKnowledgeSpaceId)}`);
  if (!response.ok) throw new Error(`knowledge gaps failed (${response.status})`);
  const gaps = await response.json() || [];
  clearElement(knowledgeGapQueue);
  if (gaps.length === 0) {
    knowledgeGapQueue.textContent = "Knowledge gaps";
    return;
  }
  for (const gap of gaps) {
    knowledgeGapQueue.append(renderKnowledgeGapRow(gap));
  }
}

async function loadKnowledgeImportJobs() {
  if (!knowledgeImportJobs) {
    return;
  }
  const response = await fetch(`${knowledgeImportListPath}?space_id=${encodeURIComponent(selectedKnowledgeSpaceId)}`);
  if (!response.ok) {
    throw new Error(`knowledge imports failed (${response.status})`);
  }
  const jobs = await response.json() || [];
  clearElement(knowledgeImportJobs);
  if (jobs.length === 0) {
    knowledgeImportJobs.textContent = "Recent imports";
    return;
  }
  for (const job of jobs) {
    knowledgeImportJobs.append(renderKnowledgeImportJob(job));
  }
}

function createKnowledgeNoteFromGap(gap) {
  selectedKnowledgeGapId = gap.id;
  if (knowledgeNoteGapContext) {
    knowledgeNoteGapContext.textContent = `Gap ${gap.id}: ${gap.question}`;
  }
  if (knowledgeNoteTitle && !knowledgeNoteTitle.value.trim()) {
    knowledgeNoteTitle.value = gap.question;
  }
  if (knowledgeNoteBody && !knowledgeNoteBody.value.trim()) {
    knowledgeNoteBody.value = `Question: ${gap.question}\n\nAnswer this gap with durable source text.\n\n`;
  }
  knowledgeNoteTitle?.focus();
}

function startKnowledgeEdit() {
  const documentRecord = selectedKnowledgeDocument?.document;
  if (!documentRecord) {
    setKnowledgeStatus("Select a document before editing");
    return;
  }
  knowledgeEditDraft = {
    document_id: documentRecord.id,
    name: documentRecord.name || "",
    source_label: documentRecord.metadata?.source_label || "",
    content: (documentRecord.chunks || []).map((chunk) => chunk.text).join("\n\n"),
  };
  if (knowledgeEditName) {
    knowledgeEditName.value = knowledgeEditDraft.name;
  }
  if (knowledgeEditSourceLabel) {
    knowledgeEditSourceLabel.value = knowledgeEditDraft.source_label;
  }
  if (knowledgeEditContent) {
    knowledgeEditContent.value = knowledgeEditDraft.content;
  }
  knowledgeEditForm?.removeAttribute("hidden");
  knowledgeEditName?.focus();
}

function cancelKnowledgeEdit() {
  knowledgeEditDraft = null;
  if (knowledgeEditName) {
    knowledgeEditName.value = "";
  }
  if (knowledgeEditSourceLabel) {
    knowledgeEditSourceLabel.value = "";
  }
  if (knowledgeEditContent) {
    knowledgeEditContent.value = "";
  }
  knowledgeEditForm?.setAttribute("hidden", "hidden");
}

async function saveKnowledgeEdit() {
  if (!selectedKnowledgeDocumentId) {
    setKnowledgeStatus("Select a document before saving an edit");
    return;
  }
  const updated = await postJSON(knowledgeUpdatePath, {
    document_id: selectedKnowledgeDocumentId,
    name: knowledgeEditName?.value || "",
    content: knowledgeEditContent?.value || "",
    source_label: knowledgeEditSourceLabel?.value || "",
  });
  setKnowledgeStatus(`Updated ${updated.id}`);
  cancelKnowledgeEdit();
  await refreshKnowledgeWorkspace();
  const detailURL = `${knowledgeDetailPathPrefix}${updated.id}/detail`;
  const detail = await fetch(detailURL);
  if (!detail.ok) throw new Error(`${detailURL} failed (${detail.status})`);
  renderKnowledgeDetail(await detail.json());
}

function applyKnowledgeFilters() {
  return loadKnowledge();
}

async function resetKnowledgeFilters() {
  if (knowledgeFilterQuery) {
    knowledgeFilterQuery.value = "";
  }
  if (knowledgeFilterStatus) {
    knowledgeFilterStatus.value = "";
  }
  if (knowledgeFilterReviewStatus) {
    knowledgeFilterReviewStatus.value = "";
  }
  if (knowledgeFilterSourceType) {
    knowledgeFilterSourceType.value = "";
  }
  if (knowledgeFilterGapLinked) {
    knowledgeFilterGapLinked.checked = false;
  }
  await loadKnowledge();
}

async function knowledgeGapInvestigate(gap) {
  await postJSON(knowledgeGapUpdatePath, {
    gap_id: gap.id,
    status: "investigating"
  });
  setKnowledgeStatus(`Investigating gap ${gap.id}`);
  createKnowledgeNoteFromGap(gap);
  await refreshKnowledgeWorkspace();
}

async function runKnowledgeGapDiagnostics(gap) {
  const diagnostics = await postJSON("/admin/knowledge/retrieval-diagnostics", {
    query: gap.question,
    mode: knowledgeQueryMode?.value || "auto",
    space_id: gap.space_id || selectedKnowledgeSpaceId,
    limit: 3
  });
  knowledgeGapDiagnosticsState.set(gap.id, diagnostics);
  if (knowledgeQuery) {
    knowledgeQuery.value = gap.question;
  }
  renderKnowledgeDebugResults(diagnostics);
  if ((diagnostics.explanations || []).length > 0) {
    setKnowledgeStatus(`Diagnostics found ${(diagnostics.explanations || []).length} ranked chunks for ${gap.id}`);
  } else if (diagnostics.no_source_reason) {
    setKnowledgeStatus(`No source for ${gap.id}: ${diagnostics.no_source_reason}`);
  } else {
    setKnowledgeStatus(`No ranked chunks for ${gap.id}`);
  }
}

async function resolveKnowledgeGap(gap) {
  const diagnostics = knowledgeGapDiagnosticsState.get(gap.id);
  const hasEvidence = (diagnostics?.explanations || []).length > 0;
  if (!hasEvidence) {
    const proceed = window.confirm("Diagnostics still show no ranked chunks for this gap. Resolve anyway?");
    if (!proceed) {
      return;
    }
  }
  const resolvedByDocumentID = window.prompt("Resolved by document ID (optional)", "") || "";
  const resolutionNote = window.prompt("Resolution note (optional)", "") || "";
  await postJSON(knowledgeGapUpdatePath, {
    gap_id: gap.id,
    status: "resolved",
    resolved_by_document_id: resolvedByDocumentID.trim(),
    resolution_note: resolutionNote.trim()
  });
  setKnowledgeStatus(`Resolved gap ${gap.id}`);
  await refreshKnowledgeWorkspace();
}

async function refreshKnowledgeWorkspace() {
  await loadKnowledge();
  await loadKnowledgeHealth();
  await loadKnowledgeGaps();
  await loadKnowledgeImportJobs();
  await loadKnowledgeReviewQueue();
}

function draftPayload() {
  const identity = personaDraft.value.split(",")[0]?.trim() || "Digital Twin";
  return {
    id: "advisor",
    identity,
    role: "professional digital advisor",
    tone: ["calm", "precise"],
    boundaries: ["state uncertainty when confidence is low"],
    allowed_claims: ["can explain planning tradeoffs"],
    locale: "en-US"
  };
}

async function postJSON(url, body) {
  const response = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body)
  });
  if (!response.ok) {
    const error = new Error(`${url} failed (${response.status})`);
    try {
      error.data = await response.json();
    } catch {
      error.data = null;
    }
    throw error;
  }
  return response.json();
}

async function loadActivePersona() {
  const response = await fetch("/admin/persona/active");
  if (!response.ok) throw new Error(`active failed (${response.status})`);
  const active = await response.json();
  if (active.status === "none") {
    setPersonaStatus("No active persona published");
    return;
  }
  activeVersionId = active.id;
  setPersonaStatus(`Active: ${active.persona.identity}`);
}

async function loadMemory() {
  const response = await fetch("/admin/memory");
  if (!response.ok) return;
  const records = await response.json();
  memoryTableBody.textContent = "";
  if (records.length === 0) {
    const row = document.createElement("tr");
    const cell = document.createElement("td");
    cell.colSpan = 3;
    cell.textContent = "No active memory";
    row.append(cell);
    memoryTableBody.append(row);
    return;
  }
  for (const record of records) {
    memoryTableBody.append(renderMemoryRow(record));
  }
}

function renderMemoryRow(record) {
  const row = document.createElement("tr");
  const idCell = document.createElement("td");
  idCell.textContent = record.id;
  const statusCell = document.createElement("td");
  statusCell.textContent = record.status;
  const actionCell = document.createElement("td");
  if (record.status === "active") {
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = "Disable";
    button.addEventListener("click", async () => {
      await postJSON("/admin/memory/disable", { memory_id: record.id });
      await loadMemory();
    });
    actionCell.append(button);
  } else {
    actionCell.textContent = "Disabled";
  }
  row.append(idCell, statusCell, actionCell);
  return row;
}

async function loadKnowledge() {
  const params = new URLSearchParams({ space_id: selectedKnowledgeSpaceId });
  if (knowledgeFilterQuery?.value.trim()) {
    params.set("query", knowledgeFilterQuery.value.trim());
  }
  if (knowledgeFilterStatus?.value) {
    params.set("status", knowledgeFilterStatus.value);
  }
  if (knowledgeFilterReviewStatus?.value) {
    params.set("review_status", knowledgeFilterReviewStatus.value);
  }
  if (knowledgeFilterSourceType?.value) {
    params.set("source_type", knowledgeFilterSourceType.value);
  }
  if (knowledgeFilterGapLinked?.checked) {
    params.set("gap_linked", "true");
  }
  const response = await fetch(`${knowledgeListPath}?${params.toString()}`);
  if (!response.ok) return;
  const documents = await response.json();
  knowledgeTableBody.textContent = "";
  if (documents.length === 0) {
    const row = document.createElement("tr");
    const cell = document.createElement("td");
    cell.colSpan = 5;
    cell.textContent = "No knowledge loaded";
    row.append(cell);
    knowledgeTableBody.append(row);
    if (knowledgeDetailTitle) {
      knowledgeDetailTitle.textContent = "Document detail";
    }
    if (knowledgeDetailMeta) {
      knowledgeDetailMeta.textContent = "No document selected";
    }
    if (knowledgeDetailRelations) {
      knowledgeDetailRelations.textContent = "Source relationships";
    }
    if (knowledgeReviewActions) {
      knowledgeReviewActions.textContent = "";
    }
    knowledgeDetailBody.textContent = "Chunk preview";
    clearElement(knowledgeDetailFlags);
    selectedKnowledgeDocumentId = "";
    selectedKnowledgeDocument = null;
    cancelKnowledgeEdit();
    return;
  }
  for (const documentRecord of documents) {
    knowledgeTableBody.append(renderKnowledgeRow(documentRecord));
  }
}

async function loadKnowledgeSpaces() {
  const response = await fetch("/admin/knowledge/spaces");
  if (!response.ok) return;
  const spaces = await response.json();
  knowledgeSpaceSelect.textContent = "";
  for (const space of spaces) {
    const option = document.createElement("option");
    option.value = space.id;
    option.textContent = space.name;
    if (space.id === selectedKnowledgeSpaceId) {
      option.selected = true;
    }
    knowledgeSpaceSelect.append(option);
  }
  if (spaces.length > 0) {
    const selected = spaces.find((space) => space.id === selectedKnowledgeSpaceId) || spaces[0];
    selectedKnowledgeSpaceId = selected.id;
    knowledgeSpaceSelect.value = selected.id;
  }
}

function updateKnowledgeImportMode() {
  const sourceType = knowledgeImportSourceType?.value || "local_text_file";
  if (sourceType === "url_text_snapshot") {
    knowledgeImportFileFields?.setAttribute("hidden", "hidden");
    knowledgeImportURLFields?.removeAttribute("hidden");
  } else {
    knowledgeImportURLFields?.setAttribute("hidden", "hidden");
    knowledgeImportFileFields?.removeAttribute("hidden");
  }
}

async function readKnowledgeImportSources() {
  const sourceType = knowledgeImportSourceType?.value || "local_text_file";
  if (sourceType === "url_text_snapshot") {
    return [{
      uri: (knowledgeImportURL?.value || "").trim(),
      content: knowledgeImportContent?.value || "",
    }];
  }

  const files = Array.from(knowledgeUpload?.files || []);
  return Promise.all(files.map(async (file) => ({
    name: file.name,
    content: await file.text(),
    size_bytes: file.size,
  })));
}

async function importKnowledge() {
  const sourceType = knowledgeImportSourceType?.value || "local_text_file";
  const sourceLabel = (knowledgeImportSourceLabel?.value || "").trim();
  const sources = await readKnowledgeImportSources();
  if (sources.length === 0) {
    setKnowledgeStatus("Knowledge import error: select a source first");
    return;
  }

  try {
    const job = await postJSON(knowledgeImportPath, {
      space_id: selectedKnowledgeSpaceId,
      source_type: sourceType,
      source_label: sourceLabel,
      sources
    });
    if (knowledgeUpload) {
      knowledgeUpload.value = "";
    }
    if (knowledgeImportURL) {
      knowledgeImportURL.value = "";
    }
    if (knowledgeImportContent) {
      knowledgeImportContent.value = "";
    }
    setKnowledgeStatus(`Imported ${job.imported_document_ids?.length ?? 0} documents via ${job.id}`);
    await refreshKnowledgeWorkspace();
  } catch (error) {
    if (error.data?.job) {
      await loadKnowledgeImportJobs();
    }
    const cause = error.data?.cause || error.message;
    setKnowledgeStatus(`Knowledge import error: ${cause}`);
  }
}

function renderKnowledgeRow(documentRecord) {
  const row = document.createElement("tr");

  const nameCell = document.createElement("td");
  nameCell.textContent = documentRecord.name;

  const statusCell = document.createElement("td");
  statusCell.textContent = `${documentRecord.status} / ${effectiveReviewStatus(documentRecord)}`;

  const chunkCountCell = document.createElement("td");
  chunkCountCell.textContent = String(documentRecord.chunk_count ?? documentRecord.chunks?.length ?? 0);

  const qualityCell = document.createElement("td");
  qualityCell.textContent = documentRecord.metadata?.vector_status || "pending";

  const actionCell = document.createElement("td");
  const inspectButton = document.createElement("button");
  inspectButton.type = "button";
  inspectButton.textContent = "Inspect";
  inspectButton.addEventListener("click", async () => {
    await inspectKnowledgeDocument(documentRecord.id);
  });

  const toggleButton = document.createElement("button");
  toggleButton.type = "button";
  toggleButton.textContent = documentRecord.status === "disabled" ? "Enable" : "Disable";
  toggleButton.addEventListener("click", async () => {
    const url = documentRecord.status === "disabled" ? "/admin/knowledge/enable" : "/admin/knowledge/disable";
    await postJSON(url, { document_id: documentRecord.id });
    await refreshKnowledgeWorkspace();
  });

  const reindexButton = document.createElement("button");
  reindexButton.type = "button";
  reindexButton.textContent = "Reindex";
  reindexButton.addEventListener("click", async () => {
    await postJSON("/admin/knowledge/reindex", {
      document_id: documentRecord.id,
      content: (documentRecord.chunks || []).map((chunk) => chunk.text).join("\n\n")
    });
    await refreshKnowledgeWorkspace();
  });

  const deleteButton = document.createElement("button");
  deleteButton.type = "button";
  deleteButton.textContent = "Delete";
  deleteButton.addEventListener("click", async () => {
    await postJSON("/admin/knowledge/delete", { document_id: documentRecord.id });
    await refreshKnowledgeWorkspace();
  });

  actionCell.append(inspectButton, toggleButton, reindexButton, deleteButton);
  row.append(nameCell, statusCell, chunkCountCell, qualityCell, actionCell);
  return row;
}

knowledgeUploadMock?.addEventListener("click", async () => {
  try {
    const uploaded = await postJSON("/admin/knowledge/upload", {
      id: `kb-${Date.now()}`,
      space_id: selectedKnowledgeSpaceId,
      name: "mock.md",
      content: "Phase 4 adds a digital human UI.\n\nIt includes persona, memory, and knowledge admin controls."
    });
    const citation = await postJSON("/admin/knowledge/citation-test", { query: "digital human UI" });
    setKnowledgeStatus(`Uploaded ${uploaded.chunk_count ?? uploaded.chunks.length} chunks; citation ${citation.chunk_id}`);
    await refreshKnowledgeWorkspace();
  } catch (error) {
    setKnowledgeStatus(`Knowledge error: ${error.message}`);
  }
});

knowledgeImportSourceType?.addEventListener("change", () => {
  updateKnowledgeImportMode();
});

knowledgeImportRun?.addEventListener("click", async () => {
  await importKnowledge();
});

knowledgeNoteCreate?.addEventListener("click", async () => {
  const title = (knowledgeNoteTitle?.value || "").trim();
  const body = (knowledgeNoteBody?.value || "").trim();
  if (!title) {
    setKnowledgeStatus("Knowledge error: missing note title");
    return;
  }
  if (!body) {
    setKnowledgeStatus("Knowledge error: missing note body");
    return;
  }
  try {
    const created = await postJSON(knowledgeNoteCreatePath, {
      space_id: selectedKnowledgeSpaceId,
      title,
      body,
      source_gap_id: selectedKnowledgeGapId || undefined
    });
    setKnowledgeStatus(`Created note ${created.name}`);
    if (knowledgeNoteTitle) {
      knowledgeNoteTitle.value = "";
    }
    if (knowledgeNoteBody) {
      knowledgeNoteBody.value = "";
    }
    selectedKnowledgeGapId = "";
    if (knowledgeNoteGapContext) {
      knowledgeNoteGapContext.textContent = "No gap selected";
    }
    await refreshKnowledgeWorkspace();
  } catch (error) {
    setKnowledgeStatus(`Knowledge error: ${error.message}`);
  }
});

knowledgeQueryRun?.addEventListener("click", async () => {
  try {
    const diagnostics = await postJSON("/admin/knowledge/retrieval-diagnostics", {
      query: knowledgeQuery?.value || "",
      mode: knowledgeQueryMode?.value || "auto",
      space_id: selectedKnowledgeSpaceId,
      limit: 3
    });
    const topResult = diagnostics.results?.[0];
    if (topResult) {
      setKnowledgeStatus(`Top match ${topResult.chunk_id} (${topResult.score})`);
    } else if (diagnostics.no_source_reason) {
      setKnowledgeStatus(`No source: ${diagnostics.no_source_reason}`);
    } else {
      setKnowledgeStatus("No retrieval results");
    }
    renderKnowledgeDebugResults(diagnostics);
  } catch (error) {
    setKnowledgeStatus(`Knowledge error: ${error.message}`);
  }
});

knowledgeSpaceSelect?.addEventListener("change", async () => {
  selectedKnowledgeSpaceId = knowledgeSpaceSelect.value || "default";
  await refreshKnowledgeWorkspace();
});

knowledgeFilterApply?.addEventListener("click", async () => {
  await applyKnowledgeFilters();
});

knowledgeFilterReset?.addEventListener("click", async () => {
  await resetKnowledgeFilters();
});

knowledgeEditToggle?.addEventListener("click", () => {
  startKnowledgeEdit();
});

knowledgeEditCancel?.addEventListener("click", () => {
  cancelKnowledgeEdit();
});

knowledgeEditSave?.addEventListener("click", async () => {
  try {
    await saveKnowledgeEdit();
  } catch (error) {
    setKnowledgeStatus(`Knowledge update failed: ${error.message}`);
  }
});

knowledgeSpaceCreateButton?.addEventListener("click", async () => {
  const rawName = (knowledgeSpaceCreate?.value || "").trim();
  const id = rawName.toLowerCase().replace(/\s+/g, "-");
  if (!id) {
    setKnowledgeStatus("Knowledge error: missing space name");
    return;
  }
  try {
    await postJSON("/admin/knowledge/spaces/create", {
      id,
      name: rawName
    });
    selectedKnowledgeSpaceId = id;
    if (knowledgeSpaceCreate) {
      knowledgeSpaceCreate.value = "";
    }
    await loadKnowledgeSpaces();
    await refreshKnowledgeWorkspace();
  } catch (error) {
    setKnowledgeStatus(`Knowledge error: ${error.message}`);
  }
});

toolSavePolicy?.addEventListener("click", async () => {
  try {
    const allowedTools = toolKnowledgeSearch.checked ? ["knowledge.search"] : [];
    await postJSON("/admin/tools/policy", {
      persona_id: "advisor",
      allowed_tools: allowedTools,
      approval_mode: "manual"
    });
    await postJSON("/admin/tools/authorize", {
      persona_id: "advisor",
      tool_name: "knowledge.search"
    });
    toolStatus.textContent = "Policy saved and verified";
  } catch (error) {
    toolStatus.textContent = `Policy error: ${error.message}`;
  }
});

async function loadAudit() {
  const response = await fetch("/admin/audit");
  if (!response.ok) return;
  const records = await response.json();
  auditTableBody.textContent = "";
  if (records.length === 0) {
    const row = document.createElement("tr");
    const cell = document.createElement("td");
    cell.colSpan = 5;
    cell.textContent = "No audit records";
    row.append(cell);
    auditTableBody.append(row);
    return;
  }
  for (const record of records) {
    const row = document.createElement("tr");
    const conversationCell = document.createElement("td");
    conversationCell.textContent = record.conversation_id;
    const statusCell = document.createElement("td");
    statusCell.textContent = record.status;
    const trustCell = document.createElement("td");
    trustCell.textContent = renderAuditTrust(record);
    const evidenceCell = document.createElement("td");
    appendAuditEvidence(evidenceCell, record);
    const agentCell = document.createElement("td");
    agentCell.textContent = record.agent_name;
    row.append(conversationCell, statusCell, trustCell, evidenceCell, agentCell);
    auditTableBody.append(row);
  }
}

async function loadAuditTimeline() {
  if (!auditTimelineBody) {
    return;
  }
  const response = await fetch(auditTimelineQuery());
  if (!response.ok) {
    throw new Error(`audit timeline failed (${response.status})`);
  }
  const items = await response.json();
  clearElement(auditTimelineBody);
  if (!Array.isArray(items) || items.length === 0) {
    auditTimelineBody.textContent = "No timeline records";
    return;
  }
  for (const item of items) {
    auditTimelineBody.append(renderAuditTimelineItem(item));
  }
}

saveDraftButton?.addEventListener("click", async () => {
  try {
    const draft = await postJSON("/admin/persona/drafts", draftPayload());
    currentDraftId = draft.id;
    setPersonaStatus(`Draft saved: ${draft.id}`);
  } catch (error) {
    setPersonaStatus(`Draft error: ${error.message}`);
  }
});

publishButton?.addEventListener("click", async () => {
  try {
    const published = await postJSON("/admin/persona/publish", { version_id: currentDraftId });
    activeVersionId = published.id;
    setPersonaStatus(`Published: ${published.persona.identity}`);
  } catch (error) {
    setPersonaStatus(`Publish error: ${error.message}`);
  }
});

rollbackButton?.addEventListener("click", async () => {
  try {
    const rolledBack = await postJSON("/admin/persona/rollback", { version_id: activeVersionId });
    setPersonaStatus(`Rolled back: ${rolledBack.persona.identity}`);
  } catch (error) {
    setPersonaStatus(`Rollback error: ${error.message}`);
  }
});

loadActivePersona().catch((error) => {
  setPersonaStatus(`Active error: ${error.message}`);
});
loadKnowledgeSpaces().catch(() => {});
loadMemory().catch(() => {});
updateKnowledgeImportMode();
refreshKnowledgeWorkspace().catch(() => {});
auditRefresh?.addEventListener("click", () => {
  loadAudit().catch(() => {});
});
auditTimelineRefresh?.addEventListener("click", () => {
  loadAuditTimeline().catch((error) => {
    setKnowledgeStatus(`Audit timeline error: ${error.message}`);
  });
});
loadAudit().catch(() => {});
loadAuditTimeline().catch(() => {});
