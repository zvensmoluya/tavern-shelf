import type { Character, Manifest } from "@/types";

export function characterTone(name = ""): number {
  let hash = 0;
  for (const char of name) hash = ((hash << 5) - hash + (char.codePointAt(0) || 0)) | 0;
  return Math.abs(hash % 8);
}

export function initialOf(name = "?"): string {
  return Array.from(name || "?")[0] || "?";
}

export function formatSize(bytes: number): string {
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.ceil(bytes / 1024))} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

export function formatImported(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
}

export function formatCardDate(unixSeconds?: number): string {
  if (!unixSeconds) return "";
  return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium" }).format(new Date(unixSeconds * 1000));
}

export function manifestOf(character: Character): Manifest {
  return character.manifest || {
    schemaVersion: 0,
    character: { name: character.name, creator: character.creator, tags: character.tags || [] },
    greetings: { alternate: [], groupOnly: [], totalCount: 0, alternateCount: 0, groupOnlyCount: 0 },
    regexScripts: [],
    extensions: [],
    assets: [],
    sources: [],
    interaction: { hasHtml: false, hasJavaScript: false, hasInteractiveExtension: false, markers: [] },
  };
}
