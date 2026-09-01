<script setup lang="ts">
import { BookOpenText, ChevronLeft, ChevronRight } from "@lucide/vue";
import { computed, ref, watch } from "vue";
import ShelfIconButton from "@/components/ui/ShelfIconButton.vue";
import type { CharacterBook, CharacterBookEntry } from "@/types";

const props = defineProps<{ book: CharacterBook }>();
const pageSize = 6;
const selectedIndex = ref(0);

const page = computed(() => Math.floor(selectedIndex.value / pageSize));
const pageCount = computed(() => Math.max(1, Math.ceil(props.book.entries.length / pageSize)));
const pageStart = computed(() => page.value * pageSize);
const pageEntries = computed(() => props.book.entries.slice(pageStart.value, pageStart.value + pageSize));
const selectedEntry = computed(() => props.book.entries[selectedIndex.value] || null);

watch(() => props.book, () => { selectedIndex.value = 0; });

function flags(entry: CharacterBookEntry): string[] {
  const values: string[] = [];
  if (entry.constant) values.push("常驻");
  if (entry.selective) values.push("选择性匹配");
  if (entry.useRegex) values.push("Regex keys");
  if (entry.caseSensitive) values.push("区分大小写");
  return values;
}

function changePage(offset: number) {
  const nextPage = Math.min(pageCount.value - 1, Math.max(0, page.value + offset));
  selectedIndex.value = nextPage * pageSize;
}
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap gap-2">
      <span class="rounded-md bg-shelf-soft px-2.5 py-1.5 text-[10px] text-shelf-muted">{{ book.entryCount }} 条目</span>
      <span class="rounded-md bg-shelf-soft px-2.5 py-1.5 text-[10px] text-shelf-muted">{{ book.enabledEntryCount }} 条启用</span>
      <span v-if="book.scanDepth != null" class="rounded-md bg-shelf-soft px-2.5 py-1.5 text-[10px] text-shelf-muted">扫描深度 {{ book.scanDepth }}</span>
      <span v-if="book.tokenBudget != null" class="rounded-md bg-shelf-soft px-2.5 py-1.5 text-[10px] text-shelf-muted">Token {{ book.tokenBudget }}</span>
    </div>
    <p v-if="book.description" class="mb-4 whitespace-pre-wrap text-[13px] leading-[1.85] text-shelf-text-soft/90">{{ book.description }}</p>

    <div v-if="selectedEntry" class="grid min-h-[330px] grid-cols-[minmax(150px,34%)_minmax(0,1fr)] overflow-hidden rounded-xl border border-shelf-line max-[760px]:grid-cols-1">
      <aside class="border-r border-shelf-line bg-shelf-canvas/45 p-2 max-[760px]:border-b max-[760px]:border-r-0">
        <div class="grid gap-1 max-[760px]:grid-cols-2">
          <button
            v-for="(entry, localIndex) in pageEntries"
            :key="`${entry.insertionOrder}-${entry.name}`"
            type="button"
            class="flex min-h-10 min-w-0 items-center gap-2 rounded-lg px-2.5 py-2 text-left text-[11px] text-shelf-muted transition hover:bg-white/[.04] hover:text-shelf-text"
            :class="selectedIndex === pageStart + localIndex ? 'bg-shelf-soft text-shelf-text' : ''"
            @click="selectedIndex = pageStart + localIndex"
          >
            <BookOpenText :size="14" :class="entry.enabled ? 'text-shelf-success' : 'text-shelf-quiet'" aria-hidden="true" />
            <span class="truncate">{{ entry.name }}</span>
          </button>
        </div>
        <div v-if="pageCount > 1" class="mt-2 flex items-center justify-between border-t border-shelf-line px-1 pt-2">
          <ShelfIconButton :icon="ChevronLeft" label="上一组世界书条目" :size="14" class="size-7" :disabled="page === 0" @click="changePage(-1)" />
          <span class="text-[9px] text-shelf-muted">{{ page + 1 }} / {{ pageCount }}</span>
          <ShelfIconButton :icon="ChevronRight" label="下一组世界书条目" :size="14" class="size-7" :disabled="page === pageCount - 1" @click="changePage(1)" />
        </div>
      </aside>

      <article class="shelf-scrollbar max-h-[430px] min-h-0 overflow-y-auto overscroll-contain p-5">
        <header class="mb-4 flex items-start justify-between gap-3">
          <div>
            <h3 class="text-[15px] font-semibold text-shelf-text">{{ selectedEntry.name }}</h3>
            <p v-if="selectedEntry.comment && selectedEntry.comment !== selectedEntry.name" class="mt-1 text-[11px] text-shelf-muted">{{ selectedEntry.comment }}</p>
          </div>
          <span class="shrink-0 rounded-full px-2 py-1 text-[9px]" :class="selectedEntry.enabled ? 'bg-emerald-300/10 text-shelf-success' : 'bg-white/[.04] text-shelf-quiet'">{{ selectedEntry.enabled ? "启用" : "停用" }}</span>
        </header>

        <div v-if="selectedEntry.keys.length || selectedEntry.secondaryKeys.length" class="mb-3">
          <p class="mb-2 text-[9px] font-semibold uppercase tracking-[.1em] text-shelf-quiet">匹配关键词</p>
          <div class="flex flex-wrap gap-1.5">
            <span v-for="key in [...selectedEntry.keys, ...selectedEntry.secondaryKeys]" :key="key" class="rounded bg-shelf-soft px-2 py-1 font-mono text-[10px] text-shelf-muted">{{ key }}</span>
          </div>
        </div>

        <div v-if="flags(selectedEntry).length" class="mb-3 flex flex-wrap gap-1.5">
          <span v-for="flag in flags(selectedEntry)" :key="flag" class="rounded border border-shelf-line px-2 py-1 text-[10px] text-shelf-muted">{{ flag }}</span>
        </div>
        <p v-if="selectedEntry.content" class="whitespace-pre-wrap text-[12px] leading-[1.85] text-shelf-text-soft/90">{{ selectedEntry.content }}</p>
        <p v-else class="text-[11px] text-shelf-quiet">这个条目没有正文内容。</p>
      </article>
    </div>
  </div>
</template>
