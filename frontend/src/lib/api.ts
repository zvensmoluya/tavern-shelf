import type { Character, ImportResult, ShelfResource, ShelfStatus, TransferSession, TransferTarget } from "@/types";

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
  listResources: () => request<ShelfResource[]>("/api/resources"),
  status: () => request<ShelfStatus>("/api/status"),
  chooseInbox: () => request<{ path: string }>("/api/desktop/choose-inbox", { method: "POST" }),
  addInbox: (path: string) => request<void>("/api/inboxes", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ path }),
  }),
  removeInbox: (path: string) => request<void>("/api/inboxes", {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ path }),
  }),
	startScanOnce: (path: string) => request<ShelfStatus["oneShotScan"]>("/api/scans", {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ path }),
	}),
	importFile: (file: File) => {
		const body = new FormData();
		body.append("file", file, file.name);
		return request<ImportResult>("/api/imports", { method: "POST", body });
	},
  openInbox: (path: string) => request<void>("/api/desktop/open-inbox", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ path }),
  }),
  setAutoStart: (enabled: boolean) => request<void>("/api/desktop/autostart", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ enabled }),
  }),
  removeCharacter: (id: string) => request<void>(`/api/characters/${id}`, { method: "DELETE" }),
  removeResource: (id: string) => request<void>(`/api/resources/${id}`, { method: "DELETE" }),
  createTransfer: (target: TransferTarget) => request<TransferSession>("/api/transfers", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ kind: target.kind, id: target.id }),
  }),
  revokeTransfer: (id: string) => request<void>(`/api/transfers/${id}`, { method: "DELETE" }),
};
