<script setup lang="ts">
import { vScrollbars } from '../scrollbars'
import { computed,nextTick,onUnmounted,toRefs,watch } from 'vue'
import { ArrowLeft,ArrowRight,ArrowUp,ArrowDown,Refresh,Close,Loading,FolderOpened,Switch } from '@element-plus/icons-vue'
import FileRows from './FileRows.vue'
import { base,parent } from '../format'
import { paneEntries,type BrowserPane } from '../browser-pane'
import { useMarqueeSelection } from '../marquee-selection'
import type { DragPreview } from '../file-drag'
import type { Entry } from '../types'

const props=defineProps<{pane:BrowserPane;index:number;active:boolean;multiple:boolean;running:boolean;batchSizeLoading:boolean;dragPreview?:DragPreview}>()
const emit=defineEmits<{
 activate:[];close:[];navigate:[path:string,record?:boolean];refresh:[];browse:[];
 context:[event:MouseEvent,entry?:Entry];error:[message:string];dragStart:[event:PointerEvent];cancelDrag:[];
 scrollElement:[element:HTMLElement|undefined];row:[entry:Entry,element:HTMLElement|null];
 expanded:[path:string,value:boolean];listed:[path:string,entries:Entry[]];
 calculateSize:[path:string];calculateAll:[];calculateLevel:[path:string,entries:Entry[]];
}>()
const {sortField,sortDirection,expandedDirectories,directorySizes,sizeLoading,showHidden,browserVersion,browserLoading,revealFolder,page}=toRefs(props.pane)
const selected=computed({get:()=>props.pane.selected,set:value=>{props.pane.selected=value}})
const visible=computed(()=>paneEntries(props.pane)),pageEntries=computed(()=>visible.value.slice(props.pane.page*200,props.pane.page*200+200))
const selectedPaths=computed(()=>props.pane.selected.map(entry=>entry.path))
const sizeDirectories=computed(()=>props.pane.entries.filter(entry=>entry.dir&&!entry.link))
const sortOptions=[{value:'name',label:'名称'},{value:'size',label:'大小'},{value:'type',label:'类型'},{value:'modified',label:'修改日期'}] as const
const {scroll:selectionScroll,box:selectionBox,start:startSelection,update:updateSelection,registerRow,cancelDrag:cancelSelection}=useMarqueeSelection(selected,computed(()=>props.running||props.pane.browserLoading))
watch(selectionScroll,element=>emit('scrollElement',element),{flush:'post'})
onUnmounted(()=>emit('scrollElement',undefined))
watch(()=>[props.pane.page,props.pane.search,props.pane.showHidden,props.pane.browserVersion,props.pane.sortField,props.pane.sortDirection],cancelSelection,{flush:'sync'})
watch(()=>[props.pane.search,props.pane.showHidden],()=>{
 props.pane.page=0
 if(!props.pane.showHidden)selected.value=selected.value.filter(entry=>!entry.name.startsWith('.'))
})
function select(entry:Entry,event:MouseEvent){
 if(entry.link||props.running)return
 const index=selected.value.findIndex(item=>item.path===entry.path)
 const additive=event.metaKey||event.ctrlKey||event.shiftKey||(event.target as HTMLElement).tagName==='INPUT'
 if(additive){if(index>=0)selected.value.splice(index,1);else selected.value.push(entry)}else selected.value=[entry]
}
function selectAll(invert=false){
 if(props.running||props.pane.browserLoading)return
 const previous=new Set(selectedPaths.value)
 selected.value=visible.value.filter(entry=>!entry.link&&(!invert||!previous.has(entry.path)))
}
function changeSort(field:typeof sortOptions[number]['value']){
 props.pane.sortDirection=props.pane.sortField===field&&props.pane.sortDirection==='asc'?'desc':'asc';props.pane.sortField=field;props.pane.page=0
}
function goHistory(delta:number){
 const next=props.pane.historyIndex+delta
 if(next<0||next>=props.pane.history.length)return
 props.pane.historyIndex=next;emit('navigate',props.pane.history[next]!,false)
}
function context(event:MouseEvent,entry?:Entry){emit('activate');emit('context',event,entry)}
function contextKey(event:KeyboardEvent){
 if(event.key==='ContextMenu'||event.shiftKey&&event.key==='F10'){
  event.preventDefault();const rect=(event.currentTarget as HTMLElement).getBoundingClientRect()
  context(new MouseEvent('contextmenu',{clientX:rect.left+40,clientY:rect.top+40}),selected.value[0])
 }
}
let pendingReveal=''
watch(()=>props.pane.revealFolder,path=>{pendingReveal=path},{flush:'sync'})
function registerFileRow(entry:Entry,element:HTMLElement|null){
 registerRow(entry,element);emit('row',entry,element)
 if(element&&pendingReveal===entry.path){pendingReveal='';nextTick(()=>element.scrollIntoView({block:'nearest'}))}
}
</script>

