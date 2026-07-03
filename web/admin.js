const personaDraft = document.querySelector("#persona-draft");
const personaStatus = document.querySelector("#persona-status");
const saveDraftButton = document.querySelector("#persona-save-draft");
const publishButton = document.querySelector("#persona-publish");
const rollbackButton = document.querySelector("#persona-rollback");
const memoryTableBody = document.querySelector("#memory-table-body");
const knowledgeSpaceSelect = document.querySelector("#knowledge-space-select");
const knowledgeSpaceCreate = document.querySelector("#knowledge-space-create");
const knowledgeSpaceCreateButton = document.querySelector("#knowledge-space-create-button");
const knowledgeUpload = document.querySelector("#knowledge-upload");
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
const knowledgeDetailFlags = document.querySelector("#knowledge-detail-flags");
const knowledgeDetailBody = document.querySelector("#knowledge-detail-body");
const knowledgeDebugResults = document.querySelector("#knowledge-debug-results");
const knowledgeGapQueue = document.querySelector("#knowledge-gap-queue");
const toolKnowledgeSearch = document.querySelector("#tool-knowledge-search");
const toolSavePolicy = document.querySelector("#tool-save-policy");
const toolStatus = document.querySelector("#tool-status");
const auditRefresh = document.querySelector("#audit-refresh");
const auditTableBody = document.querySelector("#audit-table-body");
const knowledgeDetailPathPrefix = "/admin/knowledge/";
const knowledgeListPath = "/admin/knowledge";
const knowledgeHealthPath = "/admin/knowledge/health";
const knowledgeGapListPath = "/admin/knowledge/gaps";
const knowledgeGapUpdatePath = "/admin/knowledge/gaps/update";

let currentDraftId = "";
let activeVersionId = "";
let selectedKnowledgeSpaceId = "default";

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
  clearElement(knowledgeDetailFlags);
  if (qualityFlags.length === 0) {
    knowledgeDetailFlags.append(renderKnowledgeFlag("healthy"));
  } else {
    for (const flag of qualityFlags) {
      knowledgeDetailFlags.append(renderKnowledgeFlag(flag));
    }
  }
  knowledgeDetailBody.textContent = [
    `document: ${documentRecord.name || documentRecord.id || "unknown"}`,
    `status: ${documentRecord.status || "unknown"}`,
    `space_id: ${documentRecord.space_id || selectedKnowledgeSpaceId}`,
    `index_state: ${indexState}`,
    `last_error_code: ${lastErrorCode}`,
    "",
    chunks
  ].join("\n");
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
  summary.textContent = `${gap.status}: ${gap.question} (${gap.no_source_reason})`;
  row.append(summary);

  const actions = document.createElement("div");
  actions.className = "knowledge-gap-actions";

  if (gap.status !== "resolved") {
    const resolveButton = document.createElement("button");
    resolveButton.type = "button";
    resolveButton.textContent = "Resolve";
    resolveButton.addEventListener("click", async () => {
      await postJSON(knowledgeGapUpdatePath, {
        gap_id: gap.id,
        status: "resolved"
      });
      await refreshKnowledgeWorkspace();
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

async function refreshKnowledgeWorkspace() {
  await loadKnowledge();
  await loadKnowledgeHealth();
  await loadKnowledgeGaps();
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
  if (!response.ok) throw new Error(`${url} failed (${response.status})`);
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
  const response = await fetch(`${knowledgeListPath}?space_id=${encodeURIComponent(selectedKnowledgeSpaceId)}`);
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
    knowledgeDetailBody.textContent = "Chunk preview";
    clearElement(knowledgeDetailFlags);
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

function renderKnowledgeRow(documentRecord) {
  const row = document.createElement("tr");

  const nameCell = document.createElement("td");
  nameCell.textContent = documentRecord.name;

  const statusCell = document.createElement("td");
  statusCell.textContent = documentRecord.status;

  const chunkCountCell = document.createElement("td");
  chunkCountCell.textContent = String(documentRecord.chunk_count ?? documentRecord.chunks?.length ?? 0);

  const qualityCell = document.createElement("td");
  qualityCell.textContent = documentRecord.metadata?.vector_status || "pending";

  const actionCell = document.createElement("td");
  const inspectButton = document.createElement("button");
  inspectButton.type = "button";
  inspectButton.textContent = "Inspect";
  inspectButton.addEventListener("click", async () => {
    const detailURL = `${knowledgeDetailPathPrefix}${documentRecord.id}/detail`;
    const detail = await fetch(detailURL);
    if (!detail.ok) throw new Error(`${detailURL} failed (${detail.status})`);
    const loaded = await detail.json();
    renderKnowledgeDetail(loaded);
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
    cell.colSpan = 3;
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
    const agentCell = document.createElement("td");
    agentCell.textContent = record.agent_name;
    row.append(conversationCell, statusCell, agentCell);
    auditTableBody.append(row);
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
refreshKnowledgeWorkspace().catch(() => {});
auditRefresh?.addEventListener("click", () => {
  loadAudit().catch(() => {});
});
loadAudit().catch(() => {});
