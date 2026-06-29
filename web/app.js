const form = document.querySelector("#message-form");
const input = document.querySelector("#message-input");
const transcript = document.querySelector("#transcript");
const avatarFace = document.querySelector("#avatar-face");
const subtitleLine = document.querySelector("#subtitle-line");
const mockVoice = document.querySelector("#mock-voice-button");
const audioStatus = document.querySelector("#audio-status");
const stopButton = document.querySelector("#stop-button");
const runtimeStatus = document.querySelector("#runtime-status");
const statusChip = document.querySelector("#status-chip");
const sessionProvider = document.querySelector("#session-provider");
const sessionModel = document.querySelector("#session-model");
const sessionMode = document.querySelector("#session-mode");
const sessionFallbackPolicy = document.querySelector("#session-fallback-policy");
const providerStrip = document.querySelector("#provider-strip");

const conversationId = "web-session";
const providerStatus = {
  environment: "local",
  provider: "local",
  model: "deterministic",
  fallback_policy: "fallback_to_local",
  generation_mode_hint: "local",
  base_url: ""
};

let activeRequestController = null;
let activeAssistantLine = null;
let latestAssistantText = "";

function transcriptLineClass(role, extraClass) {
  const classes = [`transcript-line`, `transcript-line-${role}`];
  if (extraClass) {
    classes.push(extraClass);
  }
  return classes.join(" ");
}

function appendLine(role, text, extraClass) {
  const line = document.createElement("article");
  line.className = transcriptLineClass(role, extraClass);

  const head = document.createElement("div");
  head.className = "transcript-head";

  const roleLabel = document.createElement("span");
  roleLabel.className = "transcript-role";
  roleLabel.textContent = role;

  const meta = document.createElement("div");
  head.append(roleLabel, meta);

  const content = document.createElement("div");
  content.className = "transcript-content";
  content.textContent = text;

  line.append(head, content);
  line.dataset.role = role;
  transcript.append(line);
  return line;
}

function ensureActiveAssistantLine() {
  if (!activeAssistantLine) {
    activeAssistantLine = appendLine("assistant", "", "transcript-line-pending");
  }
  return activeAssistantLine;
}

function setLineBadge(line, text, tone) {
  if (!line) {
    return null;
  }
  const head = line.querySelector(".transcript-head div");
  let badge = head.querySelector(".transcript-badge");
  if (!badge) {
    badge = document.createElement("span");
    badge.className = "transcript-badge";
    head.append(badge);
  }
  badge.textContent = text;
  badge.className = "transcript-badge";
  if (tone) {
    badge.classList.add(`transcript-badge-${tone}`);
  }
  return badge;
}

function fallbackBadge(line) {
  return setLineBadge(line, "fallback", "fallback");
}

function ensureTranscriptMeta(line) {
  if (!line) {
    return null;
  }
  let meta = line.querySelector(".transcript-meta");
  if (!meta) {
    meta = document.createElement("div");
    meta.className = "transcript-meta";
    line.append(meta);
  }
  return meta;
}

function clearTranscriptMeta(line) {
  const meta = ensureTranscriptMeta(line);
  if (!meta) {
    return null;
  }
  meta.textContent = "";
  return meta;
}

function renderCitationSummary(line, citations) {
  if (!line || !Array.isArray(citations) || citations.length === 0) {
    return;
  }
  const meta = ensureTranscriptMeta(line);
  for (const citation of citations) {
    const chip = document.createElement("span");
    chip.className = "transcript-citation";
    chip.textContent = `${citation.document_name || citation.document_id} #${citation.rank || 1}`;
    meta.append(chip);
  }
}

function renderGroundingState(line, metadata) {
  if (!line || !metadata) {
    return;
  }
  const meta = clearTranscriptMeta(line);
  const state = document.createElement("span");
  state.className = "transcript-citation";
  if (metadata.knowledge_used) {
    state.textContent = "Knowledge grounded";
    meta.append(state);
    renderCitationSummary(line, metadata.knowledge_citations);
  } else {
    state.textContent = "No source used";
    meta.append(state);
  }
  if (metadata.memory_used) {
    const memory = document.createElement("span");
    memory.className = "transcript-citation";
    memory.textContent = "Memory considered";
    meta.append(memory);
  }
}

function updateActiveAssistantLine(text, finalized) {
  const line = ensureActiveAssistantLine();
  latestAssistantText = text;
  const content = line.querySelector(".transcript-content");
  content.textContent = text;
  if (finalized) {
    line.classList.remove("transcript-line-pending");
    activeAssistantLine = null;
  }
}

function markActiveAssistantLineNotSaved(reason) {
  if (!activeAssistantLine) {
    latestAssistantText = "";
    return;
  }
  const content = activeAssistantLine.querySelector(".transcript-content");
  content.textContent = `${content.textContent} (${reason}; not-saved)`;
  activeAssistantLine.classList.add("transcript-line-status", "transcript-line-error");
  setLineBadge(activeAssistantLine, "not saved", "not-saved");
  activeAssistantLine = null;
  latestAssistantText = "";
}

