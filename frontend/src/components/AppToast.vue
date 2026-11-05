<script setup lang="ts">
import { onUnmounted,ref } from 'vue'
import { Check,Warning,Close } from '@element-plus/icons-vue'

type ToastType='success'|'error'
const messages=ref<{id:number;message:string;type:ToastType}[]>([])
const timers=new Map<number,ReturnType<typeof setTimeout>>()
let nextId=0
function dismiss(id:number){
 clearTimeout(timers.get(id));timers.delete(id)
 messages.value=messages.value.filter(message=>message.id!==id)
}
function show(message:string,type:ToastType='success'){
 if(!message.trim())return
 const id=++nextId
 messages.value.push({id,message,type})
 timers.set(id,setTimeout(()=>dismiss(id),3000))
}
onUnmounted(()=>{timers.forEach(timer=>clearTimeout(timer));timers.clear()})
defineExpose({show})
</script>

<template>
 <Teleport to="body">
  <TransitionGroup name="toast" tag="div" class="toast-container" aria-label="系统提示">
   <div v-for="item in messages" :key="item.id" :class="['app-toast',`toast-${item.type}`]" :role="item.type==='error'?'alert':'status'" aria-atomic="true">
    <Warning v-if="item.type==='error'" aria-hidden="true"/><Check v-else aria-hidden="true"/>
    <span class="toast-message">{{item.message}}</span>
    <button type="button" class="icon-button toast-close" aria-label="关闭提示" @click="dismiss(item.id)"><Close/></button>
   </div>
  </TransitionGroup>
 </Teleport>
</template>

<style scoped>
.toast-container{position:fixed;z-index:100;top:20px;left:50%;transform:translateX(-50%);display:flex;flex-direction:column;align-items:center;gap:10px;width:max-content;max-width:calc(100vw - 32px);max-height:calc(100dvh - 40px);overflow:auto;pointer-events:none;padding:4px}
.app-toast{display:flex;align-items:flex-start;gap:9px;max-width:min(520px,100%);padding:12px 14px;border:1px solid;border-radius:9px;box-shadow:0 5px 18px #253a601a;font-size:12px;line-height:22px;pointer-events:auto}
.app-toast>svg{width:18px;height:18px;margin-top:2px}
.toast-success{background:#edf8f3;border-color:#c9e8d8;color:#287157}
.toast-error{background:#fff0ef;border-color:#f1ccca;color:#a13f3f}
.toast-message{min-width:0;white-space:pre-line;overflow-wrap:anywhere}
.toast-close{color:inherit;margin:-2px -4px -2px 4px}
.toast-close:hover{background:#253a600a;color:inherit}
.toast-enter-active,.toast-leave-active,.toast-move{transition:opacity .18s,transform .18s}
.toast-enter-from,.toast-leave-to{opacity:0;transform:translateY(-8px)}
</style>
