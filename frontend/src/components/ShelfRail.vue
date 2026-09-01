<script setup lang="ts">
import { LibraryBig, SlidersHorizontal } from "@lucide/vue";

defineProps<{ toolsOpen: boolean }>();
defineEmits<{ toggleTools: [] }>();
const brandMarkURL = `${import.meta.env.BASE_URL}brand-mark.svg`;
</script>

<template>
  <aside class="sticky top-0 z-20 flex h-screen w-[76px] flex-col items-center border-r border-shelf-line bg-shelf-rail px-2 py-4 max-[610px]:fixed max-[610px]:inset-x-0 max-[610px]:bottom-0 max-[610px]:top-auto max-[610px]:h-16 max-[610px]:w-full max-[610px]:flex-row max-[610px]:border-r-0 max-[610px]:border-t max-[610px]:px-3 max-[610px]:py-2">
    <img :src="brandMarkURL" alt="Tavern Shelf" class="mb-8 size-8 rounded-lg max-[610px]:hidden">

    <nav aria-label="主导航" class="max-[610px]:flex">
      <a
        href="#/"
        aria-current="page"
        class="flex w-[60px] flex-col items-center gap-1.5 rounded-lg bg-shelf-soft px-2 py-2.5 text-shelf-text no-underline"
      >
        <LibraryBig :size="18" :stroke-width="1.7" aria-hidden="true" />
        <span class="text-[9px] font-medium">Library</span>
      </a>
    </nav>

    <div class="flex-1" />
    <button
      type="button"
      class="flex w-[60px] flex-col items-center gap-1.5 rounded-lg px-2 py-2.5 text-shelf-muted transition hover:bg-white/[.045] hover:text-shelf-text"
      :class="toolsOpen ? 'bg-shelf-soft text-shelf-text' : ''"
      :aria-expanded="toolsOpen"
      @click="$emit('toggleTools')"
    >
      <SlidersHorizontal :size="18" :stroke-width="1.7" aria-hidden="true" />
      <span class="text-[9px] font-medium">工具</span>
    </button>
  </aside>
</template>
