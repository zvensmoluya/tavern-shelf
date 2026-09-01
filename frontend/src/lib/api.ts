import type { Character, ShelfStatus } from "@/types";

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, { cache: "no-store", ...init });
  if (!response.ok) {
    const payload = await response.json().catch(() => null) as { error?: string } | null;
    throw new Error(payload?.error || `HTTP ${response.status}`);
  }
  return response.status === 204 ? undefined as T : response.json() as Promise<T>;
}

export const api = {
  listCharacters: () => request<Character[]>("/api/characters"),
  status: () => request<ShelfStatus>("/api/status"),
  openInbox: () => request<void>("/api/desktop/open-inbox", { method: "POST" }),
  setAutoStart: (enabled: boolean) => request<void>("/api/desktop/autostart", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ enabled }),
  }),
  removeCharacter: (id: string) => request<void>(`/api/characters/${id}`, { method: "DELETE" }),
};
