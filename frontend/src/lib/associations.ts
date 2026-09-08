import type { Character } from "@/types";

export function groupID(character: Character): string { return character.groupId || character.id; }
export function contentID(character: Character): string { return character.contentHash || character.id; }
export function membersOf(characters: Character[], character: Character): Character[] {
  return characters.filter(item => groupID(item) === groupID(character))
    .sort((a, b) => (Date.parse(a.importedAt) - Date.parse(b.importedAt)) || a.id.localeCompare(b.id));
}
// Input is already sorted and filtered. Keep every matching card, placing
// related cards next to their first-ranked member without changing any metadata.
export function adjacentCharacters(characters: Character[]): Character[] {
  const groups = new Map<string, Character[]>();
  for (const character of characters) {
    const key = groupID(character);
    const members = groups.get(key) || [];
    members.push(character);
    groups.set(key, members);
  }
  return [...groups.values()].flat();
}

// A display summary, never an identity comparison: unknown fields remain covered
// by the Core's full-payload fingerprint even if they have no display projection.
export function changeSummary(current: Character, previous?: Character): string {
  if (!current.contentHash) return "数据未能可靠比较 · 原件完整保留";
  if (!previous) return "关联卡";
  const a = current.manifest, b = previous.manifest;
  const changed: string[] = [];
  if (JSON.stringify(a.character) !== JSON.stringify(b.character)) changed.push("角色资料");
  if (JSON.stringify(a.greetings) !== JSON.stringify(b.greetings)) changed.push("开场白");
  if (JSON.stringify(a.characterBook) !== JSON.stringify(b.characterBook)) changed.push("世界书");
  if (JSON.stringify(a.regexScripts) !== JSON.stringify(b.regexScripts)) changed.push("正则脚本");
  if (JSON.stringify(a.extensions) !== JSON.stringify(b.extensions)) changed.push("扩展");
  return changed.length ? `与当前卡相比：${changed.join("、")}有变化` : "其他卡片数据有变化";
}
