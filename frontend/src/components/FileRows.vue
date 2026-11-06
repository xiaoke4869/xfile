<script setup lang="ts">
import { computed,onUnmounted,ref,watch } from 'vue'
import { ArrowRight, Loading } from '@element-plus/icons-vue'
import FileIcon from './FileIcon.vue'
import { api } from '../api'
import { size,entryType,sortEntries,pathKey,type SortField } from '../format'
import type { Entry } from '../types'
const props=withDefaults(defineProps<{entries:Entry[];selected:string[];expanded:Record<string,boolean>;depth?:number;sortField?:SortField;sortDirection?:'asc'|'desc';showHidden?:boolean;directorySizes:Record<string,number>;sizeLoading:Record<string,boolean>;sizeDisabled:boolean;session?:number;dropTarget?:string;dropGroup?:boolean;dragging?:string[];revealPath?:string}>(),{depth:0,sortField:'name',sortDirection:'asc',showHidden:false,session:0})
const emit=defineEmits<{row:[entry:Entry,element:HTMLElement|null];select:[entry:Entry,event:MouseEvent];navigate:[path:string];error:[message:string];context:[event:MouseEvent,entry:Entry];calculateSize:[path:string];calculateLevel:[parent:string,entries:Entry[]];expanded:[path:string,value:boolean];listed:[path:string,entries:Entry[]]}>()
const children=ref<Record<string,Entry[]>>({}),loading=ref<Record<string,boolean>>({})
let disposed=false
onUnmounted(()=>{disposed=true})
function calculateLevel(path:string){
 if(props.expanded[path]&&children.value[path]&&props.directorySizes[path]!==undefined)emit('calculateLevel',path,children.value[path])
}
watch(()=>props.entries.map(entry=>props.directorySizes[entry.path]),()=>{for(const entry of props.entries)calculateLevel(entry.path)})
const ordered=computed(()=>sortEntries(props.entries.filter(e=>props.showHidden||!e.name.startsWith('.')),props.sortField,props.sortDirection,props.directorySizes))
watch(()=>ordered.value.filter(entry=>entry.dir&&!entry.link&&props.expanded[entry.path]),entries=>{for(const entry of entries)void loadChildren(entry)},{immediate:true})
watch(()=>props.revealPath,async path=>{
 if(!path)return
 const ancestor=ordered.value.find(entry=>entry.dir&&!entry.link&&pathKey(path).startsWith(pathKey(entry.path)+'/'))
 if(ancestor&&!props.expanded[ancestor.path])expand(ancestor)
},{immediate:true})
async function open(entry:Entry){
 if(entry.link)return
 if(entry.dir){emit('navigate',entry.path);return}
 try{await api('OpenFile',entry.path)}catch(err){emit('error','打开文件失败：'+String(err))}
}
function expand(e:Entry){emit('expanded',e.path,!props.expanded[e.path])}
async function loadChildren(e:Entry){
 if(children.value[e.path]){calculateLevel(e.path);return}
 if(loading.value[e.path])return
 const session=props.session
 loading.value[e.path]=true
 try{
  const result=await api<Entry[]>('List',e.path)
  if(disposed||session!==props.session)return
  emit('listed',e.path,result)
  children.value[e.path]=result
  calculateLevel(e.path)
 }catch(err){if(!disposed&&session===props.session){emit('error',String(err));emit('expanded',e.path,false);emit('listed',e.path,[])}}
 finally{
  loading.value[e.path]=false
  if(!disposed&&session!==props.session&&props.expanded[e.path])void loadChildren(e)
 }
}
</script>
<template>
 <template v-for="entry in ordered" :key="entry.path">
  <tr :ref="el=>emit('row',entry,el as HTMLElement|null)" :class="{selected:selected.includes(entry.path),'drag-source':dragging?.includes(entry.path),'drop-folder':dropTarget===entry.path&&!dropGroup,'drop-group':dropTarget===entry.path&&dropGroup}" @click="emit('select',entry,$event)" @contextmenu.prevent.stop="emit('context',$event,entry)" @dblclick="open(entry)">
   <td class="file-name" :style="{paddingLeft:`${16+depth*22}px`}">
    <button v-if="entry.dir&&!entry.link" class="icon-button disclosure" :aria-label="`${expanded[entry.path]?'收起':'展开'} ${entry.name}`" @click.stop="expand(entry)" @dblclick.stop><Loading v-if="loading[entry.path]" class="spin"/><ArrowRight v-else :class="{rotated:expanded[entry.path]}"/></button><span v-else class="disclosure-spacer"></span>
    <input type="checkbox" :checked="selected.includes(entry.path)" :disabled="entry.link" :aria-label="`选择 ${entry.name}`" @click.stop="emit('select',entry,$event)" @dblclick.stop />
    <span class="file-entry-target"><span class="file-drag-icon"><FileIcon :name="entry.name" :dir="entry.dir"/></span><span class="ellipsis file-label" :title="entry.path">{{entry.name}}</span></span><span v-if="entry.link" class="badge neutral">链接</span>
   </td>
   <td><template v-if="entry.dir"><button v-if="sizeLoading[entry.path]||directorySizes[entry.path]===undefined" class="text-button folder-size-button" :aria-label="`计算文件夹 ${entry.name} 的大小`" :aria-busy="!!sizeLoading[entry.path]" :disabled="entry.link||sizeDisabled||sizeLoading[entry.path]" @click.stop="emit('calculateSize',entry.path)" @dblclick.stop>{{sizeLoading[entry.path]?'计算中…':'计算'}}</button><span v-else>{{size(directorySizes[entry.path]!)}}</span></template><template v-else>{{size(entry.size)}}</template></td><td>{{entryType(entry)}}</td><td>{{new Date(entry.modified).toLocaleDateString('zh-CN')}}</td>
  </tr>
  <FileRows v-if="expanded[entry.path]&&children[entry.path]" :entries="children[entry.path]" :selected="selected" :expanded="expanded" :depth="depth+1" :sort-field="sortField" :sort-direction="sortDirection" :show-hidden="showHidden" :directory-sizes="directorySizes" :size-loading="sizeLoading" :size-disabled="sizeDisabled" :session="session" :drop-target="dropTarget" :drop-group="dropGroup" :dragging="dragging" :reveal-path="revealPath" @row="(entry,el)=>emit('row',entry,el)" @expanded="(path,value)=>emit('expanded',path,value)" @listed="(path,entries)=>emit('listed',path,entries)" @calculate-size="emit('calculateSize',$event)" @calculate-level="(parent,entries)=>emit('calculateLevel',parent,entries)" @select="(e,event)=>emit('select',e,event)" @navigate="emit('navigate',$event)" @context="(event,entry)=>emit('context',event,entry)" @error="emit('error',$event)"/>
 </template>
</template>
