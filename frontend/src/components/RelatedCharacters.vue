<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { Link2, Ellipsis, Undo2 } from "@lucide/vue";
import { PopoverRoot, PopoverTrigger, PopoverPortal, PopoverContent } from "reka-ui";
import CharacterCover from "@/components/CharacterCover.vue";
import ShelfButton from "@/components/ui/ShelfButton.vue";
import ShelfIconButton from "@/components/ui/ShelfIconButton.vue";
import { api } from "@/lib/api";
import { membersOf, groupID, changeSummary, contentID } from "@/lib/associations";
import { formatImported } from "@/lib/format";
import type { Character } from "@/types";

const props = defineProps<{ character: Character; characters: Character[] }>();
const emit = defineEmits<{ select: [id: string]; changed: [] }>();
const related = computed(() => membersOf(props.characters, props.character).filter(item => item.id !== props.character.id));
const candidates = computed(() => props.characters.filter(item => groupID(item) !== groupID(props.character)));
const target = ref("");
const busy = ref(false);
const menuOpen = ref(false);
const error = ref("");
const undo = ref<{ id: string; target: string } | null>(null);
const message = ref("");
watch(() => props.character.id, () => { target.value = ""; error.value = ""; message.value = ""; undo.value = null; });
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
      <h3 class="flex flex-1 items-center gap-1.5 text-[11px] text-shelf-muted"><Link2 :size="13" />{{ related.length ? `关联卡 · ${related.length}` : '暂无关联卡' }}</h3>
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
    <div v-if="related.length" class="shelf-scrollbar flex snap-x gap-3 overflow-x-auto pb-2">
      <button v-for="item in related" :key="item.id" type="button" :aria-label="`查看关联卡 ${item.sourceFilename}`" class="flex w-56 shrink-0 snap-start gap-3 rounded-lg border border-shelf-line bg-white/[.02] p-2.5 text-left transition hover:border-shelf-line-strong hover:bg-white/[.05]" @click="emit('select', item.id)">
        <div class="h-20 w-14 shrink-0 overflow-hidden rounded"><CharacterCover :src="item.avatarUrl" :name="item.name" /></div>
        <div class="min-w-0 text-[10px] leading-5">
          <p class="truncate text-[11px] text-shelf-text-soft">{{ item.name }}</p>
          <p class="truncate text-shelf-muted" :title="item.sourceFilename">{{ item.sourceFilename }}</p>
          <p class="line-clamp-2 text-shelf-muted" :title="relation(item)">{{ relation(item) }}</p>
          <p class="truncate text-shelf-quiet">收藏于 {{ formatImported(item.importedAt) }}</p>
        </div>
      </button>
    </div>
    <p v-if="error && !menuOpen" role="alert" class="mt-2 text-[11px] text-red-300">{{ error }}</p>
    <div v-if="message" role="status" class="mt-2 flex items-center gap-2 text-[11px] text-shelf-muted"><span>{{ message }}</span><ShelfButton v-if="undo" :icon="Undo2" :disabled="busy" @click="change('join', undo.target)">撤销</ShelfButton></div>
  </section>
</template>
