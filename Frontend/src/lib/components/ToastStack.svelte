<script lang="ts">
  import { toasts } from "../stores/toast";
</script>

<div class="toast-stack">
  {#each $toasts as t (t.id)}
    <div class="toast toast-{t.kind}">
      <span class="dot"></span>
      <span>{t.text}</span>
    </div>
  {/each}
</div>

<style>
  .toast-stack {
    position: fixed;
    bottom: 20px;
    right: 20px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    z-index: 1000;
    max-width: 320px;
  }

  .toast {
    display: flex;
    align-items: center;
    gap: 10px;
    background: var(--ink);
    color: var(--bg);
    padding: 12px 16px;
    border-radius: var(--radius-sm);
    font-size: 13px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.18);
    animation: rise 0.18s ease;
  }

  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    flex-shrink: 0;
    background: var(--gold);
  }

  .toast-error .dot {
    background: #e08a72;
  }

  .toast-success .dot {
    background: #7fbf9a;
  }

  @keyframes rise {
    from {
      opacity: 0;
      transform: translateY(6px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
