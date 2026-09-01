<script setup lang="ts">
import { BookOpenText, ChevronLeft, ChevronRight } from "@lucide/vue";
import { ref } from "vue";
import ShelfIconButton from "@/components/ui/ShelfIconButton.vue";
import type { CharacterBook, CharacterBookEntry } from "@/types";

const props = defineProps<{ book: CharacterBook; characterName: string }>();
const track = ref<HTMLElement | null>(null);

function flags(entry: CharacterBookEntry): string[] {
  const values: string[] = [];
  if (entry.constant) values.push("常驻");
  if (entry.selective) values.push("选择性匹配");
  if (entry.useRegex) values.push("Regex keys");
  if (entry.caseSensitive) values.push("区分大小写");
  return values;
}

function preview(text = "", limit = 430): string {
  const readable = text
    .replace(/<START>/gi, "")
    .replace(/\{\{user\}\}\s*:/gi, "User — ")
    .replace(/\{\{char\}\}\s*:/gi, `${props.characterName} — `)
    .trim();
  if (readable.length <= limit) return readable;
  const value = readable.slice(0, limit).replace(/\s+\S*$/, "").trimEnd();
  return `${value}…`;
}

function move(direction: -1 | 1) {
  track.value?.scrollBy({ left: direction * 330, behavior: "smooth" });
}
</script>

<template>
  <div>
    <div class="mb-3 flex items-start justify-between gap-4">
      <div class="flex flex-wrap gap-2">
        <span class="rounded-md bg-shelf-soft px-2.5 py-1.5 text-[10px] text-shelf-muted">{{ book.entryCount }} 条目</span>
        <span class="rounded-md bg-shelf-soft px-2.5 py-1.5 text-[10px] text-shelf-muted">{{ book.enabledEntryCount }} 条启用</span>
        <span v-if="book.scanDepth != null" class="rounded-md bg-shelf-soft px-2.5 py-1.5 text-[10px] text-shelf-muted">扫描深度 {{ book.scanDepth }}</span>
        <span v-if="book.tokenBudget != null" class="rounded-md bg-shelf-soft px-2.5 py-1.5 text-[10px] text-shelf-muted">Token {{ book.tokenBudget }}</span>
      </div>
      <div v-if="book.entries.length > 1" class="flex shrink-0 gap-1.5">
        <ShelfIconButton :icon="ChevronLeft" label="向左浏览世界书" :size="14" class="size-8" @click="move(-1)" />
        <ShelfIconButton :icon="ChevronRight" label="向右浏览世界书" :size="14" class="size-8" @click="move(1)" />
      </div>
    </div>
    <p v-if="book.description" class="mb-4 whitespace-pre-wrap text-[13px] leading-[1.85] text-shelf-text-soft/90">{{ book.description }}</p>

    <div ref="track" data-testid="worldbook-track" class="shelf-scrollbar flex snap-x snap-mandatory gap-3 overflow-x-auto overscroll-x-contain pb-3">
      <article
        v-for="(entry, index) in book.entries"
        :key="`${entry.insertionOrder}-${entry.name}`"
        data-testid="worldbook-entry"
        class="flex min-h-[310px] w-[310px] shrink-0 snap-start flex-col rounded-xl border border-shelf-line bg-white/[.018] p-4"
      >
        <header class="mb-3 flex items-start gap-2.5">
          <BookOpenText :size="16" :class="entry.enabled ? 'text-shelf-success' : 'text-shelf-quiet'" aria-hidden="true" />
          <div class="min-w-0 flex-1">
            <p class="mb-1 text-[9px] uppercase tracking-[.11em] text-shelf-quiet">Entry {{ String(index + 1).padStart(2, "0") }}</p>
            <h4 class="truncate text-[14px] font-semibold text-shelf-text">{{ entry.name }}</h4>
          </div>
          <span class="shrink-0 rounded-full px-2 py-1 text-[9px]" :class="entry.enabled ? 'bg-emerald-300/10 text-shelf-success' : 'bg-white/[.04] text-shelf-quiet'">{{ entry.enabled ? "启用" : "停用" }}</span>
        </header>

        <div v-if="entry.keys.length || entry.secondaryKeys.length" class="mb-3">
          <p class="mb-1.5 text-[9px] font-semibold uppercase tracking-[.1em] text-shelf-quiet">匹配关键词</p>
          <div class="flex flex-wrap gap-1.5">
            <span v-for="key in [...entry.keys, ...entry.secondaryKeys].slice(0, 6)" :key="key" class="max-w-full truncate rounded bg-shelf-soft px-2 py-1 font-mono text-[9px] text-shelf-muted">{{ key }}</span>
            <span v-if="entry.keys.length + entry.secondaryKeys.length > 6" class="rounded bg-shelf-soft px-2 py-1 text-[9px] text-shelf-muted">+{{ entry.keys.length + entry.secondaryKeys.length - 6 }}</span>
          </div>
        </div>

        <p v-if="entry.content" class="line-clamp-9 whitespace-pre-wrap text-[11px] leading-[1.75] text-shelf-text-soft/85">{{ preview(entry.content) }}</p>
        <p v-else class="text-[11px] text-shelf-quiet">这个条目没有正文内容。</p>

        <footer v-if="flags(entry).length" class="mt-auto flex flex-wrap gap-1.5 border-t border-shelf-line pt-3">
          <span v-for="flag in flags(entry)" :key="flag" class="rounded border border-shelf-line px-2 py-1 text-[9px] text-shelf-muted">{{ flag }}</span>
        </footer>
      </article>
    </div>
  </div>
</template>
