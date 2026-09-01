<script setup lang="ts">
import { Activity, FolderInput, FolderMinus, FolderOpen, FolderPlus, Power, X } from "@lucide/vue";
import type { ShelfStatus } from "@/types";
import ShelfButton from "@/components/ui/ShelfButton.vue";
import ShelfIconButton from "@/components/ui/ShelfIconButton.vue";

defineProps<{ open: boolean; status: ShelfStatus | null; busy: boolean }>();
defineEmits<{
  close: [];
  openInbox: [path: string];
  addInbox: [];
  scanOnce: [];
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
            <div v-for="inbox in status.paths.inboxDetails" :key="inbox.path" class="rounded-lg border border-shelf-line bg-black/10 px-2 py-1.5">
              <div class="flex items-center gap-1.5">
                <button type="button" class="min-w-0 flex-1 truncate text-left font-mono text-[9px] leading-5 text-shelf-muted hover:text-shelf-text-soft" :title="inbox.path" :disabled="busy" @click="$emit('openInbox', inbox.path)">{{ inbox.path }}</button>
                <ShelfIconButton :icon="FolderOpen" :label="status.desktop.available ? '打开目录' : '复制路径'" :size="14" class="size-7 shrink-0 border-transparent bg-transparent" :disabled="busy" @click="$emit('openInbox', inbox.path)" />
                <ShelfIconButton :icon="FolderMinus" label="停止扫描此目录" :size="13" class="size-7 shrink-0 border-transparent bg-transparent text-shelf-muted hover:text-shelf-danger" :disabled="busy || status.paths.inboxes.length <= 1" @click="$emit('removeInbox', inbox.path)" />
              </div>
              <span class="ml-0.5 inline-flex rounded px-1.5 py-0.5 text-[8px]" :class="inbox.mode === 'move' ? 'bg-amber-300/10 text-amber-200/70' : 'bg-shelf-success/10 text-shelf-success/75'">{{ inbox.mode === "move" ? "由 Shelf 接管" : "长期监视 · 保留原文件" }}</span>
            </div>
          </div>
          <p v-else class="mb-3 text-[10px] text-shelf-muted">正在读取路径…</p>
          <div class="flex flex-wrap gap-2">
            <ShelfButton :icon="FolderPlus" :disabled="busy || !status" @click="$emit('addInbox')">添加长期监视目录</ShelfButton>
            <ShelfButton :icon="FolderInput" :disabled="busy || !status || status.oneShotScan.running" @click="$emit('scanOnce')">扫描一次</ShelfButton>
          </div>
          <p class="mt-2 text-[9px] leading-5 text-shelf-quiet">默认 Inbox 中的资源由 Shelf 接管；长期监视、扫描一次和界面拖拽都只复制收藏，原文件始终保留。</p>
          <div v-if="status?.oneShotScan.id" class="mt-2 rounded-lg border border-shelf-line bg-black/10 px-2.5 py-2 text-[9px] leading-5 text-shelf-muted">
            <p class="truncate" :title="status.oneShotScan.directory">{{ status.oneShotScan.running ? "正在扫描一次" : "上次扫描完成" }} · {{ status.oneShotScan.directory }}</p>
            <p>发现 {{ status.oneShotScan.total }} · 新增 {{ status.oneShotScan.imported }} · 已有 {{ status.oneShotScan.duplicates }}<template v-if="status.oneShotScan.failed"> · 未收录 {{ status.oneShotScan.failed }}</template></p>
          </div>
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
