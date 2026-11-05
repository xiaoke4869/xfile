<script setup lang="ts">
import { computed,nextTick,onUnmounted,reactive,ref,watch } from 'vue'
import { Close,MoreFilled,ArrowRight } from '@element-plus/icons-vue'
import { api } from '../api'
import { base } from '../format'
import AppSelect from './AppSelect.vue'
import type { Entry,OperationResult,RenameConfig,RenamePreview } from '../types'

const props=defineProps<{running:boolean}>()
const emit=defineEmits<{busy:[value:boolean];complete:[result:OperationResult]}>()
const dialog=ref<HTMLDialogElement>(),nameInput=ref<HTMLInputElement>(),entries=ref<Entry[]>([]),opened=ref(false),working=ref(false),expanded=ref(false),loading=ref(false),error=ref(''),preview=ref<RenamePreview|null>(null)
const config=reactive<RenameConfig>({mode:'single',name:'',find:'',replace:'',text:'',format:'<原文件名>-',position:'after',start:1})
let revision=0,timer:ReturnType<typeof setTimeout>|undefined
const single=computed(()=>entries.value.length===1)
const canRename=computed(()=>!props.running&&!working.value&&!loading.value&&!error.value&&!!preview.value&&!preview.value.issues.length&&preview.value.changed>0)
function clearPreview(){clearTimeout(timer);revision++;loading.value=false;preview.value=null}
function cancel(){if(!working.value){dialog.value?.close();opened.value=false;clearPreview()}}
async function open(selected:Entry[]){
 entries.value=selected.map(e=>({...e}));expanded.value=false;error.value='';preview.value=null
 Object.assign(config,{mode:selected.length===1?'single':'format',name:selected[0]?.name||'',find:'',replace:'',text:'',format:'<原文件名>-',position:'after',start:1})
 opened.value=true;await nextTick();dialog.value?.showModal();schedule()
 if(single.value){nameInput.value?.focus();const name=config.name,dot=name.lastIndexOf('.');nameInput.value?.setSelectionRange(0,!entries.value[0]?.dir&&dot>0?dot:name.length)}
}
function schedule(){
 clearPreview();error.value=''
 if(!opened.value)return
 const id=revision,paths=entries.value.map(e=>e.path),options={...config};loading.value=true
 timer=setTimeout(async()=>{
  try{
   if(config.mode==='format'&&(!Number.isSafeInteger(options.start)||options.start<0))throw new Error('开始数字须为非负整数')
   const result=await api<RenamePreview>('PreviewRename',paths,options)
   if(id===revision)preview.value=result
  }catch(e){if(id===revision)error.value=String(e)}finally{if(id===revision)loading.value=false}
 },180)
}
watch(config,schedule)
onUnmounted(clearPreview)
async function perform(){
 if(!canRename.value)return
 working.value=true;error.value='';emit('busy',true)
 try{
  const result=await api<OperationResult>('RenameEntries',entries.value.map(e=>e.path),{...config})
  emit('complete',result);dialog.value?.close();opened.value=false;clearPreview()
 }catch(e){error.value=String(e)}finally{working.value=false;emit('busy',false)}
}
defineExpose({open,cancel})
</script>

