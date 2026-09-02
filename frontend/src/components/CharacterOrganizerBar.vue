<script setup lang="ts">
import { FolderPlus, Pencil, Trash2 } from "@lucide/vue";
import ShelfIconButton from "@/components/ui/ShelfIconButton.vue";
import type { Collection } from "@/types";

defineProps<{
  collections: Collection[];
  activeView: string;
  sort: string;
  format: string;
  feature: string;
  tag: string;
  tags: string[];
}>();

defineEmits<{
  "update:activeView": [value: string];
  "update:sort": [value: string];
  "update:format": [value: string];
  "update:feature": [value: string];
  "update:tag": [value: string];
  createCollection: [];
  renameCollection: [];
  removeCollection: [];
}>();
</script>

<template>
  <div class="border-b border-shelf-line bg-black/[.08] px-9 py-3 max-[860px]:px-5">
    <div class="shelf-scrollbar flex items-center gap-2 overflow-x-auto pb-1">
      <button v-for="view in [{ id: 'all', name: '全部' }, { id: 'favorites', name: '收藏' }, { id: 'recent', name: '最近加入' }, { id: 'unfiled', name: '未分类' }]" :key="view.id" type="button" class="shrink-0 rounded-lg border px-3 py-2 text-[10px] transition" :class="activeView === view.id ? 'border-shelf-line-strong bg-shelf-soft text-shelf-text' : 'border-transparent text-shelf-muted hover:bg-white/[.035] hover:text-shelf-text-soft'" @click="$emit('update:activeView', view.id)">{{ view.name }}</button>
      <span v-if="collections.length" class="mx-1 h-5 w-px shrink-0 bg-shelf-line" />
      <button v-for="collection in collections" :key="collection.id" type="button" class="shrink-0 rounded-lg border px-3 py-2 text-[10px] transition" :class="activeView === collection.id ? 'border-shelf-line-strong bg-shelf-soft text-shelf-text' : 'border-transparent text-shelf-muted hover:bg-white/[.035] hover:text-shelf-text-soft'" @click="$emit('update:activeView', collection.id)">{{ collection.name }} <span class="ml-1 text-shelf-quiet">{{ collection.characterCount }}</span></button>
      <ShelfIconButton :icon="FolderPlus" label="新建收藏夹" :size="14" class="size-8 shrink-0" @click="$emit('createCollection')" />
      <template v-if="collections.some(collection => collection.id === activeView)">
        <ShelfIconButton :icon="Pencil" label="重命名当前收藏夹" :size="13" class="size-8 shrink-0" @click="$emit('renameCollection')" />
        <ShelfIconButton :icon="Trash2" label="删除当前收藏夹" :size="13" class="size-8 shrink-0 text-red-200/70" @click="$emit('removeCollection')" />
      </template>
    </div>

    <div class="mt-2 flex flex-wrap items-center gap-2">
      <select :value="sort" aria-label="角色排序" class="organizer-select" @change="$emit('update:sort', ($event.target as HTMLSelectElement).value)">
        <option value="newest">最近收录</option><option value="oldest">最早收录</option><option value="name">名称</option><option value="creator">创作者</option><option value="favorite">收藏优先</option>
      </select>
      <select :value="format" aria-label="文件格式筛选" class="organizer-select" @change="$emit('update:format', ($event.target as HTMLSelectElement).value)">
        <option value="all">全部格式</option><option value="png">PNG</option><option value="json">JSON</option>
      </select>
      <select :value="feature" aria-label="角色特性筛选" class="organizer-select" @change="$emit('update:feature', ($event.target as HTMLSelectElement).value)">
        <option value="all">全部特性</option><option value="worldbook">含世界书</option><option value="regex">含 Regex</option><option value="extensions">含扩展</option><option value="interactive">含交互内容</option>
      </select>
      <select :value="tag" aria-label="角色标签筛选" class="organizer-select max-w-[180px]" @change="$emit('update:tag', ($event.target as HTMLSelectElement).value)">
        <option value="">全部标签</option><option v-for="value in tags" :key="value" :value="value">{{ value }}</option>
      </select>
    </div>
  </div>
</template>

<style scoped>
.organizer-select {
  height: 32px;
  border: 1px solid var(--color-shelf-line);
  border-radius: 8px;
  background: color-mix(in srgb, var(--color-shelf-surface) 90%, black);
  padding: 0 28px 0 10px;
  color: var(--color-shelf-muted);
  font-size: 10px;
  outline: none;
}
.organizer-select:focus { border-color: var(--color-shelf-line-strong); color: var(--color-shelf-text); }
</style>
