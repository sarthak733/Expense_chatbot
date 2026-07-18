import { writable } from "svelte/store";

export interface AuthUser {
  id: number;
  username: string;
  createdAt: string;
}

interface AuthState {
  token: string | null;
  user: AuthUser | null;
}

const STORAGE_KEY = "ledger.auth";

function loadInitial(): AuthState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw);
  } catch {
    // corrupted/blocked storage — fall through to logged-out state
  }
  return { token: null, user: null };
}

export const auth = writable<AuthState>(loadInitial());

auth.subscribe((state) => {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    // storage unavailable (private browsing, etc.) — auth just won't persist
  }
});

export function setAuth(token: string, user: AuthUser) {
  auth.set({ token, user });
}

export function clearAuth() {
  auth.set({ token: null, user: null });
}

export function getToken(): string | null {
  let token: string | null = null;
  auth.subscribe((s) => (token = s.token))();
  return token;
}

export function isAuthed(): boolean {
  return getToken() !== null;
}
