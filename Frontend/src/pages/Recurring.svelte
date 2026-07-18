<script lang="ts">
  import { onMount } from "svelte";
  import { expenseClient } from "../lib/api/clients";
  import { pushToast, describeError } from "../lib/stores/toast";
  import { formatMoney, formatDate, currency } from "../lib/utils/format";
  import type { RecurringExpense, Category } from "../gen/expense/v1/expense_pb";

  let items: RecurringExpense[] = [];
  let categories: Category[] = [];
  let loading = true;

  let showForm = false;
  let creating = false;
  let form = { title: "", amount: "", categoryId: 0, interval: "monthly", nextRunDate: "" };

  let editingId: number | null = null;
  let editTitle = "";
  let editAmount = "";
  let editInterval = "";
  let editNextRun = "";

  async function load() {
    loading = true;
    try {
      const [recRes, catRes] = await Promise.all([
        expenseClient.listRecurringExpenses({}),
        expenseClient.listCategories({}),
      ]);
      items = recRes.recurringExpenses;
      categories = catRes.categories;
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      loading = false;
    }
  }

  async function create() {
    const amount = parseFloat(form.amount);
    if (!form.title.trim() || isNaN(amount) || !form.nextRunDate) {
      pushToast("Fill in a title, amount, and next run date.", "error");
      return;
    }
    creating = true;
    try {
      await expenseClient.createRecurringExpense({
        title: form.title.trim(),
        amount,
        categoryId: form.categoryId,
        interval: form.interval,
        nextRunDate: form.nextRunDate,
      });
      pushToast("Recurring expense added.", "success");
      showForm = false;
      form = { title: "", amount: "", categoryId: 0, interval: "monthly", nextRunDate: "" };
      load();
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      creating = false;
    }
  }

  function startEdit(r: RecurringExpense) {
    editingId = r.id;
    editTitle = r.title;
    editAmount = String(r.amount);
    editInterval = r.interval;
    editNextRun = r.nextRunDate;
  }

  async function saveEdit(r: RecurringExpense) {
    const amount = parseFloat(editAmount);
    if (!editTitle.trim() || isNaN(amount)) {
      pushToast("Enter a valid title and amount.", "error");
      return;
    }
    try {
      await expenseClient.updateRecurringExpense({
        id: r.id,
        title: editTitle.trim(),
        amount,
        categoryId: r.categoryId,
        interval: editInterval,
        nextRunDate: editNextRun,
        isActive: r.isActive,
      });
      pushToast("Updated.", "success");
      editingId = null;
      load();
    } catch (err) {
      pushToast(describeError(err), "error");
    }
  }

  async function toggleActive(r: RecurringExpense) {
    try {
      await expenseClient.updateRecurringExpense({
        id: r.id,
        title: r.title,
        amount: r.amount,
        categoryId: r.categoryId,
        interval: r.interval,
        nextRunDate: r.nextRunDate,
        isActive: !r.isActive,
      });
      load();
    } catch (err) {
      pushToast(describeError(err), "error");
    }
  }

  async function remove(id: number) {
    if (!confirm("Delete this recurring expense?")) return;
    try {
      await expenseClient.deleteRecurringExpense({ id });
      pushToast("Deleted.", "success");
      load();
    } catch (err) {
      pushToast(describeError(err), "error");
    }
  }

  onMount(load);
</script>

