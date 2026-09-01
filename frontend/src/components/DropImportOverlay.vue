<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import { FileDown, LoaderCircle } from "@lucide/vue";

defineProps<{ importing: boolean }>();
const emit = defineEmits<{ import: [files: File[]] }>();
const active = ref(false);
let dragDepth = 0;

function hasFiles(event: DragEvent) {
  return Array.from(event.dataTransfer?.types || []).includes("Files");
}

function onDragEnter(event: DragEvent) {
  if (!hasFiles(event)) return;
  event.preventDefault();
  dragDepth++;
  active.value = true;
}

function onDragOver(event: DragEvent) {
  if (!hasFiles(event)) return;
  event.preventDefault();
  if (event.dataTransfer) event.dataTransfer.dropEffect = "copy";
}

function onDragLeave(event: DragEvent) {
  if (!hasFiles(event)) return;
  event.preventDefault();
  dragDepth = Math.max(0, dragDepth - 1);
  if (!dragDepth) active.value = false;
}

function onDrop(event: DragEvent) {
  if (!hasFiles(event)) return;
  event.preventDefault();
  dragDepth = 0;
  active.value = false;
  const files = Array.from(event.dataTransfer?.files || []);
  if (files.length) emit("import", files);
}

onMounted(() => {
  window.addEventListener("dragenter", onDragEnter);
  window.addEventListener("dragover", onDragOver);
  window.addEventListener("dragleave", onDragLeave);
  window.addEventListener("drop", onDrop);
});

onBeforeUnmount(() => {
  window.removeEventListener("dragenter", onDragEnter);
  window.removeEventListener("dragover", onDragOver);
  window.removeEventListener("dragleave", onDragLeave);
  window.removeEventListener("drop", onDrop);
});
</script>

<template>
  <Teleport to="body">
    <Transition enter-active-class="transition duration-150" enter-from-class="opacity-0" leave-active-class="transition duration-150" leave-to-class="opacity-0">
      <div v-if="active || importing" class="pointer-events-none fixed inset-3 z-[100] grid place-items-center rounded-2xl border border-dashed border-shelf-success/60 bg-shelf-canvas/90 shadow-2xl backdrop-blur-md">
        <div class="text-center">
          <LoaderCircle v-if="importing" :size="38" :stroke-width="1.4" class="mx-auto animate-spin text-shelf-success" aria-hidden="true" />
          <FileDown v-else :size="42" :stroke-width="1.35" class="mx-auto text-shelf-success" aria-hidden="true" />
          <p class="mt-4 text-[18px] font-semibold text-shelf-text-soft">{{ importing ? "正在收藏到 Shelf…" : "松开即可收藏到 Shelf" }}</p>
          <p class="mt-2 text-[11px] text-shelf-muted">支持角色卡 PNG、角色卡 / 世界书 / 预设 JSON · 原文件保持不变</p>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