<template>
 <dialog ref="dialog" class="confirm-dialog rename-dialog" aria-labelledby="rename-title" @cancel.prevent="cancel" @close="opened=false;clearPreview()">
  <form @submit.prevent="perform">
   <header class="rename-header"><div><h2 id="rename-title">{{single?'重命名':'批量重命名'}}</h2><span v-if="!single">已选择 {{entries.length}} 个项目</span></div><button type="button" class="icon-button" aria-label="关闭重命名" :disabled="working" @click="cancel"><Close/></button></header>
   <fieldset :disabled="working" class="rename-fields">
    <label v-if="single" class="form-label">名称<input ref="nameInput" v-model="config.name" autofocus autocomplete="off" /></label>
    <template v-else>
     <label class="form-label">重命名方式<AppSelect v-model="config.mode" :disabled="working" aria-label="重命名方式" :options="[{value:'format',label:'按格式'},{value:'replace',label:'替换文本'},{value:'add',label:'添加文本'}]"/></label>
     <template v-if="config.mode==='replace'"><label class="form-label">查找文本<input v-model="config.find" autofocus autocomplete="off" placeholder="输入要查找的字符"/></label><label class="form-label">替换为<input v-model="config.replace" autocomplete="off" placeholder="留空则删除匹配文本"/></label></template>
     <label v-else-if="config.mode==='add'" class="form-label">添加文本<input v-model="config.text" autocomplete="off" placeholder="输入要添加的文本"/></label>
     <label v-else class="form-label">自定义格式<input v-model="config.format" autocomplete="off" placeholder="例如：<原文件名>- 或 壁纸"/><small>使用 &lt;原文件名&gt; 保留原名称，文件扩展名自动保留。</small></label>
     <div v-if="config.mode!=='replace'" class="rename-options"><label class="form-label">{{config.mode==='format'?'编号位置':'添加位置'}}<AppSelect v-model="config.position" :disabled="working" :aria-label="config.mode==='format'?'编号位置':'添加位置'" :options="[{value:'before',label:'在名称之前'},{value:'after',label:'在名称之后'}]"/></label><label v-if="config.mode==='format'" class="form-label">开始数字<input v-model.number="config.start" type="number" min="0" max="9007199254740991" step="1" required/></label></div>
    </template>
   </fieldset>
   <p v-if="error" class="rename-error" role="alert">{{error}}</p>
   <ul v-if="preview?.issues.length" class="rename-errors" role="alert"><li v-for="issue in preview.issues" :key="issue">{{issue}}</li></ul>
   <div class="dialog-actions"><button type="button" class="button" :disabled="working" @click="cancel">取消</button><button type="submit" class="button primary" :disabled="!canRename">{{working?'正在重命名…':'重命名'}}</button></div>
   <footer class="rename-preview" :aria-busy="loading">
    <div class="rename-preview-bar"><span class="rename-preview-label">预览</span><div class="rename-example" role="status" aria-live="polite"><span v-if="loading" class="muted">正在计算…</span><template v-else-if="preview?.items[0]"><span :title="base(preview.items[0].source)">{{base(preview.items[0].source)}}</span><ArrowRight/><strong :title="base(preview.items[0].target)">{{base(preview.items[0].target)}}</strong></template><span v-else class="muted">填写规则后查看新名称</span></div><button v-if="!single" type="button" class="icon-button" :aria-label="expanded?'收起详细预览':'展开详细预览'" :aria-expanded="expanded" aria-controls="rename-details" @click="expanded=!expanded"><MoreFilled/></button></div>
    <div v-if="expanded&&!single" id="rename-details" class="rename-details"><div class="rename-preview-summary">{{preview?`共 ${preview.total} 项，${preview.changed} 项将更改；显示前 ${preview.items.length} 项`:'等待预览'}} · 按选择顺序编号</div><div v-for="item in preview?.items||[]" :key="item.source" class="rename-detail-row"><span :title="item.source">{{base(item.source)}}</span><ArrowRight/><span :class="{'error-text':item.error}" :title="item.error||item.target">{{item.error||base(item.target)}}</span></div></div>
   </footer>
  </form>
 </dialog>
</template>

<style scoped>
.rename-dialog{width:500px;max-width:calc(100vw - 40px);max-height:calc(100dvh - 40px);padding:0;overflow:hidden}
.rename-dialog form{display:flex;flex-direction:column;max-height:calc(100dvh - 42px)}.rename-header,.rename-fields,.dialog-actions{flex-shrink:0}
.rename-header{display:flex;align-items:center;justify-content:space-between;padding:22px 24px 0}
.rename-header h2{margin:0;font-size:18px}.rename-header span{display:block;margin-top:7px;font-size:11px;color:var(--muted)}
.rename-fields{padding:2px 24px 0}.form-label{font-size:12px}.form-label input{font-size:12px;min-height:36px}.form-label small{font-size:10px;color:var(--muted);line-height:1.6}.rename-options{display:flex;gap:16px}.rename-options>label{flex:1;min-width:0}.rename-options input{width:100%}
.dialog-actions{margin:24px;padding:0}.rename-error,.rename-errors{font-size:11px;color:#b34545;margin:16px 24px 0;white-space:pre-line;overflow-wrap:anywhere;max-height:100px;overflow:auto}.rename-errors{padding-left:16px}
.rename-preview{border-top:1px solid var(--line);background:#f8fafd;display:flex;flex-direction:column;min-height:53px}.rename-preview-bar{display:flex;align-items:center;gap:12px;padding:12px 18px;min-height:52px;flex-shrink:0}.rename-preview-label{font-size:10px;color:var(--muted);flex-shrink:0}.rename-example{display:flex;align-items:center;gap:8px;min-width:0;flex:1;font-size:11px}.rename-example span,.rename-example strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;min-width:0;font-weight:400}.rename-example strong{color:var(--accent)}.rename-example svg,.rename-detail-row svg{width:12px;height:12px;flex-shrink:0;color:var(--muted)}
.rename-details{border-top:1px solid var(--line);padding:10px 18px;max-height:230px;min-height:0;overflow:auto}.rename-preview-summary{font-size:10px;color:var(--muted);margin-bottom:8px}.rename-detail-row{display:grid;grid-template-columns:minmax(0,1fr) 12px minmax(0,1fr);align-items:center;gap:12px;padding:7px 0;font-size:11px}.rename-detail-row span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.rename-detail-row span:last-child{color:var(--accent)}
</style>
