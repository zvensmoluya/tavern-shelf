<script setup lang="ts">
import { ref, watch } from "vue";
import { initialOf } from "@/lib/format";

const props = defineProps<{ name: string; src?: string }>();
const failed = ref(false);
watch(() => props.src, () => { failed.value = false; });
</script>

<template>
  <img v-if="src && !failed" :src="src" :alt="`${name} 的角色卡封面`" class="block size-full object-cover object-top" @error="failed = true">
  <div v-else class="cover-fallback grid size-full place-items-center" aria-hidden="true">{{ initialOf(name) }}</div>
</template>
