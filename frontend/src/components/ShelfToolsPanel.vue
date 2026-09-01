<script setup lang="ts">
import { Activity, FolderOpen, Power, X } from "@lucide/vue";
import type { ShelfStatus } from "@/types";
import ShelfButton from "@/components/ui/ShelfButton.vue";
import ShelfIconButton from "@/components/ui/ShelfIconButton.vue";

defineProps<{ open: boolean; status: ShelfStatus | null; busy: boolean }>();
defineEmits<{
  close: [];
  openInbox: [];
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
          <p class="mb-3 break-all font-mono text-[10px] leading-5 text-shelf-muted">{{ status?.paths.inbox || "正在读取路径…" }}</p>
          <ShelfButton :icon="FolderOpen" :disabled="busy" @click="$emit('openInbox')">{{ status?.desktop.available ? "打开收件目录" : "复制收件目录" }}</ShelfButton>
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
              <template v-else-if="status?.scanner.running">{{ status.scanner.pending ? `正在确认 ${status.scanner.pending} 个文件` : "正在监视 Inbox" }}</template>
              <template v-else>扫描器已停止</template>
            </p>
          </div>
        </section>
      </aside>
    </div>
  </Teleport>
</template>
