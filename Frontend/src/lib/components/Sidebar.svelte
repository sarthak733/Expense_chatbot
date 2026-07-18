<script lang="ts">
  import { link, router } from "svelte-spa-router";
  import { auth, clearAuth } from "../stores/auth";
  import { authClient } from "../api/clients";
  import { pushToast } from "../stores/toast";

  const links = [
    { href: "/chat", label: "Chat", icon: "◆" },
    { href: "/expenses", label: "Expenses", icon: "≣" },
    { href: "/categories", label: "Categories", icon: "◐" },
    { href: "/budgets", label: "Budgets", icon: "▤" },
    { href: "/recurring", label: "Recurring", icon: "↻" },
    { href: "/insights", label: "Insights", icon: "△" },
    { href: "/export", label: "Export", icon: "↓" },
    { href: "/settings", label: "Settings", icon: "⚙" },
  ];

  async function handleLogout() {
    try {
      await authClient.logout({});
    } catch {
      // even if the server call fails, clear local state so the user
      // isn't stuck signed in on a dead session
    }
    clearAuth();
    pushToast("Signed out.", "info");
  }
</script>

<aside class="sidebar">
  <div class="brand">
    <span class="mark">§</span>
    <span class="name">Ledger</span>
  </div>

  <nav>
    {#each links as l}
      <a
        href={l.href}
        use:link
        class="nav-link"
        class:active={router.location === l.href}
      >
        <span class="icon">{l.icon}</span>
        {l.label}
      </a>
    {/each}
  </nav>

  <div class="sidebar-footer">
    {#if $auth.user}
      <div class="user-chip">
        <span class="avatar">{$auth.user.username.slice(0, 1).toUpperCase()}</span>
        <span class="uname">{$auth.user.username}</span>
      </div>
    {/if}
    <button class="btn btn-ghost logout-btn" on:click={handleLogout}>Sign out</button>
  </div>
</aside>

<style>
  .sidebar {
    width: var(--sidebar-w);
    flex-shrink: 0;
    height: 100vh;
    position: sticky;
    top: 0;
    background: var(--surface);
    border-right: 1px solid var(--line);
    display: flex;
    flex-direction: column;
    padding: 24px 16px;
  }

  .brand {
    display: flex;
    align-items: baseline;
    gap: 8px;
    padding: 0 8px 24px;
    margin-bottom: 8px;
    border-bottom: 1px dashed var(--line-strong);
  }

  .mark {
    font-family: var(--font-display);
    font-size: 26px;
    color: var(--gold);
  }

  .name {
    font-family: var(--font-display);
    font-size: 20px;
    font-weight: 600;
  }

  nav {
    display: flex;
    flex-direction: column;
    gap: 2px;
    flex: 1;
  }

  .nav-link {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 12px;
    border-radius: var(--radius-sm);
    text-decoration: none;
    color: var(--ink-soft);
    font-size: 14px;
    font-weight: 500;
    transition: background 0.12s ease, color 0.12s ease;
  }

  .icon {
    width: 18px;
    text-align: center;
    color: var(--muted);
    font-size: 14px;
  }

  .nav-link:hover {
    background: var(--surface-sunk);
    color: var(--ink);
  }

  .nav-link.active {
    background: var(--gold-soft);
    color: var(--gold);
    font-weight: 600;
  }

  .nav-link.active .icon {
    color: var(--gold);
  }

  .sidebar-footer {
    border-top: 1px dashed var(--line-strong);
    padding-top: 16px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .user-chip {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 0 8px;
  }

  .avatar {
    width: 28px;
    height: 28px;
    border-radius: 50%;
    background: var(--ink);
    color: var(--bg);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 13px;
    font-weight: 600;
    flex-shrink: 0;
  }

  .uname {
    font-size: 13px;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .logout-btn {
    width: 100%;
    font-size: 13px;
    padding: 8px 12px;
  }
</style>