function setAvatar(state, subtitle) {
  avatarFace.dataset.state = state;
  avatarFace.textContent = state;
  subtitleLine.textContent = subtitle || "";
}

function appendStatus(text) {
  const line = appendLine("status", text, "transcript-line-status");
  line.dataset.kind = "status";
}

function setRequestActive(active) {
  activeRequestController = active ? activeRequestController : null;
  if (input) input.disabled = active;
  if (mockVoice) mockVoice.disabled = active;
  if (stopButton) stopButton.disabled = !active;
}

function conversationPayload(text) {
  const now = new Date().toISOString();
  return {
    id: conversationId,
    tenant_id: "tenant-1",
    user_id: "user-1",
    messages: [
      {
        id: `msg-${Date.now()}`,
        role: "user",
        content: text,
        created_at: now
      }
    ],
    created_at: now,
    updated_at: now
  };
}

function setProviderStatus(status) {
  providerStatus.environment = status.environment || providerStatus.environment;
  providerStatus.provider = status.provider || providerStatus.provider;
  providerStatus.model = status.model || providerStatus.model;
  providerStatus.fallback_policy = status.fallback_policy || providerStatus.fallback_policy;
  providerStatus.generation_mode_hint = status.generation_mode_hint || providerStatus.generation_mode_hint;
  providerStatus.base_url = status.base_url || "";

  sessionProvider.textContent = providerStatus.provider;
  sessionModel.textContent = providerStatus.model || "deterministic";
  sessionMode.textContent = providerStatus.generation_mode_hint;
  sessionFallbackPolicy.textContent = providerStatus.fallback_policy;
  providerStrip.title = providerStatus.base_url || `${providerStatus.provider} session`;
  if (runtimeStatus) {
    runtimeStatus.dataset.environment = providerStatus.environment;
  }
}

async function fetchRuntimeStatus() {
  try {
    const response = await fetch("/runtime/status");
    if (!response.ok) {
      appendStatus(`runtime status unavailable (${response.status})`);
      return;
    }
    const status = await response.json();
    setProviderStatus(status);
  } catch (error) {
    appendStatus(`runtime status unavailable: ${error.message}`);
  }
}

function setStatusChip(text, tone) {
  statusChip.textContent = text;
  statusChip.dataset.tone = tone;
}

async function sendMessage(text) {
  appendLine("user", text);
  latestAssistantText = "";
  setStatusChip("thinking", "thinking");
  setAvatar("thinking", "Preparing response...");
  activeRequestController = new AbortController();
  setRequestActive(true);
  try {
    const response = await fetch("/experience/stream", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(conversationPayload(text)),
      signal: activeRequestController.signal
    });
    if (!response.ok || !response.body) {
      appendStatus(`error: stream failed (${response.status})`);
      setStatusChip("error", "error");
      setAvatar("error", "The stream could not be opened.");
      return;
    }
    await readPresentationStream(response);
  } catch (error) {
    if (error.name === "AbortError") {
      markActiveAssistantLineNotSaved("canceled");
      appendStatus("canceled");
      setStatusChip("interrupted", "interrupted");
      setAvatar("interrupted", "Request canceled.");
      return;
    }
    appendStatus(`error: ${error.message}`);
    setStatusChip("error", "error");
    setAvatar("error", "Request failed.");
  } finally {
    setRequestActive(false);
  }
}

async function sendMockVoice(audioText) {
  appendStatus("Mock voice: sending local transcript");
  latestAssistantText = "";
  setStatusChip("listening", "ready");
  setAvatar("listening", "Mock voice input captured");
  activeRequestController = new AbortController();
  setRequestActive(true);
  try {
    const response = await fetch("/experience/mock-voice/stream", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ audio_text: audioText }),
      signal: activeRequestController.signal
    });
    if (!response.ok || !response.body) {
      appendStatus(`error: mock voice failed (${response.status})`);
      setStatusChip("error", "error");
      setAvatar("error", "Mock voice stream failed.");
      return;
    }
    await readPresentationStream(response);
  } catch (error) {
    if (error.name === "AbortError") {
      markActiveAssistantLineNotSaved("canceled");
      appendStatus("canceled");
      setStatusChip("interrupted", "interrupted");
      setAvatar("interrupted", "Mock voice canceled.");
      return;
    }
    appendStatus(`error: ${error.message}`);
    setStatusChip("error", "error");
    setAvatar("error", "Mock voice failed.");
  } finally {
    setRequestActive(false);
  }
}

