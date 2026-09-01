<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { AlertCircle, LoaderCircle, SearchX } from "@lucide/vue";
import CharacterDetailDialog from "@/components/CharacterDetailDialog.vue";
import EmptyLibrary from "@/components/EmptyLibrary.vue";
import LibraryCard from "@/components/LibraryCard.vue";
import LibraryHeader from "@/components/LibraryHeader.vue";
import ShelfRail from "@/components/ShelfRail.vue";
import ShelfToolsPanel from "@/components/ShelfToolsPanel.vue";
import { api } from "@/lib/api";
import type { Character, ShelfStatus } from "@/types";

const characters = ref<Character[]>([]);
const status = ref<ShelfStatus | null>(null);
const query = ref("");
const loading = ref(true);
const refreshing = ref(false);
const toolsOpen = ref(false);
const selectedID = ref<string | null>(null);
const deleting = ref(false);
const toolBusy = ref(false);
const notice = ref<{ message: string; error: boolean } | null>(null);
let noticeTimer: ReturnType<typeof setTimeout> | null = null;
let pollTimer: ReturnType<typeof setInterval> | null = null;

const filteredCharacters = computed(() => {
  const needle = query.value.trim().toLocaleLowerCase();
  if (!needle) return characters.value;
  return characters.value.filter(character =>
    `${character.name} ${character.creator || ""} ${(character.tags || []).join(" ")}`.toLocaleLowerCase().includes(needle),
  );
});

const selected = computed(() => characters.value.find(character => character.id === selectedID.value) || null);

function showNotice(message: string, error = false, timeout = 4500) {
  if (noticeTimer) clearTimeout(noticeTimer);
  notice.value = { message, error };
  if (timeout) noticeTimer = setTimeout(() => { notice.value = null; }, timeout);
}

async function loadLibrary(quiet = false) {
  if (!quiet) refreshing.value = true;
  try {
    characters.value = await api.listCharacters();
    if (selectedID.value && !selected.value) selectedID.value = null;
  } catch (error) {
    showNotice(`无法读取 Library：${error instanceof Error ? error.message : "未知错误"}`, true, 0);
  } finally {
    loading.value = false;
    refreshing.value = false;
  }
}

async function loadStatus() {
  try {
    status.value = await api.status();
  } catch {
    // Polling will retry without interrupting Library browsing.
  }
}

async function openInbox() {
  if (!status.value) return;
  toolBusy.value = true;
  try {
    if (status.value.desktop.available) {
      await api.openInbox();
    } else {
      await navigator.clipboard.writeText(status.value.paths.inbox);
      showNotice("Inbox 路径已复制");
    }
  } catch (error) {
    showNotice(`无法打开 Inbox：${error instanceof Error ? error.message : "未知错误"}`, true);
  } finally {
    toolBusy.value = false;
  }
}

async function setAutoStart(enabled: boolean) {
  if (!status.value) return;
  const previous = status.value.desktop.autoStart;
  status.value.desktop.autoStart = enabled;
  toolBusy.value = true;
  try {
    await api.setAutoStart(enabled);
  } catch (error) {
    status.value.desktop.autoStart = previous;
    showNotice(`无法更新开机自启：${error instanceof Error ? error.message : "未知错误"}`, true);
  } finally {
    toolBusy.value = false;
  }
}

async function removeCharacter(character: Character) {
  const confirmed = window.confirm(`确认从 Library 移除“${character.name}”？\n\n原始卡会移入 Tavern Shelf 自己的 Trash，不会触碰其他文件。`);
  if (!confirmed) return;
  deleting.value = true;
  try {
    await api.removeCharacter(character.id);
    selectedID.value = null;
    await loadLibrary(true);
    showNotice(`“${character.name}”已移至 Shelf Trash`);
  } catch (error) {
    showNotice(`删除失败：${error instanceof Error ? error.message : "原始卡仍被保留"}`, true);
  } finally {
    deleting.value = false;
  }
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === "Escape" && toolsOpen.value) toolsOpen.value = false;
}

