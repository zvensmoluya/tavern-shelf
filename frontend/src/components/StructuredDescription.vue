<script setup lang="ts">
import { ChevronDown, ChevronUp } from "@lucide/vue";
import { computed, ref } from "vue";
import ExpandableText from "@/components/ui/ExpandableText.vue";
import ShelfDisclosure from "@/components/ui/ShelfDisclosure.vue";

type Segment =
  | { type: "prose"; text: string }
  | { type: "fields"; label: string; values: string[] }
  | { type: "dialogue"; messages: Array<{ role: "user" | "char"; text: string }> };

const props = defineProps<{ text: string; characterName: string }>();
const expandedGroups = ref<number[]>([]);

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
  const escapedName = props.characterName.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const withoutOwner = label.replace(new RegExp(`^${escapedName}(?:'s|’s)\\s+`, "i"), "");
  const known: Record<string, string> = {
    personality: "Personality · 性格",
    body: "Body · 外观",
    appearance: "Appearance · 外观",
    clothing: "Clothing · 服装",
    clothes: "Clothing · 服装",
    likes: "Likes · 喜好",
    dislikes: "Dislikes · 厌恶",
  };
  return known[withoutOwner.toLocaleLowerCase()] || withoutOwner;
}

function isExpanded(index: number): boolean {
  return expandedGroups.value.includes(index);
}

function toggle(index: number) {
  expandedGroups.value = isExpanded(index)
    ? expandedGroups.value.filter(value => value !== index)
    : [...expandedGroups.value, index];
}
</script>

<template>
  <div class="grid gap-3">
    <template v-for="(segment, index) in segments" :key="index">
      <ExpandableText v-if="segment.type === 'prose'" :text="segment.text" variant="lead" label="展开完整描述" />
      <ShelfDisclosure v-else-if="segment.type === 'dialogue'" title="附带对话片段" :meta="`${segment.messages.length} 条`" compact>
        <div class="grid gap-2.5">
          <div v-for="(message, messageIndex) in segment.messages" :key="messageIndex" class="rounded-lg bg-shelf-canvas/60 p-3.5">
            <p class="mb-1.5 text-[9px] font-semibold uppercase tracking-[.1em] text-shelf-quiet">{{ message.role === "user" ? "User" : characterName }}</p>
            <p class="whitespace-pre-wrap text-[12px] leading-[1.8] text-shelf-text-soft/90">{{ message.text }}</p>
          </div>
        </div>
      </ShelfDisclosure>
      <section v-else class="rounded-xl border border-shelf-line bg-white/[.022] p-4">
        <header class="mb-3 flex items-baseline justify-between gap-4">
          <h3 class="text-[13px] font-semibold text-shelf-text-soft">{{ segment.label }}</h3>
          <span class="text-[10px] text-shelf-muted">{{ segment.values.length }} 项</span>
        </header>
        <div class="flex flex-wrap gap-2">
          <span
            v-for="(value, valueIndex) in (isExpanded(index) ? segment.values : segment.values.slice(0, 12))"
            :key="`${value}-${valueIndex}`"
            class="rounded-full border border-shelf-line bg-shelf-soft/70 px-2.5 py-1.5 text-[11px] leading-4 text-shelf-text-soft"
          >{{ value }}</span>
        </div>
        <button
          v-if="segment.values.length > 12"
          type="button"
          class="mt-3 inline-flex items-center gap-1.5 text-[10px] text-shelf-muted transition hover:text-shelf-text"
          :aria-expanded="isExpanded(index)"
          @click="toggle(index)"
        >
          {{ isExpanded(index) ? "收起" : `显示另外 ${segment.values.length - 12} 项` }}
          <ChevronUp v-if="isExpanded(index)" :size="13" aria-hidden="true" />
          <ChevronDown v-else :size="13" aria-hidden="true" />
        </button>
      </section>
    </template>
  </div>
</template>
