<script setup lang="ts">
import { computed,ref } from 'vue'
import { ArrowRight, Loading } from '@element-plus/icons-vue'
import FileIcon from './FileIcon.vue'
import { api } from '../api'
import { size,entryType,sortEntries,type SortField } from '../format'
import type { Entry } from '../types'
const props=withDefaults(defineProps<{entries:Entry[];selected:string[];depth?:number;sortField?:SortField;sortDirection?:'asc'|'desc';showHidden?:boolean;directorySizes:Record<string,number>;sizeLoading:Record<string,boolean>;sizeDisabled:boolean}>(),{depth:0,sortField:'name',sortDirection:'asc',showHidden:false})
const emit=defineEmits<{select:[entry:Entry,event:MouseEvent];navigate:[path:string];error:[message:string];context:[event:MouseEvent,entry:Entry];calculateSize:[path:string]}>()
const expanded=ref<Record<string,boolean>>({}),children=ref<Record<string,Entry[]>>({}),loading=ref<Record<string,boolean>>({})
const ordered=computed(()=>sortEntries(props.entries.filter(e=>props.showHidden||!e.name.startsWith('.')),props.sortField,props.sortDirection,props.directorySizes))
async function open(entry:Entry){
 if(entry.link)return
 if(entry.dir){emit('navigate',entry.path);return}
 try{await api('OpenFile',entry.path)}catch(err){emit('error','打开文件失败：'+String(err))}
}
async function expand(e:Entry){
 expanded.value[e.path]=!expanded.value[e.path]
 if(!expanded.value[e.path]||children.value[e.path])return
 loading.value[e.path]=true
 try{children.value[e.path]=await api<Entry[]>('List',e.path)}catch(err){emit('error',String(err));expanded.value[e.path]=false}finally{loading.value[e.path]=false}
}
</script>
<template>
 <template v-for="entry in ordered" :key="entry.path">
  <tr :class="{selected:selected.includes(entry.path)}" @click="emit('select',entry,$event)" @contextmenu.prevent.stop="emit('context',$event,entry)" @dblclick="open(entry)">
   <td class="file-name" :style="{paddingLeft:`${16+depth*22}px`}">
    <button v-if="entry.dir&&!entry.link" class="icon-button disclosure" :aria-label="`${expanded[entry.path]?'收起':'展开'} ${entry.name}`" @click.stop="expand(entry)" @dblclick.stop><Loading v-if="loading[entry.path]" class="spin"/><ArrowRight v-else :class="{rotated:expanded[entry.path]}"/></button><span v-else class="disclosure-spacer"></span>
    <input type="checkbox" :checked="selected.includes(entry.path)" :disabled="entry.link" :aria-label="`选择 ${entry.name}`" @click.stop="emit('select',entry,$event)" @dblclick.stop />
    <FileIcon :name="entry.name" :dir="entry.dir"/>
    <span class="ellipsis" :title="entry.path">{{entry.name}}</span><span v-if="entry.link" class="badge neutral">链接</span>
   </td>
   <td><template v-if="entry.dir"><button v-if="sizeLoading[entry.path]||directorySizes[entry.path]===undefined" class="text-button folder-size-button" :aria-label="`计算文件夹 ${entry.name} 的大小`" :aria-busy="!!sizeLoading[entry.path]" :disabled="entry.link||sizeDisabled||sizeLoading[entry.path]" @click.stop="emit('calculateSize',entry.path)" @dblclick.stop>{{sizeLoading[entry.path]?'计算中…':'计算'}}</button><span v-else>{{size(directorySizes[entry.path]!)}}</span></template><template v-else>{{size(entry.size)}}</template></td><td>{{entryType(entry)}}</td><td>{{new Date(entry.modified).toLocaleDateString('zh-CN')}}</td>
  </tr>
  <FileRows v-if="expanded[entry.path]&&children[entry.path]" :entries="children[entry.path]" :selected="selected" :depth="depth+1" :sort-field="sortField" :sort-direction="sortDirection" :show-hidden="showHidden" :directory-sizes="directorySizes" :size-loading="sizeLoading" :size-disabled="sizeDisabled" @calculate-size="emit('calculateSize',$event)" @select="(e,event)=>emit('select',e,event)" @navigate="emit('navigate',$event)" @context="(event,entry)=>emit('context',event,entry)" @error="emit('error',$event)"/>
 </template>
</template>