onMounted(() => {
  void Promise.all([loadLibrary(), loadStatus()]);
  pollTimer = setInterval(() => {
    void loadLibrary(true);
    void loadStatus();
  }, 3000);
  document.addEventListener("keydown", onKeydown);
});

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer);
  if (noticeTimer) clearTimeout(noticeTimer);
  document.removeEventListener("keydown", onKeydown);
});
</script>

<template>
  <div class="grid min-h-screen grid-cols-[76px_minmax(0,1fr)] bg-shelf-canvas max-[610px]:block">
    <ShelfRail :tools-open="toolsOpen" @toggle-tools="toolsOpen = !toolsOpen" />

    <main class="min-w-0 max-[610px]:pb-16">
      <LibraryHeader v-model:query="query" :count="characters.length" :refreshing="refreshing" @refresh="loadLibrary()" />

      <section class="px-9 pb-16 pt-7 max-[860px]:px-5" aria-live="polite">
        <div v-if="loading" class="grid min-h-[48vh] place-content-center justify-items-center gap-3 text-[11px] text-shelf-muted">
          <LoaderCircle :size="23" class="animate-spin" aria-hidden="true" />
          <p>正在读取 Library…</p>
        </div>

        <EmptyLibrary v-else-if="!characters.length" @open-inbox="openInbox" />

        <div v-else>
          <header class="mb-4 flex items-center justify-between">
            <h2 class="text-[12px] font-medium text-shelf-text-soft">{{ query.trim() ? "搜索结果" : "全部角色" }}</h2>
            <span class="text-[10px] text-shelf-quiet">{{ filteredCharacters.length }}{{ query.trim() ? ` / ${characters.length}` : "" }}</span>
          </header>

          <div v-if="filteredCharacters.length" class="grid grid-cols-[repeat(auto-fill,minmax(178px,1fr))] gap-x-5 gap-y-8 max-[860px]:grid-cols-[repeat(auto-fill,minmax(145px,1fr))] max-[860px]:gap-x-3.5 max-[860px]:gap-y-6">
            <LibraryCard v-for="character in filteredCharacters" :key="character.id" :character="character" @open="selectedID = $event.id" />
          </div>

          <div v-else class="grid min-h-[42vh] place-content-center justify-items-center gap-3 text-center text-shelf-muted">
            <SearchX :size="28" :stroke-width="1.4" aria-hidden="true" />
            <div><h2 class="text-[14px] font-semibold text-shelf-text-soft">没有匹配的角色</h2><p class="mt-1 text-[11px]">换一个角色名、创作者或标签试试。</p></div>
          </div>
        </div>
      </section>
    </main>
  </div>

  <ShelfToolsPanel
    :open="toolsOpen"
    :status="status"
    :busy="toolBusy"
    @close="toolsOpen = false"
    @open-inbox="openInbox"
    @set-auto-start="setAutoStart"
  />

  <CharacterDetailDialog
    :open="Boolean(selected)"
    :character="selected"
    :deleting="deleting"
    @update:open="open => { if (!open) selectedID = null; }"
    @remove="removeCharacter"
  />

  <Transition enter-active-class="transition duration-150" enter-from-class="translate-y-2 opacity-0" leave-active-class="transition duration-150" leave-to-class="translate-y-2 opacity-0">
    <div v-if="notice" role="status" class="fixed bottom-5 right-5 z-[80] flex max-w-[430px] items-center gap-2 rounded-lg border px-3.5 py-2.5 text-[11px] shadow-2xl" :class="notice.error ? 'border-red-400/25 bg-[#2a1919]/95 text-red-200' : 'border-shelf-line-strong bg-shelf-raised/95 text-shelf-text-soft'">
      <AlertCircle v-if="notice.error" :size="15" aria-hidden="true" />
      {{ notice.message }}
    </div>
  </Transition>
</template>
