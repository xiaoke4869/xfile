<script setup lang="ts">
import { nextTick,onMounted,onUnmounted,ref,useId } from 'vue'
import { ArrowDown,Close,InfoFilled,Setting } from '@element-plus/icons-vue'
import { Browser } from '@wailsio/runtime'
import { native } from '../api'
import { version } from '../../package.json'
import appIcon from '../../../build/appicon.png'

const id=useId(),menu=ref<HTMLElement>(),trigger=ref<HTMLButtonElement>(),item=ref<HTMLButtonElement>(),about=ref<HTMLDialogElement>(),opened=ref(false)
const repository='https://github.com/xiaoke4869/xfile',linkError=ref('')
function dismiss(event:PointerEvent){if(!menu.value?.contains(event.target as Node))opened.value=false}
async function openMenu(){opened.value=true;await nextTick();item.value?.focus()}
function menuKeys(event:KeyboardEvent){
 if(event.key==='Escape'){event.preventDefault();event.stopPropagation();opened.value=false;trigger.value?.focus()}
 else if(event.key==='Tab')opened.value=false
 else if(['ArrowDown','ArrowUp','Home','End'].includes(event.key)){event.preventDefault();openMenu()}
}
function showAbout(){opened.value=false;linkError.value='';about.value?.showModal()}
async function openRepository(event:MouseEvent){
 if(!native)return
 event.preventDefault();linkError.value=''
 try{await Browser.OpenURL(repository)}catch{linkError.value='无法打开浏览器，请复制链接后访问。'}
}
onMounted(()=>window.addEventListener('pointerdown',dismiss))
onUnmounted(()=>window.removeEventListener('pointerdown',dismiss))
</script>

<template>
 <div ref="menu" class="app-settings toolbar-menu" @keydown="menuKeys">
  <button ref="trigger" class="menu-trigger settings-trigger" aria-haspopup="menu" :aria-expanded="opened" :aria-controls="`${id}-menu`" @click="opened=!opened"><Setting/><span>设置</span><ArrowDown/></button>
  <div v-if="opened" :id="`${id}-menu`" class="toolbar-menu-popup" role="menu" aria-label="设置菜单">
   <button ref="item" role="menuitem" @click="showAbout"><InfoFilled/>关于 XFile</button>
  </div>
 </div>
 <dialog ref="about" class="confirm-dialog about-dialog" :aria-labelledby="`${id}-title`" @close="trigger?.focus()">
  <button class="icon-button about-close" aria-label="关闭关于" @click="about?.close()"><Close/></button>
  <img class="about-icon" :src="appIcon" alt="XFile 应用图标" width="96" height="96"/>
  <h2 :id="`${id}-title`">XFile</h2>
  <p class="about-version">版本 {{version}}</p>
  <p class="about-credit"><a class="about-repository" :href="repository" target="_blank" rel="noopener noreferrer" title="在浏览器中打开 GitHub 仓库" @click="openRepository"><svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M12 .75a11.25 11.25 0 0 0-3.56 21.92c.56.1.77-.24.77-.54v-2.1c-3.13.68-3.79-1.33-3.79-1.33-.51-1.3-1.25-1.65-1.25-1.65-1.02-.7.08-.68.08-.68 1.13.08 1.72 1.16 1.72 1.16 1 1.72 2.63 1.22 3.27.94.1-.73.39-1.22.71-1.5-2.5-.29-5.13-1.25-5.13-5.56 0-1.23.44-2.23 1.16-3.02-.12-.28-.5-1.43.11-2.98 0 0 .95-.3 3.1 1.15a10.8 10.8 0 0 1 5.62 0c2.15-1.46 3.1-1.15 3.1-1.15.61 1.55.23 2.7.11 2.98.72.79 1.16 1.8 1.16 3.02 0 4.32-2.64 5.27-5.15 5.55.4.35.76 1.04.76 2.1v3.17c0 .3.2.65.77.54A11.25 11.25 0 0 0 12 .75Z"/></svg><span>xiaoke4869/xfile</span></a></p>
  <p v-if="linkError" class="about-link-error" role="alert">{{linkError}}</p>
 </dialog>
</template>
