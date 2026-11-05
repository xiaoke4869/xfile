<script setup lang="ts">
import { computed } from 'vue'
import { fileIconId, fileIconUrl } from '../file-icons'

const props = withDefaults(defineProps<{ name: string; dir?: boolean; format?: 'video' | 'video-ts' | 'private-key' }>(), { dir: false })
const source = computed(() => fileIconUrl(props.dir ? 'folder' : props.format || fileIconId(props.name)))
</script>

<template>
 <img class="file-icon" :src="source" width="24" height="24" alt="" aria-hidden="true" draggable="false" />
</template>

<style scoped>
.file-icon{display:inline-block;width:24px;height:24px;flex-shrink:0;object-fit:contain;vertical-align:middle}
</style>
