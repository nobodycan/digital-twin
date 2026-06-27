const form = document.querySelector("#message-form");
const input = document.querySelector("#message-input");
const transcript = document.querySelector("#transcript");
const avatarFace = document.querySelector("#avatar-face");
const subtitleLine = document.querySelector("#subtitle-line");
const mockVoice = document.querySelector("#mock-voice-button");
const audioStatus = document.querySelector("#audio-status");
const stopButton = document.querySelector("#stop-button");

const conversationId = "web-session";
let activeRequestController = null;
let activeAssistantLine = null;
let latestAssistantText = "";

function appendLine(role, text, extraClass) {
  const line = document.createElement("p");
  line.className = `transcript-line transcript-line-${role}`;
  if (extraClass) {
    line.classList.add(extraClass);
  }
  line.textContent = `${role}: ${text}`;
  transcript.append(line);
  return line;
}

function ensureActiveAssistantLine() {
  if (!activeAssistantLine) {
    activeAssistantLine = appendLine("assistant", "", "transcript-line-pending");
  }
  return activeAssistantLine;
}

function updateActiveAssistantLine(text, finalized) {
  const line = ensureActiveAssistantLine();
  latestAssistantText = text;
  line.textContent = `assistant: ${text}`;
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
  activeAssistantLine.textContent = `${activeAssistantLine.textContent} (${reason}; not saved)`;
  activeAssistantLine.classList.add("transcript-line-status");
  activeAssistantLine = null;
  latestAssistantText = "";
}

function setAvatar(state, subtitle) {
  avatarFace.dataset.state = state;
  avatarFace.textContent = state;
  subtitleLine.textContent = subtitle || "";
}

function appendStatus(text) {
  const line = document.createElement("p");
  line.className = "transcript-line transcript-line-status";
  line.textContent = text;
  transcript.append(line);
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

async function sendMessage(text) {
  appendLine("user", text);
  latestAssistantText = "";
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
      setAvatar("error", "The stream could not be opened.");
      return;
    }
    await readPresentationStream(response);
  } catch (error) {
    if (error.name === "AbortError") {
      markActiveAssistantLineNotSaved("canceled");
      appendStatus("canceled");
      setAvatar("interrupted", "Request canceled.");
      return;
    }
    appendStatus(`error: ${error.message}`);
    setAvatar("error", "Request failed.");
  } finally {
    setRequestActive(false);
  }
}

async function sendMockVoice(audioText) {
  appendStatus("Mock voice: sending local transcript");
  latestAssistantText = "";
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
      setAvatar("error", "Mock voice stream failed.");
      return;
    }
    await readPresentationStream(response);
  } catch (error) {
    if (error.name === "AbortError") {
      markActiveAssistantLineNotSaved("canceled");
      appendStatus("canceled");
      setAvatar("interrupted", "Mock voice canceled.");
      return;
    }
    appendStatus(`error: ${error.message}`);
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

function renderPresentationEvent(eventName, rawData) {
  let event;
  try {
    event = JSON.parse(rawData);
  } catch {
    appendStatus(`error: malformed ${eventName} event`);
    return;
  }
  const payload = event.payload || {};
  switch (eventName) {
    case "assistant_text_delta":
      updateActiveAssistantLine((payload.text || "").trimEnd(), false);
      break;
    case "asr_final":
      appendLine("voice", payload.text || "");
      break;
    case "subtitle":
      const subtitleText = renderSubtitle(payload.subtitles || []);
      if (subtitleText && !activeAssistantLine) {
        updateActiveAssistantLine(subtitleText, false);
      }
      break;
    case "avatar_state":
      setAvatar(payload.state || "idle", subtitleLine.textContent);
      break;
    case "audio_chunk":
      audioStatus.textContent = `${payload.provider || "mock"} audio ready`;
      break;
    case "error":
      markActiveAssistantLineNotSaved(payload.problem || "failed");
      appendStatus(`error: ${payload.problem || "unknown"}; ${payload.fix || "retry"}`);
      setAvatar("error", payload.problem || "error");
      break;
    case "done":
      const finalAssistantText = latestAssistantText || subtitleLine.textContent.trim();
      if (activeAssistantLine) {
        updateActiveAssistantLine(finalAssistantText || activeAssistantLine.textContent.replace(/^assistant:\s*/, ""), true);
      } else if (finalAssistantText) {
        appendLine("assistant", finalAssistantText);
      }
      appendStatus("done");
      break;
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
    setAvatar("error", "Request failed.");
    setRequestActive(false);
  });
});

mockVoice?.addEventListener("click", () => {
  if (activeRequestController) return;
  sendMockVoice("Mock voice input").catch((error) => {
    appendStatus(`error: ${error.message}`);
    setAvatar("error", "Mock voice failed.");
    setRequestActive(false);
  });
});

stopButton?.addEventListener("click", () => {
  if (!activeRequestController) return;
  activeRequestController.abort();
});

setRequestActive(false);
