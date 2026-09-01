<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { AlertCircle, BookOpenText, LoaderCircle, SearchX, SlidersHorizontal } from "@lucide/vue";
import CharacterDetailDialog from "@/components/CharacterDetailDialog.vue";
import EmptyLibrary from "@/components/EmptyLibrary.vue";
import LibraryCard from "@/components/LibraryCard.vue";
import LibraryHeader from "@/components/LibraryHeader.vue";
import ResourceCard from "@/components/ResourceCard.vue";
import ResourceDetailDialog from "@/components/ResourceDetailDialog.vue";
import ShelfRail from "@/components/ShelfRail.vue";
import ShelfToolsPanel from "@/components/ShelfToolsPanel.vue";
import { api } from "@/lib/api";
import type { Character, LibrarySection, ShelfResource, ShelfStatus } from "@/types";

const characters = ref<Character[]>([]);
const resources = ref<ShelfResource[]>([]);
const status = ref<ShelfStatus | null>(null);
const activeSection = ref<LibrarySection>("characters");
const query = ref("");
const loading = ref(true);
const refreshing = ref(false);
const toolsOpen = ref(false);
const selectedID = ref<string | null>(null);
const selectedResourceID = ref<string | null>(null);
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

const sectionResources = computed(() => resources.value.filter(resource =>
  activeSection.value === "worldbooks" ? resource.kind === "worldbook" : resource.kind === "preset",
));

const filteredResources = computed(() => {
  const needle = query.value.trim().toLocaleLowerCase();
  if (!needle) return sectionResources.value;
  return sectionResources.value.filter(resource =>
    `${resource.name} ${resource.description || ""} ${resource.subtype || ""} ${resource.sourceFilename}`.toLocaleLowerCase().includes(needle),
  );
});

const sectionMeta = computed(() => {
  if (activeSection.value === "worldbooks") {
    return { title: "世界书", count: sectionResources.value.length, countLabel: "本世界书", placeholder: "搜索世界书名称或文件名" };
  }
  if (activeSection.value === "presets") {
    return { title: "预设", count: sectionResources.value.length, countLabel: "个预设", placeholder: "搜索预设名称或类型" };
  }
  return { title: "角色", count: characters.value.length, countLabel: "位角色", placeholder: "搜索角色、创作者或标签" };
});

const selected = computed(() => characters.value.find(character => character.id === selectedID.value) || null);
const selectedResource = computed(() => resources.value.find(resource => resource.id === selectedResourceID.value) || null);

function showNotice(message: string, error = false, timeout = 4500) {
  if (noticeTimer) clearTimeout(noticeTimer);
  notice.value = { message, error };
  if (timeout) noticeTimer = setTimeout(() => { notice.value = null; }, timeout);
}

