<script lang="ts">
  import { expenseClient } from "../lib/api/clients";
  import { pushToast, describeError } from "../lib/stores/toast";

  let startDate = "";
  let endDate = "";
  let exportingCsv = false;
  let exportingPdf = false;

  function download(bytes: Uint8Array, filename: string, mime: string) {
    const blob = new Blob([bytes.slice().buffer], { type: mime });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  }

  async function exportCsv() {
    exportingCsv = true;
    try {
      const res = await expenseClient.exportExpensesCSV({ startDate, endDate });
      download(res.csvData, "expenses.csv", "text/csv");
      pushToast("CSV downloaded.", "success");
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      exportingCsv = false;
    }
  }

  async function exportPdf() {
    exportingPdf = true;
    try {
      const res = await expenseClient.exportExpensesPDF({ startDate, endDate });
      download(res.pdfData, "expenses.pdf", "application/pdf");
      pushToast("PDF downloaded.", "success");
    } catch (err) {
      pushToast(describeError(err), "error");
    } finally {
      exportingPdf = false;
    }
  }
</script>

<div class="page">
  <header class="page-header">
    <h1>Export</h1>
    <p>Download your expenses for a date range, or leave both blank for everything.</p>
  </header>

  <div class="card export-card">
    <div class="filters">
      <div class="field">
        <label for="e-start">From</label>
        <input id="e-start" type="date" bind:value={startDate} />
      </div>
      <div class="field">
        <label for="e-end">To</label>
        <input id="e-end" type="date" bind:value={endDate} />
      </div>
    </div>

    <div class="export-actions">
      <button class="btn btn-gold" on:click={exportCsv} disabled={exportingCsv}>
        {exportingCsv ? "Preparing…" : "Download CSV"}
      </button>
      <button class="btn btn-primary" on:click={exportPdf} disabled={exportingPdf}>
        {exportingPdf ? "Preparing…" : "Download PDF"}
      </button>
    </div>
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

  .export-card {
    display: flex;
    flex-direction: column;
    gap: 24px;
    max-width: 460px;
  }

  .filters {
    display: flex;
    gap: 16px;
  }

  .filters .field {
    flex: 1;
  }

  .export-actions {
    display: flex;
    gap: 12px;
  }

  .export-actions .btn {
    flex: 1;
    padding: 12px;
  }
</style>
