<script setup lang="ts">
import { BookOpenText, Braces, SlidersHorizontal } from "@lucide/vue";
import { computed } from "vue";
import type { ShelfResource } from "@/types";

const props = defineProps<{ resource: ShelfResource }>();
defineEmits<{ open: [resource: ShelfResource] }>();

const subtypeLabels: Record<string, string> = {
  openai: "Chat Completion",
  instruct: "Instruct",
  context: "Context",
  "system-prompt": "System Prompt",
  reasoning: "Reasoning",
  textgen: "Text Generation",
  novel: "NovelAI",
  kobold: "Kobold",
};

const meta = computed(() => {
  if (props.resource.kind === "worldbook") {
    const book = props.resource.worldbook;
    return `${book?.entryCount || 0} 条目 · ${book?.enabledEntryCount || 0} 启用`;
  }
  const preset = props.resource.preset;
  const label = subtypeLabels[props.resource.subtype || ""] || props.resource.subtype || "Preset";
  return preset?.promptCount ? `${label} · ${preset.promptCount} prompts` : label;
});
</script>

<template>
  <button
    type="button"
    class="group flex min-h-[164px] min-w-0 flex-col rounded-xl border border-shelf-line bg-shelf-surface p-5 text-left shadow-shelf-card transition duration-200 hover:-translate-y-0.5 hover:border-shelf-line-strong hover:bg-shelf-raised focus-visible:-translate-y-0.5 focus-visible:border-shelf-line-strong"
    :aria-label="`打开 ${resource.name}`"
    @click="$emit('open', resource)"
  >
    <div class="mb-6 flex items-start justify-between gap-4">
      <span class="grid size-10 place-items-center rounded-lg border border-shelf-line bg-shelf-soft text-shelf-text-soft">
        <BookOpenText v-if="resource.kind === 'worldbook'" :size="19" :stroke-width="1.6" aria-hidden="true" />
        <SlidersHorizontal v-else :size="19" :stroke-width="1.6" aria-hidden="true" />
      </span>
      <span class="rounded-full border border-shelf-line px-2.5 py-1 text-[9px] uppercase tracking-[.1em] text-shelf-quiet">
        {{ resource.kind === "worldbook" ? "Worldbook" : "Preset" }}
      </span>
    </div>

    <h2 class="truncate text-[15px] font-semibold tracking-[-.015em] text-shelf-text">{{ resource.name }}</h2>
    <p class="mt-1 truncate text-[10px] text-shelf-muted">{{ meta }}</p>
    <p v-if="resource.description" class="mt-3 line-clamp-2 text-[10px] leading-5 text-shelf-quiet">{{ resource.description }}</p>
    <div v-else class="mt-3 flex items-center gap-1.5 text-[9px] text-shelf-quiet">
      <Braces :size="12" aria-hidden="true" />{{ resource.sourceFilename }}
    </div>
  </button>
</template>
