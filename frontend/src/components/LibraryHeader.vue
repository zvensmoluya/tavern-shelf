<script setup lang="ts">
import { RefreshCw, Search } from "@lucide/vue";
import ShelfIconButton from "@/components/ui/ShelfIconButton.vue";

defineProps<{ count: number; query: string; refreshing: boolean; title: string; countLabel: string; searchPlaceholder: string }>();
defineEmits<{ "update:query": [value: string]; refresh: [] }>();
</script>

<template>
  <header class="sticky top-0 z-10 flex min-h-[104px] items-end justify-between gap-7 border-b border-shelf-line bg-shelf-canvas/90 px-9 pb-5 pt-6 backdrop-blur-xl max-[860px]:min-h-[118px] max-[860px]:flex-col max-[860px]:items-stretch max-[860px]:gap-4 max-[860px]:px-5 max-[860px]:py-5">
    <div class="flex items-baseline gap-3">
      <h1 class="m-0 text-[29px] font-semibold tracking-[-.035em]">{{ title }}</h1>
      <span class="text-[11px] text-shelf-muted">{{ count ? `${count} ${countLabel}` : `${title}收藏` }}</span>
    </div>

    <div class="flex items-center gap-2">
      <label class="flex h-10 w-[min(36vw,350px)] items-center gap-2.5 rounded-lg border border-shelf-line bg-white/[.035] px-3 text-shelf-muted transition focus-within:border-shelf-line-strong focus-within:bg-white/[.05] max-[860px]:w-full">
        <Search :size="17" :stroke-width="1.7" aria-hidden="true" />
        <span class="sr-only">搜索 Library</span>
        <input
          :value="query"
          type="search"
          autocomplete="off"
          :placeholder="searchPlaceholder"
          class="w-full border-0 bg-transparent p-0 text-[12px] text-shelf-text outline-none placeholder:text-shelf-quiet"
          @input="$emit('update:query', ($event.target as HTMLInputElement).value)"
        >
      </label>
      <ShelfIconButton :icon="RefreshCw" label="刷新 Library" :class="refreshing ? '[&_svg]:animate-spin' : ''" @click="$emit('refresh')" />
    </div>
  </header>
</template>
