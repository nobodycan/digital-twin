const personaDraft = document.querySelector("#persona-draft");
const personaStatus = document.querySelector("#persona-status");
const saveDraftButton = document.querySelector("#persona-save-draft");
const publishButton = document.querySelector("#persona-publish");
const rollbackButton = document.querySelector("#persona-rollback");
const memoryTableBody = document.querySelector("#memory-table-body");
const knowledgeUpload = document.querySelector("#knowledge-upload");
const knowledgeUploadMock = document.querySelector("#knowledge-upload-mock");
const knowledgeQuery = document.querySelector("#knowledge-query");
const knowledgeQueryRun = document.querySelector("#knowledge-query-run");
const knowledgeStatus = document.querySelector("#knowledge-status");
const knowledgeTableBody = document.querySelector("#knowledge-table-body");
const knowledgeDetail = document.querySelector("#knowledge-detail");
const toolKnowledgeSearch = document.querySelector("#tool-knowledge-search");
const toolSavePolicy = document.querySelector("#tool-save-policy");
const toolStatus = document.querySelector("#tool-status");
const auditRefresh = document.querySelector("#audit-refresh");
const auditTableBody = document.querySelector("#audit-table-body");
const knowledgeDetailPathPrefix = "/admin/knowledge/";

let currentDraftId = "";
let activeVersionId = "";

function setPersonaStatus(text) {
  personaStatus.textContent = text;
}

function setKnowledgeStatus(text) {
  knowledgeStatus.textContent = text;
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
  const response = await fetch("/admin/knowledge");
  if (!response.ok) return;
  const documents = await response.json();
  knowledgeTableBody.textContent = "";
  if (documents.length === 0) {
    const row = document.createElement("tr");
    const cell = document.createElement("td");
    cell.colSpan = 4;
    cell.textContent = "No knowledge loaded";
    row.append(cell);
    knowledgeTableBody.append(row);
    knowledgeDetail.textContent = "Chunk preview";
    return;
  }
  for (const documentRecord of documents) {
    knowledgeTableBody.append(renderKnowledgeRow(documentRecord));
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

  const actionCell = document.createElement("td");
  const inspectButton = document.createElement("button");
  inspectButton.type = "button";
  inspectButton.textContent = "Inspect";
  inspectButton.addEventListener("click", async () => {
    const detailURL = `${knowledgeDetailPathPrefix}${documentRecord.id}`;
    const detail = await fetch(detailURL);
    if (!detail.ok) throw new Error(`${detailURL} failed (${detail.status})`);
    const loaded = await detail.json();
    knowledgeDetail.textContent = (loaded.chunks || []).map((chunk) => chunk.text).join("\n\n") || "Chunk preview";
  });

  const toggleButton = document.createElement("button");
  toggleButton.type = "button";
  toggleButton.textContent = documentRecord.status === "disabled" ? "Enable" : "Disable";
  toggleButton.addEventListener("click", async () => {
    const url = documentRecord.status === "disabled" ? "/admin/knowledge/enable" : "/admin/knowledge/disable";
    await postJSON(url, { document_id: documentRecord.id });
    await loadKnowledge();
  });

  const reindexButton = document.createElement("button");
  reindexButton.type = "button";
  reindexButton.textContent = "Reindex";
  reindexButton.addEventListener("click", async () => {
    await postJSON("/admin/knowledge/reindex", {
      document_id: documentRecord.id,
      content: (documentRecord.chunks || []).map((chunk) => chunk.text).join("\n\n")
    });
    await loadKnowledge();
  });

  const deleteButton = document.createElement("button");
  deleteButton.type = "button";
  deleteButton.textContent = "Delete";
  deleteButton.addEventListener("click", async () => {
    await postJSON("/admin/knowledge/delete", { document_id: documentRecord.id });
    await loadKnowledge();
  });

  actionCell.append(inspectButton, toggleButton, reindexButton, deleteButton);
  row.append(nameCell, statusCell, chunkCountCell, actionCell);
  return row;
}

knowledgeUploadMock?.addEventListener("click", async () => {
  try {
    const uploaded = await postJSON("/admin/knowledge/upload", {
      id: `kb-${Date.now()}`,
      name: "mock.md",
      content: "Phase 4 adds a digital human UI.\n\nIt includes persona, memory, and knowledge admin controls."
    });
    const citation = await postJSON("/admin/knowledge/citation-test", { query: "digital human UI" });
    setKnowledgeStatus(`Uploaded ${uploaded.chunk_count ?? uploaded.chunks.length} chunks; citation ${citation.chunk_id}`);
    await loadKnowledge();
  } catch (error) {
    setKnowledgeStatus(`Knowledge error: ${error.message}`);
  }
});

knowledgeQueryRun?.addEventListener("click", async () => {
  try {
    const citation = await postJSON("/admin/knowledge/citation-test", { query: knowledgeQuery?.value || "" });
    setKnowledgeStatus(`Citation match ${citation.chunk_id}`);
    knowledgeDetail.textContent = citation.text;
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
loadMemory().catch(() => {});
loadKnowledge().catch(() => {});
auditRefresh?.addEventListener("click", () => {
  loadAudit().catch(() => {});
});
loadAudit().catch(() => {});
