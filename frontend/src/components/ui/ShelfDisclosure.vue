<script setup lang="ts">
import { ChevronDown } from "@lucide/vue";
import { CollapsibleContent, CollapsibleRoot, CollapsibleTrigger } from "reka-ui";
import { ref } from "vue";

const props = withDefaults(defineProps<{
  title: string;
  meta?: string;
  defaultOpen?: boolean;
  compact?: boolean;
}>(), { meta: "", defaultOpen: false, compact: false });

const open = ref(props.defaultOpen);
</script>

<template>
  <CollapsibleRoot v-model:open="open" class="rounded-lg border border-shelf-line bg-white/[.018]">
    <CollapsibleTrigger as-child>
      <button
        type="button"
        class="group flex w-full items-center gap-3 rounded-lg text-left text-shelf-text-soft transition hover:bg-white/[.025] hover:text-shelf-text"
        :class="compact ? 'min-h-10 px-3 py-2' : 'min-h-11 px-3.5 py-2.5'"
      >
        <slot name="leading" />
        <span class="min-w-0 flex-1 truncate text-[11px] font-medium">{{ title }}</span>
        <span v-if="meta" class="shrink-0 text-[9px] text-shelf-quiet">{{ meta }}</span>
        <ChevronDown
          :size="15"
          :stroke-width="1.7"
          class="shrink-0 text-shelf-quiet transition-transform duration-150"
          :class="open ? 'rotate-180' : ''"
          aria-hidden="true"
        />
      </button>
    </CollapsibleTrigger>
    <CollapsibleContent class="border-t border-shelf-line px-3.5 py-3 text-[11px] leading-7 text-shelf-muted">
      <slot />
    </CollapsibleContent>
  </CollapsibleRoot>
</template>
