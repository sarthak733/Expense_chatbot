<script lang="ts">
  import { onMount } from "svelte";
  import { push } from "svelte-spa-router";
  import { userClient } from "../lib/api/clients";
  import { pushToast, describeError } from "../lib/stores/toast";
  import { clearAuth } from "../lib/stores/auth";
  import { currency as currencyStore } from "../lib/utils/format";
  import { applyTheme } from "../lib/stores/theme";
  import type { UserProfile } from "../gen/expense/v1/user_pb";

  let profile: UserProfile | null = null;
  let loading = true;
  let savingProfile = false;

  let email = "";
  let firstName = "";
  let lastName = "";
  let currency = "INR";
  let theme = "system";
  let weeklyStart = "monday";
  let monthlyStartDay = 1;
  let budgetAlertThreshold = 80;

  let oldPassword = "";
  let newPassword = "";
  let changingPassword = false;

  let deleteConfirmText = "";
  let deleting = false;

  async function load() {
    loading = true;
    try {
      const res = await userClient.getProfile({});
      profile = res.profile ?? null;
      if (profile) {
        email = profile.email;
        firstName = profile.firstName;
        lastName = profile.lastName;
        currency = profile.currency || "INR";
        theme = profile.theme || "system";
        weeklyStart = profile.weeklyStart || "monday";
        monthlyStartDay = profile.monthlyStartDay || 1;
        budgetAlertThreshold = profile.budgetAlertThreshold || 80;
        currencyStore.set(currency);
        if (theme === "light" || theme === "dark" || theme === "system") {
          applyTheme(theme);
        }
      }
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      loading = false;
    }
  }

  async function saveProfile() {
    if (email.trim() && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) {
      pushToast("That doesn't look like a valid email address.", "error");
      return;
    }
    savingProfile = true;
    try {
      const res = await userClient.updateProfile({
        email,
        firstName,
        lastName,
        currency,
        theme,
        weeklyStart,
        monthlyStartDay,
        budgetAlertThreshold,
      });
      profile = res.profile ?? profile;
      currencyStore.set(currency);
      if (theme === "light" || theme === "dark" || theme === "system") {
        applyTheme(theme);
      }
      pushToast(res.message || "Profile updated.", "success");
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      savingProfile = false;
    }
  }

  async function changePassword() {
    if (!oldPassword || !newPassword) {
      pushToast("Fill in both password fields.", "error");
      return;
    }
    changingPassword = true;
    try {
      const res = await userClient.updatePassword({ oldPassword, newPassword });
      pushToast(res.message || "Password updated.", "success");
      oldPassword = "";
      newPassword = "";
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      changingPassword = false;
    }
  }

  async function deleteAccount() {
    if (deleteConfirmText !== profile?.username) {
      pushToast("Type your username exactly to confirm.", "error");
      return;
    }
    deleting = true;
    try {
      await userClient.deleteProfile({});
      clearAuth();
      pushToast("Account deleted.", "info");
      push("/login");
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      deleting = false;
    }
  }

  onMount(load);
</script>

<div class="page">
  <header class="page-header">
    <h1>Settings</h1>
    <p>Your account, preferences, and how the numbers get shown.</p>
  </header>

  {#if loading}
    <p class="loading">Loading your profile…</p>
  {:else if profile}
    <div class="card section-card">
      <h2>Profile</h2>
      <div class="form-grid">
        <div class="field">
          <label for="s-username">Username</label>
          <input id="s-username" type="text" value={profile.username} disabled />
        </div>
        <div class="field">
          <label for="s-email">Email</label>
          <input id="s-email" type="email" bind:value={email} />
        </div>
        <div class="field">
          <label for="s-first">First name</label>
          <input id="s-first" type="text" bind:value={firstName} />
        </div>
        <div class="field">
          <label for="s-last">Last name</label>
          <input id="s-last" type="text" bind:value={lastName} />
        </div>
      </div>
    </div>

    <div class="card section-card">
      <h2>Preferences</h2>
      <div class="form-grid">
        <div class="field">
          <label for="s-currency">Currency</label>
          <select id="s-currency" bind:value={currency}>
            <option value="INR">INR (₹)</option>
            <option value="USD">USD ($)</option>
            <option value="EUR">EUR (€)</option>
            <option value="GBP">GBP (£)</option>
          </select>
        </div>
        <div class="field">
          <label for="s-theme">Theme</label>
          <select id="s-theme" bind:value={theme}>
            <option value="system">System</option>
            <option value="light">Light</option>
            <option value="dark">Dark</option>
          </select>
        </div>
        <div class="field">
          <label for="s-week">Week starts on</label>
          <select id="s-week" bind:value={weeklyStart}>
            <option value="monday">Monday</option>
            <option value="sunday">Sunday</option>
          </select>
        </div>
        <div class="field">
          <label for="s-month-start">Month starts on day</label>
          <input id="s-month-start" type="number" min="1" max="28" bind:value={monthlyStartDay} />
        </div>
        <div class="field">
          <label for="s-alert">Budget alert at (%)</label>
          <input id="s-alert" type="number" min="1" max="100" bind:value={budgetAlertThreshold} />
        </div>
      </div>
      <button class="btn btn-gold" on:click={saveProfile} disabled={savingProfile}>
        {savingProfile ? "Saving…" : "Save changes"}
      </button>
    </div>

    <div class="card section-card">
      <h2>Change password</h2>
      <div class="form-grid two-col">
        <div class="field">
          <label for="s-old-pw">Current password</label>
          <input id="s-old-pw" type="password" bind:value={oldPassword} />
        </div>
        <div class="field">
          <label for="s-new-pw">New password</label>
          <input id="s-new-pw" type="password" bind:value={newPassword} />
        </div>
      </div>
      <button class="btn btn-primary" on:click={changePassword} disabled={changingPassword}>
        {changingPassword ? "Updating…" : "Update password"}
      </button>
    </div>

    <div class="card section-card danger-card">
      <h2>Danger zone</h2>
      <p class="danger-copy">
        Deleting your account removes your expenses, categories, and chat history for good.
        Type your username <strong>{profile.username}</strong> to confirm.
      </p>
      <div class="danger-row">
        <input type="text" bind:value={deleteConfirmText} placeholder={profile.username} />
        <button class="btn btn-danger" on:click={deleteAccount} disabled={deleting}>
          {deleting ? "Deleting…" : "Delete account"}
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
  .page-header {
    margin-bottom: 20px;
  }
  .page-header p {
    margin-top: 4px;
    font-size: 14px;
  }

  .loading {
    text-align: center;
    color: var(--muted);
    padding: 32px;
  }

  .section-card {
    margin-bottom: 20px;
  }

  .section-card h2 {
    font-size: 16px;
    margin-bottom: 16px;
  }

  .form-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
    gap: 14px;
    margin-bottom: 18px;
  }

  .form-grid.two-col {
    grid-template-columns: repeat(2, 1fr);
    max-width: 500px;
  }

  .field input:disabled {
    background: var(--surface-sunk);
    color: var(--muted);
  }

  .danger-card {
    border-color: var(--rust-soft);
  }

  .danger-copy {
    font-size: 13px;
    margin-bottom: 14px;
    line-height: 1.5;
  }

  .danger-row {
    display: flex;
    gap: 10px;
  }

  .danger-row input {
    flex: 1;
    padding: 10px 12px;
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-sm);
    font-size: 14px;
  }
</style>