<div class="page">
  <header class="page-header row-between">
    <div>
      <h1>Recurring</h1>
      <p>Expenses that repeat on their own — rent, subscriptions, and the like.</p>
    </div>
    <button class="btn btn-gold" on:click={() => (showForm = !showForm)}>
      {showForm ? "Close" : "New recurring"}
    </button>
  </header>

  {#if showForm}
    <div class="card form-card">
      <div class="form-grid">
        <div class="field">
          <label for="r-title">Title</label>
          <input id="r-title" type="text" bind:value={form.title} placeholder="e.g. Netflix" />
        </div>
        <div class="field">
          <label for="r-amount">Amount</label>
          <input id="r-amount" type="number" step="0.01" bind:value={form.amount} />
        </div>
        <div class="field">
          <label for="r-cat">Category</label>
          <select id="r-cat" bind:value={form.categoryId}>
            <option value={0}>Other</option>
            {#each categories as c}
              <option value={c.id}>{c.name}</option>
            {/each}
          </select>
        </div>
        <div class="field">
          <label for="r-interval">Repeats</label>
          <select id="r-interval" bind:value={form.interval}>
            <option value="daily">Daily</option>
            <option value="weekly">Weekly</option>
            <option value="monthly">Monthly</option>
            <option value="yearly">Yearly</option>
          </select>
        </div>
        <div class="field">
          <label for="r-next">Next run</label>
          <input id="r-next" type="date" bind:value={form.nextRunDate} />
        </div>
      </div>
      <button class="btn btn-primary" on:click={create} disabled={creating}>
        {creating ? "Adding…" : "Add recurring expense"}
      </button>
    </div>
  {/if}

  <div class="card list-card">
    {#if loading}
      <p class="loading">Loading…</p>
    {:else if items.length === 0}
      <div class="empty-state">
        <h3>Nothing recurring yet</h3>
        <p>Add rent, a subscription, or anything that repeats.</p>
      </div>
    {:else}
      {#each items as r (r.id)}
        {#if editingId === r.id}
          <div class="edit-row">
            <input type="text" bind:value={editTitle} />
            <input type="number" step="0.01" bind:value={editAmount} />
            <select bind:value={editInterval}>
              <option value="daily">Daily</option>
              <option value="weekly">Weekly</option>
              <option value="monthly">Monthly</option>
              <option value="yearly">Yearly</option>
            </select>
            <input type="date" bind:value={editNextRun} />
            <button class="btn btn-gold" on:click={() => saveEdit(r)}>Save</button>
            <button class="btn btn-ghost" on:click={() => (editingId = null)}>Cancel</button>
          </div>
        {:else}
          <div class="receipt-row rec-row" class:inactive={!r.isActive}>
            <div class="label-group">
              <span class="label">{r.title}</span>
              <span class="pill pill-gold">{r.interval}</span>
              <span class="date mono">next {formatDate(r.nextRunDate)}</span>
            </div>
            <span class="leader"></span>
            <span class="value">{formatMoney(r.amount, $currency)}</span>
            <div class="row-actions">
              <label class="switch">
                <input type="checkbox" checked={r.isActive} on:change={() => toggleActive(r)} />
                <span class="slider"></span>
              </label>
              <button class="icon-btn" title="Edit" on:click={() => startEdit(r)}>✎</button>
              <button class="icon-btn danger" title="Delete" on:click={() => remove(r.id)}>✕</button>
            </div>
          </div>
        {/if}
      {/each}
    {/if}
  </div>
</div>

<style>
  .row-between {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 20px;
  }
  .page-header p {
    margin-top: 4px;
    font-size: 14px;
  }

  .form-card {
    margin-bottom: 20px;
  }

  .form-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
    gap: 12px;
    margin-bottom: 16px;
  }

  .list-card {
    padding: 8px 20px;
  }

  .loading {
    text-align: center;
    color: var(--muted);
    padding: 32px;
  }

  .rec-row {
    gap: 12px;
  }

  .rec-row.inactive {
    opacity: 0.5;
  }

  .label-group {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-shrink: 0;
  }

  .date {
    font-size: 12px;
    color: var(--muted);
  }

  .row-actions {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
  }

  .icon-btn {
    background: transparent;
    border: none;
    color: var(--muted);
    font-size: 14px;
    padding: 4px 8px;
    border-radius: var(--radius-sm);
  }

  .icon-btn:hover {
    background: var(--surface-sunk);
    color: var(--ink);
  }

  .icon-btn.danger:hover {
    color: var(--rust);
    background: var(--rust-soft);
  }

  .edit-row {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    padding: 10px 0;
  }

  .edit-row input,
  .edit-row select {
    padding: 8px 10px;
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-sm);
    font-size: 13px;
  }

  .switch {
    position: relative;
    display: inline-block;
    width: 32px;
    height: 18px;
  }

  .switch input {
    opacity: 0;
    width: 0;
    height: 0;
  }

  .slider {
    position: absolute;
    cursor: pointer;
    inset: 0;
    background: var(--line-strong);
    border-radius: 999px;
    transition: 0.15s;
  }

  .slider::before {
    content: "";
    position: absolute;
    width: 14px;
    height: 14px;
    left: 2px;
    top: 2px;
    background: white;
    border-radius: 50%;
    transition: 0.15s;
  }

  .switch input:checked + .slider {
    background: var(--sage);
  }

  .switch input:checked + .slider::before {
    transform: translateX(14px);
  }
</style>
