<script setup lang="ts">
import { BookOpenText, ChevronLeft, ChevronRight } from "@lucide/vue";
import { computed, nextTick, ref, watch } from "vue";
import ShelfIconButton from "@/components/ui/ShelfIconButton.vue";
import type { CharacterBook, CharacterBookEntry } from "@/types";

const props = defineProps<{ book: CharacterBook; characterName: string }>();
const selectedIndex = ref(0);
const track = ref<HTMLElement | null>(null);

const selectedEntry = computed(() => props.book.entries[selectedIndex.value]);

watch(() => props.book.entries.length, (length) => {
  selectedIndex.value = length ? Math.min(selectedIndex.value, length - 1) : 0;
});

function flags(entry: CharacterBookEntry): string[] {
  const values: string[] = [];
  if (entry.constant) values.push("常驻");
  if (entry.selective) values.push("选择性匹配");
  if (entry.useRegex) values.push("Regex keys");
  if (entry.caseSensitive) values.push("区分大小写");
  return values;
}

function readable(text = ""): string {
  return text
    .replace(/<START>/gi, "")
    .replace(/\{\{user\}\}\s*:/gi, "User — ")
    .replace(/\{\{char\}\}\s*:/gi, `${props.characterName} — `)
    .replace(/\{\{user\}\}/gi, "User")
    .replace(/\{\{char\}\}/gi, props.characterName)
    .trim();
}

async function select(index: number) {
  if (!props.book.entries.length) return;
  selectedIndex.value = Math.max(0, Math.min(index, props.book.entries.length - 1));
  await nextTick();
  track.value
    ?.querySelector<HTMLElement>(`[data-entry-index="${selectedIndex.value}"]`)
    ?.scrollIntoView({ behavior: "smooth", block: "nearest", inline: "nearest" });
}
</script>

<template>
  <div v-if="book.entries.length">
    <div class="mb-4 flex items-center gap-2">
      <ShelfIconButton
        v-if="book.entries.length > 1"
        :icon="ChevronLeft"
        label="上一本世界书"
        :size="15"
        :disabled="selectedIndex === 0"
        class="size-8"
        @click="select(selectedIndex - 1)"
      />

      <div
        ref="track"
        role="tablist"
        aria-label="世界书条目"
        data-testid="worldbook-track"
        class="shelf-scrollbar flex min-w-0 flex-1 gap-2 overflow-x-auto overscroll-x-contain pb-1"
      >
        <button
          v-for="(entry, index) in book.entries"
          :key="`${entry.insertionOrder}-${entry.name}`"
          type="button"
          role="tab"
          :aria-selected="selectedIndex === index"
          :tabindex="selectedIndex === index ? 0 : -1"
          :data-entry-index="index"
          data-testid="worldbook-entry"
          class="group flex min-w-[148px] max-w-[210px] shrink-0 items-center gap-2.5 rounded-lg border px-3 py-2.5 text-left transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white/55"
          :class="selectedIndex === index
            ? 'border-shelf-line-strong bg-shelf-soft text-shelf-text'
            : 'border-transparent bg-white/[.018] text-shelf-muted hover:border-shelf-line hover:bg-white/[.035] hover:text-shelf-text-soft'"
          @click="select(index)"
          @keydown.left.prevent="select(index - 1)"
          @keydown.right.prevent="select(index + 1)"
        >
          <BookOpenText
            :size="16"
            :class="selectedIndex === index ? 'text-shelf-text' : entry.enabled ? 'text-shelf-success/75' : 'text-shelf-quiet'"
            aria-hidden="true"
          />
          <span class="min-w-0">
            <span class="block truncate text-[12px] font-semibold">{{ entry.name || `书本 ${index + 1}` }}</span>
            <span class="mt-0.5 block text-[9px] text-shelf-quiet">书本 {{ String(index + 1).padStart(2, "0") }}</span>
          </span>
        </button>
      </div>

      <ShelfIconButton
        v-if="book.entries.length > 1"
        :icon="ChevronRight"
        label="下一本世界书"
        :size="15"
        :disabled="selectedIndex === book.entries.length - 1"
        class="size-8"
        @click="select(selectedIndex + 1)"
      />
    </div>

    <article v-if="selectedEntry" role="tabpanel" class="border-t border-shelf-line pt-5">
      <header class="flex items-start justify-between gap-4">
        <div class="min-w-0">
          <p class="mb-1.5 text-[9px] font-semibold uppercase tracking-[.12em] text-shelf-quiet">当前世界书</p>
          <h4 class="text-[18px] font-semibold tracking-[-.015em] text-shelf-text">{{ selectedEntry.name || `书本 ${selectedIndex + 1}` }}</h4>
          <p v-if="selectedEntry.comment && selectedEntry.comment !== selectedEntry.name" class="mt-1 text-[11px] leading-5 text-shelf-muted">{{ selectedEntry.comment }}</p>
        </div>
        <span
          class="shrink-0 rounded-full px-2.5 py-1 text-[9px]"
          :class="selectedEntry.enabled ? 'bg-emerald-300/10 text-shelf-success' : 'bg-white/[.04] text-shelf-quiet'"
        >{{ selectedEntry.enabled ? "启用" : "停用" }}</span>
      </header>

      <div v-if="selectedEntry.keys.length || selectedEntry.secondaryKeys.length" class="mt-4 grid gap-3 sm:grid-cols-2">
        <div v-if="selectedEntry.keys.length">
          <p class="mb-2 text-[9px] font-semibold uppercase tracking-[.1em] text-shelf-quiet">匹配关键词</p>
          <div class="flex flex-wrap gap-1.5">
            <span v-for="key in selectedEntry.keys" :key="key" class="max-w-full rounded bg-shelf-soft px-2 py-1 font-mono text-[10px] text-shelf-muted">{{ key }}</span>
          </div>
        </div>
        <div v-if="selectedEntry.secondaryKeys.length">
          <p class="mb-2 text-[9px] font-semibold uppercase tracking-[.1em] text-shelf-quiet">辅助关键词</p>
          <div class="flex flex-wrap gap-1.5">
            <span v-for="key in selectedEntry.secondaryKeys" :key="key" class="max-w-full rounded bg-shelf-soft px-2 py-1 font-mono text-[10px] text-shelf-muted">{{ key }}</span>
          </div>
        </div>
      </div>

      <div v-if="flags(selectedEntry).length" class="mt-3 flex flex-wrap gap-1.5">
        <span v-for="flag in flags(selectedEntry)" :key="flag" class="rounded border border-shelf-line px-2 py-1 text-[9px] text-shelf-muted">{{ flag }}</span>
      </div>

      <div class="mt-5">
        <p class="mb-2 text-[9px] font-semibold uppercase tracking-[.1em] text-shelf-quiet">正文</p>
        <p v-if="selectedEntry.content" class="whitespace-pre-wrap text-[13px] leading-[1.9] text-shelf-text-soft/95">{{ readable(selectedEntry.content) }}</p>
        <p v-else class="text-[12px] text-shelf-quiet">这个条目没有正文内容。</p>
      </div>
    </article>
  </div>

  <p v-else class="text-[12px] text-shelf-quiet">这本世界书没有条目。</p>
</template>
