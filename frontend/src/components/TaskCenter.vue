<script setup lang="ts">
import { vScrollbars } from '../scrollbars'
import { computed,nextTick,onMounted,onUnmounted,ref,useId,watch } from 'vue'
import { Check,Close,Loading,Warning } from '@element-plus/icons-vue'
import { base,size,taskName,taskStatus } from '../format'
import type { SizeTask,Task } from '../types'

const props=defineProps<{sizeTasks:SizeTask[];operation:Task|null;running:boolean;session:number}>()
const emit=defineEmits<{inspect:[task:Task];open:[];retry:[path:string]}>()
const id=useId(),root=ref<HTMLElement>(),trigger=ref<HTMLButtonElement>(),panel=ref<HTMLElement>(),opened=ref(false)
const notice=ref<{message:string;success:boolean}|null>(null)
let timer:ReturnType<typeof setTimeout>|undefined
const pending=computed(()=>props.sizeTasks.filter(t=>t.status==='queued'||t.status==='running').length)
const succeeded=computed(()=>props.sizeTasks.filter(t=>t.status==='succeeded').length)
const failed=computed(()=>props.sizeTasks.filter(t=>t.status==='failed').length)
const current=computed(()=>props.sizeTasks.find(t=>t.status==='running'))
const active=computed(()=>props.running||pending.value>0)
const completed=computed(()=>succeeded.value+failed.value)
const progress=computed(()=>props.sizeTasks.length?Math.round(completed.value/props.sizeTasks.length*100):0)
const statuses={queued:'等待计算',running:'计算中',succeeded:'已完成',failed:'计算失败'}
function flash(message:string,success:boolean){
 clearTimeout(timer);notice.value={message,success}
 timer=setTimeout(()=>{notice.value=null},2600)
}
watch(pending,(value,previous)=>{
 if(previous&&!value&&props.sizeTasks.length)flash(failed.value?`计算结束，${failed.value} 个失败 · 点击查看`:'',!failed.value)
})
watch(()=>props.session,()=>{clearTimeout(timer);notice.value=null})
watch(()=>[props.operation?.id,props.operation?.status] as const,([id,status],[previousID,previousStatus])=>{
 if(id===previousID&&previousStatus==='running'&&status!=='running'&&props.operation)flash(`${taskName(props.operation)}${status==='succeeded'?'已完成':'已结束 · 点击查看结果'}`,status==='succeeded')
})
function close(restore=false){opened.value=false;if(restore)trigger.value?.focus()}
async function open(){opened.value=true;emit('open');await nextTick();panel.value?.querySelector<HTMLButtonElement>('.close-center')?.focus()}
function toggle(){if(opened.value)close();else void open()}
function outside(event:PointerEvent){if(!root.value?.contains(event.target as Node))close()}
function inspect(task:Task){emit('inspect',task);if(task.operation)close()}
onMounted(()=>window.addEventListener('pointerdown',outside))
onUnmounted(()=>{clearTimeout(timer);window.removeEventListener('pointerdown',outside)})
defineExpose({open,close})
</script>

<template>
 <div ref="root" class="task-center" @keydown.esc.prevent.stop="close(true)">
  <button ref="trigger" class="icon-button task-center-trigger" :class="{'center-success':!active&&notice?.success,'center-warning':!active&&(failed||notice&&!notice.success)}" aria-label="任务中心" :title="active?'任务进行中 · 点击查看':'任务中心'" :aria-busy="active" aria-haspopup="dialog" :aria-expanded="opened" :aria-controls="id" @click="toggle">
   <svg class="task-center-glyph" :class="{spin:active}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <circle v-if="active" cx="12" cy="12" r="8" stroke-dasharray="38 13"/>
    <path v-else-if="notice?.success" d="m5 12 4 4L19 6"/>
    <g v-else-if="failed||notice&&!notice.success"><path d="m12 3 9 17H3L12 3Z"/><path d="M12 9v5m0 3h.01"/></g>
    <g v-else><rect x="4" y="3" width="16" height="18" rx="2"/><path d="m7 8 1.5 1.5L11 7m2 2h4M7 14l1.5 1.5L11 13m2 2h4"/></g>
   </svg>
   <span v-if="failed" class="failure-count" :aria-label="`${failed} 个计算失败`">{{failed>99?'99+':failed}}</span>
  </button>
  <div v-if="notice?.message&&!opened" class="task-center-notice" :class="{success:notice.success}" role="status" aria-live="polite">{{notice.message}}</div>
  <section v-if="opened" :id="id" ref="panel" class="task-center-panel" role="dialog" aria-label="任务中心">
   <header><div><h2>任务中心</h2><p>{{active?'任务正在进行，可以收起此面板':'本次任务状态'}}</p></div><button class="icon-button close-center" aria-label="收起任务中心" @click="close(true)"><Close/></button></header>
   <div v-scrollbars class="task-center-content">
    <section v-if="sizeTasks.length" class="size-activity" aria-label="文件夹大小计算">
     <div class="activity-heading"><h3>文件夹大小</h3><span>{{pending?'计算中':'已结束'}}</span></div>
     <p class="size-summary" role="status">{{completed}} / {{sizeTasks.length}} 个 · 成功 {{succeeded}}<span v-if="failed" class="error-text"> · 失败 {{failed}}</span></p>
     <div class="progress-track"><div :style="{width:progress+'%'}"></div></div>
     <p v-if="current" class="size-current" :title="current.path">{{base(current.path)}} · 已扫描 {{current.files}} 个文件 · {{size(current.bytes)}}</p>
     <ul class="size-details"><li v-for="item in sizeTasks" :key="item.path" :class="{'size-failed':item.status==='failed'}">
       <div class="size-item-heading"><strong>{{base(item.path)}}</strong><span>{{statuses[item.status]}}</span></div>
       <p class="size-item-path">{{item.path}}</p>
       <p v-if="item.status==='succeeded'" class="size-item-result">{{size(item.bytes)}} · {{item.files}} 个文件</p>
       <p v-if="item.error" class="size-item-error">{{item.error}}</p>
       <button v-if="item.status==='failed'" class="text-button" :disabled="running||!!pending" :aria-label="`重新计算 ${item.path}`" @click="emit('retry',item.path)">重新计算</button>
     </li></ul>
    </section>
    <slot/>
    <section v-if="operation?.operation" class="operation-activities" aria-label="文件操作任务">
     <h3>文件操作</h3>
     <button class="center-operation" @click="inspect(operation)">
      <Loading v-if="operation.status==='running'" class="spin"/><Check v-else-if="operation.status==='succeeded'" class="center-success"/><Warning v-else class="center-warning"/>
      <span><strong>{{taskName(operation)}} · {{taskStatus(operation)}}</strong><small>{{operation.moved}} / {{operation.total}} 个项目<template v-if="operation.started"> · {{new Date(operation.started).toLocaleString('zh-CN')}}</template></small><small v-if="operation.error" class="error-text">{{operation.error}}</small></span>
     </button>
    </section>
    <p v-if="!sizeTasks.length&&!operation" class="center-empty">{{running?'正在准备任务…':'暂无任务'}}</p>
   </div>
  </section>
 </div>
