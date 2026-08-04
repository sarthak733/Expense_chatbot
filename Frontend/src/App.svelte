<script lang="ts">
  import Router, { router, push } from "svelte-spa-router";
  import { wrap } from "svelte-spa-router/wrap";
  import { onMount } from "svelte";
  import { auth } from "./lib/stores/auth";
  import { userClient } from "./lib/api/clients";
  import { currency } from "./lib/utils/format";
  import { applyTheme } from "./lib/stores/theme";
  import AppShell from "./lib/components/AppShell.svelte";
  import ToastStack from "./lib/components/ToastStack.svelte";

  import Login from "./pages/Login.svelte";
  import Register from "./pages/Register.svelte";
  import Chat from "./pages/Chat.svelte";
  import Expenses from "./pages/Expenses.svelte";
  import Categories from "./pages/Categories.svelte";
  import Budgets from "./pages/Budgets.svelte";
  import Recurring from "./pages/Recurring.svelte";
  import Insights from "./pages/Insights.svelte";
  import ExportPage from "./pages/Export.svelte";
  import Settings from "./pages/Settings.svelte";

  // Route guard: bounce to /login if there's no token, and away from
  // /login /register if there already is one.
  function requireAuth() {
    let authed = false;
    auth.subscribe((s) => (authed = s.token !== null))();
    if (!authed) {
      push("/login");
      return false;
    }
    return true;
  }

  function requireGuest() {
    let authed = false;
    auth.subscribe((s) => (authed = s.token !== null))();
    if (authed) {
      push("/chat");
      return false;
    }
    return true;
  }

  const routes = {
    "/login": wrap({ component: Login, conditions: [requireGuest] }),
    "/register": wrap({ component: Register, conditions: [requireGuest] }),
    "/chat": wrap({ component: Chat, conditions: [requireAuth] }),
    "/expenses": wrap({ component: Expenses, conditions: [requireAuth] }),
    "/categories": wrap({ component: Categories, conditions: [requireAuth] }),
    "/budgets": wrap({ component: Budgets, conditions: [requireAuth] }),
    "/recurring": wrap({ component: Recurring, conditions: [requireAuth] }),
    "/insights": wrap({ component: Insights, conditions: [requireAuth] }),
    "/export": wrap({ component: ExportPage, conditions: [requireAuth] }),
    "/settings": wrap({ component: Settings, conditions: [requireAuth] }),
    "*": wrap({ component: Chat, conditions: [requireAuth] }),
  };

  let authed = false;
  auth.subscribe((s) => (authed = s.token !== null));

  onMount(() => {
    if (window.location.hash === "" || window.location.hash === "#/") {
      push(authed ? "/chat" : "/login");
    }
    if (authed) {
      // pre-load the currency preference so amounts format correctly
      // even before the user visits Settings
      userClient
        .getProfile({})
        .then((res) => {
          if (res.profile?.currency) currency.set(res.profile.currency);
          const t = res.profile?.theme;
          if (t === "light" || t === "dark") {
            applyTheme(t);
          } else {
            applyTheme("light");
          }
        })
        .catch(() => {
          // not fatal — pages fall back to the INR default and last theme
        });
    }
  });
</script>

{#if router.location.startsWith("/login") || router.location.startsWith("/register")}
  <Router {routes} />
{:else}
  <AppShell>
    <Router {routes} />
  </AppShell>
{/if}

<ToastStack />
