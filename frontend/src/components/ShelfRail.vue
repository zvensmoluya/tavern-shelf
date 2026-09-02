<script setup lang="ts">
import { BookOpenText, LibraryBig, Settings2, SlidersHorizontal } from "@lucide/vue";
import type { LibrarySection } from "@/types";

defineProps<{ toolsOpen: boolean; activeSection: LibrarySection }>();
defineEmits<{ toggleTools: []; selectSection: [section: LibrarySection] }>();
const brandMarkURL = `${import.meta.env.BASE_URL}brand-mark.png`;
</script>

<template>
  <aside class="sticky top-0 z-20 flex h-screen w-[76px] flex-col items-center border-r border-shelf-line bg-shelf-rail px-2 py-4 max-[610px]:fixed max-[610px]:inset-x-0 max-[610px]:bottom-0 max-[610px]:top-auto max-[610px]:h-16 max-[610px]:w-full max-[610px]:flex-row max-[610px]:border-r-0 max-[610px]:border-t max-[610px]:px-3 max-[610px]:py-2">
    <img :src="brandMarkURL" alt="Tavern Shelf" class="mb-8 size-8 rounded-lg max-[610px]:hidden">

    <nav aria-label="主导航" class="space-y-1 max-[610px]:flex max-[610px]:space-x-1 max-[610px]:space-y-0">
      <button
        type="button"
        class="flex w-[60px] flex-col items-center gap-1.5 rounded-lg px-2 py-2.5 transition"
        :class="activeSection === 'characters' ? 'bg-shelf-soft text-shelf-text' : 'text-shelf-muted hover:bg-white/[.045] hover:text-shelf-text'"
        :aria-current="activeSection === 'characters' ? 'page' : undefined"
        @click="$emit('selectSection', 'characters')"
      >
        <LibraryBig :size="18" :stroke-width="1.7" aria-hidden="true" />
        <span class="text-[9px] font-medium">角色</span>
      </button>
      <button
        type="button"
        class="flex w-[60px] flex-col items-center gap-1.5 rounded-lg px-2 py-2.5 transition"
        :class="activeSection === 'worldbooks' ? 'bg-shelf-soft text-shelf-text' : 'text-shelf-muted hover:bg-white/[.045] hover:text-shelf-text'"
        :aria-current="activeSection === 'worldbooks' ? 'page' : undefined"
        @click="$emit('selectSection', 'worldbooks')"
      >
        <BookOpenText :size="18" :stroke-width="1.7" aria-hidden="true" />
        <span class="text-[9px] font-medium">世界书</span>
      </button>
      <button
        type="button"
        class="flex w-[60px] flex-col items-center gap-1.5 rounded-lg px-2 py-2.5 transition"
        :class="activeSection === 'presets' ? 'bg-shelf-soft text-shelf-text' : 'text-shelf-muted hover:bg-white/[.045] hover:text-shelf-text'"
        :aria-current="activeSection === 'presets' ? 'page' : undefined"
        @click="$emit('selectSection', 'presets')"
      >
        <SlidersHorizontal :size="18" :stroke-width="1.7" aria-hidden="true" />
        <span class="text-[9px] font-medium">预设</span>
      </button>
    </nav>

    <div class="flex-1" />
    <button
      type="button"
      class="flex w-[60px] flex-col items-center gap-1.5 rounded-lg px-2 py-2.5 text-shelf-muted transition hover:bg-white/[.045] hover:text-shelf-text"
      :class="toolsOpen ? 'bg-shelf-soft text-shelf-text' : ''"
      :aria-expanded="toolsOpen"
      @click="$emit('toggleTools')"
    >
      <Settings2 :size="18" :stroke-width="1.7" aria-hidden="true" />
      <span class="text-[9px] font-medium">工具</span>
    </button>
  </aside>
</template>
