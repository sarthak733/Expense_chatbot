import { writable } from "svelte/store";

export type ThemePref = "light" | "dark";

const STORAGE_KEY = "ledger.theme";

export const theme = writable<ThemePref>(
  (localStorage.getItem(STORAGE_KEY) as ThemePref) || "light"
);

// Call once at startup, and again whenever the saved preference changes.
export function applyTheme(pref: ThemePref) {
  theme.set(pref);
  try {
    localStorage.setItem(STORAGE_KEY, pref);
  } catch {
    // storage unavailable
  }
  document.documentElement.dataset.theme = pref;
}
