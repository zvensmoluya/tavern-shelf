<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { Check, Clipboard, Clock3, LoaderCircle, QrCode, Radio, ShieldCheck, Smartphone, X } from "@lucide/vue";
import QRCode from "qrcode";
import {
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
} from "reka-ui";
import ShelfButton from "@/components/ui/ShelfButton.vue";
import ShelfIconButton from "@/components/ui/ShelfIconButton.vue";
import { api } from "@/lib/api";
import type { TransferSession, TransferTarget } from "@/types";

const props = defineProps<{ open: boolean; target: TransferTarget | null }>();
const emit = defineEmits<{ "update:open": [open: boolean] }>();

const session = ref<TransferSession | null>(null);
const selectedURL = ref("");
const qrDataURL = ref("");
const loading = ref(false);
const stopping = ref(false);
const copied = ref(false);
const error = ref("");
const now = ref(Date.now());
let generation = 0;
let timer: ReturnType<typeof setInterval> | null = null;

const secondsRemaining = computed(() => session.value
  ? Math.max(0, Math.ceil((new Date(session.value.expiresAt).getTime() - now.value) / 1000))
  : 0);
const timeRemaining = computed(() => {
  const minutes = Math.floor(secondsRemaining.value / 60);
  const seconds = secondsRemaining.value % 60;
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
});

watch(() => [props.open, props.target?.kind, props.target?.id] as const, async ([open]) => {
  if (!open || !props.target) return;
  const current = ++generation;
  session.value = null;
  qrDataURL.value = "";
  error.value = "";
  loading.value = true;
  try {
    const created = await api.createTransfer(props.target);
    if (current !== generation || !props.open) {
      void api.revokeTransfer(created.id).catch(() => undefined);
      return;
    }
    session.value = created;
    selectedURL.value = created.url;
    now.value = Date.now();
  } catch (reason) {
    if (current === generation) error.value = reason instanceof Error ? reason.message : "无法创建传输会话";
  } finally {
    if (current === generation) loading.value = false;
  }
}, { immediate: true });

watch(selectedURL, async value => {
  qrDataURL.value = "";
  if (!value) return;
  try {
    const generated = await QRCode.toDataURL(value, {
      width: 300,
      margin: 2,
      errorCorrectionLevel: "M",
      color: { dark: "#0d0f12", light: "#ffffff" },
    });
    if (selectedURL.value === value) qrDataURL.value = generated;
  } catch {
    error.value = "二维码生成失败";
  }
});

watch(() => props.open, open => {
  if (open && !timer) timer = setInterval(() => { now.value = Date.now(); }, 1000);
  if (!open && timer) {
    clearInterval(timer);
    timer = null;
  }
}, { immediate: true });

async function copyURL() {
  if (!selectedURL.value) return;
  try {
    await navigator.clipboard.writeText(selectedURL.value);
    copied.value = true;
    setTimeout(() => { copied.value = false; }, 1600);
  } catch {
    error.value = "无法复制链接";
  }
}

function addressHost(address: string) {
  return new URL(address).host;
}

async function stop() {
  generation++;
  stopping.value = true;
  try {
    if (session.value) await api.revokeTransfer(session.value.id);
  } catch {
    // Expiration and shutdown already make the URL unusable.
  } finally {
    stopping.value = false;
    session.value = null;
    emit("update:open", false);
  }
}

onBeforeUnmount(() => {
  generation++;
  if (timer) clearInterval(timer);
  if (session.value) void api.revokeTransfer(session.value.id).catch(() => undefined);
});
</script>

