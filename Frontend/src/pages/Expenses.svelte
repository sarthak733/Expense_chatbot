<script lang="ts">
  import { onMount } from "svelte";
  import { expenseClient } from "../lib/api/clients";
  import { pushToast, describeError } from "../lib/stores/toast";
  import { formatMoney, formatDate, currency } from "../lib/utils/format";
  import type { Expense, Category } from "../gen/expense/v1/expense_pb";

  let expenses: Expense[] = [];
  let categories: Category[] = [];
  let totalCount = 0;
  let loading = true;

  let startDate = "";
  let endDate = "";
  const LIMIT = 20;
  let offset = 0;

  let editingId: number | null = null;
  let editTitle = "";
  let editAmount = "";
  let editCategoryId = 0;

  async function loadCategories() {
    try {
      const res = await expenseClient.listCategories({});
      categories = res.categories;
    } catch (err) {
      pushToast(describeError(err), "error");
    }
  }

  async function loadExpenses() {
    loading = true;
    try {
      const res = await expenseClient.listExpenses({
        limit: LIMIT,
        offset,
        startDate,
        endDate,
      });
      expenses = res.expenses;
      totalCount = res.totalCount;
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      loading = false;
    }
  }

  function applyFilters() {
    offset = 0;
    loadExpenses();
  }

  function clearFilters() {
    startDate = "";
    endDate = "";
    offset = 0;
    loadExpenses();
  }

  function nextPage() {
    if (offset + LIMIT < totalCount) {
      offset += LIMIT;
      loadExpenses();
    }
  }

  function prevPage() {
    if (offset > 0) {
      offset = Math.max(0, offset - LIMIT);
      loadExpenses();
    }
  }

  function startEdit(exp: Expense) {
    editingId = exp.id;
    editTitle = exp.title;
    editAmount = String(exp.amount);
    editCategoryId = exp.categoryId;
  }

  function cancelEdit() {
    editingId = null;
  }

  async function saveEdit(exp: Expense) {
    const amount = parseFloat(editAmount);
    if (!editTitle.trim() || isNaN(amount)) {
      pushToast("Enter a title and a valid amount.", "error");
      return;
    }
    try {
      const cat = categories.find((c) => c.id === editCategoryId);
      await expenseClient.updateExpense({
        id: exp.id,
        title: editTitle.trim(),
        amount,
        categoryId: editCategoryId,
        category: cat?.name ?? "",
      });
      pushToast("Expense updated.", "success");
      editingId = null;
      loadExpenses();
    } catch (err) {
      pushToast(describeError(err), "error");
    }
  }

  async function remove(id: number) {
    if (!confirm("Delete this expense? This can't be undone.")) return;
    try {
      await expenseClient.deleteExpense({ id });
      pushToast("Expense deleted.", "success");
      loadExpenses();
    } catch (err) {
      pushToast(describeError(err), "error");
    }
  }

  onMount(() => {
    loadCategories();
    loadExpenses();
  });
</script>

<div class="page">
  <header class="page-header">
    <h1>Expenses</h1>
    <p>Every line you've logged, newest first.</p>
  </header>

  <div class="filters card">
    <div class="field">
      <label for="start">From</label>
      <input id="start" type="date" bind:value={startDate} />
    </div>
    <div class="field">
      <label for="end">To</label>
      <input id="end" type="date" bind:value={endDate} />
    </div>
    <button class="btn btn-primary" on:click={applyFilters}>Filter</button>
    <button class="btn btn-ghost" on:click={clearFilters}>Clear</button>
  </div>

  <div class="card list-card">
    {#if loading}
      <p class="loading">Loading expenses…</p>
    {:else if expenses.length === 0}
      <div class="empty-state">
        <h3>No expenses in this range</h3>
        <p>Log one from the Chat page, or adjust your filters.</p>
      </div>
    {:else}
      {#each expenses as exp (exp.id)}
        {#if editingId === exp.id}
          <div class="edit-row">
            <input type="text" bind:value={editTitle} placeholder="Title" />
            <input type="number" step="0.01" bind:value={editAmount} placeholder="Amount" />
            <select bind:value={editCategoryId}>
              <option value={0}>Other</option>
              {#each categories as c}
                <option value={c.id}>{c.name}</option>
              {/each}
            </select>
            <button class="btn btn-gold" on:click={() => saveEdit(exp)}>Save</button>
            <button class="btn btn-ghost" on:click={cancelEdit}>Cancel</button>
          </div>
        {:else}
          <div class="receipt-row expense-row">
            <div class="label-group">
              <span class="label">{exp.title}</span>
              <span class="cat-pill pill pill-gold">{exp.category || "Other"}</span>
              <span class="date mono">{formatDate(exp.createdAt)}</span>
            </div>
            <span class="leader"></span>
            <span class="value">{formatMoney(exp.amount, $currency)}</span>
            <div class="row-actions">
              <button class="icon-btn" title="Edit" on:click={() => startEdit(exp)}>✎</button>
              <button class="icon-btn danger" title="Delete" on:click={() => remove(exp.id)}>✕</button>
            </div>
          </div>
        {/if}
      {/each}
    {/if}
  </div>

  {#if totalCount > LIMIT}
    <div class="pagination">
      <button class="btn btn-ghost" on:click={prevPage} disabled={offset === 0}>← Newer</button>
      <span class="page-info mono">
        {offset + 1}–{Math.min(offset + LIMIT, totalCount)} of {totalCount}
      </span>
      <button class="btn btn-ghost" on:click={nextPage} disabled={offset + LIMIT >= totalCount}>
        Older →
      </button>
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

  .filters {
    display: flex;
    align-items: flex-end;
    gap: 12px;
    margin-bottom: 20px;
    flex-wrap: wrap;
  }

  .filters .field {
    min-width: 150px;
  }

  .list-card {
    padding: 8px 20px;
  }

  .loading {
    text-align: center;
    color: var(--muted);
    padding: 32px;
  }

  .expense-row {
    gap: 12px;
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
    gap: 4px;
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
    gap: 8px;
    align-items: center;
    padding: 10px 0;
    flex-wrap: wrap;
  }

  .edit-row input,
  .edit-row select {
    padding: 8px 10px;
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-sm);
    font-size: 13px;
  }

  .edit-row input[type="text"] {
    flex: 1;
    min-width: 120px;
  }

  .edit-row input[type="number"] {
    width: 100px;
  }

  .pagination {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 20px;
    margin-top: 20px;
  }

  .page-info {
    font-size: 13px;
    color: var(--muted);
  }
</style>
