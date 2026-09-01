<script setup lang="ts">
import { computed } from "vue";
import {
  Braces,
  CalendarClock,
  Check,
  CodeXml,
  Download,
  FileCode2,
  FileText,
  Fingerprint,
  PackageOpen,
  Sparkles,
  Trash2,
  UserRound,
  X,
} from "@lucide/vue";
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
import StructuredDescription from "@/components/StructuredDescription.vue";
import ContentSection from "@/components/ui/ContentSection.vue";
import ExpandableText from "@/components/ui/ExpandableText.vue";
import ShelfButton from "@/components/ui/ShelfButton.vue";
import ShelfDisclosure from "@/components/ui/ShelfDisclosure.vue";
import ShelfIconButton from "@/components/ui/ShelfIconButton.vue";
import { characterTone, formatCardDate, formatImported, formatSize, initialOf, manifestOf } from "@/lib/format";
import type { Character, RegexScript } from "@/types";

const props = defineProps<{ open: boolean; character: Character | null; deleting: boolean }>();
const emit = defineEmits<{ "update:open": [open: boolean]; remove: [character: Character] }>();

const manifest = computed(() => props.character ? manifestOf(props.character) : null);
const profile = computed(() => manifest.value?.character);
const greetings = computed(() => manifest.value?.greetings);
const overview = computed(() => {
  const current = manifest.value;
  if (!current) return [];
  return [
    [current.greetings.totalCount, "开场"],
    [current.characterBook?.entryCount || 0, "世界书条目"],
    [current.regexScripts.length, "Regex"],
    [current.extensions.length, "扩展"],
  ].filter(([count]) => Number(count) > 0) as Array<[number, string]>;
});

const placementNames: Record<number, string> = {
  0: "Markdown 显示",
  1: "用户输入",
  2: "AI 输出",
  3: "Slash Command",
  5: "World Info",
  6: "Reasoning",
};

function regexFlags(script: RegexScript): string[] {
  const flags: string[] = [];
  if (script.promptOnly) flags.push("仅 Prompt");
  if (script.markdownOnly) flags.push("仅 Markdown");
  if (script.runOnEdit) flags.push("编辑时运行");
  if (script.minDepth != null || script.maxDepth != null) flags.push(`深度 ${script.minDepth ?? "−∞"} – ${script.maxDepth ?? "+∞"}`);
  script.placement.forEach(value => flags.push(placementNames[value] || `位置 ${value}`));
  return flags;
}

function close(open: boolean) {
  emit("update:open", open);
}
</script>

