<script lang="ts">
  import { onMount, tick } from "svelte";
  import { expenseClient } from "../lib/api/clients";
  import { pushToast, describeError } from "../lib/stores/toast";
  import { formatDateTime } from "../lib/utils/format";
  import type { ChatMessage } from "../gen/expense/v1/expense_pb";

  let messages: ChatMessage[] = [];
  let input = "";
  let sending = false;
  let loadingHistory = true;
  let hasMore = false;
  let offset = 0;
  const PAGE = 30;
  let scrollEl: HTMLDivElement;

  let isListening = false;
  let recognition: any = null;

  async function loadHistory(initial = false) {
    try {
      const res = await expenseClient.getChatHistory({ limit: PAGE, offset });
      const page = [...res.messages].reverse();
      messages = initial ? page : [...page, ...messages];
      offset += res.messages.length;
      hasMore = offset < res.totalCount;
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      loadingHistory = false;
    }
  }

  let baseInput = "";

  onMount(async () => {
    // Initialize Web Speech API
    const SpeechRecognition = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
    if (SpeechRecognition) {
      recognition = new SpeechRecognition();
      recognition.continuous = false;
      recognition.interimResults = true;
      recognition.lang = "en-IN"; // English with Indian support

      recognition.onstart = () => {
        isListening = true;
      };

      recognition.onresult = (event: any) => {
        let finalTranscript = "";
        let interimTranscript = "";

        for (let i = event.resultIndex; i < event.results.length; ++i) {
          if (event.results[i].isFinal) {
            finalTranscript += event.results[i][0].transcript;
          } else {
            interimTranscript += event.results[i][0].transcript;
          }
        }

        if (finalTranscript || interimTranscript) {
          input = baseInput + (finalTranscript + interimTranscript);
        }
      };

      recognition.onerror = (event: any) => {
        console.error("Speech recognition error:", event.error);
        if (event.error !== "no-speech") {
          pushToast("Voice input error: " + event.error, "error");
        }
      };

      recognition.onend = () => {
        isListening = false;
      };
    }

    await loadHistory(true);
    await tick();
    scrollToBottom();
  });

  function toggleListening() {
    if (!recognition) {
      pushToast("Voice input is not supported in this browser. Please use Chrome, Safari, or Edge.", "warning");
      return;
    }

    if (isListening) {
      recognition.stop();
    } else {
      baseInput = input ? input.trim() + " " : "";
      try {
        recognition.start();
      } catch (err) {
        console.error("Failed to start speech recognition:", err);
      }
    }
  }

  function scrollToBottom() {
    if (scrollEl) scrollEl.scrollTop = scrollEl.scrollHeight;
  }

  async function send() {
    const text = input.trim();
    if (!text || sending) return;
    input = "";
    sending = true;

    // optimistic placeholder for the user's own message
    const tempId = -Date.now();
    messages = [
      ...messages,
      {
        id: tempId,
        userId: 0,
        role: "user",
        messageText: text,
        createdAt: new Date().toISOString(),
      } as ChatMessage,
    ];
    await tick();
    scrollToBottom();

    try {
      const res = await expenseClient.sendChatMessage({ messageText: text });
      messages = messages.filter((m) => m.id !== tempId);
      if (res.userMessage) messages = [...messages, res.userMessage];
      if (res.botResponse) messages = [...messages, res.botResponse];
      await tick();
      scrollToBottom();
    } catch (err) {
      pushToast(describeError(err), "error");
      messages = messages.filter((m) => m.id !== tempId);
      input = text;
    } finally {
      sending = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  }
</script>

<div class="chat-page">
  <header class="chat-header">
    <h1>Chat</h1>
    <p>Tell it what you spent — "coffee 120" or "grocery run 2400 yesterday" both work.</p>
  </header>

  <div class="chat-scroll card" bind:this={scrollEl}>
    {#if hasMore}
      <button class="btn btn-ghost load-more" on:click={() => loadHistory(false)}>
        Load earlier messages
      </button>
    {/if}

    {#if loadingHistory}
      <p class="loading">Loading conversation…</p>
    {:else if messages.length === 0}
      <div class="empty-state">
        <h3>Nothing logged yet</h3>
        <p>Send your first message below to add an expense.</p>
      </div>
    {/if}

    {#each messages as m (m.id)}
      <div class="bubble-row" class:user={m.role === "user"}>
        <div class="bubble" class:user={m.role === "user"}>
          <p class="bubble-text">{m.messageText}</p>
          <span class="bubble-time mono">{formatDateTime(m.createdAt)}</span>
        </div>
      </div>
    {/each}

    {#if sending}
      <div class="bubble-row">
        <div class="bubble typing">
          <span class="dot"></span><span class="dot"></span><span class="dot"></span>
        </div>
      </div>
    {/if}
  </div>

  <div class="composer">
    <textarea
      rows="1"
      placeholder={isListening ? "Listening..." : "Log an expense or ask a question…"}
      bind:value={input}
      on:keydown={handleKeydown}
      disabled={isListening}
    ></textarea>
    <button
      class="btn btn-ghost mic-btn"
      class:listening={isListening}
      on:click={toggleListening}
      title={isListening ? "Stop listening" : "Start voice input"}
      type="button"
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
        <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3zm5.3-3c0 3-2.54 5.1-5.3 5.1S6.7 14 6.7 11H5c0 3.41 2.72 6.23 6 6.72V21h2v-3.28c3.28-.49 6-3.31 6-6.72h-1.7z"/>
      </svg>
    </button>
    <button class="btn btn-gold" on:click={send} disabled={sending || !input.trim() || isListening}>
      Send
    </button>
  </div>
</div>

<style>
  .chat-page {
    display: flex;
    flex-direction: column;
    height: calc(100vh - 64px);
  }

  .chat-header {
    margin-bottom: 16px;
  }

  .chat-header p {
    margin-top: 4px;
    font-size: 14px;
  }

  .chat-scroll {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 24px;
  }

  .load-more {
    align-self: center;
    font-size: 13px;
    padding: 6px 14px;
    margin-bottom: 8px;
  }

  .loading {
    text-align: center;
    color: var(--muted);
    padding: 24px;
  }

  .bubble-row {
    display: flex;
    justify-content: flex-start;
  }

  .bubble-row.user {
    justify-content: flex-end;
  }

  .bubble {
    max-width: 70%;
    padding: 12px 16px;
    border-radius: 12px;
    background: var(--surface-sunk);
    border: 1px solid var(--line);
  }

  .bubble.user {
    background: var(--ink);
    color: var(--bg);
    border-color: var(--ink);
  }

  .bubble.user .bubble-time {
    color: rgba(250, 249, 245, 0.55);
  }

  .bubble-text {
    color: inherit;
    font-size: 14px;
    line-height: 1.5;
    white-space: pre-wrap;
  }

  .bubble-time {
    display: block;
    margin-top: 6px;
    font-size: 11px;
    color: var(--muted);
  }

  .typing {
    display: flex;
    gap: 4px;
    padding: 14px 16px;
  }

  .typing .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--muted);
    animation: bounce 1.2s infinite ease-in-out;
  }

  .typing .dot:nth-child(2) {
    animation-delay: 0.15s;
  }
  .typing .dot:nth-child(3) {
    animation-delay: 0.3s;
  }

  @keyframes bounce {
    0%,
    60%,
    100% {
      transform: translateY(0);
      opacity: 0.5;
    }
    30% {
      transform: translateY(-4px);
      opacity: 1;
    }
  }

  .composer {
    display: flex;
    gap: 10px;
    margin-top: 16px;
    align-items: flex-end;
  }

  .composer textarea {
    flex: 1;
    resize: none;
    padding: 12px 14px;
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-sm);
    background: var(--surface);
    font-family: var(--font-body);
    font-size: 14px;
    max-height: 120px;
  }

  .composer textarea:focus {
    border-color: var(--gold);
    outline: none;
  }

  .composer .btn {
    padding: 12px 22px;
    flex-shrink: 0;
  }

  .composer .mic-btn {
    padding: 12px;
    height: 43px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: var(--radius-sm);
    color: var(--muted);
    transition: background 0.2s, color 0.2s, border-color 0.2s;
  }

  .composer .mic-btn:hover {
    color: var(--ink);
    background: var(--surface-sunk);
  }

  .composer .mic-btn.listening {
    color: #ffffff;
    background: var(--rust);
    border-color: var(--rust);
    animation: mic-pulse 1.5s infinite;
  }

  @keyframes mic-pulse {
    0% {
      box-shadow: 0 0 0 0 rgba(168, 67, 44, 0.5);
    }
    70% {
      box-shadow: 0 0 0 8px rgba(168, 67, 44, 0);
    }
    100% {
      box-shadow: 0 0 0 0 rgba(168, 67, 44, 0);
    }
  }
</style>
