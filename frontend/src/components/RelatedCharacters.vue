<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { Check, Ellipsis, Undo2 } from "@lucide/vue";
import { PopoverRoot, PopoverTrigger, PopoverPortal, PopoverContent } from "reka-ui";
import CharacterCover from "@/components/CharacterCover.vue";
import ShelfButton from "@/components/ui/ShelfButton.vue";
import ShelfIconButton from "@/components/ui/ShelfIconButton.vue";
import { api } from "@/lib/api";
import { membersOf, groupID, changeSummary, contentID } from "@/lib/associations";
import { formatImported } from "@/lib/format";
import type { Character } from "@/types";

const props = defineProps<{ character: Character; characters: Character[]; matchingIds: string[] }>();
const emit = defineEmits<{ select: [id: string]; changed: [] }>();
const related = computed(() => membersOf(props.characters, props.character).filter(item => item.id !== props.character.id));
const candidates = computed(() => props.characters.filter(item => groupID(item) !== groupID(props.character)));
const choices = computed(() => membersOf(props.characters, props.character).sort((a, b) => Number(props.matchingIds.includes(b.id)) - Number(props.matchingIds.includes(a.id))));
const strip = ref<HTMLElement | null>(null);
watch(() => props.character.id, async () => {
  await nextTick();
  const selected = strip.value?.querySelector<HTMLElement>('[aria-pressed="true"]');
  if (selected && strip.value) {
    const left = selected.offsetLeft - strip.value.offsetLeft;
    if (left < strip.value.scrollLeft || left + selected.offsetWidth > strip.value.scrollLeft + strip.value.clientWidth) strip.value.scrollLeft = Math.max(0, left - 4);
  }
}, { immediate: true });
const previewId = ref("");
const preview = computed(() => choices.value.find(item => item.id === previewId.value));
const target = ref("");
const busy = ref(false);
const menuOpen = ref(false);
const error = ref("");
const undo = ref<{ id: string; target: string } | null>(null);
const message = ref("");
watch(() => props.character.id, () => { previewId.value = ""; target.value = ""; error.value = ""; message.value = ""; undo.value = null; });
function relation(item: Character) {
  if (item.contentHash && contentID(item) === contentID(props.character)) return "卡片数据相同";
  return changeSummary(item, props.character);
}
async function change(action: string, destination = "") {
  if (busy.value) return;
  busy.value = true; error.value = "";
  try {
    const sibling = related.value[0];
    await api.changeAssociation(props.character.id, action, destination);
    undo.value = action === "split" && sibling ? { id: props.character.id, target: sibling.id } : null;
    message.value = action === "split" ? "已移出关联" : "已关联，卡片仍各自保留";
    target.value = "";
    menuOpen.value = false;
    emit("changed");
  } catch (cause) { error.value = cause instanceof Error ? cause.message : "保存失败，请重试"; }
  finally { busy.value = false; }
}
</script>

<template>
  <section class="mb-6" aria-label="关联卡">
    <header class="mb-2 flex items-center gap-2">
      <p class="min-w-0 flex-1 text-[11px] text-shelf-muted">{{ related.length ? `关联卡 · ${choices.length} 张` : '当前原件' }}</p>
      <PopoverRoot v-model:open="menuOpen">
        <PopoverTrigger as-child><ShelfIconButton :icon="Ellipsis" label="关联卡更多操作" :size="15" /></PopoverTrigger>
        <PopoverPortal>
          <PopoverContent align="end" :side-offset="6" class="z-[70] w-[min(320px,calc(100vw-32px))] rounded-xl border border-shelf-line-strong bg-shelf-surface p-4 text-[11px] shadow-2xl">
            <p class="mb-3 text-shelf-muted">只调整展示关联，每张卡的封面、收藏信息和原件分别保留。</p>
            <label v-if="candidates.length" class="block">关联到另一张卡
              <select v-model="target" aria-label="选择关联卡" class="mt-2 w-full rounded-md border border-shelf-line bg-shelf-raised p-2">
                <option value="">选择卡片</option>
                <option v-for="item in candidates" :key="item.id" :value="item.id">{{ item.name }} · {{ item.creator || '未知作者' }} · {{ item.sourceFilename }}</option>
              </select>
            </label>
            <div class="mt-3 flex flex-wrap gap-2">
              <ShelfButton v-if="candidates.length" :disabled="busy || !target" @click="change('join', target)">建立关联</ShelfButton>
              <ShelfButton v-if="related.length" :disabled="busy" @click="change('split')">移出关联</ShelfButton>
            </div>
            <p v-if="error" role="alert" class="mt-2 text-red-300">{{ error }}</p>
            <p v-if="!candidates.length && !related.length" class="text-shelf-muted">收藏更多卡片后，可以在这里建立关联。</p>
          </PopoverContent>
        </PopoverPortal>
      </PopoverRoot>
    </header>
    <div v-if="related.length" ref="strip" class="shelf-scrollbar flex gap-3 overflow-x-auto px-1 pb-3 pt-1" aria-label="选择关联卡">
      <button v-for="item in choices" :key="item.id" type="button" :aria-label="`查看关联卡 ${item.sourceFilename}`" :aria-pressed="item.id === character.id" class="w-[88px] shrink-0 rounded-md text-left outline-offset-2 focus-visible:outline focus-visible:outline-2 focus-visible:outline-amber-200" @mouseenter="previewId = item.id" @mouseleave="previewId = ''" @focus="previewId = item.id" @blur="previewId = ''" @click="emit('select', item.id)">
        <span class="relative block aspect-[2/3] overflow-hidden rounded-md border bg-shelf-raised transition" :class="item.id === character.id ? 'border-amber-200 ring-2 ring-amber-200/70 ring-offset-2 ring-offset-shelf-surface' : 'border-shelf-line hover:border-shelf-line-strong'">
          <CharacterCover :src="item.avatarUrl" :name="item.name" />
          <span v-if="item.id === character.id" class="absolute bottom-1 right-1 grid size-5 place-items-center rounded-full bg-amber-200 text-black"><Check :size="13" /></span>
        </span>
        <span class="mt-2 block truncate text-[10px]" :class="item.id === character.id ? 'text-amber-200' : 'text-shelf-muted'" :title="item.sourceFilename">{{ item.sourceFilename }}</span>
        <span class="block truncate text-[10px] text-shelf-quiet">{{ item.manifest.character.characterVersion ? `版本 ${item.manifest.character.characterVersion}` : '未标注版本' }}</span>
      </button>
    </div>
    <div class="mt-2 min-h-[64px] text-[11px] leading-5" aria-live="polite">
      <p class="break-all text-shelf-text-soft">{{ preview && preview.id !== character.id ? preview.sourceFilename : character.sourceFilename }}</p>
      <p class="text-shelf-muted">{{ preview && preview.id !== character.id ? relation(preview) : '当前查看与下载的原件' }}</p>
      <p class="text-[10px] text-shelf-quiet">收藏于 {{ formatImported((preview || character).importedAt) }}<span v-if="!matchingIds.includes((preview || character).id)"> · 不符合当前书架筛选</span></p>
    </div>
    <p v-if="error && !menuOpen" role="alert" class="mt-2 text-[11px] text-red-300">{{ error }}</p>
    <div v-if="message" role="status" class="mt-2 flex items-center gap-2 text-[11px] text-shelf-muted"><span>{{ message }}</span><ShelfButton v-if="undo" :icon="Undo2" :disabled="busy" @click="change('join', undo.target)">撤销</ShelfButton></div>
  </section>
</template>
