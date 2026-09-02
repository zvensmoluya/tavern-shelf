<script setup lang="ts">
import { ref } from "vue";
import { Activity, AlertTriangle, ArchiveRestore, Download, FolderInput, FolderMinus, FolderOpen, FolderPlus, Plug, Power, RotateCcw, Unplug, Upload, X } from "@lucide/vue";
import type { ConnectorPairing, ConnectorStatus, ShelfStatus, TrashItem } from "@/types";
import ShelfButton from "@/components/ui/ShelfButton.vue";
import ShelfIconButton from "@/components/ui/ShelfIconButton.vue";

defineProps<{ open: boolean; status: ShelfStatus | null; connectorStatus: ConnectorStatus | null; connectorPairing: ConnectorPairing | null; busy: boolean; trashItems: TrashItem[] }>();
const emit = defineEmits<{
  close: [];
  openInbox: [path: string];
  addInbox: [];
  scanOnce: [];
  removeInbox: [path: string];
  setAutoStart: [enabled: boolean];
	backup: [];
	restoreBackup: [file: File];
        restoreTrash: [item: TrashItem];
        beginConnectorPairing: [];
        revokeConnectorPairing: [];
}>();

const restoreInput = ref<HTMLInputElement | null>(null);

function chooseBackup() {
	restoreInput.value?.click();
}