</template>

<style scoped>
.task-center{position:relative;flex-shrink:0;--wails-draggable:no-drag}
.task-center-trigger{position:relative;min-width:24px;min-height:24px}.task-center-trigger>svg{width:15px;height:15px}
.task-center-trigger[aria-expanded=true]{background:#e5e9ee;color:var(--accent)}
.center-success{color:#28805b}.center-warning{color:#b17a30}
.failure-count{position:absolute;top:-5px;right:-7px;min-width:14px;padding:0 3px;height:14px;display:grid;place-items:center;border-radius:7px;background:#b17a30;color:#fff;font-size:9px;line-height:14px}
.task-center-notice{position:absolute;z-index:74;bottom:calc(100% + 10px);right:0;max-width:min(340px,calc(100vw - 32px));width:max-content;padding:9px 12px;border:1px solid #eadbc2;border-radius:7px;background:#fffaef;color:#946623;box-shadow:0 4px 15px #2034541a;font-size:11px;line-height:1.6;pointer-events:none}
.task-center-notice.success{background:#edf8f3;border-color:#c9e8d8;color:#287157}
.task-center-panel{position:absolute;z-index:73;right:0;bottom:calc(100% + 10px);width:410px;max-width:calc(100vw - 32px);max-height:calc(100dvh - 100px);display:flex;flex-direction:column;overflow:hidden;border:1px solid #d9dfe8;border-radius:10px;background:#fafbfd;box-shadow:0 8px 32px #20345429;color:#394963;text-align:left}
.task-center-panel>header{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:16px;border-bottom:1px solid var(--line);flex-shrink:0}.task-center-panel h2{font-size:13px}.task-center-panel header p{font-size:10px;color:var(--muted);margin-top:5px;line-height:1.5}
.task-center-content{overflow:auto;min-height:0;overscroll-behavior:contain}.size-activity{padding:16px;border-bottom:1px solid var(--line)}
.activity-heading{display:flex;align-items:center;justify-content:space-between;gap:12px}.task-center-panel h3{font-size:12px;font-weight:500}.activity-heading>span{font-size:10px;color:var(--muted)}
.size-summary{font-size:11px;margin-top:9px;color:#75849b;font-variant-numeric:tabular-nums}.progress-track{margin-top:12px}
.size-current{font-size:10px;color:#75849b;margin-top:10px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.size-details{list-style:none;margin:13px 0 0;padding:0;font-size:11px}.size-details li{padding:11px 0;border-top:1px solid var(--line)}
.size-item-heading{display:flex;justify-content:space-between;gap:12px;font-size:11px}.size-item-heading strong{font-weight:500;overflow-wrap:anywhere;min-width:0}.size-item-heading>span{flex-shrink:0;font-size:10px;color:var(--muted)}
.size-item-path,.size-item-result{font-size:10px;color:var(--muted);margin-top:5px;overflow-wrap:anywhere;line-height:1.6}.size-item-error{font-size:11px;color:#ae5450;margin-top:6px;overflow-wrap:anywhere;white-space:pre-line;line-height:1.6}.size-failed .size-item-heading>span{color:#ae5450}.size-failed .text-button{margin-top:6px;padding-left:0}
.operation-activities{padding:16px 8px 8px}.operation-activities>h3{padding:0 8px 8px}.center-operation{display:flex;gap:10px;align-items:flex-start;width:100%;padding:11px 8px;border:0;border-radius:6px;background:transparent;text-align:left}.center-operation:hover{background:#edf1f7}.center-operation>svg{width:16px;height:16px;flex-shrink:0}.center-operation>span{min-width:0;flex:1}.center-operation strong{display:block;font-size:11px;font-weight:500;line-height:1.5}.center-operation small{display:block;font-size:10px;color:var(--muted);margin-top:5px;overflow-wrap:anywhere;line-height:1.5}.center-operation small.error-text{color:#ae5450}
.center-empty{padding:36px 16px;text-align:center;font-size:12px;color:var(--muted)}
.task-center button:focus-visible{outline:2px solid var(--accent);outline-offset:-2px}

</style>
