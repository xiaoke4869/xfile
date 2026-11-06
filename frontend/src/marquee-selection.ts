import { onMounted,onUnmounted,ref,watch,type Ref } from 'vue'
import type { Entry } from './types'

export function useMarqueeSelection(selected:Ref<Entry[]>,disabled:Ref<boolean>){
 const scroll=ref<HTMLElement>(),box=ref<{left:number;top:number;width:number;height:number}>()
 const rows=new Map<string,{entry:Entry;element:HTMLElement}>()
 let drag:{id:number;x:number;y:number;clientX:number;clientY:number;startX:number;startY:number;previous:Entry[];additive:boolean;active:boolean}|undefined
 let frame=0,lastFrame=0,suppressClick=false
 function registerRow(entry:Entry,element:HTMLElement|null){
  if(element)rows.set(entry.path,{entry,element})
  else rows.delete(entry.path)
 }
 function update(){
  const el=scroll.value
  if(!el||!drag?.active)return
  const rect=el.getBoundingClientRect(),header=el.querySelector('thead')!.getBoundingClientRect()
  const x=Math.max(0,Math.min(el.clientWidth,drag.clientX-rect.left))+el.scrollLeft
  const y=Math.max(header.bottom-rect.top,Math.min(el.clientHeight,drag.clientY-rect.top))+el.scrollTop
  const left=Math.min(drag.x,x),top=Math.min(drag.y,y),right=Math.max(drag.x,x),bottom=Math.max(drag.y,y)
  box.value={left,top,width:right-left,height:bottom-top}
  const result=new Map((drag.additive?drag.previous:[]).map(entry=>[entry.path,entry]))
  for(const {entry,element} of rows.values()){
   if(entry.link||!el.contains(element))continue
   const bounds=element.getBoundingClientRect()
   const rowLeft=bounds.left-rect.left+el.scrollLeft,rowTop=bounds.top-rect.top+el.scrollTop
   if(right>rowLeft&&left<rowLeft+bounds.width&&bottom>rowTop&&top<rowTop+bounds.height)result.set(entry.path,entry)
  }
  const next=[...result.values()]
  if(next.length!==selected.value.length||next.some((entry,i)=>entry.path!==selected.value[i]?.path))selected.value=next
 }
 function autoScroll(time:number){
  if(!drag?.active||!scroll.value)return
  const el=scroll.value,rect=el.getBoundingClientRect(),header=el.querySelector('thead')!.getBoundingClientRect()
  const delta=Math.min(32,time-lastFrame||16);lastFrame=time
  if(drag.clientX>=rect.left&&drag.clientX<=rect.right){
   const edge=32,top=header.bottom,bottom=rect.top+el.clientHeight
   const speed=drag.clientY<top+edge?-Math.min(1,(top+edge-drag.clientY)/edge):drag.clientY>bottom-edge?Math.min(1,(drag.clientY-bottom+edge)/edge):0
   const before=el.scrollTop
   el.scrollTop+=speed*delta*.65
   if(el.scrollTop!==before)update()
  }
  frame=requestAnimationFrame(autoScroll)
 }
 function finish(cancel=false){
  if(!drag)return
  const current=drag;drag=undefined
  cancelAnimationFrame(frame);frame=0;lastFrame=0;box.value=undefined
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
  drag.clientX=event.clientX;drag.clientY=event.clientY
  if(!drag.active){
   if(Math.hypot(event.clientX-drag.startX,event.clientY-drag.startY)<4)return
   drag.active=true;suppressClick=true
   scroll.value?.setPointerCapture(drag.id)
   frame=requestAnimationFrame(autoScroll)
  }
  event.preventDefault();update()
 }
 function up(event:PointerEvent){if(event.pointerId===drag?.id){if(drag.active)move(event);finish()}}
 function start(event:PointerEvent){
  const el=scroll.value,target=event.target as HTMLElement
  if(!el||disabled.value||event.button!==0||event.pointerType!=='mouse'||target.closest('button,input,a,thead,.file-entry-target'))return
  const rect=el.getBoundingClientRect(),header=el.querySelector('thead')!.getBoundingClientRect()
  if(event.clientX>=rect.left+el.clientWidth||event.clientY>=rect.top+el.clientHeight||event.clientY<header.bottom)return
  event.preventDefault()
  el.closest<HTMLElement>('.file-panel')?.focus({preventScroll:true})
  drag={id:event.pointerId,x:event.clientX-rect.left+el.scrollLeft,y:event.clientY-rect.top+el.scrollTop,clientX:event.clientX,clientY:event.clientY,startX:event.clientX,startY:event.clientY,previous:[...selected.value],additive:event.ctrlKey||event.metaKey||event.shiftKey,active:false}
  window.addEventListener('pointermove',move,{passive:false})
  window.addEventListener('pointerup',up)
  window.addEventListener('pointercancel',cancelDrag)
  window.addEventListener('keydown',key,true)
  window.addEventListener('blur',cancelDrag)
 }
 function resetClick(){suppressClick=false}
 function click(event:MouseEvent){if(suppressClick&&event.detail){event.preventDefault();event.stopImmediatePropagation();suppressClick=false}}
 watch(disabled,value=>{if(value)cancelDrag()},{flush:'sync'})
 watch(scroll,()=>cancelDrag(),{flush:'sync'})
 onMounted(()=>{window.addEventListener('pointerdown',resetClick,true);window.addEventListener('click',click,true)})
 onUnmounted(()=>{cancelDrag();rows.clear();window.removeEventListener('pointerdown',resetClick,true);window.removeEventListener('click',click,true)})
 return {scroll,box,start,update,registerRow,cancelDrag}
}