function onBackupSelected(event: Event) {
	const input = event.target as HTMLInputElement;
	const file = input.files?.[0];
	if (file) emit("restoreBackup", file);
	input.value = "";
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-30" @keydown.esc="$emit('close')">
      <button type="button" class="absolute inset-0 cursor-default bg-transparent" aria-label="点击外部关闭工具面板" @click="$emit('close')" />
      <aside class="absolute bottom-4 left-[86px] z-10 flex max-h-[calc(100vh-32px)] w-[min(380px,calc(100vw-102px))] flex-col overflow-hidden rounded-xl border border-shelf-line-strong bg-shelf-raised/95 shadow-shelf-dialog backdrop-blur-xl max-[610px]:bottom-[74px] max-[610px]:left-3 max-[610px]:right-3 max-[610px]:max-h-[calc(100vh-90px)] max-[610px]:w-auto" aria-label="Shelf 工具">
        <header class="flex h-12 items-center justify-between border-b border-shelf-line px-4">
          <span class="text-[12px] font-semibold text-shelf-text-soft">Shelf 工具</span>
          <ShelfIconButton :icon="X" label="关闭工具面板" :size="15" class="size-8 border-transparent bg-transparent" @click="$emit('close')" />
        </header>

        <div class="overflow-y-auto">
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

		<section class="border-b border-shelf-line p-4">
			<div class="mb-2 flex items-center gap-2 text-shelf-muted">
				<ArchiveRestore :size="15" :stroke-width="1.7" aria-hidden="true" />
				<span class="text-[9px] font-semibold tracking-[.12em]">备份与恢复</span>
			</div>
			<div class="flex flex-wrap gap-2">
				<ShelfButton :icon="Download" :disabled="busy" @click="$emit('backup')">备份整个 Library</ShelfButton>
				<ShelfButton :icon="Upload" :disabled="busy" @click="chooseBackup">从备份恢复</ShelfButton>
			</div>
			<input ref="restoreInput" type="file" accept=".zip,application/zip" class="hidden" @change="onBackupSelected">
			<p class="mt-2 text-[9px] leading-5 text-shelf-quiet">备份包含全部受管原始资源。恢复会重新校验内容并按哈希合并，不覆盖现有收藏。</p>
		</section>

		<section class="border-b border-shelf-line p-4">
			<div class="mb-2 flex items-center justify-between gap-2 text-shelf-muted">
				<span class="flex items-center gap-2"><RotateCcw :size="15" :stroke-width="1.7" aria-hidden="true" /><span class="text-[9px] font-semibold tracking-[.12em]">TRASH</span></span>
				<span class="text-[9px] text-shelf-quiet">{{ trashItems.length }} 项</span>
			</div>
			<div v-if="trashItems.length" class="space-y-2">
				<div v-for="item in trashItems" :key="item.id" class="flex items-center gap-2 rounded-lg border border-shelf-line bg-black/10 px-2.5 py-2">
					<div class="min-w-0 flex-1">
						<p class="truncate text-[10px] text-shelf-text-soft" :title="item.name">{{ item.name }}</p>
						<p class="mt-0.5 truncate text-[8px] text-shelf-quiet">{{ item.kind === "character" ? "角色" : item.kind === "worldbook" ? "世界书" : item.kind === "preset" ? "预设" : "无法识别" }} · {{ new Date(item.deletedAt).toLocaleString() }}</p>
						<p v-if="item.error" class="mt-1 break-words text-[8px] leading-4 text-red-200/70">{{ item.error }}</p>
					</div>
					<ShelfIconButton :icon="RotateCcw" label="恢复到 Library" :size="14" class="size-8 shrink-0" :disabled="busy || Boolean(item.error)" @click="$emit('restoreTrash', item)" />
				</div>
			</div>
			<p v-else class="text-[9px] leading-5 text-shelf-quiet">Trash 是空的。移除的资源会先保存在这里。</p>
		</section>

        <section class="border-b border-shelf-line p-4">
          <div class="mb-2 flex items-center gap-2 text-shelf-muted">
            <Plug :size="15" :stroke-width="1.7" aria-hidden="true" />
            <span class="text-[9px] font-semibold tracking-[.12em]">SILLYTAVERN CONNECTOR</span>
          </div>
          <div class="rounded-lg border border-shelf-line bg-black/10 px-2.5 py-2 text-[9px] leading-5 text-shelf-muted">
            <p v-if="connectorStatus?.listening" class="text-shelf-success">本机服务已就绪 · {{ connectorStatus.address || "127.0.0.1:8787" }}</p>
            <p v-else class="text-red-200/80">连接器不可用<span v-if="connectorStatus?.listenerError"> · {{ connectorStatus.listenerError }}</span></p>
            <p v-if="connectorStatus?.paired" class="truncate">已配对 · {{ connectorStatus.client?.name || "SillyTavern" }}<span v-if="connectorStatus.client?.version"> {{ connectorStatus.client.version }}</span></p>
            <p v-else>尚未配对 SillyTavern 扩展</p>
          </div>
          <div v-if="connectorPairing" class="mt-2 rounded-lg border border-amber-300/20 bg-amber-300/[.05] px-3 py-2 text-center">
            <p class="font-mono text-[22px] tracking-[.28em] text-amber-100">{{ connectorPairing.code }}</p>
            <p class="mt-1 text-[8px] text-shelf-quiet">在扩展中输入；有效至 {{ new Date(connectorPairing.expiresAt).toLocaleTimeString() }}</p>
          </div>
          <div class="mt-2 flex flex-wrap gap-2">
            <ShelfButton :icon="Plug" :disabled="busy || !connectorStatus?.listening" @click="$emit('beginConnectorPairing')">{{ connectorStatus?.paired ? "重新配对" : "生成配对码" }}</ShelfButton>
            <ShelfButton v-if="connectorStatus?.paired" :icon="Unplug" :disabled="busy" @click="$emit('revokeConnectorPairing')">撤销配对</ShelfButton>
          </div>
          <p class="mt-2 text-[9px] leading-5 text-shelf-quiet">只允许本机 SillyTavern 访问。重新配对会立即撤销旧令牌。</p>
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
          <Activity :size="15" :stroke-width="1.7" class="mt-0.5" :class="status?.scanner.failures.length ? 'text-shelf-danger' : 'text-shelf-success'" aria-hidden="true" />
          <div>
            <div class="text-[9px] font-semibold tracking-[.12em] text-shelf-muted">SCANNER</div>
            <p class="mt-1 text-[10px] leading-5 text-shelf-muted">
              <template v-if="status?.scanner.failures.length">{{ status.scanner.failures.length }} 个文件暂未收录</template>
              <template v-else-if="status?.scanner.running">{{ status.scanner.pending ? `正在确认 ${status.scanner.pending} 个文件` : `正在监视 ${status.paths.inboxes.length} 个目录` }}</template>
              <template v-else>扫描器已停止</template>
            </p>
			<div v-if="status?.scanner.failures.length" class="mt-2 space-y-2">
				<div v-for="failure in status.scanner.failures" :key="failure.path" class="rounded-lg border border-red-400/15 bg-red-400/[.035] px-2.5 py-2">
					<p class="flex items-center gap-1.5 truncate text-[9px] text-red-200/85" :title="failure.path"><AlertTriangle :size="12" class="shrink-0" aria-hidden="true" />{{ failure.file }}</p>
					<p class="mt-1 break-words text-[8px] leading-4 text-shelf-muted">{{ failure.error }}</p>
					<p class="mt-1 text-[8px] text-shelf-quiet">将在 {{ new Date(failure.nextRetryAt).toLocaleString() }} 后重试；原文件仍保留。</p>
				</div>
			</div>
			<div v-if="status?.oneShotScan.issues?.length" class="mt-2 space-y-1.5">
				<p v-for="issue in status.oneShotScan.issues" :key="`${issue.file}-${issue.error}`" class="text-[8px] leading-4 text-shelf-muted"><span class="text-red-200/80">{{ issue.file }}</span> · {{ issue.error }}</p>
			</div>
          </div>
        </section>
		</div>
      </aside>
    </div>
  </Teleport>
</template>
