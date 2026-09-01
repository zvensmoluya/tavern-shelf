<script setup lang="ts">
import { BookOpenText, CalendarClock, Check, Download, FileJson, Fingerprint, QrCode, SlidersHorizontal, Trash2, X } from "@lucide/vue";
import {
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
} from "reka-ui";
import CharacterBookPanel from "@/components/CharacterBookPanel.vue";
import ShelfButton from "@/components/ui/ShelfButton.vue";
import ShelfIconButton from "@/components/ui/ShelfIconButton.vue";
import { formatImported, formatSize } from "@/lib/format";
import type { ShelfResource } from "@/types";

defineProps<{ open: boolean; resource: ShelfResource | null; deleting: boolean }>();
const emit = defineEmits<{ "update:open": [open: boolean]; remove: [resource: ShelfResource]; transfer: [resource: ShelfResource] }>();

const subtypeLabels: Record<string, string> = {
  openai: "Chat Completion",
  instruct: "Instruct Template",
  context: "Context Template",
  "system-prompt": "System Prompt",
  reasoning: "Reasoning Template",
  textgen: "Text Generation",
  novel: "NovelAI",
  kobold: "Kobold",
};
</script>

<template>
  <DialogRoot :open="open" @update:open="value => emit('update:open', value)">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-40 bg-black/80 backdrop-blur-[5px]" />
      <DialogContent
        v-if="resource"
        class="fixed left-1/2 top-1/2 z-50 flex h-[min(820px,calc(100vh-24px))] w-[min(940px,calc(100vw-24px))] -translate-x-1/2 -translate-y-1/2 flex-col overflow-hidden rounded-xl border border-shelf-line-strong bg-shelf-surface shadow-shelf-dialog max-[610px]:h-screen max-[610px]:w-screen max-[610px]:rounded-none max-[610px]:border-0"
      >
        <DialogTitle class="sr-only">{{ resource.name }}</DialogTitle>
        <DialogDescription class="sr-only">{{ resource.kind === "worldbook" ? "世界书详情" : "预设详情" }}</DialogDescription>
        <DialogClose as-child>
          <ShelfIconButton :icon="X" label="关闭详情" :size="17" class="absolute right-4 top-4 z-20 border-white/15 bg-black/35 text-shelf-text-soft backdrop-blur-lg hover:bg-black/60" />
        </DialogClose>

        <header class="shrink-0 border-b border-shelf-line bg-shelf-raised px-8 pb-6 pt-7 max-[610px]:px-5">
          <div class="flex items-start gap-4 pr-12">
            <span class="grid size-12 shrink-0 place-items-center rounded-xl border border-shelf-line-strong bg-shelf-soft text-shelf-text-soft">
              <BookOpenText v-if="resource.kind === 'worldbook'" :size="22" :stroke-width="1.6" aria-hidden="true" />
              <SlidersHorizontal v-else :size="22" :stroke-width="1.6" aria-hidden="true" />
            </span>
            <div class="min-w-0">
              <p class="mb-1 text-[9px] font-semibold uppercase tracking-[.13em] text-shelf-quiet">
                {{ resource.kind === "worldbook" ? "Worldbook" : subtypeLabels[resource.subtype || ""] || "Preset" }}
              </p>
              <h2 class="truncate text-[28px] font-semibold tracking-[-.035em] text-shelf-text">{{ resource.name }}</h2>
              <p v-if="resource.description" class="mt-1 text-[11px] text-shelf-muted">{{ resource.description }}</p>
            </div>
          </div>
        </header>

        <div class="shelf-scrollbar min-h-0 flex-1 overflow-y-auto px-8 py-7 max-[610px]:px-5">
          <CharacterBookPanel
            v-if="resource.kind === 'worldbook' && resource.worldbook"
            :key="resource.id"
            :book="resource.worldbook"
            character-name="Character"
          />

          <template v-else-if="resource.preset">
            <section v-if="resource.preset.fields.length">
              <div class="mb-3 flex items-center justify-between">
                <h3 class="text-[11px] font-semibold uppercase tracking-[.11em] text-shelf-muted">参数摘要</h3>
                <span class="text-[9px] text-shelf-quiet">{{ resource.preset.fieldCount }} 个原始字段</span>
              </div>
              <div class="grid grid-cols-2 gap-2.5 max-[680px]:grid-cols-1">
                <div v-for="field in resource.preset.fields" :key="field.key" class="rounded-lg border border-shelf-line bg-white/[.018] px-4 py-3">
                  <p class="text-[9px] uppercase tracking-[.08em] text-shelf-quiet">{{ field.label }}</p>
                  <p class="mt-1 break-words font-mono text-[11px] leading-5 text-shelf-text-soft">{{ field.value }}</p>
                </div>
              </div>
            </section>

            <section v-if="resource.preset.textBlocks.length" class="mt-7 space-y-4">
              <h3 class="text-[11px] font-semibold uppercase tracking-[.11em] text-shelf-muted">模板内容</h3>
              <div v-for="block in resource.preset.textBlocks" :key="block.key">
                <p class="mb-2 text-[9px] uppercase tracking-[.08em] text-shelf-quiet">{{ block.label }}</p>
                <pre class="shelf-scrollbar max-h-[280px] overflow-auto whitespace-pre-wrap rounded-lg border border-shelf-line bg-shelf-canvas p-4 font-mono text-[11px] leading-6 text-shelf-text-soft">{{ block.value }}</pre>
              </div>
            </section>
          </template>

          <section class="mt-8 border-t border-shelf-line pt-5">
            <div class="grid grid-cols-2 gap-x-7 gap-y-4 text-[10px] text-shelf-muted max-[680px]:grid-cols-1">
              <div><small class="mb-1 flex items-center gap-1.5 text-[9px] uppercase tracking-[.1em] text-shelf-quiet"><FileJson :size="13" />Source file</small><span class="block truncate font-mono" :title="resource.sourceFilename">{{ resource.sourceFilename }}</span></div>
              <div><small class="mb-1 flex items-center gap-1.5 text-[9px] uppercase tracking-[.1em] text-shelf-quiet"><CalendarClock :size="13" />Imported</small><span class="font-mono">{{ formatImported(resource.importedAt) }}</span></div>
              <div><small class="mb-1 text-[9px] uppercase tracking-[.1em] text-shelf-quiet">Source size</small><span class="font-mono">{{ formatSize(resource.sourceSize) }}</span></div>
              <div><small class="mb-1 flex items-center gap-1.5 text-[9px] uppercase tracking-[.1em] text-shelf-quiet"><Fingerprint :size="13" />SHA-256</small><span class="block truncate font-mono" :title="resource.sourceHash">{{ resource.sourceHash }}</span></div>
            </div>
            <div class="mt-5 flex items-center gap-2 border-t border-shelf-line pt-4">
              <a :href="`${resource.sourceUrl}?download=1`" class="inline-flex h-9 items-center gap-2 rounded-lg border border-shelf-line bg-white/[.025] px-3 text-[11px] font-medium text-shelf-text-soft no-underline transition hover:border-shelf-line-strong hover:bg-white/[.05]"><Download :size="15" />导出原始文件</a>
              <ShelfButton :icon="QrCode" @click="emit('transfer', resource)">二维码传输</ShelfButton>
              <ShelfButton :icon="Trash2" variant="danger" :disabled="deleting" class="ml-auto" @click="emit('remove', resource)">{{ deleting ? "正在移除…" : "移至 Shelf Trash" }}</ShelfButton>
            </div>
            <p class="mt-4 flex items-center gap-1.5 text-[10px] text-shelf-quiet"><Check :size="13" />只展示确定性字段，不执行预设或世界书内容。</p>
          </section>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
