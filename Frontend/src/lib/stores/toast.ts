import { writable } from "svelte/store";

export interface Toast {
  id: number;
  kind: "success" | "error" | "info";
  text: string;
}

export const toasts = writable<Toast[]>([]);

let counter = 0;

export function pushToast(text: string, kind: Toast["kind"] = "info") {
  const id = ++counter;
  toasts.update((t) => [...t, { id, kind, text }]);
  setTimeout(() => {
    toasts.update((t) => t.filter((x) => x.id !== id));
  }, 4000);
}

// Turns a ConnectError (or anything thrown by the API layer) into a
// plain, specific sentence — no apologizing, no vague "something went wrong".
export function describeError(err: unknown): string {
  const anyErr = err as any;
  if (anyErr?.message) {
    // Connect errors format as "[code] message" — strip the bracket prefix
    return String(anyErr.message).replace(/^\[[a-z_]+\]\s*/i, "");
  }
  return "That request didn't go through. Check your connection and try again.";
}
