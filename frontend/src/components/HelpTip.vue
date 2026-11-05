<script setup lang="ts">
import { nextTick,onUnmounted,ref,useId,watch } from 'vue'
import { QuestionFilled } from '@element-plus/icons-vue'

defineProps<{label:string;text:string}>()
const id=useId(),trigger=ref<HTMLButtonElement>(),tooltip=ref<HTMLDivElement>(),visible=ref(false)
const position=ref({left:'0px',top:'0px'})
let timer:ReturnType<typeof setTimeout>|undefined
async function show(){
 clearTimeout(timer)
 visible.value=true
 await nextTick()
 if(!trigger.value||!tooltip.value)return
 const anchor=trigger.value.getBoundingClientRect(),box=tooltip.value.getBoundingClientRect()
 const left=Math.max(12,Math.min(anchor.left,window.innerWidth-box.width-12))
 const top=anchor.bottom+8+box.height<=window.innerHeight-12?anchor.bottom+8:Math.max(12,anchor.top-box.height-8)
 position.value={left:`${left}px`,top:`${top}px`}
}
function cancelHide(){clearTimeout(timer)}
function hide(){cancelHide();visible.value=false}
function leave(){
 clearTimeout(timer)
 timer=setTimeout(()=>{if(document.activeElement!==trigger.value)hide()},120)
}
function escape(event:KeyboardEvent){if(event.key==='Escape')hide()}
watch(visible,open=>{
 if(open){window.addEventListener('keydown',escape);window.addEventListener('resize',hide);window.addEventListener('scroll',hide,true)}
 else cleanup()
})
function cleanup(){window.removeEventListener('keydown',escape);window.removeEventListener('resize',hide);window.removeEventListener('scroll',hide,true)}
onUnmounted(()=>{clearTimeout(timer);cleanup()})
</script>

<template>
 <button ref="trigger" type="button" class="help-trigger" :aria-label="label" :aria-describedby="visible?id:undefined" @mouseenter="show" @mouseleave="leave" @focus="show" @blur="hide" @click.stop="show"><QuestionFilled aria-hidden="true"/></button>
 <Teleport to="body">
  <div v-if="visible" :id="id" ref="tooltip" role="tooltip" class="help-tooltip" :style="position" @mouseenter="cancelHide" @mouseleave="leave">{{text}}</div>
 </Teleport>
</template>

<style scoped>
.help-trigger{display:inline-flex;align-items:center;justify-content:center;vertical-align:middle;flex-shrink:0;width:22px;height:22px;padding:3px;border:0;border-radius:4px;background:transparent;color:#929eaf;cursor:help;line-height:1}
.help-trigger svg{width:14px;height:14px}
.help-trigger:hover,.help-trigger:focus-visible{color:var(--accent);background:#edf3ff}
.help-tooltip{position:fixed;z-index:100;width:max-content;max-width:min(300px,calc(100vw - 24px));padding:10px 12px;border:1px solid #dfe5ee;border-radius:7px;background:#fff;color:#526178;box-shadow:0 4px 18px #253a6021;font:400 12px/1.75 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;overflow-wrap:anywhere}
</style>
