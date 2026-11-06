import { onMounted,onUnmounted,ref,watch,type Ref } from 'vue'
import { parent,pathKey } from './format'
import type { Entry } from './types'

export type DragPreview={x:number;y:number;sources:Entry[];target?:Entry;area?:'row'|'blank'|'favorite';count:number;scroll?:HTMLElement}

export function dropSources(sources:Entry[],target:Entry){
 if(!target.dir)return sources
 const directories=sources.filter(source=>source.dir).map(source=>pathKey(source.path)+'/')
 return sources.filter(source=>pathKey(parent(source.path))!==pathKey(target.path)&&!directories.some(directory=>pathKey(source.path).startsWith(directory)))
}

export function useFileDrag(selected:Ref<Entry[]>,disabled:Ref<boolean>,scroll:Ref<HTMLElement|undefined>,drop:(sources:Entry[],target:Entry)=>void,extraTarget:(element:Element,insideList:boolean)=>{entry:Entry;area:'blank'|'favorite'}|undefined,getScrolls:()=>HTMLElement[]=()=>scroll.value?[scroll.value]:[]){
 const rows=new Map<HTMLElement,Entry>()
 const elements=new Map<string,HTMLElement>()
 const preview=ref<DragPreview>()
 let drag:{id:number;startX:number;startY:number;x:number;y:number;previous:Entry[];sources:Entry[];active:boolean}|undefined
 let frame=0,lastFrame=0,suppressClick=false
 function registerRow(entry:Entry,element:HTMLElement|null,scope=''){
  const key=scope+'\0'+entry.path
  const previous=elements.get(key)
  if(previous)rows.delete(previous)
  if(element){elements.set(key,element);rows.set(element,entry)}
  else elements.delete(key)
 }
 function receiver(target:Entry){
  if(!drag||target.link)return false
  const key=pathKey(target.path)
  if(drag.sources.some(source=>pathKey(source.path)===key||source.dir&&key.startsWith(pathKey(source.path)+'/')))return false
  return target.dir?dropSources(drag.sources,target).length>0:drag.sources.every(source=>!source.dir&&!source.link)
 }
 function update(){
  if(!drag?.active||!scroll.value)return
  const element=document.elementFromPoint(drag.x,drag.y)
  const el=getScrolls().find(candidate=>!!element&&candidate.contains(element))
  const rect=el?.getBoundingClientRect(),header=el?.querySelector('thead')?.getBoundingClientRect()
  const insideList=!!el&&!!rect&&!!header&&drag.x>=rect.left&&drag.x<rect.left+el.clientWidth&&drag.y>=header.bottom&&drag.y<rect.top+el.clientHeight
  let target:Entry|undefined,area:'row'|'blank'|'favorite'|undefined
  if(insideList&&el&&element?.closest('.file-entry-target')){
   const row=element?.closest('tr') as HTMLElement|undefined
   const entry=row&&el.contains(row)?rows.get(row):undefined
   if(entry&&receiver(entry)){target=entry;area='row'}
  }
  if(!target&&element){
   const extra=extraTarget(element,insideList)
   if(extra&&receiver(extra.entry)){target=extra.entry;area=extra.area}
  }
  if(preview.value?.x===drag.x&&preview.value.y===drag.y&&preview.value.target?.path===target?.path&&preview.value.area===area&&preview.value.scroll===el)return
  preview.value={x:drag.x,y:drag.y,sources:drag.sources,target,area,scroll:el,count:target?dropSources(drag.sources,target).length:drag.sources.length}
 }
 function autoScroll(time:number){
  if(!drag?.active||!scroll.value)return
  const delta=Math.min(32,time-lastFrame||16);lastFrame=time
  const workspace=scroll.value.closest<HTMLElement>('.browser-content'),bounds=workspace?.getBoundingClientRect()
  if(workspace&&bounds&&drag.y>=bounds.top&&drag.y<bounds.bottom&&drag.x>=bounds.left&&drag.x<bounds.right){
   const edge=32,speed=drag.x<bounds.left+edge?-(bounds.left+edge-drag.x)/edge:drag.x>bounds.right-edge?(drag.x-bounds.right+edge)/edge:0
   workspace.scrollLeft+=speed*delta*.65
  }
  const element=document.elementFromPoint(drag.x,drag.y),el=getScrolls().find(candidate=>!!element&&candidate.contains(element))
  if(el){
   const rect=el.getBoundingClientRect(),header=el.querySelector('thead')!.getBoundingClientRect()
   const edge=32,bottom=rect.top+el.clientHeight
   const speed=drag.y<header.bottom+edge?-Math.min(1,(header.bottom+edge-drag.y)/edge):drag.y>bottom-edge?Math.min(1,(drag.y-bottom+edge)/edge):0
   el.scrollTop+=speed*delta*.65
  }
  update();frame=requestAnimationFrame(autoScroll)
 }
 function finish(cancel=false){
  if(!drag)return
  const current=drag;drag=undefined;preview.value=undefined
  cancelAnimationFrame(frame);frame=0;lastFrame=0
  window.removeEventListener('pointermove',move)
  window.removeEventListener('pointerup',up)
  window.removeEventListener('pointercancel',cancelDrag)
  window.removeEventListener('keydown',key,true)
  window.removeEventListener('blur',cancelDrag)
  if(scroll.value?.hasPointerCapture(current.id))scroll.value.releasePointerCapture(current.id)
  if(cancel&&current.active)selected.value=current.previous
 }
 function cancelDrag(){finish(true)}
 function key(event:KeyboardEvent){if(event.key==='Escape'){event.preventDefault();event.stopImmediatePropagation();cancelDrag()}}
 function move(event:PointerEvent){
  if(!drag||event.pointerId!==drag.id)return
  drag.x=event.clientX;drag.y=event.clientY
  if(!drag.active){
   if(Math.hypot(drag.x-drag.startX,drag.y-drag.startY)<4)return
   drag.active=true;suppressClick=true;selected.value=[...drag.sources]
   scroll.value?.setPointerCapture(drag.id)
   frame=requestAnimationFrame(autoScroll)
  }
  event.preventDefault();update()
 }
 function up(event:PointerEvent){
  if(!drag||event.pointerId!==drag.id)return
  if(drag.active)move(event)
  const sources=drag.sources,target=preview.value?.target
  finish()
  if(target)drop(sources,target)
 }
 function start(event:PointerEvent){
  const el=scroll.value,target=event.target as HTMLElement
  if(!el||disabled.value||event.button!==0||event.pointerType!=='mouse'||!target.closest('.file-entry-target'))return
  const row=target.closest('tr') as HTMLElement|null,entry=row?rows.get(row):undefined
  if(!entry||entry.link)return
  const sources=selected.value.some(source=>source.path===entry.path)?[...selected.value]:[entry]
  if(sources.some(source=>source.link))return
  event.preventDefault();el.closest<HTMLElement>('.file-panel')?.focus({preventScroll:true})
  drag={id:event.pointerId,startX:event.clientX,startY:event.clientY,x:event.clientX,y:event.clientY,previous:[...selected.value],sources,active:false}
  window.addEventListener('pointermove',move,{passive:false})
  window.addEventListener('pointerup',up)
  window.addEventListener('pointercancel',cancelDrag)
  window.addEventListener('keydown',key,true)
  window.addEventListener('blur',cancelDrag)
 }
 function resetClick(){suppressClick=false}
 function click(event:MouseEvent){if(suppressClick&&event.detail){event.preventDefault();event.stopImmediatePropagation();suppressClick=false}}
 watch(disabled,value=>{if(value)cancelDrag()},{flush:'sync'})
 watch(scroll,cancelDrag,{flush:'sync'})
 onMounted(()=>{window.addEventListener('pointerdown',resetClick,true);window.addEventListener('click',click,true)})
 onUnmounted(()=>{cancelDrag();rows.clear();elements.clear();window.removeEventListener('pointerdown',resetClick,true);window.removeEventListener('click',click,true)})
 return {preview,start,registerRow,cancelDrag}
}