async function readPresentationStream(response) {
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const parsed = parseSSEFrames(buffer);
    buffer = parsed.remainder;
    for (const frame of parsed.frames) {
      renderPresentationEvent(frame.event, frame.data);
    }
  }
}

function parseSSEFrames(buffer) {
  const frames = [];
  const parts = buffer.split("\n\n");
  const remainder = parts.pop() || "";
  for (const part of parts) {
    let event = "message";
    const data = [];
    for (const line of part.split("\n")) {
      if (line.startsWith("event: ")) event = line.slice(7);
      if (line.startsWith("data: ")) data.push(line.slice(6));
    }
    frames.push({ event, data: data.join("\n") });
  }
  return { frames, remainder };
}

function finalizeAssistantLine(line, metadata) {
  if (!line) {
    return;
  }
  if (metadata.generation_mode === "fallback") {
    fallbackBadge(line);
  }
  renderGroundingState(line, metadata);
  if (metadata.generation_mode === "fallback" || metadata.generation_mode === "transparency") {
    setStatusChip(metadata.generation_mode, metadata.generation_mode);
  } else {
    setStatusChip("ready", "ready");
  }
}

function renderPresentationEvent(eventName, rawData) {
  let event;
  try {
    event = JSON.parse(rawData);
  } catch {
    appendStatus(`error: malformed ${eventName} event`);
    return;
  }
  const payload = event.payload || {};
  const metadata = event.metadata || {};
  switch (eventName) {
    case "assistant_text_delta":
      setStatusChip("speaking", "ready");
      updateActiveAssistantLine(latestAssistantText + (payload.text || ""), false);
      break;
    case "asr_final":
      appendLine("voice", payload.text || "");
      break;
    case "subtitle": {
      const subtitleText = renderSubtitle(payload.subtitles || []);
      if (subtitleText && !activeAssistantLine) {
        updateActiveAssistantLine(subtitleText, false);
      }
      break;
    }
    case "avatar_state":
      setAvatar(payload.state || "idle", subtitleLine.textContent);
      break;
    case "audio_chunk":
      audioStatus.textContent = `${payload.provider || "mock"} audio ready`;
      break;
    case "error":
      markActiveAssistantLineNotSaved(payload.problem || "failed");
      appendStatus(`error: ${payload.problem || "unknown"}; ${payload.fix || "retry"}`);
      setStatusChip("error", "error");
      setAvatar("error", payload.problem || "error");
      break;
    case "done": {
      const finalAssistantText = latestAssistantText || subtitleLine.textContent.trim();
      const completedLine = activeAssistantLine;
      if (activeAssistantLine) {
        updateActiveAssistantLine(finalAssistantText || activeAssistantLine.querySelector(".transcript-content").textContent, true);
      } else if (finalAssistantText) {
        const line = appendLine("assistant", finalAssistantText);
        if (metadata.generation_mode === "fallback") {
          fallbackBadge(line);
        }
        renderGroundingState(line, metadata);
      }
      finalizeAssistantLine(completedLine, metadata);
      if (metadata.llm_provider) {
        setProviderStatus({
          provider: metadata.llm_provider,
          model: metadata.llm_model,
          generation_mode_hint: metadata.generation_mode || providerStatus.generation_mode_hint,
          fallback_policy: providerStatus.fallback_policy
        });
      }
      if (metadata.generation_mode === "fallback") {
        appendStatus(`fallback: ${metadata.fallback_category || "provider issue"}; local response shown`);
        setAvatar("fallback", "Local fallback reply displayed.");
      } else if (payload.status === "completed") {
        appendStatus("done");
        setAvatar("idle", subtitleLine.textContent);
        setStatusChip("ready", "ready");
      }
      break;
    }
    default:
      break;
  }
}

function renderSubtitle(subtitles) {
  if (subtitles.length === 0) return "";
  const text = subtitles.map((segment) => segment.text).join(" ");
  subtitleLine.textContent = text;
  latestAssistantText = text;
  return text;
}

form?.addEventListener("submit", (event) => {
  event.preventDefault();
  const text = input.value.trim();
  if (!text || activeRequestController) return;
  input.value = "";
  sendMessage(text).catch((error) => {
    appendStatus(`error: ${error.message}`);
    setStatusChip("error", "error");
    setAvatar("error", "Request failed.");
    setRequestActive(false);
  });
});

mockVoice?.addEventListener("click", () => {
  if (activeRequestController) return;
  sendMockVoice("Mock voice input").catch((error) => {
    appendStatus(`error: ${error.message}`);
    setStatusChip("error", "error");
    setAvatar("error", "Mock voice failed.");
    setRequestActive(false);
  });
});

stopButton?.addEventListener("click", () => {
  if (!activeRequestController) return;
  activeRequestController.abort();
});

setRequestActive(false);
setProviderStatus(providerStatus);
setStatusChip("ready", "ready");
fetchRuntimeStatus().catch(() => {});
