<script setup lang="ts">
import { computed,nextTick,onUnmounted,ref,useId } from 'vue'
import { ArrowLeft,ArrowRight,ArrowUp,Close,FolderOpened,House,Loading,Monitor,Warning } from '@element-plus/icons-vue'
import FileIcon from './FileIcon.vue'
import { api } from '../api'
import { base,parent,pathKey as key } from '../format'
import type { Entry } from '../types'

const props=withDefaults(defineProps<{purpose?:'move'|'favorite'|'browse'}>(),{purpose:'move'})
const titleId=useId()
const emit=defineEmits<{select:[target:string,sources:string[]]}>()
const dialog=ref<HTMLDialogElement>(),current=ref(''),pathInput=ref(''),selected=ref(''),sources=ref<string[]>([])
const entries=ref<Entry[]>([]),locations=ref<{name:string;path:string}[]>([]),showHidden=ref(false),loading=ref(false),error=ref('')
const history=ref<string[]>([]),historyIndex=ref(-1)
let revision=0,session=0
function root(p:string){return p.match(/^[A-Za-z]:[\\/]/)?.[0]||p.match(/^[\\/]{2}[^\\/]+[\\/][^\\/]+/)?.[0]||'/'}
const filesystemRoot=computed(()=>root(current.value))
const folders=computed(()=>entries.value.filter(e=>e.dir&&!e.link&&(showHidden.value||!e.name.startsWith('.'))))
const target=computed(()=>selected.value||current.value)
function withinSource(p:string){const candidate=key(p);return sources.value.some(source=>candidate===key(source)||candidate.startsWith(key(source)+'/'))}
const targetIssue=computed(()=>{
 if(!target.value)return ''
 if(withinSource(target.value))return '不能选择来源目录或其子目录'
 if(sources.value.some(source=>key(parent(source))===key(target.value)))return '有项目已在此目录，请选择其他目标'
 return ''
})
const canSelect=computed(()=>!!current.value&&!loading.value&&!error.value&&!targetIssue.value&&pathInput.value.trim()===current.value)
async function navigate(to:string,record=true){
 const id=++revision;loading.value=true;error.value='';selected.value=''
 try{
  const entry=await api<Entry>('EntryInfo',to)
  if(id!==revision||!dialog.value?.open)return false
  if(!entry.dir)throw new Error('请输入目录路径，不能跳转到文件')
  to=entry.path
  const data=await api<Entry[]>('List',to)
  if(id!==revision||!dialog.value?.open)return false
  entries.value=data;current.value=to;pathInput.value=to
  if(record){history.value=history.value.slice(0,historyIndex.value+1);history.value.push(to);historyIndex.value=history.value.length-1}
  return true
 }catch(e){if(id===revision)error.value=String(e);return false}
 finally{if(id===revision)loading.value=false}
}
async function back(){const previous=historyIndex.value-1;if(previous>=0&&await navigate(history.value[previous],false))historyIndex.value=previous}
async function open(initial:string,paths:string[]=[]){
 const activeSession=++session
 sources.value=[...paths];current.value='';pathInput.value=initial;selected.value='';entries.value=[];locations.value=[];showHidden.value=false;history.value=[];historyIndex.value=-1;error.value=''
 await nextTick();dialog.value?.showModal()
 void navigate(initial)
 try{const info=await api<{locations:{name:string;path:string}[]}>('Home');if(activeSession===session&&dialog.value?.open)locations.value=info.locations}catch{}
}
function invalidate(){revision++;session++;loading.value=false}
function cancel(){invalidate();dialog.value?.close()}
function choose(){if(!canSelect.value)return;const destination=target.value,paths=[...sources.value];cancel();emit('select',destination,paths)}
defineExpose({open,cancel})
onUnmounted(invalidate)
</script>