<template>
  <DialogRoot :open="open" @update:open="value => { if (!value) void stop(); }">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-[90] bg-black/85 backdrop-blur-[6px]" />
      <DialogContent class="fixed left-1/2 top-1/2 z-[100] w-[min(780px,calc(100vw-24px))] -translate-x-1/2 -translate-y-1/2 overflow-hidden rounded-2xl border border-shelf-line-strong bg-shelf-surface shadow-shelf-dialog max-[680px]:max-h-[calc(100vh-24px)] max-[680px]:overflow-y-auto">
        <DialogTitle class="sr-only">二维码传输 {{ target?.name }}</DialogTitle>
        <DialogDescription class="sr-only">通过局域网将 Shelf 原始资源传输到扫码设备</DialogDescription>
        <ShelfIconButton :icon="X" label="停止并关闭" :size="17" class="absolute right-4 top-4 z-10 border-white/15 bg-black/25" @click="stop" />

        <header class="border-b border-shelf-line bg-shelf-raised px-7 py-6 pr-16">
          <div class="flex items-center gap-3">
            <span class="grid size-10 place-items-center rounded-xl border border-shelf-line-strong bg-shelf-soft text-shelf-text-soft"><QrCode :size="20" /></span>
            <div class="min-w-0">
              <p class="text-[9px] font-semibold uppercase tracking-[.13em] text-shelf-quiet">LAN Transfer · Protocol v1</p>
              <h2 class="mt-1 truncate text-[20px] font-semibold text-shelf-text">传输“{{ target?.name }}”</h2>
            </div>
          </div>
        </header>

        <div v-if="loading" class="grid min-h-[430px] place-items-center">
          <div class="text-center text-shelf-muted"><LoaderCircle class="mx-auto mb-3 animate-spin" :size="28" /><p class="text-[11px]">正在启动局域网传输服务…</p></div>
        </div>

        <div v-else-if="error && !session" class="grid min-h-[360px] place-items-center px-8 text-center">
          <div><Radio class="mx-auto mb-4 text-shelf-danger" :size="32" /><p class="text-sm text-shelf-text-soft">无法开始传输</p><p class="mt-2 max-w-md text-[11px] leading-6 text-shelf-muted">{{ error }}</p><ShelfButton class="mt-5" @click="emit('update:open', false)">关闭</ShelfButton></div>
        </div>

        <div v-else-if="session" class="grid grid-cols-[330px_minmax(0,1fr)] gap-7 p-7 max-[680px]:grid-cols-1">
          <div class="rounded-2xl bg-white p-4 shadow-[0_18px_50px_rgba(0,0,0,.35)]">
            <img v-if="qrDataURL" :src="qrDataURL" class="aspect-square w-full" alt="Tavern Shelf 传输二维码" />
            <div v-else class="grid aspect-square place-items-center text-[#1b1f25]"><LoaderCircle class="animate-spin" :size="28" /></div>
          </div>

          <div class="flex min-w-0 flex-col">
            <div class="rounded-xl border border-shelf-line bg-white/[.02] p-4">
              <p class="flex items-center gap-2 text-[11px] font-medium text-shelf-text-soft"><Smartphone :size="15" />在 Tavern Player 中扫描</p>
              <ol class="mt-3 space-y-2 text-[10px] leading-5 text-shelf-muted">
                <li>1. 两台设备连接同一个局域网</li>
                <li>2. 扫描二维码并读取资源描述</li>
                <li>3. 校验 SHA-256 后导入原始文件</li>
              </ol>
            </div>

            <div class="mt-4">
              <div class="mb-2 flex items-center justify-between"><span class="text-[9px] uppercase tracking-[.1em] text-shelf-quiet">会话地址</span><span class="flex items-center gap-1 font-mono text-[10px] text-shelf-success"><Clock3 :size="12" />{{ timeRemaining }}</span></div>
              <select v-if="session.addresses.length > 1" v-model="selectedURL" class="mb-2 h-9 w-full rounded-lg border border-shelf-line bg-shelf-canvas px-2.5 font-mono text-[10px] text-shelf-text-soft outline-none focus:border-shelf-line-strong">
                <option v-for="address in session.addresses" :key="address" :value="address">{{ addressHost(address) }}</option>
              </select>
              <button type="button" class="flex w-full items-center gap-2 rounded-lg border border-shelf-line bg-shelf-canvas px-3 py-2 text-left hover:border-shelf-line-strong" @click="copyURL">
                <span class="min-w-0 flex-1 truncate font-mono text-[9px] text-shelf-muted">{{ selectedURL }}</span>
                <Check v-if="copied" :size="14" class="shrink-0 text-shelf-success" /><Clipboard v-else :size="14" class="shrink-0 text-shelf-muted" />
              </button>
              <p class="mt-2 text-[9px] leading-5 text-shelf-quiet">若扫码后无法连接，请允许 Tavern Shelf 访问 Windows 专用网络；多网卡电脑可切换上方地址后重试。</p>
            </div>

            <p v-if="error" class="mt-3 text-[10px] text-shelf-danger">{{ error }}</p>
            <div class="mt-auto pt-5">
              <p class="mb-3 flex gap-2 text-[9px] leading-5 text-shelf-quiet"><ShieldCheck :size="14" class="mt-0.5 shrink-0" />随机令牌只允许读取这一份资源；不会开放 Library、设置或本机路径。关闭窗口会立即停止本次分享。</p>
              <ShelfButton variant="danger" class="w-full" :disabled="stopping" @click="stop">{{ stopping ? "正在停止…" : "停止分享" }}</ShelfButton>
            </div>
          </div>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
