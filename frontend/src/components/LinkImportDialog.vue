<script setup lang="ts">
import { ref, watch } from "vue";
import { Link, LoaderCircle, X } from "@lucide/vue";
import { DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from "reka-ui";
import ShelfButton from "@/components/ui/ShelfButton.vue";
import ShelfIconButton from "@/components/ui/ShelfIconButton.vue";
import { api } from "@/lib/api";
import type { ImportResult } from "@/types";

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ "update:open": [open: boolean]; imported: [result: ImportResult] }>();
const url = ref("");
const busy = ref(false);
const error = ref("");
watch(() => props.open, open => { if (open && !busy.value) error.value = ""; });
function setOpen(open: boolean) { if (!busy.value) emit("update:open", open); }
async function submit() {
  if (busy.value || !url.value.trim()) return;
  busy.value = true;
  error.value = "";
  try {
    const result = await api.importURL(url.value.trim());
    url.value = "";
    emit("imported", result);
    emit("update:open", false);
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : "链接导入失败，请重试";
  } finally { busy.value = false; }
}
</script>

<template>
  <DialogRoot :open="open" @update:open="setOpen">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-40 bg-black/65 backdrop-blur-sm" />
      <DialogContent class="fixed left-1/2 top-1/2 z-50 max-h-[90dvh] w-[min(540px,calc(100vw-32px))] -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-2xl border border-shelf-line bg-shelf-canvas p-6 shadow-2xl" @escape-key-down="busy && $event.preventDefault()" @interact-outside="busy && $event.preventDefault()">
        <div class="flex items-center justify-between gap-4">
          <DialogTitle class="text-lg font-semibold">链接导入</DialogTitle>
          <ShelfIconButton :icon="X" label="关闭链接导入" :disabled="busy" @click="setOpen(false)" />
        </div>
        <DialogDescription class="mt-3 text-xs leading-6 text-shelf-muted">粘贴 PNG／JSON 附件链接，或 Chub、RisuRealm、AICC、Pygmalion 的角色卡地址。</DialogDescription>
        <form class="mt-5 space-y-4" @submit.prevent="submit">
          <label class="block space-y-2 text-xs text-shelf-text-soft">
            <span>角色卡链接</span>
            <textarea v-model="url" :disabled="busy" rows="4" maxlength="16384" autocomplete="off" spellcheck="false" placeholder="https://cdn.discordapp.com/attachments/…" class="block w-full resize-y rounded-lg border border-shelf-line bg-white/[.035] px-3 py-2 text-sm leading-6 outline-none focus:border-shelf-line-strong disabled:opacity-60" />
          </label>
          <p class="text-xs leading-5 text-shelf-muted">Discord 请复制完整附件链接，保留问号后的参数。下载后会保存文件，重复内容会自动去重。</p>
          <p v-if="error" role="alert" class="break-words text-xs leading-6 text-shelf-danger">{{ error }}</p>
          <div class="flex justify-end">
            <ShelfButton type="submit" :icon="busy ? LoaderCircle : Link" :disabled="busy || !url.trim()" :class="busy ? '[&_svg]:animate-spin' : ''">{{ busy ? "正在下载并收录…" : "导入到 Shelf" }}</ShelfButton>
          </div>
          <p v-if="busy" role="status" class="text-xs text-shelf-muted">正在获取文件，请稍候。</p>
        </form>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