<template>
     <section class="file-panel" :class="{'pane-active':active}" :data-pane-id="pane.id" tabindex="0" :aria-label="multiple?`分屏 ${index+1} 文件列表`:'文件列表'" @pointerdown.capture="emit('activate')" @focusin="emit('activate')" @contextmenu.prevent="context($event)" @keydown="contextKey">
      <header v-if="multiple" class="pane-header"><div class="nav-buttons"><button class="icon-button" aria-label="后退" :disabled="pane.historyIndex<=0" @click="goHistory(-1)"><ArrowLeft/></button><button class="icon-button" aria-label="前进" :disabled="pane.historyIndex>=pane.history.length-1" @click="goHistory(1)"><ArrowRight/></button><button class="icon-button" aria-label="上级目录" :disabled="!pane.path||parent(pane.path)===pane.path" @click="emit('navigate',parent(pane.path))"><ArrowUp/></button></div><div class="pane-directory"><span class="pane-path" :title="pane.path">{{base(pane.path)||'尚未选择目录'}}</span><button class="icon-button switch-directory" :aria-label="`切换分屏 ${index+1} 目录`" title="切换目录" :disabled="pane.browserLoading" @click="emit('browse')"><Switch/></button></div><span class="pane-count">{{visible.length}} 项</span><button class="icon-button" :aria-label="`刷新分屏 ${index+1}`" :disabled="pane.browserLoading" @click="emit('refresh')"><Refresh :class="{spin:pane.browserLoading}"/></button><button class="icon-button close-pane" :aria-label="`关闭分屏 ${index+1}`" title="关闭分屏" @click.stop="emit('close')"><Close/></button></header>
      <!-- Keep both headers in table flow: WebKit stretches absolute thead children to the table height. -->
      <div class="file-list-area">
      <div v-scrollbars ref="selectionScroll" class="table-scroll" :class="{'marquee-selecting':selectionBox,'file-dragging':dragPreview,'drop-current-directory':dragPreview?.area==='blank'&&dragPreview.scroll===selectionScroll}" @pointerdown="startSelection($event);emit('dragStart',$event)" @dragstart.prevent @scroll="updateSelection" @lostpointercapture="cancelSelection();emit('cancelDrag')"><table class="file-table"><colgroup><col v-for="option in sortOptions" :key="option.value"/></colgroup><thead>
       <tr v-if="selected.length"><th :colspan="sortOptions.length" class="selection-heading"><div class="selection-actions" role="group" aria-label="批量选择"><span>已选 {{pane.selected.length}} 项</span><button class="text-button" :disabled="running||pane.browserLoading" title="全选当前目录的搜索结果，包含所有分页" @click="selectAll()">全选</button><button class="text-button" :disabled="running||pane.browserLoading" title="反选当前目录的搜索结果，包含所有分页" @click="selectAll(true)">反选</button><button class="icon-button" aria-label="取消选择" title="取消选择" :disabled="running" @click="selected=[]"><Close/></button></div></th></tr>
       <tr v-else><th v-for="option in sortOptions" :key="option.value" :aria-sort="sortField===option.value?(pane.sortDirection==='asc'?'ascending':'descending'):'none'"><div class="column-heading"><button class="sort-heading" :class="{active:pane.sortField===option.value}" @click="changeSort(option.value)">{{option.label}}<ArrowUp v-if="sortField===option.value&&pane.sortDirection==='asc'"/><ArrowDown v-else-if="sortField===option.value"/></button><button v-if="option.value==='size'&&sizeDirectories.length" class="text-button" aria-label="计算当前目录全部文件夹大小" :aria-busy="batchSizeLoading" :disabled="batchSizeLoading||running||pane.browserLoading" :title="batchSizeLoading?'正在计算文件夹大小':'批量计算当前目录下全部文件夹的大小'" @click="emit('calculateAll')">计算</button></div></th></tr></thead><tbody><FileRows :key="browserVersion" :entries="pageEntries" :expanded="expandedDirectories" @expanded="(path,value)=>emit('expanded',path,value)" @listed="(path,entries)=>emit('listed',path,entries)" :sort-field="sortField" :sort-direction="sortDirection" :show-hidden="showHidden" :selected="selectedPaths" :directory-sizes="directorySizes" :size-loading="sizeLoading" :size-disabled="running||pane.browserLoading||batchSizeLoading" :session="pane.browserVersion" :drop-target="dragPreview?.area==='row'&&dragPreview.scroll===selectionScroll?dragPreview.target?.path:undefined" :drop-group="!!dragPreview?.target&&!dragPreview.target.dir" :dragging="dragPreview?.sources.map(entry=>entry.path)" :reveal-path="revealFolder" @row="registerFileRow" @calculate-level="(path,entries)=>emit('calculateLevel',path,entries)" @calculate-size="emit('calculateSize',$event)" @select="select" @navigate="emit('navigate',$event)" @context="context" @error="emit('error',$event)"/></tbody></table>
       <div v-if="selectionBox" class="selection-marquee" aria-hidden="true" :style="{left:selectionBox.left+'px',top:selectionBox.top+'px',width:selectionBox.width+'px',height:selectionBox.height+'px'}"></div>
       <div v-if="browserLoading" class="empty-state compact"><Loading class="spin"/><p>正在读取目录…</p></div>
       <div v-else-if="!visible.length" class="empty-state"><FolderOpened/><h2>{{pane.path?'此目录没有可显示的项目':'从一个目录开始'}}</h2><button v-if="!pane.path" class="button primary" @click="emit('browse')">选择本地目录</button></div>
      </div>
      <div v-if="dragPreview?.area==='blank'&&dragPreview.scroll===selectionScroll" class="directory-drop-outline" aria-hidden="true"></div>
      </div>
      <div v-if="visible.length>200" class="pagination"><button class="button small" :disabled="page===0" @click="page--">上一页</button><span>{{pane.page+1}} / {{Math.ceil(visible.length/200)}}</span><button class="button small" :disabled="(pane.page+1)*200>=visible.length" @click="page++">下一页</button></div>
     </section>
</template>
