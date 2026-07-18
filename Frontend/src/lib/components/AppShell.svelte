<script lang="ts">
  import Sidebar from "./Sidebar.svelte";
</script>

<div class="shell">
  <Sidebar />
  <main class="content">
    <slot />
  </main>
</div>

<style>
  .shell {
    display: flex;
    min-height: 100vh;
  }

  .content {
    flex: 1;
    min-width: 0;
    padding: 32px 40px;
    max-width: 980px;
  }

  @media (max-width: 720px) {
    .shell {
      flex-direction: column;
    }
    .content {
      padding: 20px;
    }
  }
</style>
