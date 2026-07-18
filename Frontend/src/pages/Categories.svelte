<script lang="ts">
  import { onMount } from "svelte";
  import { expenseClient } from "../lib/api/clients";
  import { pushToast, describeError } from "../lib/stores/toast";
  import type { Category } from "../gen/expense/v1/expense_pb";

  let categories: Category[] = [];
  let loading = true;

  let newName = "";
  let newColor = "#a8721f";
  let creating = false;

  let editingId: number | null = null;
  let editName = "";
  let editColor = "";

  async function load() {
    loading = true;
    try {
      const res = await expenseClient.listCategories({});
      categories = res.categories;
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      loading = false;
    }
  }

  async function create() {
    if (!newName.trim()) {
      pushToast("Give the category a name.", "error");
      return;
    }
    creating = true;
    try {
      await expenseClient.createCategory({ name: newName.trim(), color: newColor });
      pushToast("Category added.", "success");
      newName = "";
      load();
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      creating = false;
    }
  }

  function startEdit(c: Category) {
    editingId = c.id;
    editName = c.name;
    editColor = c.color || "#a8721f";
  }

  async function saveEdit(c: Category) {
    if (!editName.trim()) {
      pushToast("Category name can't be empty.", "error");
      return;
    }
    try {
      await expenseClient.updateCategory({ id: c.id, name: editName.trim(), color: editColor });
      pushToast("Category updated.", "success");
      editingId = null;
      load();
    } catch (err) {
      pushToast(describeError(err), "error");
    }
  }

  async function remove(c: Category) {
    if (!confirm(`Delete "${c.name}"? Expenses in it will fall back to "Other".`)) return;
    try {
      await expenseClient.deleteCategory({ id: c.id });
      pushToast("Category deleted.", "success");
      load();
    } catch (err) {
      pushToast(describeError(err), "error");
    }
  }

  onMount(load);
</script>

<div class="page">
  <header class="page-header">
    <h1>Categories</h1>
    <p>Global categories are shared with every account; the rest are yours.</p>
  </header>

  <div class="card create-card">
    <div class="field grow">
      <label for="new-name">New category</label>
      <input id="new-name" type="text" bind:value={newName} placeholder="e.g. Subscriptions" />
    </div>
    <div class="field">
      <label for="new-color">Color</label>
      <input id="new-color" type="color" bind:value={newColor} class="swatch-input" />
    </div>
    <button class="btn btn-gold" on:click={create} disabled={creating}>Add</button>
  </div>

  <div class="card list-card">
    {#if loading}
      <p class="loading">Loading categories…</p>
    {:else if categories.length === 0}
      <div class="empty-state">
        <h3>No categories yet</h3>
        <p>Add your first one above.</p>
      </div>
    {:else}
      {#each categories as c (c.id)}
        {#if editingId === c.id}
          <div class="row edit-row">
            <input type="color" bind:value={editColor} class="swatch-input" />
            <input type="text" bind:value={editName} class="grow-input" />
            <button class="btn btn-gold" on:click={() => saveEdit(c)}>Save</button>
            <button class="btn btn-ghost" on:click={() => (editingId = null)}>Cancel</button>
          </div>
        {:else}
          <div class="row">
            <span class="swatch" style="background:{c.color || '#a8721f'}"></span>
            <span class="cat-name">{c.name}</span>
            {#if c.userId === 0}
              <span class="pill pill-green">Global</span>
            {/if}
            <div class="row-actions">
              <button class="icon-btn" title="Edit" on:click={() => startEdit(c)}>✎</button>
              <button class="icon-btn danger" title="Delete" on:click={() => remove(c)}>✕</button>
            </div>
          </div>
        {/if}
      {/each}
    {/if}
  </div>
</div>

<style>
  .page-header {
    margin-bottom: 20px;
  }
  .page-header p {
    margin-top: 4px;
    font-size: 14px;
  }

  .create-card {
    display: flex;
    align-items: flex-end;
    gap: 12px;
    margin-bottom: 20px;
    flex-wrap: wrap;
  }

  .field.grow {
    flex: 1;
    min-width: 180px;
  }

  .swatch-input {
    width: 46px;
    height: 40px;
    padding: 2px;
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-sm);
    background: var(--surface);
  }

  .list-card {
    padding: 8px 20px;
  }

  .loading {
    text-align: center;
    color: var(--muted);
    padding: 32px;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 0;
    border-bottom: 1px dashed var(--line-strong);
  }

  .row:last-child {
    border-bottom: none;
  }

  .swatch {
    width: 14px;
    height: 14px;
    border-radius: 4px;
    flex-shrink: 0;
  }

  .cat-name {
    flex: 1;
    font-size: 14px;
  }

  .row-actions {
    display: flex;
    gap: 4px;
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

  .edit-row .grow-input {
    flex: 1;
    padding: 8px 10px;
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-sm);
    font-size: 13px;
  }
</style>
