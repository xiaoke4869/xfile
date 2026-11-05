<script setup lang="ts">
import { onUnmounted,ref } from 'vue'
import { Call,Events } from '@wailsio/runtime'
import { native } from '../api'
import { size } from '../format'
import type { Entry } from '../types'

const props=defineProps<{entries:Entry[];metric?:'size'}>()
type Counts={folders:number;files:number;bytes:number}
const counts=ref<Counts|null>(null),loading=ref(false),error=ref('')
let request:ReturnType<typeof Call.ByName>|undefined,off:(()=>void)|undefined
let disposed=false
function add(a:Counts,b:Counts):Counts{return {folders:a.folders+b.folders,files:a.files+b.files,bytes:a.bytes+b.bytes}}
async function calculate(){
 if(loading.value)return
 error.value=''
 if(!native){error.value=`请在桌面客户端中计算${props.metric==='size'?'文件夹大小':'文件数量'}`;return}
 loading.value=true;counts.value={folders:0,files:0,bytes:0}
 let total:Counts={folders:0,files:0,bytes:0},currentPath=''
 try{
  for(const entry of [...props.entries]){
   if(disposed)return
   currentPath=entry.path
   if(entry.dir&&!entry.link){
    const eventName=`directory-count:${crypto.randomUUID()}`,completed={...total}
    off=Events.On(eventName,event=>{if(!disposed&&loading.value)counts.value=add(completed,event.data)})
    request=Call.ByName('main.FileService.CountDirectory',entry.path,eventName)
    const result=await request as Counts
    off();off=undefined;request=undefined
    if(disposed)return
    total=add(total,result)
   }else total=add(total,{folders:0,files:1,bytes:entry.link?0:entry.size})
   counts.value={...total}
  }
 }catch(e){if(!disposed&&(e as Error).name!=='CancelError')error.value=`${currentPath}：${String(e)}`}
 finally{loading.value=false;off?.();off=undefined;request=undefined}
}
onUnmounted(()=>{disposed=true;off?.();request?.cancel()})
</script>
<template>
 <div class="directory-count" :aria-busy="loading">
  <span v-if="counts" role="status" aria-live="polite">{{props.metric==='size'?size(counts.bytes):`文件夹 ${counts.folders.toLocaleString()} 个 · 文件 ${counts.files.toLocaleString()} 个`}}</span>
  <button class="text-button" :disabled="loading" @click="calculate">{{loading?'计算中…':counts?'重新计算':'点击计算'}}</button>
 </div>
 <span v-if="error" class="directory-count-error" role="alert">计算未完成：{{error}}</span>
</template>