<template>
  <DialogRoot :open="open" @update:open="close">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-40 bg-black/80 backdrop-blur-[5px]" />
      <DialogContent
        v-if="character && manifest && profile && greetings"
        class="fixed left-1/2 top-1/2 z-50 grid h-[min(850px,calc(100vh-24px))] w-[min(1240px,calc(100vw-24px))] -translate-x-1/2 -translate-y-1/2 grid-cols-[minmax(310px,41%)_minmax(0,1fr)] overflow-hidden rounded-xl border border-shelf-line-strong bg-shelf-surface shadow-shelf-dialog max-[610px]:block max-[610px]:h-screen max-[610px]:w-screen max-[610px]:rounded-none max-[610px]:border-0"
        :class="`tone-${characterTone(character.name)}`"
      >
        <DialogTitle class="sr-only">{{ character.name }}</DialogTitle>
        <DialogDescription class="sr-only">角色卡内容详情</DialogDescription>

        <DialogClose as-child>
          <ShelfIconButton :icon="X" label="关闭角色详情" :size="17" class="absolute right-4 top-4 z-20 border-white/15 bg-black/55 text-shelf-text-soft backdrop-blur-lg hover:bg-black/75" />
        </DialogClose>

        <div class="relative min-h-0 overflow-hidden bg-shelf-raised max-[610px]:h-[48vh]">
          <img v-if="character.avatarUrl" :src="character.avatarUrl" :alt="`${character.name} 的角色卡封面`" class="block size-full object-cover object-top">
          <div v-else class="cover-fallback grid size-full place-items-center text-8xl font-light" aria-hidden="true">{{ initialOf(character.name) }}</div>
          <div class="detail-cover-shade pointer-events-none absolute inset-0 max-[610px]:bg-gradient-to-t max-[610px]:from-shelf-surface max-[610px]:to-transparent" />
        </div>

        <div data-testid="detail-scroll" class="shelf-scrollbar min-h-0 min-w-0 overflow-x-hidden overflow-y-auto overscroll-contain max-[610px]:h-[52vh]">
          <article class="w-full max-w-[760px] px-11 pb-14 pt-12 max-[860px]:px-7 max-[860px]:pb-10 max-[610px]:px-5 max-[610px]:pt-7">
            <p class="mb-2.5 text-[10px] font-semibold uppercase tracking-[.13em] text-shelf-muted">{{ character.specVersion ? `Character Card ${character.specVersion}` : "Character Card" }}</p>
            <h2 class="text-[clamp(30px,3.3vw,48px)] font-semibold leading-none tracking-[-.045em]">{{ character.name }}</h2>
            <div class="mb-6 mt-3 flex flex-wrap items-center gap-2 text-[12px] text-shelf-muted">
              <span v-if="profile.nickname">昵称 {{ profile.nickname }}</span>
              <span v-if="character.creator" class="before:mr-2 before:text-shelf-quiet before:content-['·']">by {{ character.creator }}</span>
              <span v-if="profile.characterVersion" class="before:mr-2 before:text-shelf-quiet before:content-['·']">版本 {{ profile.characterVersion }}</span>
            </div>

            <div v-if="profile.tags?.length" class="mb-6 flex flex-wrap gap-1.5">
              <span v-for="tag in profile.tags" :key="tag" class="rounded-full border border-shelf-line px-2.5 py-1 text-[10px] text-shelf-muted">{{ tag }}</span>
            </div>

            <div v-if="overview.length" class="mb-7 flex flex-wrap gap-x-6 gap-y-2">
              <div v-for="([count, label]) in overview" :key="label" class="flex items-baseline gap-1.5">
                <strong class="text-[17px] font-semibold text-shelf-text">{{ count }}</strong>
                <span class="text-[10px] text-shelf-muted">{{ label }}</span>
              </div>
            </div>

            <StructuredDescription v-if="profile.description" :text="profile.description" :character-name="character.name" />

            <ContentSection v-if="greetings.firstMessage || greetings.alternate.length || greetings.groupOnly.length" title="开场" :meta="`${greetings.totalCount} 个`">
              <ExpandableText v-if="greetings.firstMessage" :text="greetings.firstMessage" :limit="520" variant="opening" label="展开完整开场" />
              <div v-if="greetings.alternate.length || greetings.groupOnly.length" class="shelf-scrollbar mt-3 flex snap-x gap-3 overflow-x-auto pb-2">
                <article v-for="(message, index) in greetings.alternate" :key="`alt-${index}`" class="w-[310px] shrink-0 snap-start rounded-xl border border-shelf-line bg-white/[.018] p-4">
                  <p class="mb-2 text-[10px] font-semibold text-shelf-muted">Alternate {{ index + 1 }}</p>
                  <p class="line-clamp-8 whitespace-pre-wrap text-[12px] leading-[1.8] text-shelf-text-soft/90">{{ message }}</p>
                </article>
                <article v-for="(message, index) in greetings.groupOnly" :key="`group-${index}`" class="w-[310px] shrink-0 snap-start rounded-xl border border-shelf-line bg-white/[.018] p-4">
                  <p class="mb-2 text-[10px] font-semibold text-shelf-muted">群聊开场 {{ index + 1 }}</p>
                  <p class="line-clamp-8 whitespace-pre-wrap text-[12px] leading-[1.8] text-shelf-text-soft/90">{{ message }}</p>
                </article>
              </div>
            </ContentSection>

            <ContentSection v-if="manifest.characterBook" :title="manifest.characterBook.name || 'Character Book'" :meta="`${manifest.characterBook.entryCount} 条世界书`">
              <CharacterBookPanel :key="character.id" :book="manifest.characterBook" :character-name="character.name" />
            </ContentSection>

            <ContentSection v-if="profile.personality || profile.scenario" title="角色设定">
              <div class="grid grid-cols-2 gap-2.5 max-[760px]:grid-cols-1">
                <div v-if="profile.personality" class="rounded-lg border border-shelf-line bg-white/[.018] p-4">
                  <h4 class="mb-2.5 flex items-center gap-2 text-[11px] font-semibold text-shelf-text-soft"><UserRound :size="15" aria-hidden="true" />Personality</h4>
                  <p class="whitespace-pre-wrap text-[12px] leading-[1.8] text-shelf-muted">{{ profile.personality }}</p>
                </div>
                <div v-if="profile.scenario" class="rounded-lg border border-shelf-line bg-white/[.018] p-4">
                  <h4 class="mb-2.5 flex items-center gap-2 text-[11px] font-semibold text-shelf-text-soft"><Sparkles :size="15" aria-hidden="true" />Scenario</h4>
                  <p class="whitespace-pre-wrap text-[12px] leading-[1.8] text-shelf-muted">{{ profile.scenario }}</p>
                </div>
              </div>
            </ContentSection>

            <ContentSection v-if="profile.messageExample" title="对话示例">
              <ExpandableText :text="profile.messageExample" :limit="650" variant="prose" label="展开完整对话示例" />
            </ContentSection>

            <ContentSection v-if="manifest.regexScripts.length" title="Regex Scripts" :meta="`${manifest.regexScripts.length} 个`">
              <div class="grid gap-2">
                <ShelfDisclosure v-for="script in manifest.regexScripts" :key="script.name" :title="script.name" :meta="script.disabled ? '停用' : '启用'">
                  <template #leading><Braces :size="14" :class="script.disabled ? 'text-shelf-quiet' : 'text-shelf-success'" aria-hidden="true" /></template>
                  <div v-if="regexFlags(script).length" class="mb-2 flex flex-wrap gap-1.5"><span v-for="flag in regexFlags(script)" :key="flag" class="rounded border border-shelf-line px-2 py-0.5 text-[10px] text-shelf-muted">{{ flag }}</span></div>
                  <code v-if="script.findRegex" class="shelf-scrollbar block max-w-full overflow-x-auto rounded-md bg-shelf-canvas p-3 font-mono text-[11px] leading-6 text-shelf-text-soft">{{ script.findRegex }}</code>
                  <p v-if="script.replaceString" class="mt-2 whitespace-pre-wrap">替换为：{{ script.replaceString }}</p>
                </ShelfDisclosure>
              </div>
            </ContentSection>

            <ContentSection v-if="manifest.extensions.length || manifest.assets.length || manifest.interaction.hasHtml || manifest.interaction.hasJavaScript || manifest.interaction.hasInteractiveExtension" title="扩展与交互内容">
              <div class="flex flex-wrap gap-2">
                <span v-for="extension in manifest.extensions" :key="extension.name" class="inline-flex items-center gap-1.5 rounded-md bg-shelf-soft px-2.5 py-1.5 text-[10px] text-shelf-muted"><PackageOpen :size="13" aria-hidden="true" />{{ extension.name }} · {{ extension.kind }}</span>
                <span v-if="manifest.interaction.hasHtml" class="inline-flex items-center gap-1.5 rounded-md bg-red-400/10 px-2.5 py-1.5 text-[10px] text-red-200/85"><CodeXml :size="13" aria-hidden="true" />包含 HTML</span>
                <span v-if="manifest.interaction.hasJavaScript" class="inline-flex items-center gap-1.5 rounded-md bg-red-400/10 px-2.5 py-1.5 text-[10px] text-red-200/85"><FileCode2 :size="13" aria-hidden="true" />包含 JavaScript</span>
                <span v-if="manifest.interaction.hasInteractiveExtension" class="inline-flex items-center gap-1.5 rounded-md bg-red-400/10 px-2.5 py-1.5 text-[10px] text-red-200/85"><Sparkles :size="13" aria-hidden="true" />交互扩展</span>
                <span v-for="asset in manifest.assets" :key="`${asset.type}-${asset.name}`" class="inline-flex items-center gap-1.5 rounded-md bg-shelf-soft px-2.5 py-1.5 text-[10px] text-shelf-muted"><FileText :size="13" aria-hidden="true" />素材 · {{ asset.type || "other" }}{{ asset.name ? ` / ${asset.name}` : "" }}</span>
              </div>
            </ContentSection>

            <ContentSection v-if="profile.creatorNotes || Object.keys(profile.creatorNotesMultilingual || {}).length" title="创作者说明">
              <ExpandableText v-if="profile.creatorNotes" :text="profile.creatorNotes" :limit="520" variant="prose" label="展开完整创作者说明" />
              <div v-if="Object.keys(profile.creatorNotesMultilingual || {}).length" class="mt-3 grid gap-2">
                <ShelfDisclosure v-for="(text, language) in profile.creatorNotesMultilingual" :key="language" :title="`${String(language).toUpperCase()} 创作者说明`" compact><p class="whitespace-pre-wrap">{{ text }}</p></ShelfDisclosure>
              </div>
            </ContentSection>

            <ContentSection v-if="profile.systemPrompt || profile.postHistoryInstructions" title="提示词">
              <div class="grid grid-cols-2 gap-2.5 max-[760px]:grid-cols-1">
                <div v-if="profile.systemPrompt" class="rounded-lg border border-shelf-line bg-white/[.018] p-4"><h4 class="mb-2.5 text-[11px] font-semibold text-shelf-text-soft">System Prompt</h4><p class="whitespace-pre-wrap text-[12px] leading-[1.8] text-shelf-muted">{{ profile.systemPrompt }}</p></div>
                <div v-if="profile.postHistoryInstructions" class="rounded-lg border border-shelf-line bg-white/[.018] p-4"><h4 class="mb-2.5 text-[11px] font-semibold text-shelf-text-soft">Post-history Instructions</h4><p class="whitespace-pre-wrap text-[12px] leading-[1.8] text-shelf-muted">{{ profile.postHistoryInstructions }}</p></div>
              </div>
            </ContentSection>

            <div class="mt-3 border-t border-shelf-line pt-3">
              <ShelfDisclosure title="技术信息" meta="路径、哈希与原始文件" compact>
                <div class="grid grid-cols-2 gap-x-6 gap-y-4 max-[760px]:grid-cols-1">
                  <div><small class="mb-1 flex items-center gap-1.5 text-[9px] uppercase tracking-[.1em] text-shelf-quiet"><FileCode2 :size="13" aria-hidden="true" />Card spec</small><span class="font-mono text-[10px]">{{ character.spec || "legacy" }} {{ character.specVersion || "" }}</span></div>
                  <div><small class="mb-1 flex items-center gap-1.5 text-[9px] uppercase tracking-[.1em] text-shelf-quiet"><CalendarClock :size="13" aria-hidden="true" />Imported</small><span class="font-mono text-[10px]">{{ formatImported(character.importedAt) }}</span></div>
                  <div><small class="mb-1 text-[9px] uppercase tracking-[.1em] text-shelf-quiet">Source file</small><span class="block truncate font-mono text-[10px]" :title="character.sourceFilename">{{ character.sourceFilename }}</span></div>
                  <div><small class="mb-1 text-[9px] uppercase tracking-[.1em] text-shelf-quiet">Source size</small><span class="font-mono text-[10px]">{{ formatSize(character.sourceSize) }}</span></div>
                  <div><small class="mb-1 flex items-center gap-1.5 text-[9px] uppercase tracking-[.1em] text-shelf-quiet"><Fingerprint :size="13" aria-hidden="true" />SHA-256</small><span class="block truncate font-mono text-[10px]" :title="character.sourceHash">{{ character.sourceHash }}</span></div>
                  <div v-if="manifest.creationDate"><small class="mb-1 text-[9px] uppercase tracking-[.1em] text-shelf-quiet">Card created</small><span class="font-mono text-[10px]">{{ formatCardDate(manifest.creationDate) }}</span></div>
                  <div v-if="manifest.modifiedDate"><small class="mb-1 text-[9px] uppercase tracking-[.1em] text-shelf-quiet">Card modified</small><span class="font-mono text-[10px]">{{ formatCardDate(manifest.modifiedDate) }}</span></div>
                  <div v-if="manifest.sources.length"><small class="mb-1 text-[9px] uppercase tracking-[.1em] text-shelf-quiet">Card sources</small><span class="font-mono text-[10px]">{{ manifest.sources.join(" · ") }}</span></div>
                </div>
                <div class="mt-5 flex items-center gap-2 border-t border-shelf-line pt-4">
                  <a :href="`${character.sourceUrl}?download=1`" class="inline-flex h-9 items-center gap-2 rounded-lg border border-shelf-line bg-white/[.025] px-3 text-[11px] font-medium text-shelf-text-soft no-underline transition hover:border-shelf-line-strong hover:bg-white/[.05] hover:text-shelf-text"><Download :size="15" aria-hidden="true" />导出原始卡</a>
                  <ShelfButton :icon="Trash2" variant="danger" :disabled="deleting" class="ml-auto" @click="emit('remove', character)">{{ deleting ? "正在移除…" : "移至 Shelf Trash" }}</ShelfButton>
                </div>
              </ShelfDisclosure>
            </div>

            <p class="mt-5 flex items-center gap-1.5 text-[10px] leading-5 text-shelf-quiet"><Check :size="13" aria-hidden="true" />只做排版层面的确定性整理；卡片内容与脚本不会在 Shelf 中执行。</p>
          </article>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
