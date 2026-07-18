<script lang="ts">
  import { push } from "svelte-spa-router";
  import { authClient } from "../lib/api/clients";
  import { setAuth } from "../lib/stores/auth";
  import { pushToast, describeError } from "../lib/stores/toast";

  let username = "";
  let password = "";
  let loading = false;

  async function handleSubmit(e: Event) {
    e.preventDefault();
    if (!username || !password) {
      pushToast("Enter a username and password.", "error");
      return;
    }
    loading = true;
    try {
      const res = await authClient.login({ username, password });
      if (res.user) {
        setAuth(res.token, {
          id: res.user.id,
          username: res.user.username,
          createdAt: res.user.createdAt,
        });
      }
      pushToast(res.message || "Welcome back.", "success");
      push("/chat");
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      loading = false;
    }
  }
</script>

<div class="auth-page">
  <div class="auth-card card">
    <div class="brand">
      <span class="mark">§</span>
      <h1>Ledger</h1>
    </div>
    <p class="tagline">Sign in to keep your books.</p>

    <form on:submit={handleSubmit}>
      <div class="field">
        <label for="username">Username</label>
        <input id="username" type="text" bind:value={username} autocomplete="username" />
      </div>
      <div class="field">
        <label for="password">Password</label>
        <input id="password" type="password" bind:value={password} autocomplete="current-password" />
      </div>
      <button class="btn btn-gold submit-btn" type="submit" disabled={loading}>
        {loading ? "Signing in…" : "Sign in"}
      </button>
    </form>

    <p class="switch">
      New here? <a href="/#/register">Create an account</a>
    </p>
  </div>
</div>

<style>
  .auth-page {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg);
    padding: 20px;
  }

  .auth-card {
    width: 100%;
    max-width: 360px;
    padding: 36px 32px;
  }

  .brand {
    display: flex;
    align-items: baseline;
    gap: 8px;
    margin-bottom: 4px;
  }

  .mark {
    font-family: var(--font-display);
    font-size: 28px;
    color: var(--gold);
  }

  .tagline {
    margin-bottom: 28px;
    font-size: 14px;
  }

  form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .submit-btn {
    width: 100%;
    padding: 12px;
    margin-top: 6px;
  }

  .switch {
    margin-top: 20px;
    font-size: 13px;
    text-align: center;
  }

  .switch a {
    color: var(--gold);
    font-weight: 600;
    text-decoration: none;
  }

  .switch a:hover {
    text-decoration: underline;
  }
</style>
