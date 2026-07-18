import { writable } from "svelte/store";

export type ThemePref = "light" | "dark" | "system";

const STORAGE_KEY = "ledger.theme";

export const theme = writable<ThemePref>(
  (localStorage.getItem(STORAGE_KEY) as ThemePref) || "system"
);

let mediaQuery: MediaQueryList | null = null;
let systemListener: ((e: MediaQueryListEvent) => void) | null = null;

function resolveAndApply(pref: ThemePref) {
  const resolved =
    pref === "system"
      ? window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light"
      : pref;
  document.documentElement.dataset.theme = resolved;
}

// Call once at startup, and again whenever the saved preference changes
// (e.g. after Settings saves a new value from the backend).
export function applyTheme(pref: ThemePref) {
  theme.set(pref);
  try {
    localStorage.setItem(STORAGE_KEY, pref);
  } catch {
    // storage unavailable — theme just won't persist across reloads
  }

  resolveAndApply(pref);

  // Clean up any previous system-preference listener before adding a new one
  if (mediaQuery && systemListener) {
    mediaQuery.removeEventListener("change", systemListener);
    mediaQuery = null;
    systemListener = null;
  }

  if (pref === "system") {
    mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
    systemListener = () => resolveAndApply("system");
    mediaQuery.addEventListener("change", systemListener);
  }
}