async function loadLibrary(quiet = false) {
  if (!quiet) refreshing.value = true;
  try {
    const [nextCharacters, nextResources] = await Promise.all([api.listCharacters(), api.listResources()]);
    characters.value = nextCharacters;
    resources.value = nextResources;
    if (selectedID.value && !selected.value) selectedID.value = null;
    if (selectedResourceID.value && !selectedResource.value) selectedResourceID.value = null;
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

async function openInbox(path?: string) {
  if (!status.value) return;
  const directory = path || status.value.paths.inboxes[0] || status.value.paths.inbox;
  toolBusy.value = true;
  try {
    if (status.value.desktop.available) {
      await api.openInbox(directory);
    } else {
      await navigator.clipboard.writeText(directory);
      showNotice("Inbox 路径已复制");
    }
  } catch (error) {
    showNotice(`无法打开 Inbox：${error instanceof Error ? error.message : "未知错误"}`, true);
  } finally {
    toolBusy.value = false;
  }
}

function selectSection(section: LibrarySection) {
  activeSection.value = section;
  query.value = "";
  toolsOpen.value = false;
  selectedID.value = null;
  selectedResourceID.value = null;
}

async function addInbox() {
  if (!status.value) return;
  toolBusy.value = true;
  try {
    let path = "";
    if (status.value.desktop.available) {
      path = (await api.chooseInbox()).path;
    } else {
      path = window.prompt("输入要自动扫描的目录绝对路径")?.trim() || "";
    }
    if (!path) return;
    await api.addInbox(path);
    await loadStatus();
    showNotice("已添加扫描目录");
  } catch (error) {
    showNotice(`无法添加目录：${error instanceof Error ? error.message : "未知错误"}`, true);
  } finally {
    toolBusy.value = false;
  }
}

async function removeInbox(path: string) {
  if (!status.value || status.value.paths.inboxes.length <= 1) return;
  if (!window.confirm(`停止扫描这个目录？\n\n${path}\n\n目录和其中尚未收录的文件不会被删除。`)) return;
  toolBusy.value = true;
  try {
    await api.removeInbox(path);
    await loadStatus();
    showNotice("已停止扫描该目录");
  } catch (error) {
    showNotice(`无法移除目录：${error instanceof Error ? error.message : "未知错误"}`, true);
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

async function removeResource(resource: ShelfResource) {
  const label = resource.kind === "worldbook" ? "世界书" : "预设";
  const confirmed = window.confirm(`确认从 ${label} Library 移除“${resource.name}”？\n\n原始文件会移入 Tavern Shelf 自己的 Trash，不会触碰其他文件。`);
  if (!confirmed) return;
  deleting.value = true;
  try {
    await api.removeResource(resource.id);
    selectedResourceID.value = null;
    await loadLibrary(true);
    showNotice(`“${resource.name}”已移至 Shelf Trash`);
  } catch (error) {
    showNotice(`删除失败：${error instanceof Error ? error.message : "原始文件仍被保留"}`, true);
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
    <ShelfRail :tools-open="toolsOpen" :active-section="activeSection" @select-section="selectSection" @toggle-tools="toolsOpen = !toolsOpen" />

    <main class="min-w-0 max-[610px]:pb-16">
      <LibraryHeader
        v-model:query="query"
        :count="sectionMeta.count"
        :refreshing="refreshing"
        :title="sectionMeta.title"
        :count-label="sectionMeta.countLabel"
        :search-placeholder="sectionMeta.placeholder"
        @refresh="loadLibrary()"
      />

      <section class="px-9 pb-16 pt-7 max-[860px]:px-5" aria-live="polite">
        <div v-if="loading" class="grid min-h-[48vh] place-content-center justify-items-center gap-3 text-[11px] text-shelf-muted">
          <LoaderCircle :size="23" class="animate-spin" aria-hidden="true" />
          <p>正在读取 Library…</p>
        </div>

        <EmptyLibrary v-else-if="activeSection === 'characters' && !characters.length" @open-inbox="openInbox" />

        <div v-else-if="activeSection !== 'characters' && !sectionResources.length" class="grid min-h-[48vh] place-content-center justify-items-center px-4 text-center">
          <div class="mb-5 grid size-14 place-items-center rounded-2xl border border-shelf-line bg-shelf-surface text-shelf-muted">
            <BookOpenText v-if="activeSection === 'worldbooks'" :size="25" :stroke-width="1.5" aria-hidden="true" />
            <SlidersHorizontal v-else :size="25" :stroke-width="1.5" aria-hidden="true" />
          </div>
          <h2 class="text-[17px] font-semibold text-shelf-text-soft">还没有{{ activeSection === "worldbooks" ? "世界书" : "预设" }}</h2>
          <p class="mt-2 max-w-[430px] text-[11px] leading-6 text-shelf-muted">把单独分发的 SillyTavern JSON {{ activeSection === "worldbooks" ? "世界书" : "预设" }}放进任一扫描目录，Shelf 会自动识别并归入这里。</p>
        </div>

        <div v-else-if="activeSection === 'characters'">
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

        <div v-else>
          <header class="mb-4 flex items-center justify-between">
            <h2 class="text-[12px] font-medium text-shelf-text-soft">{{ query.trim() ? "搜索结果" : activeSection === "worldbooks" ? "全部世界书" : "全部预设" }}</h2>
            <span class="text-[10px] text-shelf-quiet">{{ filteredResources.length }}{{ query.trim() ? ` / ${sectionResources.length}` : "" }}</span>
          </header>
          <div v-if="filteredResources.length" class="grid grid-cols-[repeat(auto-fill,minmax(250px,1fr))] gap-4 max-[680px]:grid-cols-1">
            <ResourceCard v-for="resource in filteredResources" :key="resource.id" :resource="resource" @open="selectedResourceID = $event.id" />
          </div>
          <div v-else class="grid min-h-[42vh] place-content-center justify-items-center gap-3 text-center text-shelf-muted">
            <SearchX :size="28" :stroke-width="1.4" aria-hidden="true" />
            <div><h2 class="text-[14px] font-semibold text-shelf-text-soft">没有匹配的资源</h2><p class="mt-1 text-[11px]">换一个名称、类型或文件名试试。</p></div>
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
    @add-inbox="addInbox"
    @remove-inbox="removeInbox"
    @set-auto-start="setAutoStart"
  />

  <CharacterDetailDialog
    :open="Boolean(selected)"
    :character="selected"
    :deleting="deleting"
    @update:open="open => { if (!open) selectedID = null; }"
    @remove="removeCharacter"
  />

  <ResourceDetailDialog
    :open="Boolean(selectedResource)"
    :resource="selectedResource"
    :deleting="deleting"
    @update:open="open => { if (!open) selectedResourceID = null; }"
    @remove="removeResource"
  />

  <Transition enter-active-class="transition duration-150" enter-from-class="translate-y-2 opacity-0" leave-active-class="transition duration-150" leave-to-class="translate-y-2 opacity-0">
    <div v-if="notice" role="status" class="fixed bottom-5 right-5 z-[80] flex max-w-[430px] items-center gap-2 rounded-lg border px-3.5 py-2.5 text-[11px] shadow-2xl" :class="notice.error ? 'border-red-400/25 bg-[#2a1919]/95 text-red-200' : 'border-shelf-line-strong bg-shelf-raised/95 text-shelf-text-soft'">
      <AlertCircle v-if="notice.error" :size="15" aria-hidden="true" />
      {{ notice.message }}
    </div>
  </Transition>
</template>
