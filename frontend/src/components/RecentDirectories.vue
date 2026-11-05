<script setup lang="ts">
import { nextTick,onMounted,onUnmounted,ref,useId } from 'vue'
import { Clock,Close,Delete } from '@element-plus/icons-vue'
import FileIcon from './FileIcon.vue'
import { base,pathKey } from '../format'

const emit=defineEmits<{navigate:[path:string];error:[message:string]}>()
const id=useId(),root=ref<HTMLElement>(),trigger=ref<HTMLButtonElement>(),panel=ref<HTMLElement>(),opened=ref(false),paths=ref<string[]>([])
const storageKey='xfile.recentDirectories'
function save(next:string[]){
 paths.value=next
 try{localStorage.setItem(storageKey,JSON.stringify(next))}catch(e){emit('error','保存最近目录失败：'+String(e))}
}
function record(path:string){if(path)save([path,...paths.value.filter(other=>pathKey(other)!==pathKey(path))].slice(0,50))}
function close(restore=false){opened.value=false;if(restore)trigger.value?.focus()}
async function toggle(){
 if(opened.value){close();return}
 opened.value=true;await nextTick();panel.value?.querySelector<HTMLButtonElement>('.close-recent')?.focus()
}
function outside(event:PointerEvent){if(!root.value?.contains(event.target as Node))close()}
function visit(path:string){close(true);emit('navigate',path)}
function remove(path:string){save(paths.value.filter(other=>other!==path))}
onMounted(()=>{
 try{
  const saved:unknown=JSON.parse(localStorage.getItem(storageKey)||'[]')
  if(Array.isArray(saved))paths.value=saved.filter((p):p is string=>typeof p==='string'&&!!p.trim()).filter((p,i,all)=>all.findIndex(other=>pathKey(other)===pathKey(p))===i).slice(0,50)
 }catch{}
 window.addEventListener('pointerdown',outside)
})
onUnmounted(()=>window.removeEventListener('pointerdown',outside))
defineExpose({record,close})
</script>

<template>
 <div ref="root" class="recent-directories" @keydown.esc.prevent.stop="close(true)">
  <button ref="trigger" class="icon-button" aria-label="最近访问目录" title="最近访问目录" aria-haspopup="dialog" :aria-expanded="opened" :aria-controls="id" @click="toggle"><Clock/></button>
  <section v-if="opened" :id="id" ref="panel" class="recent-panel" role="dialog" aria-label="最近访问目录">
   <header><div><h2>最近目录</h2><p>{{paths.length}} / 50 个目录</p></div><button class="icon-button close-recent" aria-label="关闭最近目录" @click="close(true)"><Close/></button></header>
   <ul v-if="paths.length" class="recent-list">
    <li v-for="path in paths" :key="path"><button class="recent-visit" :title="path" :aria-label="`进入最近目录 ${path}`" @click="visit(path)"><FileIcon :name="path" dir/><span><strong>{{base(path)}}</strong><small>{{path}}</small></span></button><button class="icon-button recent-remove" :aria-label="`移除最近记录 ${path}`" title="移除这条记录" @click="remove(path)"><Close/></button></li>
   </ul>
   <p v-else class="recent-empty" role="status">暂无最近访问目录</p>
   <footer><span>仅保留最近 50 个目录</span><button class="text-button" :disabled="!paths.length" @click="save([])"><Delete/>清空记录</button></footer>
  </section>
 </div>
</template>

<style scoped>
.recent-directories{position:relative;flex-shrink:0;--wails-draggable:no-drag}
.recent-directories>.icon-button{min-width:24px;min-height:24px}
.recent-directories>.icon-button svg{width:15px;height:15px}
.recent-directories>.icon-button[aria-expanded=true]{background:#e5e9ee;color:var(--accent)}
.recent-panel{position:absolute;z-index:72;right:0;bottom:calc(100% + 10px);width:360px;max-width:calc(100vw - 32px);height:min(460px,calc(100dvh - 100px));display:flex;flex-direction:column;border:1px solid #d9dfe8;border-radius:10px;background:#fafbfd;box-shadow:0 8px 32px #20345429;overflow:hidden;color:#394963;text-align:left}
.recent-panel header{display:flex;align-items:center;justify-content:space-between;padding:16px;border-bottom:1px solid var(--line);flex-shrink:0}
.recent-panel header h2{font-size:13px}.recent-panel header p{font-size:10px;color:var(--muted);margin-top:4px}
.recent-list{list-style:none;margin:0;padding:6px;overflow-y:auto;flex:1;min-height:0}
.recent-list li{display:flex;align-items:center;border-radius:5px}.recent-list li:hover,.recent-list li:focus-within{background:#edf1f7}
.recent-visit{display:flex;align-items:center;gap:10px;min-width:0;flex:1;border:0;background:transparent;padding:10px;text-align:left}
.recent-visit>.file-icon{width:23px;height:23px}.recent-visit>span{display:flex;flex-direction:column;gap:5px;min-width:0}
.recent-visit strong{font-size:12px;font-weight:500;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.recent-visit small{font-size:10px;color:var(--muted);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.recent-remove{margin-right:5px}.recent-remove svg{width:12px;height:12px}
.recent-panel footer{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:12px 16px;border-top:1px solid var(--line);font-size:10px;color:var(--muted);flex-shrink:0}
.recent-panel footer .text-button{font-size:11px}.recent-empty{display:flex;align-items:center;justify-content:center;flex:1;color:var(--muted);font-size:12px}
.recent-panel button:focus-visible,.recent-directories>.icon-button:focus-visible{outline:2px solid var(--accent);outline-offset:-2px}
</style>
