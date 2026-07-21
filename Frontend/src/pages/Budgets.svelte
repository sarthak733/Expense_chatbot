<script lang="ts">
  import { onMount } from "svelte";
  import { expenseClient } from "../lib/api/clients";
  import { pushToast, describeError } from "../lib/stores/toast";
  import { formatMoney, formatDate, currency } from "../lib/utils/format";
  import type { Budget, Category } from "../gen/expense/v1/expense_pb";

  let budgets: Budget[] = [];
  let categories: Category[] = [];
  let loading = true;

  let showForm = false;
  let creating = false;
  let form = {
    categoryId: 0,
    amount: "",
    period: "monthly",
    startDate: "",
    endDate: "",
  };

  let editingId: number | null = null;
  let editAmount = "";
  let editStart = "";
  let editEnd = "";

  async function load() {
    loading = true;
    try {
      const [budgetsRes, catRes] = await Promise.all([
        expenseClient.listBudgets({}),
        expenseClient.listCategories({}),
      ]);
      budgets = budgetsRes.budgets;
      categories = catRes.categories;
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      loading = false;
    }
  }

  function categoryName(id: number): string {
    if (!id) return "Overall";
    return categories.find((c) => c.id === id)?.name ?? "Unknown";
  }

  function statusPillClass(status: string): string {
    if (status === "exceeded") return "pill-rust";
    if (status === "warning") return "pill-gold";
    return "pill-green";
  }

  function barColor(status: string): string {
    if (status === "exceeded") return "var(--rust)";
    if (status === "warning") return "var(--gold)";
    return "var(--sage)";
  }

  async function createBudget() {
    const amount = parseFloat(form.amount);
    if (isNaN(amount) || amount <= 0 || !form.startDate || !form.endDate) {
      pushToast("Fill in a valid amount and date range.", "error");
      return;
    }
    creating = true;
    try {
      await expenseClient.createBudget({
        categoryId: form.categoryId,
        amount,
        period: form.period,
        startDate: form.startDate,
        endDate: form.endDate,
      });
      pushToast("Budget created.", "success");
      showForm = false;
      form = { categoryId: 0, amount: "", period: "monthly", startDate: "", endDate: "" };
      load();
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      creating = false;
    }
  }

  function startEdit(b: Budget) {
    editingId = b.id;
    editAmount = String(b.amount);
    editStart = b.startDate;
    editEnd = b.endDate;
  }

  async function saveEdit(b: Budget) {
    const amount = parseFloat(editAmount);
    if (isNaN(amount) || amount <= 0) {
      pushToast("Enter a valid amount.", "error");
      return;
    }
    try {
      await expenseClient.updateBudget({
        id: b.id,
        categoryId: b.categoryId,
        amount,
        startDate: editStart,
        endDate: editEnd,
      });
      pushToast("Budget updated.", "success");
      editingId = null;
      load();
    } catch (err) {
      pushToast(describeError(err), "error");
    }
  }

  async function remove(id: number) {
    if (!confirm("Delete this budget?")) return;
    try {
      await expenseClient.deleteBudget({ id });
      pushToast("Budget deleted.", "success");
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
      <h1>Budgets</h1>
      <p>Set a ceiling per category, or an overall one, for a date range.</p>
    </div>
    <button class="btn btn-gold" on:click={() => (showForm = !showForm)}>
      {showForm ? "Close" : "New budget"}
    </button>
  </header>

  {#if showForm}
    <div class="card form-card">
      <div class="form-grid">
        <div class="field">
          <label for="b-cat">Category</label>
          <select id="b-cat" bind:value={form.categoryId}>
            <option value={0}>Overall</option>
            {#each categories as c}
              <option value={c.id}>{c.name}</option>
            {/each}
          </select>
        </div>
        <div class="field">
          <label for="b-amount">Amount</label>
          <input id="b-amount" type="number" step="0.01" bind:value={form.amount} />
        </div>
        <div class="field">
          <label for="b-period">Period</label>
          <select id="b-period" bind:value={form.period}>
            <option value="weekly">Weekly</option>
            <option value="monthly">Monthly</option>
            <option value="yearly">Yearly</option>
          </select>
        </div>
        <div class="field">
          <label for="b-start">Start date</label>
          <input id="b-start" type="date" bind:value={form.startDate} />
        </div>
        <div class="field">
          <label for="b-end">End date</label>
          <input id="b-end" type="date" bind:value={form.endDate} />
        </div>
      </div>
      <button class="btn btn-primary" on:click={createBudget} disabled={creating}>
        {creating ? "Creating…" : "Create budget"}
      </button>
    </div>
  {/if}

  <div class="budget-grid">
    {#if loading}
      <p class="loading">Loading budgets…</p>
    {:else if budgets.length === 0}
      <div class="empty-state card">
        <h3>No budgets set</h3>
        <p>Create one to start tracking how close you are to a limit.</p>
      </div>
    {:else}
      {#each budgets as b (b.id)}
        <div class="card budget-card">
          <div class="budget-top">
            <div>
              <h3 class="budget-name">{categoryName(b.categoryId)}</h3>
              <span class="range mono">{formatDate(b.startDate)} – {formatDate(b.endDate)}</span>
            </div>
            <span class="pill {statusPillClass(b.status)}">{b.status || "on track"}</span>
          </div>

          {#if editingId === b.id}
            <div class="edit-row">
              <input type="number" step="0.01" bind:value={editAmount} />
              <input type="date" bind:value={editStart} />
              <input type="date" bind:value={editEnd} />
              <button class="btn btn-gold" on:click={() => saveEdit(b)}>Save</button>
              <button class="btn btn-ghost" on:click={() => (editingId = null)}>Cancel</button>
            </div>
          {:else}
            <div class="bar-track">
              <div
                class="bar-fill"
                style="width:{Math.min(b.percentage, 100)}%; background:{barColor(b.status)}"
              ></div>
            </div>
            <div class="budget-figures">
              <span class="spent mono">{formatMoney(b.spent, $currency)} spent</span>
              <span class="of mono">of {formatMoney(b.amount, $currency)}</span>
              <span class="remaining mono">{formatMoney(b.remaining, $currency)} left</span>
            </div>
            <div class="budget-actions">
              <button class="btn btn-ghost" on:click={() => startEdit(b)}>Edit</button>
              <button class="btn btn-danger" on:click={() => remove(b.id)}>Delete</button>
            </div>
          {/if}
        </div>
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

  .budget-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 16px;
  }

  .loading {
    text-align: center;
    color: var(--muted);
    padding: 32px;
    grid-column: 1 / -1;
  }

  .budget-card {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .budget-top {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 8px;
  }

  .budget-name {
    font-size: 16px;
  }

  .range {
    font-size: 11px;
    color: var(--muted);
  }

  .bar-track {
    height: 8px;
    border-radius: 999px;
    background: var(--surface-sunk);
    overflow: hidden;
  }

  .bar-fill {
    height: 100%;
    border-radius: 999px;
    transition: width 0.3s ease;
  }

  .budget-figures {
    display: flex;
    justify-content: space-between;
    font-size: 12px;
    color: var(--ink-soft);
  }

  .budget-actions {
    display: flex;
    gap: 8px;
    margin-top: 4px;
  }

  .budget-actions .btn {
    flex: 1;
    font-size: 13px;
    padding: 8px;
  }

  .edit-row {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .edit-row input {
    padding: 8px 10px;
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-sm);
    font-size: 13px;
    flex: 1;
    min-width: 90px;
  }
</style>
