<script lang="ts">
  import { onMount } from "svelte";
  import { expenseClient } from "../lib/api/clients";
  import { pushToast, describeError } from "../lib/stores/toast";
  import { formatMoney, currency } from "../lib/utils/format";
  import type { GetMonthlyInsightsResponse } from "../gen/expense/v1/expense_pb";

  let data: GetMonthlyInsightsResponse | null = null;
  let loading = true;

  async function load() {
    loading = true;
    try {
      data = await expenseClient.getMonthlyInsights({});
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      loading = false;
    }
  }

  function trendClass(pct: number): string {
    if (pct > 0) return "trend-up";
    if (pct < 0) return "trend-down";
    return "trend-flat";
  }

  function trendSign(pct: number): string {
    return pct > 0 ? "+" : "";
  }

  onMount(load);
</script>

<div class="page">
  <header class="page-header">
    <h1>Insights</h1>
    <p>How this month stacks up against last month.</p>
  </header>

  {#if loading}
    <p class="loading">Crunching the numbers…</p>
  {:else if data}
    <div class="totals-grid">
      <div class="card total-card">
        <span class="month-label">{data.currentMonth}</span>
        <span class="total-amount mono">{formatMoney(data.currentTotal, $currency)}</span>
      </div>
      <div class="card total-card muted-card">
        <span class="month-label">{data.prevMonth}</span>
        <span class="total-amount mono">{formatMoney(data.prevTotal, $currency)}</span>
      </div>
      <div class="card total-card trend-card {trendClass(data.totalDiffPercentage)}">
        <span class="month-label">Change</span>
        <span class="total-amount mono">
          {trendSign(data.totalDiffPercentage)}{data.totalDiffPercentage.toFixed(1)}%
        </span>
      </div>
    </div>

    {#if data.categoryInsights.length > 0}
      <div class="card section-card">
        <h2>By category</h2>
        <div class="cat-list">
          {#each data.categoryInsights as ci}
            <div class="receipt-row cat-insight-row">
              <div class="label-group">
                <span class="label">{ci.category}</span>
                <span class="mono small {trendClass(ci.diffPercentage)}">
                  {trendSign(ci.diffPercentage)}{ci.diffPercentage.toFixed(1)}%
                </span>
              </div>
              <span class="leader"></span>
              <span class="value">
                {formatMoney(ci.currentSpent, $currency)}
                <span class="prev-value mono">vs {formatMoney(ci.prevSpent, $currency)}</span>
              </span>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    {#if data.generalInsights.length > 0}
      <div class="card section-card">
        <h2>Notes</h2>
        <ul class="notes">
          {#each data.generalInsights as note}
            <li>{note}</li>
          {/each}
        </ul>
      </div>
    {/if}

    {#if data.categoryInsights.length === 0 && data.generalInsights.length === 0}
      <div class="empty-state card">
        <h3>Not enough data yet</h3>
        <p>Log a few more expenses this month and last to see a comparison.</p>
      </div>
    {/if}
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

  .totals-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 16px;
    margin-bottom: 20px;
  }

  .total-card {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .muted-card {
    background: var(--surface-sunk);
  }

  .month-label {
    font-size: 12px;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    font-weight: 600;
  }

  .total-amount {
    font-size: 24px;
    font-weight: 600;
  }

  .trend-card.trend-up .total-amount {
    color: var(--rust);
  }
  .trend-card.trend-down .total-amount {
    color: var(--sage);
  }
  .trend-card.trend-flat .total-amount {
    color: var(--ink);
  }

  .section-card {
    margin-bottom: 20px;
  }

  .section-card h2 {
    font-size: 16px;
    margin-bottom: 12px;
  }

  .cat-insight-row {
    gap: 12px;
  }

  .label-group {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .small {
    font-size: 12px;
  }

  .trend-up {
    color: var(--rust);
  }
  .trend-down {
    color: var(--sage);
  }
  .trend-flat {
    color: var(--muted);
  }

  .prev-value {
    margin-left: 8px;
    font-size: 12px;
    color: var(--muted);
  }

  .notes {
    margin: 0;
    padding-left: 20px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .notes li {
    font-size: 14px;
    color: var(--ink-soft);
    line-height: 1.5;
  }

  @media (max-width: 640px) {
    .totals-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
