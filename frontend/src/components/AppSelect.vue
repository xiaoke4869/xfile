<script setup lang="ts" generic="T extends string">
import { vScrollbars } from '../scrollbars'
import { computed,nextTick,onUnmounted,ref,useId,watch } from 'vue'
import { ArrowDown,Check } from '@element-plus/icons-vue'

defineOptions({inheritAttrs:false})
const props=defineProps<{modelValue?:T;options:ReadonlyArray<{value:T;label:string}>;disabled?:boolean}>()
const emit=defineEmits<{'update:modelValue':[value:T]}>()
const id=useId(),trigger=ref<HTMLButtonElement>(),popup=ref<HTMLDivElement>(),opened=ref(false),active=ref(0),target=ref<HTMLElement>()
const position=ref({left:'0px',top:'0px',width:'0px',maxHeight:'240px',visibility:'hidden' as 'hidden'|'visible'})
const current=computed(()=>props.options.findIndex(option=>option.value===props.modelValue))
let typed='',typedAt=0
function close(){opened.value=false}
function scrollActive(){nextTick(()=>popup.value?.querySelector('[data-active="true"]')?.scrollIntoView({block:'nearest'}))}
async function open(){
 if(props.disabled||trigger.value?.matches(':disabled')||!props.options.length)return
 if(!trigger.value)return
 const anchor=trigger.value.getBoundingClientRect(),width=Math.min(Math.max(anchor.width,160),innerWidth-24)
 target.value=trigger.value?.closest('dialog')||document.body
 active.value=Math.max(0,current.value);opened.value=true
 position.value={...position.value,width:`${width}px`,visibility:'hidden'}
 await nextTick()
 place();scrollActive()
}
function place(){
 if(!trigger.value||!popup.value||!opened.value)return
 const rect=trigger.value.getBoundingClientRect(),below=innerHeight-rect.bottom-12,above=rect.top-12
 if(rect.bottom<0||rect.top>innerHeight){close();return}
 const width=Math.min(Math.max(rect.width,160),innerWidth-24)
 const upward=below<Math.min(240,popup.value.scrollHeight)&&above>below
 const height=Math.min(240,Math.max(0,upward?above:below))
 position.value={left:`${Math.max(12,Math.min(rect.left,innerWidth-width-12))}px`,top:`${upward?Math.max(12,rect.top-6-Math.min(height,popup.value.scrollHeight+2)):rect.bottom+6}px`,width:`${width}px`,maxHeight:`${height}px`,visibility:'visible'}
}
function choose(index:number){const option=props.options[index];if(option&&!props.disabled){emit('update:modelValue',option.value);close();trigger.value?.focus({preventScroll:true})}}
function keydown(event:KeyboardEvent){
 const key=event.key
 if(key==='Escape'&&opened.value){event.preventDefault();event.stopPropagation();close();return}
 if(key==='Tab'){close();return}
 if(key==='Enter'||key===' '){event.preventDefault();opened.value?choose(active.value):open();return}
 if(['ArrowDown','ArrowUp','Home','End'].includes(key)){
  event.preventDefault()
  if(!opened.value){open();if(key==='Home')active.value=0;if(key==='End')active.value=props.options.length-1}
  else active.value=key==='Home'?0:key==='End'?props.options.length-1:(active.value+(key==='ArrowUp'?-1:1)+props.options.length)%props.options.length
  scrollActive();return
 }
 if(key.length===1&&!event.ctrlKey&&!event.metaKey&&!event.altKey){
  event.preventDefault();if(!opened.value)open()
  typed=Date.now()-typedAt>600?key:typed+key;typedAt=Date.now()
  const index=props.options.findIndex(option=>option.label.toLocaleLowerCase().startsWith(typed.toLocaleLowerCase()))
  if(index>=0){active.value=index;scrollActive()}
 }
}
function outside(event:PointerEvent){if(!trigger.value?.contains(event.target as Node)&&!popup.value?.contains(event.target as Node))close()}
function scroll(event:Event){if(!popup.value?.contains(event.target as Node))place()}
function cleanup(){window.removeEventListener('pointerdown',outside,true);window.removeEventListener('scroll',scroll,true);window.removeEventListener('resize',close)}
watch(opened,value=>{
 if(value){window.addEventListener('pointerdown',outside,true);window.addEventListener('scroll',scroll,true);window.addEventListener('resize',close)}else cleanup()
})
watch(()=>props.disabled,value=>{if(value)close()})
watch(()=>props.modelValue,()=>{if(opened.value)close()})
onUnmounted(cleanup)
</script>

<template>
 <button v-bind="$attrs" ref="trigger" type="button" :class="['app-select',{expanded:opened}]" role="combobox" aria-haspopup="listbox" :disabled="disabled" :aria-expanded="opened" :aria-controls="opened?id:undefined" :aria-activedescendant="opened?`${id}-${active}`:undefined" @click="opened?close():open()" @keydown="keydown" @blur="close"><span>{{options[current]?.label||'请选择'}}</span><ArrowDown aria-hidden="true"/></button>
 <Teleport v-if="opened&&target" :to="target">
  <div v-scrollbars :id="id" ref="popup" class="app-select-menu" :style="position" role="listbox" :aria-label="String($attrs['aria-label']||'选项')" @pointerdown.prevent @click.stop>
   <div v-for="(option,index) in options" :id="`${id}-${index}`" :key="option.value" :class="['app-select-option',{active:index===active,selected:option.value===modelValue}]" role="option" :aria-selected="option.value===modelValue" :data-active="index===active" @pointermove="active=index" @click="choose(index)"><span>{{option.label}}</span><Check v-if="option.value===modelValue" aria-hidden="true"/></div>
  </div>
 </Teleport>
</template>

<style scoped>
.app-select{display:flex;align-items:center;justify-content:space-between;gap:12px;width:100%;min-width:0;min-height:36px;padding:9px 11px;background:#fff;border:1px solid #dce1e9;border-radius:var(--radius);color:#394963;font-size:12px;line-height:1.4;text-align:left;box-shadow:0 1px 2px #182b4805;transition:border-color .15s,box-shadow .15s,background .15s}
.app-select>span{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.app-select>svg{width:12px;height:12px;color:#8b97a9;transition:transform .15s}
.app-select:hover:not(:disabled){border-color:#b8bcc4;background:#fcfdff}.app-select.expanded{border-color:#b8bcc4;box-shadow:none}.app-select.expanded>svg{transform:rotate(180deg);color:var(--accent)}
.app-select:disabled{background:#f3f5f8;color:#8a96a7;opacity:.65;box-shadow:none}
.app-select-menu{position:fixed;z-index:110;overflow:auto;padding:5px;background:#fff;border:1px solid #dfe5ee;border-radius:9px;box-shadow:0 8px 26px #253a601f,0 2px 6px #253a600a;color:#394963;font:400 12px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;overscroll-behavior:contain}
.app-select-option{display:flex;align-items:center;justify-content:space-between;gap:14px;padding:9px 10px;border-radius:5px;cursor:pointer;min-height:35px}.app-select-option>span{overflow-wrap:anywhere}.app-select-option.active{background:#f0f4fb}.app-select-option.selected{color:var(--accent);font-weight:500}.app-select-option.selected.active{background:#edf3ff}.app-select-option svg{width:14px;height:14px;flex-shrink:0}
</style>
