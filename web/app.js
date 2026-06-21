const form = document.querySelector("#message-form");
const input = document.querySelector("#message-input");
const transcript = document.querySelector("#transcript");
const avatarFace = document.querySelector("#avatar-face");
const subtitleLine = document.querySelector("#subtitle-line");
const mockVoice = document.querySelector("#mock-voice-button");
const audioStatus = document.querySelector("#audio-status");

function appendLine(role, text) {
  const line = document.createElement("p");
  line.className = `transcript-line transcript-line-${role}`;
  line.textContent = `${role}: ${text}`;
  transcript.append(line);
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

function conversationPayload(text) {
  const now = new Date().toISOString();
  return {
    id: "web-session",
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
  setAvatar("thinking", "Preparing response...");
  const response = await fetch("/experience/stream", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(conversationPayload(text))
  });
  if (!response.ok || !response.body) {
    appendStatus(`error: stream failed (${response.status})`);
    setAvatar("error", "The stream could not be opened.");
    return;
  }
  await readPresentationStream(response);
}

async function sendMockVoice(audioText) {
  appendStatus("Mock voice: sending local transcript");
  setAvatar("listening", "Mock voice input captured");
  const response = await fetch("/experience/mock-voice/stream", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ audio_text: audioText })
  });
  if (!response.ok || !response.body) {
    appendStatus(`error: mock voice failed (${response.status})`);
    setAvatar("error", "Mock voice stream failed.");
    return;
  }
  await readPresentationStream(response);
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
      appendLine("assistant", payload.text || "");
      break;
    case "asr_final":
      appendLine("voice", payload.text || "");
      break;
    case "subtitle":
      renderSubtitle(payload.subtitles || []);
      break;
    case "avatar_state":
      setAvatar(payload.state || "idle", subtitleLine.textContent);
      break;
    case "audio_chunk":
      audioStatus.textContent = `${payload.provider || "mock"} audio ready`;
      break;
    case "error":
      appendStatus(`error: ${payload.problem || "unknown"}; ${payload.fix || "retry"}`);
      setAvatar("error", payload.problem || "error");
      break;
    case "done":
      appendStatus("done");
      break;
    default:
      break;
  }
}

function renderSubtitle(subtitles) {
  if (subtitles.length === 0) return;
  subtitleLine.textContent = subtitles.map((segment) => segment.text).join(" ");
}

form?.addEventListener("submit", (event) => {
  event.preventDefault();
  const text = input.value.trim();
  if (!text) return;
  input.value = "";
  sendMessage(text).catch((error) => {
    appendStatus(`error: ${error.message}`);
    setAvatar("error", "Request failed.");
  });
});

mockVoice?.addEventListener("click", () => {
  sendMockVoice("Mock voice input").catch((error) => {
    appendStatus(`error: ${error.message}`);
    setAvatar("error", "Mock voice failed.");
  });
});
