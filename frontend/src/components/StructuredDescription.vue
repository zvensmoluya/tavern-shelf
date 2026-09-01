<script setup lang="ts">
import { computed } from "vue";
import ExpandableText from "@/components/ui/ExpandableText.vue";

type Segment =
  | { type: "prose"; text: string }
  | { type: "fields"; label: string; values: string[] }
  | { type: "dialogue"; messages: Array<{ role: "user" | "char"; text: string }> };

const props = defineProps<{ text: string; characterName: string }>();

const segments = computed<Segment[]>(() => {
  const output: Segment[] = [];
  const assignment = /\[([^\[\]\r\n=]{1,100})\s*=\s*((?:"(?:\\.|[^"\\])*"\s*,?\s*)+)\]/g;
  let cursor = 0;
  let match: RegExpExecArray | null;
  while ((match = assignment.exec(props.text)) !== null) {
    appendText(output, props.text.slice(cursor, match.index));
    const values = Array.from(match[2].matchAll(/"((?:\\.|[^"\\])*)"/g), value => decodeQuoted(value[1]));
    if (values.length) output.push({ type: "fields", label: cleanLabel(match[1].trim()), values });
    cursor = match.index + match[0].length;
  }
  appendText(output, props.text.slice(cursor));
  return output.some(segment => segment.type !== "prose") ? output : [{ type: "prose", text: props.text }];
});

function appendText(output: Segment[], text: string) {
  for (const chunk of text.split(/<START>/i)) {
    const trimmed = chunk.trim();
    if (!trimmed) continue;
    const rolePattern = /\{\{(user|char)\}\}\s*:\s*([\s\S]*?)(?=\r?\n\s*\{\{(?:user|char)\}\}\s*:|$)/gi;
    const matches = Array.from(trimmed.matchAll(rolePattern));
    if (!matches.length) {
      output.push({ type: "prose", text: trimmed });
      continue;
    }
    const prefix = trimmed.slice(0, matches[0].index).trim();
    if (prefix) output.push({ type: "prose", text: prefix });
    output.push({
      type: "dialogue",
      messages: matches.map(match => ({ role: match[1].toLocaleLowerCase() as "user" | "char", text: match[2].trim() })),
    });
  }
}

function decodeQuoted(value: string): string {
  try { return JSON.parse(`"${value}"`) as string; }
  catch { return value.replaceAll('\\"', '"').replaceAll("\\\\", "\\"); }
}

function cleanLabel(label: string): string {
  return label.replace(/\s+/g, " ").trim();
}

function dialogueText(messages: Array<{ role: "user" | "char"; text: string }>): string {
  return messages.map(message => `${message.role === "user" ? "User" : props.characterName}\n${message.text}`).join("\n\n");
}
</script>

<template>
  <div class="grid gap-3">
    <template v-for="(segment, index) in segments" :key="index">
      <ExpandableText v-if="segment.type === 'prose'" :text="segment.text" variant="lead" label="展开完整描述" />
      <div v-else-if="segment.type === 'dialogue'" class="border-l-2 border-shelf-line-strong py-1 pl-4">
        <ExpandableText :text="dialogueText(segment.messages)" :limit="380" variant="prose" label="展开完整内容" />
      </div>
      <div v-else class="border-l-2 border-shelf-line-strong py-1 pl-4">
        <p class="mb-1.5 text-[11px] font-semibold text-shelf-muted">{{ segment.label }}</p>
        <p class="text-[13px] leading-[1.85] text-shelf-text-soft">{{ segment.values.join(" · ") }}</p>
      </div>
    </template>
  </div>
</template>
