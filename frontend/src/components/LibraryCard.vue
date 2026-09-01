<script setup lang="ts">
import { computed } from "vue";
import { characterTone, initialOf, manifestOf } from "@/lib/format";
import type { Character } from "@/types";

const props = defineProps<{ character: Character }>();
defineEmits<{ open: [character: Character] }>();

const manifest = computed(() => manifestOf(props.character));
const metrics = computed(() => {
  const values: string[] = [];
  if (manifest.value.greetings.totalCount) values.push(`${manifest.value.greetings.totalCount} 开场`);
  if (manifest.value.characterBook?.entryCount) values.push(`${manifest.value.characterBook.entryCount} 世界书`);
  if (manifest.value.regexScripts.length) values.push(`${manifest.value.regexScripts.length} Regex`);
  return values;
});
</script>

<template>
  <button
    type="button"
    class="group min-w-0 cursor-pointer border-0 bg-transparent p-0 text-left"
    :class="`tone-${characterTone(character.name)}`"
    :aria-label="`打开 ${character.name}`"
    @click="$emit('open', character)"
  >
    <div class="relative aspect-[2/3] overflow-hidden rounded-lg border border-shelf-line bg-shelf-surface shadow-shelf-card transition duration-200 group-hover:-translate-y-1 group-hover:border-shelf-line-strong group-hover:shadow-[0_19px_42px_rgba(0,0,0,.44)] group-focus-visible:-translate-y-1 group-focus-visible:border-shelf-line-strong">
      <img v-if="character.avatarUrl" :src="character.avatarUrl" :alt="`${character.name} 的角色卡封面`" class="block size-full object-cover object-top">
      <div v-else class="cover-fallback grid size-full place-items-center text-5xl font-light" aria-hidden="true">{{ initialOf(character.name) }}</div>
      <div v-if="metrics.length" class="absolute inset-x-0 bottom-0 flex flex-wrap gap-1.5 bg-gradient-to-t from-black/80 to-transparent px-2.5 pb-2.5 pt-14 opacity-0 transition group-hover:opacity-100 group-focus-visible:opacity-100">
        <span v-for="metric in metrics" :key="metric" class="rounded-md bg-black/65 px-2 py-1 text-[9px] text-shelf-text-soft backdrop-blur">{{ metric }}</span>
      </div>
    </div>
    <h2 class="mt-3 truncate px-px text-[13px] font-semibold leading-5 text-shelf-text">{{ character.name }}</h2>
    <p class="mt-0.5 truncate px-px text-[10px] text-shelf-muted">{{ character.creator || "未知创作者" }}</p>
  </button>
</template>
