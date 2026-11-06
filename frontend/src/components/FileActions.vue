<script setup lang="ts">
import { vScrollbars } from '../scrollbars'
import { nextTick,onUnmounted,ref,watch } from 'vue'
import { CopyDocument,Delete,FolderOpened,DocumentCopy,EditPen,Plus,Document,TopRight,Refresh } from '@element-plus/icons-vue'
import { api } from '../api'
import { base } from '../format'
import { dropSources } from '../file-drag'
import MoveDirectoryPicker from './MoveDirectoryPicker.vue'
import RenameDialog from './RenameDialog.vue'
import type { Entry,OperationResult,GroupEntriesResult } from '../types'

const props=defineProps<{selected:Entry[];directory:string;running:boolean;active:boolean;loading?:boolean;paneId?:number}>()
const emit=defineEmits<{busy:[value:boolean];refresh:[];changed:[];grouped:[directory:string,folder:string];notice:[message:string];error:[message:string];newFolder:[];info:[];navigate:[path:string]}>()
const clipboard=ref<string[]>([]),menu=ref<HTMLDivElement>(),shown=ref(false),position=ref({left:'0px',top:'0px'})
const dialog=ref<HTMLDialogElement>(),pending=ref<{kind:'move'|'delete';paths:string[];target:string}|null>(null),working=ref(false)
const directoryPicker=ref<InstanceType<typeof MoveDirectoryPicker>>()
const renameDialog=ref<InstanceType<typeof RenameDialog>>()
const background=ref(false)
let origin:HTMLElement|null=null
function close(restore=false){shown.value=false;if(restore)origin?.focus({preventScroll:true})}
async function open(event:MouseEvent,emptyArea=false){
 close()
 background.value=emptyArea
 const point={x:event.clientX,y:event.clientY}
 origin=event.target instanceof HTMLElement?event.target.closest<HTMLElement>('button,input,[tabindex]')||document.querySelector('.file-panel'):document.querySelector('.file-panel')
 await nextTick()
 if(!props.active||props.running||background.value&&!props.directory||!background.value&&!props.selected.length&&!clipboard.value.length)return
 shown.value=true
 await nextTick()
 if(!menu.value)return
 const box=menu.value.getBoundingClientRect()
 position.value={left:`${Math.max(8,Math.min(point.x,innerWidth-box.width-8))}px`,top:`${Math.max(8,Math.min(point.y,innerHeight-box.height-8))}px`}
 menu.value.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus({preventScroll:true})
}
function outside(event:PointerEvent){if(menu.value&&!menu.value.contains(event.target as Node))close()}
function dismiss(){close()}
watch(shown,value=>{
 if(value){window.addEventListener('pointerdown',outside);window.addEventListener('resize',dismiss);window.addEventListener('scroll',dismiss,true)}
 else cleanup()
})
function cleanup(){window.removeEventListener('pointerdown',outside);window.removeEventListener('resize',dismiss);window.removeEventListener('scroll',dismiss,true)}
watch(()=>[props.active,props.paneId],()=>{close();dialog.value?.close();directoryPicker.value?.cancel();renameDialog.value?.cancel()})
watch(()=>props.running,running=>{if(running)close()})
onUnmounted(cleanup)
defineExpose({open,dropEntries})
function keydown(event:KeyboardEvent){
 if(event.key==='Escape'||event.key==='Tab'){if(event.key==='Escape')event.preventDefault();close(true);return}
 const buttons=Array.from(menu.value?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')||[])
 const index=buttons.indexOf(document.activeElement as HTMLButtonElement)
 const next=event.key==='ArrowDown'?(index+1)%buttons.length:event.key==='ArrowUp'?(index+buttons.length-1)%buttons.length:event.key==='Home'?0:event.key==='End'?buttons.length-1:-1
 if(next>=0){event.preventDefault();buttons[next]?.focus()}
}
function copy(){clipboard.value=props.selected.map(e=>e.path);close(true);emit('notice',`已复制 ${clipboard.value.length} 个项目`)}
function refresh(){
 if(!background.value||!props.directory||props.running||props.loading||!props.active)return
 close(true);emit('refresh')
}
async function openSelected(){
 const entry=props.selected.length===1?props.selected[0]:undefined
 close(true)
 if(!entry||entry.link||background.value||props.running||!props.active)return
 if(entry.dir){emit('navigate',entry.path);return}
 try{await api('OpenFile',entry.path)}catch(error){emit('error','打开文件失败：'+String(error))}
}
function rename(){close(true);if(props.selected.length&&!props.running)renameDialog.value?.open(props.selected)}
async function groupSelected(){
 if(props.running||working.value||!props.active||background.value||!props.directory||props.selected.length<2||props.selected.some(entry=>entry.link))return
 await groupFiles(props.selected.map(entry=>entry.path))
}
async function dropEntries(sources:Entry[],target:Entry){
 if(props.running||working.value||props.loading||!props.active||!sources.length||target.link||sources.some(entry=>entry.link))return
 if(target.dir){
  const paths=dropSources(sources,target).map(entry=>entry.path)
  if(paths.length)await perform('move',paths,target.path)
 }
 else if(sources.every(entry=>!entry.dir))await groupFiles(sources.map(entry=>entry.path),target.path)
}
async function groupFiles(paths:string[],target=''){
 const directory=props.directory
 close(true);working.value=true;emit('notice','');emit('busy',true)
 let folder=''
 try{
  const result=target?await api<GroupEntriesResult>('GroupFiles',paths,target):await api<GroupEntriesResult>('GroupEntries',paths,directory);folder=result.folder
  const successful=result.items.filter(item=>!item.error),failed=result.items.filter(item=>item.error)
  forgetMoved(successful.map(item=>item.source))
  if(failed.length)emit('error',`${failed.length} 个项目移动失败，请在任务中心查看详情`)
  else emit('notice',`已将 ${successful.length} 个项目移入 ${base(folder)}`)
 }catch(error){emit('error',String(error))}
 finally{working.value=false;emit('busy',false);emit('grouped',directory,folder)}
}
function renamed(result:OperationResult){
 const successful=result.items.filter(i=>!i.error&&i.source!==i.target),failed=result.items.filter(i=>i.error)
 forgetMoved(successful.map(i=>i.source))
 if(successful.length)emit('notice',`已重命名 ${successful.length} 个项目`)
 if(failed.length)emit('error',failed.map(i=>`${base(i.source)}：${i.error}`).join('\n'))
 emit('changed')
}
async function prepare(kind:'move'|'delete'){
 const paths=props.selected.map(e=>e.path);close(true)
 if(!paths.length||props.running)return
 try{
  if(kind==='move'){await directoryPicker.value?.open(props.directory,paths);return}
  pending.value={kind,paths,target:''};await nextTick();dialog.value?.showModal()
 }catch(e){emit('error',String(e))}
}
async function selectMoveTarget(target:string,paths:string[]){
 if(props.running||!props.active)return
 pending.value={kind:'move',paths,target};await nextTick();dialog.value?.showModal()
}
function forgetMoved(paths:string[]){
 const normalized=(p:string)=>p.replace(/\\/g,'/').replace(/\/+$/,'')
 clipboard.value=clipboard.value.filter(p=>!paths.some(source=>normalized(p)===normalized(source)||normalized(p).startsWith(normalized(source)+'/')))
}
async function perform(kind:'copy'|'move'|'delete',paths:string[],target:string){
 if(props.running||working.value)return
 close();dialog.value?.close();working.value=true;emit('notice','');emit('busy',true)
 try{
  const result=kind==='delete'?await api<OperationResult>('DeleteEntries',paths):await api<OperationResult>('TransferEntries',paths,target,kind==='move')
  const successful=result.items.filter(i=>!i.error),failed=result.items.filter(i=>i.error)
  if(kind!=='copy')forgetMoved(successful.map(i=>i.source))
  if(successful.length)emit('notice',`${kind==='copy'?'已粘贴':kind==='move'?'已移动':'已移入废纸篓或回收站'} ${successful.length} 个项目`)
  if(failed.length)emit('error',failed.map(i=>`${base(i.source)}：${i.error}`).join('\n'))
 }catch(e){emit('error',String(e))}
 finally{working.value=false;emit('busy',false);emit('changed')}
}
function confirm(){const action=pending.value;if(action)perform(action.kind,action.paths,action.target)}
function paste(){const paths=[...clipboard.value],target=props.directory;if(paths.length&&target)perform('copy',paths,target)}
</script>

<template>
 <Teleport to="body">
  <div v-if="shown" ref="menu" class="file-context-menu" :style="position" role="menu" aria-label="文件操作" @keydown="keydown" @contextmenu.prevent>
   <div class="context-caption">{{background?'当前目录':`已选 ${selected.length} 个项目`}}</div>
   <template v-if="background">
    <button role="menuitem" @click="close(true);emit('newFolder')"><Plus/>新建文件夹</button>
    <button role="menuitem" @click="close(true);emit('info')"><Document/>查看简介</button>
    <div class="context-separator" role="separator"></div>
    <button role="menuitem" disabled><CopyDocument/>复制</button>
    <button role="menuitem" disabled><FolderOpened/>移动</button>
    <button role="menuitem" disabled><DocumentCopy/>粘贴</button>
   </template>
   <template v-else>
   <template v-if="selected.length">
    <button v-if="selected.length===1&&!selected[0]!.link" role="menuitem" @click="openSelected"><FolderOpened v-if="selected[0]!.dir"/><TopRight v-else/>{{selected[0]!.dir?'进入':'打开'}}</button>
    <button v-if="selected.length>1" role="menuitem" :disabled="!directory||working||selected.some(entry=>entry.link)" @click="groupSelected"><Plus/>用所选项目新建文件夹（{{selected.length}}个项目）</button>
    <button role="menuitem" @click="close(true);emit('info')"><Document/>显示简介</button>
    <div class="context-separator" role="separator"></div>
    <button role="menuitem" @click="copy"><CopyDocument/>复制</button>
    <button role="menuitem" @click="prepare('move')"><FolderOpened/>移动<span>选择目标目录…</span></button>
    <button role="menuitem" @click="rename"><EditPen/>重命名<span>{{selected.length>1?'批量配置…':'输入名称…'}}</span></button>
    <button class="danger-action" role="menuitem" @click="prepare('delete')"><Delete/>删除</button>
   </template>
   <template v-if="clipboard.length">
    <div v-if="selected.length" class="context-separator" role="separator"></div>
    <button role="menuitem" :disabled="!directory" @click="paste"><DocumentCopy/>粘贴<span>到当前目录 · {{clipboard.length}} 项</span></button>
   </template>
   </template>
   <div class="context-separator" role="separator"></div>
   <button role="menuitem" :disabled="!background||!directory||running||loading" @click="refresh"><Refresh/>刷新</button>
  </div>
 </Teleport>
 <MoveDirectoryPicker ref="directoryPicker" @select="selectMoveTarget"/>
 <RenameDialog ref="renameDialog" :running="running" @busy="working=$event;emit('busy',$event)" @complete="renamed"/>
 <dialog v-scrollbars ref="dialog" class="confirm-dialog" @close="pending=null">
  <template v-if="pending">
   <div class="dialog-icon"><Delete v-if="pending.kind==='delete'"/><FolderOpened v-else/></div>
   <h2>{{pending.kind==='delete'?'删除所选项目？':'移动所选项目？'}}</h2>
   <p v-if="pending.kind==='delete'">将 {{pending.paths.length}} 个项目移入系统废纸篓或回收站。</p>
   <p v-else>将 {{pending.paths.length}} 个项目移动到以下目录。同名项目自动加序号，不覆盖现有内容。</p>
   <div v-if="pending.target" class="confirm-target">{{pending.target}}</div>
   <ul v-scrollbars class="operation-paths"><li v-for="p in pending.paths" :key="p" :title="p">{{p}}</li></ul>
   <div class="dialog-actions"><button class="button" @click="dialog?.close()">取消</button><button :class="['button',pending.kind==='delete'?'danger-button':'primary']" :disabled="running||working" @click="confirm">{{pending.kind==='delete'?'确认删除':'确认移动'}}</button></div>
  </template>
 </dialog>
</template>
