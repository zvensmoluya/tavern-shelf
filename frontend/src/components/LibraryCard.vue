<script setup lang="ts">
import { computed } from "vue";
import { Link2, Star } from "@lucide/vue";
import { characterTone, manifestOf } from "@/lib/format";
import CharacterCover from "@/components/CharacterCover.vue";
import type { Character } from "@/types";

const props = defineProps<{ character: Character; relatedCount: number; stacked?: boolean; matchingCount?: number; covers?: Character[] }>();
defineEmits<{ open: [character: Character]; favorite: [character: Character] }>();

const backCovers = computed(() => (props.covers || []).filter(item => item.id !== props.character.id).slice(0, 2).reverse());
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
  <article
    class="group relative min-w-0 text-left"
    :class="`tone-${characterTone(character.name)}`"
  >
    <button type="button" class="relative block w-full cursor-pointer border-0 bg-transparent p-0 text-left" :aria-label="`打开 ${character.name}${stacked ? `，${relatedCount + 1} 张关联卡` : ''}`" @click="$emit('open', character)">
    <div class="cover-slot relative aspect-[2/3]" :class="stacked ? 'is-stack' : ''">
      <div v-for="(item, index) in stacked ? backCovers : []" :key="item.id" aria-hidden="true" class="stack-back absolute overflow-hidden rounded-lg border border-white/25 bg-shelf-surface shadow-lg" :class="backCovers.length === 2 && index === 0 ? 'stack-back-far' : 'stack-back-near'">
        <CharacterCover :src="item.avatarUrl" :name="item.name" />
      </div>
    <div class="stack-front relative aspect-[2/3] overflow-hidden rounded-lg border border-shelf-line bg-shelf-surface shadow-shelf-card transition duration-200 group-hover:-translate-y-1 group-hover:border-shelf-line-strong group-hover:shadow-[0_19px_42px_rgba(0,0,0,.44)] group-focus-within:-translate-y-1 group-focus-within:border-shelf-line-strong">
      <CharacterCover :src="character.avatarUrl" :name="character.name" class="text-5xl font-light" />
      <span v-if="stacked" class="absolute bottom-2 left-2 rounded-md border border-white/15 bg-black/75 px-2 py-1 text-[10px] text-white backdrop-blur">{{ matchingCount && matchingCount < relatedCount + 1 ? `${matchingCount}/${relatedCount + 1} 张匹配` : `${relatedCount + 1} 张关联卡` }}</span>
      <div v-if="metrics.length && !stacked" class="absolute inset-x-0 bottom-0 flex flex-wrap gap-1.5 bg-gradient-to-t from-black/80 to-transparent px-2.5 pb-2.5 pt-14 opacity-0 transition group-hover:opacity-100 group-focus-visible:opacity-100">
        <span v-for="metric in metrics" :key="metric" class="rounded-md bg-black/65 px-2 py-1 text-[9px] text-shelf-text-soft backdrop-blur">{{ metric }}</span>
      </div>
    </div>
    </div>
    <h2 class="mt-3 truncate px-px text-[13px] font-semibold leading-5 text-shelf-text">{{ character.name }}</h2>
    <p class="mt-0.5 truncate px-px text-[10px] text-shelf-muted">{{ character.creator || "未知创作者" }}</p>
    <p class="mt-1 truncate text-[10px] text-shelf-quiet" :title="character.sourceFilename">{{ character.sourceFilename }}</p>
    <p v-if="relatedCount && !stacked" class="mt-2 inline-flex items-center gap-1 text-[10px] text-shelf-muted"><Link2 :size="12" />{{ relatedCount }} 张关联卡</p>
    </button>
    <button v-if="!stacked" type="button" class="absolute right-6 top-5 grid size-8 place-items-center rounded-full border border-white/15 bg-black/60 text-white/70 opacity-0 shadow-lg backdrop-blur transition hover:bg-black/80 hover:text-amber-200 group-hover:opacity-100 group-focus-within:opacity-100" :class="character.favorite ? '!opacity-100 text-amber-300' : ''" :aria-label="character.favorite ? `取消收藏 ${character.name}` : `收藏 ${character.name}`" @click.stop="$emit('favorite', character)">
      <Star :size="15" :fill="character.favorite ? 'currentColor' : 'none'" aria-hidden="true" />
    </button>
  </article>
</template>

<style scoped>
/* Reserve the same small gutter for every cover so mixed rows stay aligned. */
.cover-slot .stack-front { position: absolute; inset: 12px 12px 0 0; aspect-ratio: auto; z-index: 2; }
.stack-back { aspect-ratio: auto; }
.stack-back-near { inset: 6px 6px 6px 6px; }
.stack-back-far { inset: 0 0 12px 12px; }
</style>
