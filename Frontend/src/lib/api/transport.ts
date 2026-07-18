import { createConnectTransport } from "@connectrpc/connect-web";
import type { Interceptor } from "@connectrpc/connect";
import { getToken, clearAuth } from "../stores/auth";

// Point this at your Go backend. Override with a .env file:
//   VITE_API_URL=http://localhost:8080
const BASE_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

// Attaches the JWT (if we have one) to every outgoing call, exactly like
// the backend's AuthInterceptor expects: `Authorization: Bearer <token>`.
const authInterceptor: Interceptor = (next) => async (req) => {
  const token = getToken();
  if (token) {
    req.header.set("Authorization", `Bearer ${token}`);
  }
  try {
    return await next(req);
  } catch (err: any) {
    // Session expired or invalid — drop local auth state so the UI
    // routes back to login instead of silently failing repeatedly.
    if (err?.code === 16 /* Unauthenticated */) {
      clearAuth();
    }
    throw err;
  }
};

export const transport = createConnectTransport({
  baseUrl: BASE_URL,
  interceptors: [authInterceptor],
});
