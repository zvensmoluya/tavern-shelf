<script setup lang="ts">
import { Activity, FolderMinus, FolderOpen, FolderPlus, Power, X } from "@lucide/vue";
import type { ShelfStatus } from "@/types";
import ShelfButton from "@/components/ui/ShelfButton.vue";
import ShelfIconButton from "@/components/ui/ShelfIconButton.vue";

defineProps<{ open: boolean; status: ShelfStatus | null; busy: boolean }>();
defineEmits<{
  close: [];
  openInbox: [path: string];
  addInbox: [];
  removeInbox: [path: string];
  setAutoStart: [enabled: boolean];
}>();
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-30" @keydown.esc="$emit('close')">
      <button type="button" class="absolute inset-0 cursor-default bg-transparent" aria-label="点击外部关闭工具面板" @click="$emit('close')" />
      <aside class="absolute bottom-4 left-[86px] z-10 w-[min(340px,calc(100vw-102px))] overflow-hidden rounded-xl border border-shelf-line-strong bg-shelf-raised/95 shadow-shelf-dialog backdrop-blur-xl max-[610px]:bottom-[74px] max-[610px]:left-3 max-[610px]:right-3 max-[610px]:w-auto" aria-label="Shelf 工具">
        <header class="flex h-12 items-center justify-between border-b border-shelf-line px-4">
          <span class="text-[12px] font-semibold text-shelf-text-soft">Shelf 工具</span>
          <ShelfIconButton :icon="X" label="关闭工具面板" :size="15" class="size-8 border-transparent bg-transparent" @click="$emit('close')" />
        </header>

        <section class="border-b border-shelf-line p-4">
          <div class="mb-2 flex items-center gap-2 text-shelf-muted">
            <FolderOpen :size="15" :stroke-width="1.7" aria-hidden="true" />
            <span class="text-[9px] font-semibold tracking-[.12em]">INBOX</span>
          </div>
          <div v-if="status" class="mb-3 space-y-2">
            <div v-for="path in status.paths.inboxes" :key="path" class="flex items-center gap-1.5 rounded-lg border border-shelf-line bg-black/10 px-2 py-1.5">
              <button type="button" class="min-w-0 flex-1 truncate text-left font-mono text-[9px] leading-5 text-shelf-muted hover:text-shelf-text-soft" :title="path" :disabled="busy" @click="$emit('openInbox', path)">{{ path }}</button>
              <ShelfIconButton :icon="FolderOpen" :label="status.desktop.available ? '打开目录' : '复制路径'" :size="14" class="size-7 shrink-0 border-transparent bg-transparent" :disabled="busy" @click="$emit('openInbox', path)" />
              <ShelfIconButton :icon="FolderMinus" label="停止扫描此目录" :size="13" class="size-7 shrink-0 border-transparent bg-transparent text-shelf-muted hover:text-shelf-danger" :disabled="busy || status.paths.inboxes.length <= 1" @click="$emit('removeInbox', path)" />
            </div>
          </div>
          <p v-else class="mb-3 text-[10px] text-shelf-muted">正在读取路径…</p>
          <ShelfButton :icon="FolderPlus" :disabled="busy || !status" @click="$emit('addInbox')">添加扫描目录</ShelfButton>
        </section>

        <section v-if="status?.desktop.available" class="border-b border-shelf-line p-4">
          <label class="flex cursor-pointer items-center justify-between gap-4">
            <span class="flex items-center gap-2 text-[11px] text-shelf-text-soft"><Power :size="15" :stroke-width="1.7" aria-hidden="true" />登录后静默启动</span>
            <input
              type="checkbox"
              :checked="status.desktop.autoStart"
              :disabled="busy"
              class="size-4 accent-shelf-muted"
              @change="$emit('setAutoStart', ($event.target as HTMLInputElement).checked)"
            >
          </label>
        </section>

        <section class="flex items-start gap-2.5 p-4">
          <Activity :size="15" :stroke-width="1.7" class="mt-0.5" :class="status?.scanner.lastError ? 'text-shelf-danger' : 'text-shelf-success'" aria-hidden="true" />
          <div>
            <div class="text-[9px] font-semibold tracking-[.12em] text-shelf-muted">SCANNER</div>
            <p class="mt-1 text-[10px] leading-5 text-shelf-muted">
              <template v-if="status?.scanner.lastError">{{ status.scanner.lastErrorFile }} 暂未收录</template>
              <template v-else-if="status?.scanner.running">{{ status.scanner.pending ? `正在确认 ${status.scanner.pending} 个文件` : `正在监视 ${status.paths.inboxes.length} 个目录` }}</template>
              <template v-else>扫描器已停止</template>
            </p>
          </div>
        </section>
      </aside>
    </div>
  </Teleport>
</template>
