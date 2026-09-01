<script setup lang="ts">
import { FolderOpen } from "@lucide/vue";
import ShelfButton from "@/components/ui/ShelfButton.vue";

defineEmits<{ openInbox: [] }>();
</script>

<template>
  <div class="grid min-h-[calc(100vh-210px)] place-items-center text-center">
    <div>
      <div class="mx-auto grid h-36 w-28 place-items-center rounded-lg border border-shelf-line-strong bg-gradient-to-br from-shelf-soft to-shelf-surface shadow-[17px_11px_0_-1px_#0d0f12,17px_11px_0_#292e36]">
        <FolderOpen :size="28" :stroke-width="1.35" class="text-shelf-quiet" aria-hidden="true" />
      </div>
      <h2 class="mb-2 mt-8 text-lg font-semibold tracking-tight">Library 还没有角色</h2>
      <p class="mx-auto mb-5 max-w-[410px] text-[12px] leading-7 text-shelf-muted">把 SillyTavern 的 PNG 或 JSON 角色卡放进 Inbox，Shelf 会在文件稳定后自动收录。</p>
      <ShelfButton :icon="FolderOpen" @click="$emit('openInbox')">打开 Inbox</ShelfButton>
    </div>
  </div>
</template>
