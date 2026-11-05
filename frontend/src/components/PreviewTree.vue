<script setup lang="ts">
import { ref } from 'vue'
import { ArrowRight } from '@element-plus/icons-vue'
import FileIcon from './FileIcon.vue'
import type { TreeNode } from '../types'
withDefaults(defineProps<{nodes:TreeNode[];depth?:number}>(),{depth:0})
const emit=defineEmits<{inspect:[node:TreeNode]}>()
const collapsed=ref<Record<string,boolean>>({})
</script>
<template>
 <template v-for="node in nodes" :key="node.key">
  <div class="tree-row" :style="{paddingLeft:`${18+depth*22}px`}" @click="emit('inspect',node)">
   <button v-if="node.dir" class="icon-button disclosure" :aria-label="`${collapsed[node.key]?'展开':'收起'} ${node.name}`" @click.stop="collapsed[node.key]=!collapsed[node.key]"><ArrowRight :class="{rotated:!collapsed[node.key]}"/></button><span v-else class="disclosure-spacer"></span>
   <FileIcon :name="node.name" :dir="node.dir"/>
   <span class="ellipsis">{{node.name}}</span>
   <span v-if="node.group&&node.group.length>1" class="badge blue">合并 {{node.group.length}} 个来源</span>
   <span v-else-if="node.group?.some(g=>!g.matched)" class="badge amber">未匹配，保留原名</span>
   <span v-else-if="node.renamed" class="badge amber">重名改名</span>
   <span v-else-if="node.existing" class="badge neutral">已有</span>
   <span v-else class="badge green">{{node.dir?'将创建':'将移入'}}</span>
  </div>
  <PreviewTree v-if="node.dir&&!collapsed[node.key]" :nodes="node.children" :depth="depth+1" @inspect="emit('inspect',$event)"/>
 </template>
</template>