<template>
 <dialog ref="dialog" class="confirm-dialog directory-picker" :aria-labelledby="titleId" @close="invalidate" @cancel="invalidate">
  <header class="picker-header"><div><h2 :id="titleId">{{props.purpose==='favorite'?'收藏目录':props.purpose==='browse'?'打开目录':'选择移动目标'}}</h2><span v-if="props.purpose==='move'">{{sources.length}} 个项目</span></div><button type="button" class="icon-button" aria-label="关闭目录选择" @click="cancel"><Close/></button></header>
  <div class="picker-toolbar">
   <button type="button" class="icon-button" aria-label="返回上个目录" :disabled="loading||historyIndex<=0" @click="back"><ArrowLeft/></button>
   <button type="button" class="icon-button" aria-label="返回上一级目录" :disabled="loading||!current||key(current)===key(filesystemRoot)" @click="navigate(parent(current))"><ArrowUp/></button>
   <form class="path-field" @submit.prevent="navigate(pathInput.trim())"><FolderOpened/><input v-model="pathInput" autofocus aria-label="目标选择路径" placeholder="输入目录路径"/><button type="submit" class="icon-button" aria-label="打开输入的目录"><ArrowRight/></button></form>
  </div>
  <div class="picker-body">
   <nav class="picker-locations" aria-label="目录选择常用位置">
    <button type="button" :class="{active:key(current)===key(filesystemRoot)}" :disabled="loading" @click="navigate(filesystemRoot)"><Monitor/><span>{{filesystemRoot==='/'?'磁盘根目录':filesystemRoot}}</span></button>
    <button v-for="loc in locations" :key="loc.path" type="button" :title="loc.path" :class="{active:key(current)===key(loc.path)}" :disabled="loading" @click="navigate(loc.path)"><House/><span>{{loc.name}}</span></button>
   </nav>
   <section class="picker-content" :aria-busy="loading" aria-label="可选目录">
    <div class="picker-list-heading"><span>{{current?base(current):'目录'}}</span><label class="inline-check"><input v-model="showHidden" type="checkbox"/>显示隐藏目录</label></div>
    <div v-if="error" class="picker-error" role="alert"><Warning/><span>{{error}}</span></div>
    <div v-if="loading" class="picker-empty"><Loading class="spin"/>正在读取目录…</div>
    <ul v-else class="picker-folders">
     <li v-for="folder in folders" :key="folder.path" :class="{selected:selected===folder.path,blocked:withinSource(folder.path)}">
      <button type="button" class="picker-folder-choice" :disabled="withinSource(folder.path)||!!error" :aria-pressed="selected===folder.path" :aria-label="`选择目录 ${folder.name}`" :title="folder.path" @click="selected=folder.path" @dblclick="navigate(folder.path)" @keydown.enter.prevent="navigate(folder.path)"><FileIcon :name="folder.name" dir/><span>{{folder.name}}</span><small v-if="withinSource(folder.path)">来源目录</small></button>
      <button type="button" class="icon-button" :aria-label="`进入目录 ${folder.name}`" :disabled="withinSource(folder.path)||!!error" @click="navigate(folder.path)"><ArrowRight/></button>
     </li>
     <li v-if="!folders.length&&!error" class="picker-empty">此目录没有子目录</li>
    </ul>
   </section>
  </div>
  <footer class="picker-footer"><div class="picker-destination"><span>{{props.purpose==='favorite'?'收藏目录':props.purpose==='browse'?'打开目录':'目标目录'}}</span><strong :title="target">{{target||'请选择目录'}}</strong><p v-if="targetIssue&&!loading&&!error" class="picker-target-issue">{{targetIssue}}</p></div><div class="dialog-actions"><button type="button" class="button" @click="cancel">取消</button><button type="button" class="button primary" :disabled="!canSelect" @click="choose">{{props.purpose==='favorite'?'固定到侧边栏':props.purpose==='browse'?'打开此目录':'选择此目录'}}</button></div></footer>
 </dialog>
</template>

<style scoped>
.directory-picker{width:680px;max-width:calc(100vw - 40px);height:500px;max-height:calc(100dvh - 40px);padding:0;overflow:hidden}
.directory-picker[open]{display:flex;flex-direction:column}
.picker-header{display:flex;align-items:center;justify-content:space-between;padding:18px 20px;border-bottom:1px solid var(--line);flex-shrink:0}
.picker-header>div{display:flex;align-items:center;gap:12px}
.picker-header h2{margin:0;font-size:16px}
.picker-header span{font-size:11px;color:var(--muted)}
.picker-toolbar{display:flex;gap:6px;align-items:center;padding:12px 16px;border-bottom:1px solid var(--line);flex-shrink:0}
.picker-toolbar .path-field{margin-left:3px}
.picker-body{display:grid;grid-template-columns:146px minmax(0,1fr);min-height:0;flex:1}
.picker-locations{padding:12px 8px;background:#fafbfd;border-right:1px solid var(--line);overflow:auto}
.picker-locations button{display:flex;align-items:center;gap:8px;width:100%;padding:9px 8px;margin-bottom:3px;border:0;border-radius:5px;background:transparent;text-align:left;font-size:11px;color:#697588}
.picker-locations button svg{width:15px;height:15px}
.picker-locations button span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.picker-locations button:hover{background:#edf1f7}
.picker-locations button.active{background:#eaf0fc;color:var(--accent)}
.picker-content{min-width:0;min-height:0;display:flex;flex-direction:column}
.picker-list-heading{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:11px 14px;color:var(--muted);font-size:11px;border-bottom:1px solid #f0f2f6;flex-shrink:0}
.picker-list-heading>span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.picker-list-heading .inline-check{margin:0;white-space:nowrap;font-size:10px}
.picker-folders{list-style:none;overflow:auto;padding:6px;margin:0;min-height:0;flex:1}
.picker-folders>li:not(.picker-empty){display:flex;align-items:center;gap:3px;border-radius:5px;margin:2px 0}
.picker-folders>li:not(.blocked):hover{background:#f6f8fc}
.picker-folders>li.selected{background:#edf3ff!important}
.picker-folder-choice{display:flex;align-items:center;gap:8px;border:0;background:transparent;text-align:left;padding:10px 8px;min-width:0;flex:1;font-size:12px;color:#445067}
.picker-folder-choice svg{width:18px;height:18px;color:#c6a05b}
.picker-folder-choice>span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.picker-folder-choice small{font-size:10px;color:var(--muted);margin-left:auto;white-space:nowrap}
.picker-folders>li>.icon-button{margin-right:3px}
.picker-empty{display:flex;align-items:center;justify-content:center;gap:8px;min-height:100px;padding:20px;color:var(--muted);font-size:12px}
.picker-empty svg{width:16px;height:16px}
.picker-error{display:flex;gap:7px;align-items:flex-start;padding:12px 14px;background:#fff2f1;color:#ae5450;font-size:11px;overflow-wrap:anywhere;overflow:auto;max-height:110px}
.picker-error svg{width:15px;height:15px}
.picker-footer{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:14px 18px;border-top:1px solid var(--line);flex-shrink:0;background:#fafbfd}
.picker-destination{min-width:0;font-size:10px;color:var(--muted)}
.picker-destination strong{display:block;font-size:11px;line-height:1.6;font-weight:500;color:#526178;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;margin-top:3px}
.picker-destination .picker-target-issue{font-size:10px;color:#a16e13;margin-top:4px;line-height:1.5}
.picker-footer .dialog-actions{margin:0;flex-shrink:0}
</style>
