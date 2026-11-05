<script setup lang="ts">
import { computed,nextTick,onMounted,onUnmounted,reactive,ref,watch } from 'vue'
import { Folder,FolderOpened,Document,ArrowLeft,ArrowRight,ArrowUp,ArrowDown,Refresh,Search,Plus,Close,Operation,Clock,Check,Warning,Download,House,Monitor,Files,Loading,Setting,Delete,CopyDocument } from '@element-plus/icons-vue'
import { Call,Clipboard } from '@wailsio/runtime'
import FileRows from './components/FileRows.vue'
import AppToast from './components/AppToast.vue'
import FileIcon from './components/FileIcon.vue'
import DirectoryCount from './components/DirectoryCount.vue'
import PreviewTree from './components/PreviewTree.vue'
import HelpTip from './components/HelpTip.vue'
import FileActions from './components/FileActions.vue'
import AppSelect from './components/AppSelect.vue'
import MoveDirectoryPicker from './components/MoveDirectoryPicker.vue'
import AppSettings from './components/AppSettings.vue'
import RecentDirectories from './components/RecentDirectories.vue'
import { api,native } from './api'
import { base,parent,size,statuses,taskName,taskStatus,extractionTarget,pathKey,pathBreadcrumbs,entryType,sortEntries,type SortField } from './format'
import type { Config,Entry,Plan,Task,TreeNode,Group,Move } from './types'

const view=ref<'files'|'merge'|'structure'|'tasks'>('files'),home=ref(''),locations=ref<{name:string;path:string}[]>([]),path=ref('')
const favorites=ref<string[]>([]),favoritePicker=ref<InstanceType<typeof MoveDirectoryPicker>>()
const browsePicker=ref<InstanceType<typeof MoveDirectoryPicker>>()
const recentDirectories=ref<InstanceType<typeof RecentDirectories>>()
const sidebarMin=72,sidebarMax=ref(Math.min(360,Math.max(sidebarMin,window.innerWidth-640))),sidebarWidth=ref(220),resizingSidebar=ref(false)
const sidebarCompact=computed(()=>sidebarWidth.value<140)
let sidebarDrag:{pointerId:number;startX:number;startWidth:number}|undefined
function setSidebarWidth(width:number){sidebarWidth.value=Math.round(Math.max(sidebarMin,Math.min(sidebarMax.value,width)))}
function saveSidebarWidth(){try{localStorage.setItem('xfile.sidebarWidth',String(sidebarWidth.value))}catch{}}
function fitSidebar(){sidebarMax.value=Math.min(360,Math.max(sidebarMin,window.innerWidth-640));setSidebarWidth(sidebarWidth.value)}
function startSidebarResize(event:PointerEvent){
 if(event.button!==0)return
 event.preventDefault();(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
 sidebarDrag={pointerId:event.pointerId,startX:event.clientX,startWidth:sidebarWidth.value};resizingSidebar.value=true
}
function resizeSidebar(event:PointerEvent){if(sidebarDrag?.pointerId===event.pointerId)setSidebarWidth(sidebarDrag.startWidth+event.clientX-sidebarDrag.startX)}
function finishSidebarResize(){if(!sidebarDrag)return;sidebarDrag=undefined;resizingSidebar.value=false;saveSidebarWidth()}
function sidebarKeys(event:KeyboardEvent){
 if(!['ArrowLeft','ArrowRight','Home','End'].includes(event.key))return
 event.preventDefault();setSidebarWidth(event.key==='Home'?sidebarMin:event.key==='End'?sidebarMax.value:sidebarWidth.value+(event.key==='ArrowLeft'?-16:16));saveSidebarWidth()
}
const entries=ref<Entry[]>([]),selected=ref<Entry[]>([]),search=ref(''),showHidden=ref(false),browserLoading=ref(false)
const history=ref<string[]>([]),historyIndex=ref(-1),page=ref(0),task=ref<Task|null>(null),tasks=ref<Task[]>([])
const config=reactive<Config>({mode:'merge',duplicate:'suffix',sources:[],target:'',prefix:false,rule:'numeric',separator:'whitespace',length:3,pattern:'^([A-Z]+-[0-9]+)',unmatched:'keep',strict:true})
const plan=ref<Plan|null>(null),previewLoading=ref(false),previewError=ref(''),previewTab=ref('tree'),filter=ref('all'),detail=ref<TreeNode|null>(null),previewPage=ref(0)
const confirmDialog=ref<HTMLDialogElement|null>(null),showTask=ref(true),busy=ref(false)
const recoveryMode=ref(false)
const toast=ref<InstanceType<typeof AppToast>>()
function showToast(message:string,type:'success'|'error'='success'){toast.value?.show(message,type)}
const loadingMore=ref(false),browserVersion=ref(0)
const directorySizes=ref<Record<string,number>>({}),sizeLoading=ref<Record<string,boolean>>({}),batchSizeLoading=ref(false)
const sizeRequests=new Map<string,ReturnType<typeof Call.ByName>>()
const sizeDirectories=computed(()=>entries.value.filter(entry=>entry.dir&&!entry.link))
let sizeRevision=0
function resetDirectorySizes(){
 sizeRevision++;sizeRequests.forEach(request=>request.cancel());sizeRequests.clear()
 directorySizes.value={};sizeLoading.value={};batchSizeLoading.value=false
}
async function calculateDirectorySize(target:string){
 const pending=sizeRequests.get(target)
 if(pending){try{await pending}catch{};return}
 if(running.value||browserLoading.value)return
 if(!native){showToast('请在桌面客户端中计算文件夹大小','error');return}
 const current=sizeRevision
 const request=Call.ByName('main.FileService.CountDirectory',target,`directory-count:${crypto.randomUUID()}`)
 sizeRequests.set(target,request);sizeLoading.value[target]=true
 try{const result=await request as {bytes:number};if(current===sizeRevision)directorySizes.value[target]=result.bytes}
 catch(e){if(current===sizeRevision&&(e as Error).name!=='CancelError')showToast(`计算文件夹大小失败：${target}\n${String(e)}`,'error')}
 finally{if(current===sizeRevision){sizeRequests.delete(target);delete sizeLoading.value[target]}}
}
async function calculateAllDirectorySizes(){
 if(batchSizeLoading.value||running.value||browserLoading.value)return
 if(!native){showToast('请在桌面客户端中计算文件夹大小','error');return}
 const current=sizeRevision,targets=sizeDirectories.value.map(entry=>entry.path)
 batchSizeLoading.value=true
 try{for(const target of targets){if(current!==sizeRevision)break;await calculateDirectorySize(target)}}
 finally{if(current===sizeRevision)batchSizeLoading.value=false}
}
let defaultExtractionTarget=''
const operationName=computed(()=>config.mode==='extract'?'提取文件':'归并目录')
const fileActions=ref<InstanceType<typeof FileActions>>(),fileBusy=ref(false)
const directoriesOnly=computed(()=>selected.value.every(e=>e.dir&&!e.link))
const toolsDisabled=computed(()=>running.value||!directoriesOnly.value||(!selected.value.length&&!path.value))
const structureSource=ref(''),structureText=ref(''),structureLoading=ref(false),structureError=ref(''),exportingStructure=ref(false)
let structureRevision=0
let revision=0,previewTimer:ReturnType<typeof setTimeout>|undefined,poll:ReturnType<typeof setInterval>|undefined,navRevision=0
const running=computed(()=>fileBusy.value||busy.value||task.value?.status==='running')
const sortField=ref<SortField>('name'),sortDirection=ref<'asc'|'desc'>('asc')
const sortOptions:ReadonlyArray<{value:SortField;label:string}>=[{value:'name',label:'名称'},{value:'size',label:'大小'},{value:'type',label:'类型'},{value:'modified',label:'修改日期'}]
const activeMenu=ref(''),toolbarMenus=ref<HTMLElement>()
const folderDialog=ref<HTMLDialogElement>(),folderInput=ref<HTMLInputElement>(),folderName=ref(''),folderError=ref(''),creatingFolder=ref(false)
const infoDialog=ref<HTMLDialogElement>(),infoEntries=ref<Entry[]>([])
const operationDialog=ref<HTMLDialogElement>(),operationTask=ref<Task|null>(null)
const logDeleteDialog=ref<HTMLDialogElement>(),pendingLogDelete=ref<string|null>(null),deletingLogs=ref(false)
const searchDialog=ref<HTMLDialogElement>(),searchInput=ref<HTMLInputElement>()
const jumpDialog=ref<HTMLDialogElement>(),jumpInput=ref<HTMLInputElement>(),jumpPath=ref(''),navigationError=ref(''),jumping=ref(false)
async function openJump(){
 activeMenu.value='';jumpPath.value=path.value||home.value;navigationError.value='';jumpDialog.value?.showModal()
 await nextTick();jumpInput.value?.focus();jumpInput.value?.select()
}
async function jumpDirectory(){
 const target=jumpPath.value.trim();if(!target||jumping.value)return
 jumping.value=true;navigationError.value=''
 try{
  const entry=await api<Entry>('EntryInfo',target)
  if(!entry.dir)throw new Error('请输入目录路径，不能跳转到文件')
  if(await navigate(entry.path))jumpDialog.value?.close()
 }catch(e){navigationError.value=String(e)}finally{jumping.value=false}
}
async function openSearch(){
 activeMenu.value='';searchDialog.value?.showModal()
 await nextTick();searchInput.value?.focus();searchInput.value?.select()
}
const statusPath=computed(()=>selected.value.length===1?selected.value[0].path:path.value)
const breadcrumbs=computed(()=>pathBreadcrumbs(statusPath.value)),pathBar=ref<HTMLElement>()
function breadcrumbIsFolder(index:number){return index<breadcrumbs.value.length-1||selected.value.length!==1||selected.value[0].dir&&!selected.value[0].link}
watch(statusPath,async()=>{await nextTick();if(pathBar.value)pathBar.value.scrollLeft=pathBar.value.scrollWidth})
// Keep one active sort field shared by the header and menu.
function changeSort(field:SortField){
 sortDirection.value=sortField.value===field&&sortDirection.value==='asc'?'desc':'asc';sortField.value=field;page.value=0
}
function dismissMenu(event:PointerEvent){if(!toolbarMenus.value?.contains(event.target as Node))activeMenu.value=''}
function menuKeys(event:KeyboardEvent){
 const group=(event.target as HTMLElement).closest('.toolbar-menu');if(!group)return
 if(event.key==='Escape'){event.preventDefault();activeMenu.value='';group.querySelector<HTMLButtonElement>('.menu-trigger')?.focus();return}
 if(event.key==='Tab'){activeMenu.value='';return}
 if(!['ArrowDown','ArrowUp','Home','End'].includes(event.key))return
 event.preventDefault()
 const buttons=Array.from(group.querySelectorAll<HTMLButtonElement>('.toolbar-menu-popup button:not(:disabled)'))
 const index=buttons.indexOf(document.activeElement as HTMLButtonElement)
 const next=event.key==='Home'?0:event.key==='End'?buttons.length-1:event.key==='ArrowUp'?(index<0?buttons.length-1:(index+buttons.length-1)%buttons.length):(index+1)%buttons.length
 buttons[next]?.focus()
}
async function copyPath(target=statusPath.value){
 if(!target)return
 try{if(native)await Clipboard.SetText(target);else await navigator.clipboard.writeText(target);showToast('已复制路径')}catch(e){showToast('复制路径失败：'+String(e),'error')}
}
async function newFolder(){
 activeMenu.value='';folderName.value='新建文件夹';folderError.value='';folderDialog.value?.showModal()
 await nextTick();folderInput.value?.select()
}
async function createFolder(){
 if(creatingFolder.value||running.value)return
 creatingFolder.value=true;folderError.value=''
 try{await api<string>('CreateFolder',path.value,folderName.value);folderDialog.value?.close();await refreshFiles();showToast('文件夹已创建')}catch(e){folderError.value=String(e)}finally{creatingFolder.value=false}
}
async function showInfo(){
 activeMenu.value=''
 try{infoEntries.value=selected.value.length?[...selected.value]:[await api<Entry>('EntryInfo',path.value)];infoDialog.value?.showModal()}catch(e){showToast(String(e),'error')}
}
const currentName=computed(()=>base(path.value)||'文件')
const visible=computed(()=>sortEntries(entries.value.filter(e=>(showHidden.value||!e.name.startsWith('.'))&&e.name.toLowerCase().includes(search.value.toLowerCase())),sortField.value,sortDirection.value,directorySizes.value))
const pageEntries=computed(()=>visible.value.slice(page.value*200,page.value*200+200))
const selectedPaths=computed(()=>selected.value.map(e=>e.path))
const canExecute=computed(()=>!running.value&&!previewLoading.value&&!previewError.value&&!!plan.value&&!plan.value.issues.length&&plan.value.fileCount>0&&plan.value.revision===revision)
const progress=computed(()=>task.value?.total?Math.round(task.value.moved/task.value.total*100):0)
const filteredMoves=computed(()=>plan.value?.files.filter(f=>filter.value!=='renamed'||f.renamed)||[])
const movePage=computed(()=>filteredMoves.value.slice(previewPage.value*100,previewPage.value*100+100))
const unmatched=computed(()=>plan.value?.groups.filter(g=>!g.matched).length||0)
const tree=computed(()=>{
 const root:TreeNode={key:'',name:'',dir:true,existing:false,renamed:false,sources:[],children:[]}
 const byPath=new Map<string,TreeNode>()
 function add(target:string,dir:boolean,existing:boolean,source?:string,renamed=false){
  const targetRoot=plan.value!.config.target.replace(/[\\/]+$/,'')
  const rel=target.slice(targetRoot.length).replace(/^[\\/]+/,'');if(!rel)return
  const parts=rel.split(/[\\/]/);let node=root;let key=''
  parts.forEach((name,index)=>{key+=(key?'/':'')+name;let child=byPath.get(key);if(!child){child={key,name,dir:index<parts.length-1||dir,existing,renamed:false,sources:[],children:[]};node.children.push(child);byPath.set(key,child)};if(existing)child.existing=true;if(index===parts.length-1){if(source&&!child.sources.includes(source))child.sources.push(source);child.renamed=renamed};node=child})
 }
 if(!plan.value)return []
 for(const e of plan.value.existing)add(e.path,e.dir,true)
 for(const g of plan.value.groups){const dest=plan.value.config.target.replace(/[\\/]+$/,'')+'/'+g.name;add(dest,true,false,g.source);const n=byPath.get(g.name);if(n){n.group??=[];n.group.push(g)}}
 for(const f of plan.value.files)add(f.target,false,false,f.source,f.renamed)
 function sort(n:TreeNode){n.children.sort((a,b)=>Number(b.dir)-Number(a.dir)||a.name.localeCompare(b.name,'zh-CN',{numeric:true}));n.children.forEach(sort)}sort(root);return root.children
})

async function navigate(to:string,record=true,remember=record){
 activeMenu.value='';resetDirectorySizes();const id=++navRevision;browserLoading.value=true;navigationError.value=''
 try{const data=await api<Entry[]>('List',to);if(id!==navRevision)return false;entries.value=data;browserVersion.value++;path.value=to;selected.value=[];page.value=0;search.value='';if(record){history.value=history.value.slice(0,historyIndex.value+1);history.value.push(to);historyIndex.value=history.value.length-1};if(remember)recentDirectories.value?.record(to);return true}
 catch(e){if(id===navRevision){navigationError.value=String(e);if(!jumping.value)showToast(navigationError.value,'error')};return false}finally{if(id===navRevision)browserLoading.value=false}
}
async function chooseBrowse(){activeMenu.value='';try{await browsePicker.value?.open(path.value||home.value)}catch(e){showToast(String(e),'error')}}
async function chooseFavorite(){try{await favoritePicker.value?.open(path.value||home.value)}catch(e){showToast(String(e),'error')}}
function saveFavorites(paths:string[]){
 try{localStorage.setItem('xfile.favorites',JSON.stringify(paths));favorites.value=paths;return true}
 catch(e){showToast('保存收藏失败：'+String(e),'error');return false}
}
async function pinFavorite(target:string){
 const exists=locations.value.some(loc=>pathKey(loc.path)===pathKey(target))||favorites.value.some(p=>pathKey(p)===pathKey(target))
 if(!exists&&!saveFavorites([...favorites.value,target]))return
 showToast(exists?'目录已在侧边栏':'已固定到侧边栏');view.value='files';await navigate(target)
}
function unpinFavorite(target:string){saveFavorites(favorites.value.filter(p=>p!==target))}
function goHistory(n:number){const next=historyIndex.value+n;if(next>=0&&next<history.value.length){historyIndex.value=next;navigate(history.value[next],false,true)}}
function select(entry:Entry,event:MouseEvent){if(entry.link||running.value)return;const i=selected.value.findIndex(e=>e.path===entry.path);const additive=event.metaKey||event.ctrlKey||event.shiftKey||(event.target as HTMLElement).tagName==='INPUT';if(additive){if(i>=0)selected.value.splice(i,1);else selected.value.push(entry)}else{selected.value=[entry]}}
function selectAll(invert=false){
 if(running.value||browserLoading.value)return
 const previous=new Set(selectedPaths.value)
 selected.value=visible.value.filter(e=>!e.link&&(!invert||!previous.has(e.path)))
}
function fileContext(event:MouseEvent,entry?:Entry){
 if(running.value)return
 if(entry){if(entry.link)return;if(!selected.value.some(e=>e.path===entry.path))selected.value=[entry]}else selected.value=[]
 fileActions.value?.open(event,!entry)
}
function contextKey(event:KeyboardEvent){
 if(event.key==='ContextMenu'||event.shiftKey&&event.key==='F10'){event.preventDefault();const rect=(event.currentTarget as HTMLElement).getBoundingClientRect();fileContext(new MouseEvent('contextmenu',{clientX:rect.left+40,clientY:rect.top+40}),selected.value[0])}
}
async function refreshFiles(){if(path.value)await navigate(path.value,false);await loadTasks()}
function beginMerge(mode:'merge'|'extract'='merge'){
 if(toolsDisabled.value)return
 const sources=selected.value.length?selected.value.map(e=>({path:e.path,depth:0})):path.value?[{path:path.value,depth:0}]:[]
 if(!sources.length)return
 recoveryMode.value=false;showTask.value=false;config.mode=mode;config.sources=sources;defaultExtractionTarget=mode==='extract'?extractionTarget(sources[0]!.path):'';config.target=defaultExtractionTarget;plan.value=null;previewTab.value='tree';view.value='merge';schedulePreview()
}
function beginStructure(){
 if(toolsDisabled.value)return
 if(selected.value.length>1){showToast('查看文件结构时请选择一个目录，或取消选择后导出当前目录','error');return}
 const source=selected.value[0]?.path||path.value;if(!source)return
 structureSource.value=source;structureText.value='';showTask.value=false;view.value='structure';refreshStructure()
}
async function refreshStructure(){
 const current=++structureRevision;structureLoading.value=true;structureError.value=''
 try{const text=await api<string>('DirectoryText',structureSource.value);if(current===structureRevision)structureText.value=text}catch(e){if(current===structureRevision)structureError.value=String(e)}finally{if(current===structureRevision)structureLoading.value=false}
}
async function exportStructure(){
 exportingStructure.value=true
 try{const p=await api<string>('ExportDirectoryText',structureSource.value);if(p){showToast('目录结构已保存：'+p);if(path.value)await navigate(path.value,false);await refreshStructure()}}catch(e){showToast(String(e),'error')}finally{exportingStructure.value=false}
}
async function addSource(){try{const p=await api<string>('ChooseDirectory');if(p&&!config.sources.some(s=>s.path===p))config.sources.push({path:p,depth:0})}catch(e){previewError.value=String(e)}}
async function chooseTarget(){try{const p=await api<string>('ChooseDirectory');if(p)config.target=p}catch(e){previewError.value=String(e)}}
function schedulePreview(refresh=false){
 if(view.value!=='merge'||running.value||recoveryMode.value)return
 clearTimeout(previewTimer);const current=++revision;previewLoading.value=true;previewError.value='';detail.value=null
 previewTimer=setTimeout(async()=>{
  if(!config.sources.length||!config.target){if(current===revision){plan.value=null;previewLoading.value=false};return}
  try{const result=await api<Plan>('Preview',JSON.parse(JSON.stringify(config)),current,refresh);if(current!==revision)return;plan.value=result;previewPage.value=0;saveSettings()}
  catch(e){if(current===revision)previewError.value=String(e)}finally{if(current===revision)previewLoading.value=false}
 },250)
}
watch(()=>config.sources[0]?.path,source=>{
 if(config.mode==='extract'&&!recoveryMode.value&&(!config.target||config.target===defaultExtractionTarget)){
  defaultExtractionTarget=source?extractionTarget(source):'';config.target=defaultExtractionTarget
 }
})
watch(()=>JSON.stringify(config),()=>schedulePreview())
watch([search,showHidden],()=>{page.value=0;if(!showHidden.value)selected.value=selected.value.filter(e=>!e.name.startsWith('.'))})
watch(view,()=>{activeMenu.value='';if(view.value!=='files')resetDirectorySizes()})
watch(running,value=>{if(value)resetDirectorySizes()})
function saveSettings(){const {sources,target,mode,...settings}=config;localStorage.setItem('xfile.settings',JSON.stringify(settings))}
async function loadMore(){if(!plan.value||loadingMore.value)return;loadingMore.value=true;const id=plan.value.id;try{
 if(plan.value.files.length<plan.value.fileCount){const data=await api<Move[]>('PlanPage',id,plan.value.files.length,false);if(plan.value?.id===id)plan.value.files.push(...data)}
 if(plan.value?.id===id&&plan.value.existing.length<plan.value.existingCount){const data=await api<Entry[]>('PlanPage',id,plan.value.existing.length,true);if(plan.value?.id===id)plan.value.existing.push(...data)}
 }catch(e){previewError.value=String(e)}finally{loadingMore.value=false}}
async function start(){
 if(!plan.value||!canExecute.value)return
 confirmDialog.value?.close();busy.value=true;showTask.value=true
 try{await api<string>('Start',plan.value.id,plan.value.revision);task.value=await api<Task>('Task');startPolling()}
 catch(e){showToast(String(e),'error')}finally{busy.value=false}
}
function startPolling(){if(poll)clearInterval(poll);let checking=false;poll=setInterval(async()=>{if(checking)return;checking=true;try{const t=await api<Task>('Task');task.value=t;if(t.status!=='running'){clearInterval(poll);poll=undefined;plan.value=null;recoveryMode.value=false;await loadTasks();if(t.status==='succeeded')showToast('移动完成，文件数量与内容校验通过');if(path.value)await navigate(path.value,false);if(view.value==='merge')schedulePreview(true)}}catch(e){showToast(String(e),'error')}finally{checking=false}},500)}
async function cancelTask(){try{await api('Cancel')}catch(e){showToast(String(e),'error')}}
async function loadTasks(){try{tasks.value=await api<Task[]>('Tasks')}catch(e){showToast(String(e),'error')}}
async function requestLogDelete(id=''){
 if(running.value||deletingLogs.value)return
 pendingLogDelete.value=id;await nextTick();logDeleteDialog.value?.showModal()
}
async function deleteLogs(){
 const id=pendingLogDelete.value
 if(id===null||running.value||deletingLogs.value)return
 deletingLogs.value=true
 try{
  if(id)await api('DeleteTask',id);else await api('ClearTasks')
  if(task.value&&(!id||task.value.id===id)){task.value=null;showTask.value=false}
  if(operationTask.value&&(!id||operationTask.value.id===id))operationDialog.value?.close()
  if(plan.value?.recoveryFrom&&(!id||plan.value.recoveryFrom===id)){plan.value=null;recoveryMode.value=false}
  logDeleteDialog.value?.close();showToast(id?'记录已删除':'全部记录已清空')
 }catch(e){showToast(String(e),'error')}
 finally{await loadTasks();deletingLogs.value=false}
}
async function openReports(id:string){try{await api('OpenReports',id)}catch(e){showToast(String(e),'error')}}
async function exportReports(id:string){try{const p=await api<string>('ExportReports',id);if(p)showToast('报告已导出到 '+p)}catch(e){showToast(String(e),'error')}}
async function inspectTask(t:Task){
 if(t.operation){try{operationTask.value=await api<Task>('InspectTask',t.id);await nextTick();operationDialog.value?.showModal()}catch(e){showToast(String(e),'error')};return}
 task.value=t;showTask.value=true;if(t.status==='recovery_required'){try{task.value=await api<Task>('InspectTask',t.id);await loadTasks()}catch(e){showToast(String(e),'error')}}
}
async function prepareRecovery(id:string){
 clearTimeout(previewTimer);const current=++revision;recoveryMode.value=true;view.value='merge';previewLoading.value=true;previewError.value='';detail.value=null;showTask.value=false
 try{const p=await api<Plan>('PrepareRecovery',id,current);if(current!==revision)return;Object.assign(config,{mode:'merge',duplicate:'suffix'},p.config);plan.value=p;showToast('恢复计划仅包含尚未移动的文件。规则沿用原任务，已完成文件不会重复处理。')}catch(e){previewError.value=String(e)}finally{previewLoading.value=false}
}
function inspectGroup(g:Group){detail.value={key:g.name,name:g.name||base(config.target),dir:true,existing:false,renamed:!!g.name&&g.original!==g.name,sources:[g.source],children:[],group:[g]}}
function taskPercent(t:Task){return t.total?Math.round(t.moved/t.total*100):0}
onMounted(async()=>{
 window.addEventListener('pointerdown',dismissMenu)
 window.addEventListener('resize',fitSidebar)
 try{const saved=Number(localStorage.getItem('xfile.sidebarWidth'));if(Number.isFinite(saved)&&saved>0)setSidebarWidth(saved)}catch{}
 try{const saved:unknown=JSON.parse(localStorage.getItem('xfile.favorites')||'[]');if(Array.isArray(saved))favorites.value=saved.filter((p):p is string=>typeof p==='string'&&!!p.trim()).filter((p,i,all)=>all.findIndex(other=>pathKey(other)===pathKey(p))===i)}catch{}
 try{const saved=JSON.parse(localStorage.getItem('xfile.settings')||'{}');for(const k of ['duplicate','prefix','rule','separator','length','pattern','unmatched','strict'] as const){if(k in saved)(config as any)[k]=saved[k]}}catch{}
 if(!native)return
 try{const info=await api<{home:string;locations:{name:string;path:string}[]}>('Home');home.value=info.home;locations.value=info.locations;await navigate(home.value);await loadTasks()}catch(e){showToast(String(e),'error')}
})
onUnmounted(()=>{resetDirectorySizes();window.removeEventListener('pointerdown',dismissMenu);window.removeEventListener('resize',fitSidebar);clearTimeout(previewTimer);if(poll)clearInterval(poll)})
</script>

<template>
 <AppToast ref="toast"/>
 <div class="app-shell" :class="{'sidebar-compact':sidebarCompact,'sidebar-resizing':resizingSidebar}" :style="{'--sidebar-width':`${sidebarWidth}px`}">
  <aside id="app-sidebar" class="sidebar" aria-label="侧边栏">
   <div class="window-drag-region" aria-hidden="true"></div>
   <div class="sidebar-section">工作空间</div>
   <button :class="['nav-item',{active:view!=='tasks'}]" aria-label="文件浏览" title="文件浏览" @click="view='files'"><FolderOpened/><span class="nav-label">文件浏览</span></button>
   <button :class="['nav-item',{active:view==='tasks'}]" aria-label="任务与日志" title="任务与日志" @click="view='tasks';loadTasks()"><Clock/><span class="nav-label">任务与日志</span><span v-if="tasks.length" class="nav-count">{{tasks.length}}</span></button>
   <div class="sidebar-section section-gap locations-heading"><span>常用位置</span><button class="icon-button" aria-label="收藏目录" title="收藏目录" @click="chooseFavorite"><Plus/></button></div>
   <div class="sidebar-locations">
    <button v-for="(loc,i) in locations" :key="loc.path" :class="['nav-item location',{active:view==='files'&&pathKey(path)===pathKey(loc.path)}]" :aria-label="loc.name" :title="`${loc.name}：${loc.path}`" @click="view='files';navigate(loc.path)"><House v-if="i===0"/><Monitor v-else-if="loc.name==='桌面'"/><Download v-else-if="loc.name==='下载'"/><Document v-else/><span class="nav-label ellipsis">{{loc.name}}</span></button>
    <div v-for="favorite in favorites" :key="favorite" class="favorite-location">
     <button :class="['nav-item location',{active:view==='files'&&pathKey(path)===pathKey(favorite)}]" :aria-label="base(favorite)" :title="favorite" @click="view='files';navigate(favorite)"><FileIcon :name="favorite" dir/><span class="nav-label ellipsis">{{base(favorite)}}</span></button>
     <button class="icon-button unpin-favorite" :aria-label="`取消收藏 ${base(favorite)}`" :title="`取消收藏 ${base(favorite)}`" @click="unpinFavorite(favorite)"><Close/></button>
    </div>
   </div>
   <div class="sidebar-resizer" role="separator" tabindex="0" aria-label="调整侧边栏宽度" aria-orientation="vertical" aria-controls="app-sidebar" :aria-valuemin="sidebarMin" :aria-valuemax="sidebarMax" :aria-valuenow="sidebarWidth" @pointerdown="startSidebarResize" @pointermove="resizeSidebar" @pointerup="finishSidebarResize" @pointercancel="finishSidebarResize" @lostpointercapture="finishSidebarResize" @keydown="sidebarKeys"></div>
  </aside>
  <main :class="{'browser-preview':!native}">
   <AppSettings/>
   <div v-if="!native" class="banner amber-banner">浏览器界面预览 · 本地文件功能请在 XFile 桌面客户端中使用</div>

   <template v-if="view==='files'">
    <header class="toolbar browser-toolbar">
     <div class="nav-buttons"><button class="icon-button" aria-label="后退" :disabled="historyIndex<=0" @click="goHistory(-1)"><ArrowLeft/></button><button class="icon-button" aria-label="前进" :disabled="historyIndex>=history.length-1" @click="goHistory(1)"><ArrowRight/></button><button class="icon-button" aria-label="上一级" :disabled="!path||parent(path)===path" @click="navigate(parent(path))"><ArrowUp/></button></div>
     <h1 class="directory-title" :title="currentName">{{currentName}}</h1>
     <div class="toolbar-actions">
      <div ref="toolbarMenus" class="toolbar-menus" @keydown="menuKeys">
       <div v-for="menu in ['文件','功能','操作','查看']" :key="menu" class="toolbar-menu">
        <button class="menu-trigger" aria-haspopup="menu" :aria-expanded="activeMenu===menu" @click="activeMenu=activeMenu===menu?'':menu" @keydown.down.prevent.stop="activeMenu=menu;nextTick(()=>toolbarMenus?.querySelector<HTMLButtonElement>('.toolbar-menu-popup button:not(:disabled)')?.focus())">{{menu}}<ArrowDown/></button>
        <div v-if="activeMenu===menu" class="toolbar-menu-popup" role="menu" :aria-label="menu+'菜单'" @click="($event.target as HTMLElement).closest('button')&&(activeMenu='')">
         <template v-if="menu==='文件'">
          <button role="menuitem" :disabled="!path||running||browserLoading" @click="newFolder"><Plus/>新建文件夹</button>
          <button role="menuitem" :disabled="!path||browserLoading" @click="showInfo"><Document/>显示简介</button>
          <div class="context-separator" role="separator"></div><div class="context-caption">排序方式</div>
          <button v-for="option in sortOptions" :key="option.value" role="menuitemradio" :aria-checked="sortField===option.value" @click="changeSort(option.value)"><Check v-if="sortField===option.value"/><span v-else class="menu-icon-space"></span>{{option.label}}<span v-if="sortField===option.value" class="sort-direction">{{sortDirection==='asc'?'升序':'降序'}}</span></button>
         </template>
         <template v-else-if="menu==='功能'">
          <button role="menuitem" :disabled="toolsDisabled" @click="beginMerge()"><Operation/>归并目录</button>
          <button role="menuitem" :disabled="toolsDisabled" @click="beginMerge('extract')"><Files/>递归提取文件</button>
          <button role="menuitem" :disabled="toolsDisabled||selected.length>1" @click="beginStructure"><Document/>查看目录结构</button>
         </template>
         <button v-else-if="menu==='操作'" role="menuitem" @click="openJump"><ArrowRight/>跳转目录</button>
         <button v-else role="menuitemcheckbox" :aria-checked="showHidden" @click="showHidden=!showHidden"><Check v-if="showHidden"/><span v-else class="menu-icon-space"></span>查看隐藏项目</button>
        </div>
       </div>
      </div>
      <button class="icon-button" aria-label="打开目录" title="打开目录" @click="chooseBrowse"><FolderOpened/></button>
      <button class="icon-button" aria-label="刷新当前目录" title="刷新当前目录" :disabled="!path||browserLoading" @click="refreshFiles"><Refresh :class="{spin:browserLoading}"/></button>
      <button class="icon-button" :class="{'search-active':search}" aria-label="搜索当前目录" :title="search?`搜索：${search}`:'搜索当前目录'" @click="openSearch"><Search/></button>
      <button v-if="search" class="icon-button" aria-label="清除搜索" title="清除搜索" @click="search=''"><Close/></button>
     </div>
    </header>
    <div class="browser-content">
     <section class="file-panel" tabindex="0" aria-label="文件列表" @contextmenu.prevent="fileContext($event)" @keydown="contextKey">
      <!-- Keep both headers in table flow: WebKit stretches absolute thead children to the table height. -->
      <div class="table-scroll"><table class="file-table"><colgroup><col v-for="option in sortOptions" :key="option.value"/></colgroup><thead>
       <tr v-if="selected.length"><th :colspan="sortOptions.length" class="selection-heading"><div class="selection-actions" role="group" aria-label="批量选择"><span>已选 {{selected.length}} 项</span><button class="text-button" :disabled="running||browserLoading" title="全选当前目录的搜索结果，包含所有分页" @click="selectAll()">全选</button><button class="text-button" :disabled="running||browserLoading" title="反选当前目录的搜索结果，包含所有分页" @click="selectAll(true)">反选</button><button class="icon-button" aria-label="取消选择" title="取消选择" :disabled="running" @click="selected=[]"><Close/></button></div></th></tr>
       <tr v-else><th v-for="option in sortOptions" :key="option.value" :aria-sort="sortField===option.value?(sortDirection==='asc'?'ascending':'descending'):'none'"><div class="column-heading"><button class="sort-heading" :class="{active:sortField===option.value}" @click="changeSort(option.value)">{{option.label}}<ArrowUp v-if="sortField===option.value&&sortDirection==='asc'"/><ArrowDown v-else-if="sortField===option.value"/></button><button v-if="option.value==='size'&&sizeDirectories.length" class="text-button" aria-label="计算当前目录全部文件夹大小" :aria-busy="batchSizeLoading" :disabled="batchSizeLoading||running||browserLoading" :title="batchSizeLoading?'正在计算文件夹大小':'批量计算当前目录下全部文件夹的大小'" @click="calculateAllDirectorySizes">计算</button></div></th></tr></thead><tbody><FileRows :key="browserVersion" :entries="pageEntries" :sort-field="sortField" :sort-direction="sortDirection" :show-hidden="showHidden" :selected="selectedPaths" :directory-sizes="directorySizes" :size-loading="sizeLoading" :size-disabled="running||browserLoading||batchSizeLoading" @calculate-size="calculateDirectorySize" @select="select" @navigate="navigate" @context="fileContext" @error="showToast($event,'error')"/></tbody></table>
       <div v-if="browserLoading" class="empty-state compact"><Loading class="spin"/><p>正在读取目录…</p></div>
       <div v-else-if="!visible.length" class="empty-state"><FolderOpened/><h2>{{path?'此目录没有可显示的项目':'从一个目录开始'}}</h2><button v-if="!path" class="button primary" @click="chooseBrowse">选择本地目录</button></div>
      </div>
      <div v-if="visible.length>200" class="pagination"><button class="button small" :disabled="page===0" @click="page--">上一页</button><span>{{page+1}} / {{Math.ceil(visible.length/200)}}</span><button class="button small" :disabled="(page+1)*200>=visible.length" @click="page++">下一页</button></div>
     </section>
    </div>
    <footer class="browser-status">
     <nav ref="pathBar" class="path-breadcrumbs" aria-label="目录路径"><ol v-if="breadcrumbs.length"><li v-for="(crumb,index) in breadcrumbs" :key="crumb.path"><ArrowRight v-if="index" class="breadcrumb-separator" aria-hidden="true"/><button class="breadcrumb-button" :title="crumb.path" :aria-label="`${breadcrumbIsFolder(index)?'进入目录':'文件'} ${crumb.path}`" :aria-current="pathKey(crumb.path)===pathKey(path)?'location':undefined" :disabled="!breadcrumbIsFolder(index)" @click="navigate(crumb.path)"><Monitor v-if="index===0" class="breadcrumb-root" aria-hidden="true"/><FileIcon v-else :name="crumb.name" :dir="breadcrumbIsFolder(index)"/><span>{{crumb.name}}</span></button></li></ol><span v-else class="muted">尚未选择目录</span></nav>
     <span class="status-count" role="status" aria-live="polite">{{visible.length}} 个项目</span>
     <button class="icon-button copy-status-path" :disabled="!statusPath" :title="statusPath?`复制路径：${statusPath}`:'复制路径'" aria-label="复制当前路径" @click="copyPath()"><CopyDocument/></button>
     <RecentDirectories ref="recentDirectories" @navigate="navigate($event)" @error="showToast($event,'error')"/>
    </footer>
   </template>

   <template v-else-if="view==='merge'">
    <header class="workspace-header"><button class="icon-button" aria-label="返回文件浏览" @click="view='files'"><ArrowLeft/></button><div><h1 class="with-help">{{operationName}}<HelpTip :label="operationName+'说明'" :text="config.mode==='extract'?'递归提取所有普通文件，集中移动到一个目录。':'调整规则，在右侧查看操作后的目录结构。'"/></h1></div><span class="badge neutral">预览阶段不修改文件</span></header>
    <div class="merge-workspace">
     <section class="config-panel"><fieldset :disabled="running||recoveryMode">
      <div class="form-section"><div class="section-title"><h2 class="with-help">来源目录<HelpTip label="来源目录说明" :text="config.mode==='extract'?'递归处理全部层级，目标子目录会自动排除。':'深度 0：直接处理选中目录及其全部内容。'"/></h2><button class="text-button" @click="addSource"><Plus/>添加</button></div>
       <div v-for="(source,i) in config.sources" :key="source.path" class="source-card"><div class="source-name"><FileIcon :name="source.path" dir/><strong>{{base(source.path)}}</strong><button class="icon-button" :aria-label="`移除来源 ${base(source.path)}`" @click="config.sources.splice(i,1)"><Close/></button></div><div class="source-path" :title="source.path">{{source.path}}</div><label v-if="config.mode!=='extract'" class="depth-field"><span class="with-help">查找相对深度<HelpTip label="查找深度说明" text="深度 0 直接处理选中目录及其全部内容；深度 1 从直接子目录查找，依此类推。"/></span><input type="number" v-model.number="source.depth" min="0" max="128" aria-label="查找相对深度"/></label></div>
       <div v-if="!config.sources.length" class="mini-empty">请添加至少一个来源目录</div>
      </div>
      <div class="form-section"><h2 class="with-help">目标目录<HelpTip label="目标目录说明" :text="(config.mode==='extract'?'默认保存到第一个来源目录下的 result-当天日期，可自行修改目标路径。':'')+'目标不存在时，执行移动时自动创建。可以位于来源目录内；整个目标目录会从来源扫描中排除，已有文件保留。'"/></h2><div class="target-input"><input v-model="config.target" placeholder="选择目录或输入新目录路径" aria-label="目标目录"/><button class="icon-button" aria-label="选择目标目录" @click="chooseTarget"><FolderOpened/></button></div>
      </div>
      <div v-if="config.mode!=='extract'" class="form-section"><h2 class="with-help">目标目录名<HelpTip label="目标目录名说明" text="默认使用完整目录名，例如“1.2 检验资料”。启用前缀后，可按规则提取目录名。"/></h2><label class="option-check"><input type="checkbox" v-model="config.prefix"/><span>仅保留目录名前缀</span></label>
       <template v-if="config.prefix"><label class="form-label"><span class="with-help">提取方式<HelpTip label="前缀提取说明" :text="config.rule==='numeric'?'1.2 检验资料 → 1.2。支持 001、1.2.3；编号后须为空白或名称结束。':config.rule==='separator'?'取第一个分隔符之前的内容。':config.rule==='length'?'提取名称开头指定数量的可见字符。':'从开头匹配，有捕获组时取第一组。使用 Go 正则语法。'"/></span><AppSelect v-model="config.rule" :disabled="running||recoveryMode" aria-label="前缀提取方式" :options="[{value:'numeric',label:'数字编号（推荐）'},{value:'separator',label:'分隔符之前'},{value:'length',label:'前 N 个字符'},{value:'regex',label:'自定义正则（高级）'}]"/></label>
        <template v-if="config.rule==='separator'"><label class="form-label"><span class="with-help">分隔文本<HelpTip label="分隔文本说明" text="取第一个分隔符之前的内容。whitespace 表示空白，也可输入 _、- 等。"/></span><input v-model="config.separator" aria-label="分隔文本" placeholder="输入分隔文本"/></label><div class="preset-buttons"><button @click="config.separator='whitespace'">空白</button><button @click="config.separator='_'">下划线 _</button><button @click="config.separator='-'">短横线 -</button></div></template>
        <label v-if="config.rule==='length'" class="form-label"><span class="with-help">字符数量<HelpTip label="字符数量说明" text="中文与 emoji 按可见字符计算。"/></span><input type="number" v-model.number="config.length" min="1" max="255" aria-label="前缀字符数量"/></label>
        <label v-if="config.rule==='regex'" class="form-label"><span class="with-help">正则表达式<HelpTip label="正则表达式说明" text="从开头匹配，有捕获组时取第一组。使用 Go 正则语法。"/></span><input class="mono" v-model="config.pattern" aria-label="正则表达式"/></label>
        <label class="form-label">未匹配时<AppSelect v-model="config.unmatched" :disabled="running||recoveryMode" aria-label="未匹配处理" :options="[{value:'keep',label:'保留完整名称并提示'},{value:'block',label:'阻止执行，先修正规则'}]"/></label>
       </template>
      </div>
      <div v-if="config.mode==='extract'" class="form-section"><h2 class="with-help">重复名称如何命名<HelpTip label="重名处理说明" text="保留扩展名，连续寻找可用名称。已有文件和本批次文件均不会被覆盖。"/></h2><label class="form-label">重名处理<AppSelect v-model="config.duplicate" :disabled="running||recoveryMode" aria-label="重名处理" :options="[{value:'suffix',label:'追加序号：文件名__2.ext'},{value:'parentheses',label:'括号序号：文件名 (2).ext'},{value:'block',label:'遇到重名阻止执行'}]"/></label></div>
      <div class="form-section safety-summary"><h2 class="with-help">文件保护<HelpTip label="文件保护说明" :text="(config.mode==='extract'?'按所选规则处理重名，不覆盖文件；文件集中到目标根目录，保留源空目录。':'同名文件自动添加 __2、__3；保留内部子目录结构。')+'逐文件校验内容并记录移动日志；跨磁盘验证副本后才删除来源。'"/></h2></div>
     </fieldset></section>
     <section class="preview-panel">
      <div class="preview-heading"><div><h2 class="with-help">操作后目录<HelpTip label="操作后目录说明" text="设置完成后，目录结构、合并关系和文件改名会自动显示在这里。更改配置后自动更新，也可重新扫描。"/> <span class="muted">{{recoveryMode?'恢复任务预览':'预览'}}</span></h2><p :title="config.target">{{config.target||'请选择目标目录'}}</p></div><button class="button small" :disabled="running" @click="recoveryMode&&plan?.recoveryFrom?prepareRecovery(plan.recoveryFrom):schedulePreview(true)"><Refresh :class="{spin:previewLoading}"/>重新扫描</button></div>
      <div class="preview-tabs"><button :class="{active:previewTab==='tree'}" @click="previewTab='tree'">目录结构</button><button :class="{active:previewTab==='mapping'}" @click="previewTab='mapping'">目录映射</button><button :class="{active:previewTab==='changes'}" @click="previewTab='changes'">文件明细</button><span v-if="previewLoading" class="updating"><Loading class="spin"/>正在更新…</span><span v-else-if="plan&&!previewError" class="ready"><Check/>预览已就绪</span></div>
      <div v-if="previewError" class="inline-error" role="alert"><Warning/>{{previewError}}</div>
      <div v-if="plan?.issues.length" class="issue-box"><strong>请先处理 {{plan.issues.length}} 个问题</strong><ul><li v-for="issue in plan.issues.slice(0,8)" :key="issue">{{issue}}</li></ul><span v-if="plan.issues.length>8">还有 {{plan.issues.length-8}} 项，请缩小范围后检查。</span></div>
      <div :class="['preview-body',{stale:previewLoading||previewError}]" :aria-busy="previewLoading">
       <div v-if="!plan" class="empty-state"><Operation/><h2>{{previewLoading?'正在构建预览…':'先选择来源与目标'}}</h2></div>
       <template v-else>
        <div v-if="previewTab==='tree'" class="tree-view"><div class="tree-root"><FileIcon :name="plan.config.target" dir/>{{base(plan.config.target)}}<span class="badge neutral">目标目录</span></div><PreviewTree :nodes="tree" @inspect="detail=$event"/></div>
        <div v-else-if="previewTab==='mapping'" class="mapping-view"><div v-for="g in plan.groups" :key="g.source" class="mapping-row" @click="inspectGroup(g)"><div><strong>{{g.original}}</strong><p :title="g.source">{{g.source}}</p></div><ArrowRight/><div><strong>{{g.name||base(config.target)}}</strong><p>{{g.count}} 个文件 <span v-if="!g.matched" class="amber-text">· 未匹配，保留原名</span></p></div></div></div>
        <template v-else><div class="detail-filters"><button :class="{active:filter==='all'}" @click="filter='all';previewPage=0">全部文件</button><button :class="{active:filter==='renamed'}" @click="filter='renamed';previewPage=0">重名改名 {{plan.renamedCount}}</button></div><div class="changes-list"><div v-for="f in movePage" :key="f.source" class="change-row"><FileIcon :name="f.target"/><div><div :title="f.source">{{f.source}}</div><div class="target-line" :title="f.target">→ {{f.target}}</div></div><span v-if="f.renamed" class="badge amber">重名改名</span><span class="file-size">{{size(f.size)}}</span></div></div><div v-if="filteredMoves.length>100" class="pagination"><button class="button small" :disabled="previewPage===0" @click="previewPage--">上一页</button><span>{{previewPage+1}} / {{Math.ceil(filteredMoves.length/100)}}</span><button class="button small" :disabled="(previewPage+1)*100>=filteredMoves.length" @click="previewPage++">下一页</button></div></template>
        <div v-if="plan.files.length<plan.fileCount||plan.existing.length<plan.existingCount" class="load-more"><p class="with-help">已显示 {{plan.files.length}} / {{plan.fileCount}} 个任务文件、{{plan.existing.length}} / {{plan.existingCount}} 个已有项目<HelpTip label="预览分页说明" text="计划已完整计算；加载更多仅增加预览中显示的项目，不影响移动范围。"/></p><button class="button small" :disabled="loadingMore" @click="loadMore">{{loadingMore?'正在加载…':'加载更多预览项目'}}</button></div>
        <details v-if="plan.excluded.length" class="excluded"><summary>{{plan.excluded.length}} 个排除项<HelpTip label="排除项说明" text="链接、特殊文件或选定深度以上的文件不会参与移动。"/></summary><p v-for="x in plan.excluded.slice(0,100)" :key="x">{{x}}</p></details>
       </template>
      </div>
      <div v-if="detail" class="node-detail"><button class="icon-button" aria-label="关闭条目详情" @click="detail=null"><Close/></button><strong class="with-help">{{detail.name}}<HelpTip v-if="detail.existing||detail.renamed" label="条目说明" :text="(detail.existing?'目标中已有的内容会保留。':'')+(detail.renamed?(detail.dir?'按前缀规则映射目标目录名。':'同名文件将自动改名，不覆盖已有文件。'):'')"/></strong><p>目标：{{config.target}}/{{detail.key}}</p><p v-for="s in detail.sources" :key="s">来源：{{s}}</p></div>
     </section>
    </div>
    <footer class="merge-footer"><div :class="{stale:previewLoading||previewError}"><strong>{{plan?.fileCount??0}}</strong> 个文件 <span>·</span> {{plan?.groups.length??0}} 个来源 → {{plan?.groupCount??0}} 个{{config.mode==='extract'?'目标目录':'分组'}} <span>·</span> 改名 {{plan?.renamedCount??0}} <span>·</span> 未匹配 {{unmatched}}<small>{{previewLoading?'正在更新，上一版结果仅供参考':plan?size(plan.bytes)+' · '+(plan.issues.length?'请先处理问题':'预览阶段不修改文件'):'等待配置'}}</small></div><button class="button primary" :disabled="!canExecute" @click="confirmDialog?.showModal()"><Operation/>执行移动</button></footer>
   </template>

   <template v-else-if="view==='structure'">
    <header class="workspace-header"><button class="icon-button" aria-label="返回文件浏览" @click="view='files'"><ArrowLeft/></button><div><h1 class="with-help">文件结构<HelpTip label="文件结构说明" text="单层目录一行一个文件名，多层目录自动展示树形结构。包含隐藏项目，链接仅列出名称、不展开；使用 UTF-8，已有文件不会覆盖。"/></h1></div></header>
    <div class="structure-toolbar"><div><strong>{{base(structureSource)}}</strong><p :title="structureSource">{{structureSource}}</p></div><button class="button" :disabled="structureLoading||exportingStructure" @click="refreshStructure"><Refresh :class="{spin:structureLoading}"/>刷新预览</button><button class="button primary" :disabled="structureLoading||!!structureError||exportingStructure" @click="exportStructure"><Download/>{{exportingStructure?'正在保存…':'保存 TXT'}}</button></div>
    <div v-if="structureError" class="inline-error" role="alert">{{structureError}}</div>
    <div v-if="structureLoading" class="empty-state"><Loading class="spin"/><p>正在读取目录结构…</p></div>
    <pre v-else class="structure-text" aria-label="目录结构文本">{{structureText||'（空目录）'}}</pre>
   </template>

   <template v-else>
    <header class="page-heading tasks-heading"><div><div class="eyebrow">操作记录</div><h1 class="with-help">任务与日志<HelpTip label="任务与日志说明" text="记录归并、提取、移动、复制、重命名、删除、新建文件夹和导出目录结构的执行结果。复制在粘贴写入文件后记录；浏览、简介、统计和预览等只读操作不记录。"/></h1></div><div class="heading-actions"><button class="button" :disabled="!tasks.length||running||deletingLogs" @click="requestLogDelete()"><Delete/>清空全部记录</button><button class="button" :disabled="deletingLogs" @click="loadTasks"><Refresh/>刷新</button></div></header>
    <div class="tasks-list"><div v-if="!tasks.length" class="empty-state"><Clock/><h2>还没有执行记录</h2><button class="button" @click="view='files'">浏览文件</button></div><article v-for="t in tasks" :key="t.id" class="task-card"><div class="task-icon" :class="{success:t.status==='succeeded'}"><Check v-if="t.status==='succeeded'"/><Warning v-else/></div><div class="task-info"><h3>{{taskName(t)}} · {{taskStatus(t)}}</h3><p>{{t.id}} · {{t.moved}} / {{t.total}} {{t.operation?'个项目已完成':'个文件已移动'}}</p><p v-if="t.error" class="error-text">{{t.error}}</p></div><button class="button small" @click="inspectTask(t)">{{t.status==='recovery_required'?'核对中断任务':'查看结果'}}</button><button class="icon-button" aria-label="打开任务报告" @click="openReports(t.id)"><FolderOpened/></button><button class="icon-button delete-log" :aria-label="`删除日志 ${t.id}`" title="删除记录" :disabled="running||deletingLogs||t.status==='running'" @click="requestLogDelete(t.id)"><Delete/></button></article></div>
   </template>

   <section v-if="task&&showTask" class="task-dock" aria-live="polite"><div class="task-dock-title"><Loading v-if="running" class="spin"/><Check v-else-if="task.status==='succeeded'" class="green-text"/><Warning v-else class="amber-text"/><strong>{{statuses[task.status]||task.status}}</strong><span>{{task.phase}}</span><button v-if="!running" class="icon-button" aria-label="收起任务结果" @click="showTask=false"><Close/></button><button v-else class="text-button" @click="cancelTask">取消任务</button></div><div class="progress-track"><div :style="{width:progress+'%'}"></div></div><div class="task-dock-summary"><span>{{task.moved}} / {{task.total}} 个文件 · {{size(task.doneBytes||0)}} / {{size(task.bytes||0)}}</span><span>{{progress}}%</span></div><p v-if="task.current" class="task-current" :title="task.current">{{task.current}}</p><p v-if="running&&task.copyBytes>0" class="task-current">当前文件已复制 {{size(task.copyBytes)}}</p><p v-if="task.error" class="error-text">{{task.error}}</p><template v-if="!running&&task.directory"><div class="result-counts">目标原有 {{task.before}} · 执行后 {{task.after}} · 源剩余 {{task.remaining}} · 双份 {{task.retained}} · 待核对 {{task.unknown}}</div><div class="result-actions"><button class="button small" @click="openReports(task.id)"><FolderOpened/>打开日志</button><button class="button small" @click="exportReports(task.id)"><Download/>导出报告</button><button v-if="['cancelled','failed','recovery_required','needs_review'].includes(task.status)" class="button small" @click="prepareRecovery(task.id)">核对并恢复</button></div></template><button v-if="!running&&task.status==='stale_plan'" class="button small" @click="schedulePreview(true)">重新扫描</button></section>
   <FileActions ref="fileActions" :selected="selected" :directory="path" :running="running" :active="view==='files'" @busy="fileBusy=$event" @refresh="refreshFiles" @new-folder="newFolder" @info="showInfo" @notice="showToast($event)" @error="showToast($event,'error')"/>
  </main>
  <MoveDirectoryPicker ref="favoritePicker" purpose="favorite" @select="pinFavorite"/>
  <MoveDirectoryPicker ref="browsePicker" purpose="browse" @select="navigate($event)"/>
  <dialog ref="jumpDialog" class="confirm-dialog" aria-labelledby="jump-title" @cancel="jumping&&$event.preventDefault()">
   <form @submit.prevent="jumpDirectory"><h2 id="jump-title">跳转目录</h2><label class="form-label">目录路径<input ref="jumpInput" v-model="jumpPath" aria-label="跳转目录路径" placeholder="输入目录路径" :disabled="jumping" :aria-invalid="!!navigationError" :aria-describedby="navigationError?'jump-error':undefined" required/></label><p v-if="navigationError" id="jump-error" class="error-text" role="alert">{{navigationError}}</p><div class="dialog-actions"><button type="button" class="button" :disabled="jumping" @click="jumpDialog?.close()">取消</button><button type="submit" class="button primary" :disabled="jumping||!jumpPath.trim()">{{jumping?'正在跳转…':'跳转'}}</button></div></form>
  </dialog>
  <dialog ref="folderDialog" class="confirm-dialog" @close="folderError=''">
   <form @submit.prevent="createFolder"><h2>新建文件夹</h2><label class="form-label">文件夹名称<input ref="folderInput" v-model="folderName" aria-label="文件夹名称" :disabled="creatingFolder" required/></label><p v-if="folderError" class="error-text" role="alert">{{folderError}}</p><div class="dialog-actions"><button type="button" class="button" :disabled="creatingFolder" @click="folderDialog?.close()">取消</button><button class="button primary" :disabled="creatingFolder||!folderName.trim()">{{creatingFolder?'正在创建…':'创建'}}</button></div></form>
  </dialog>
  <dialog ref="searchDialog" class="confirm-dialog search-dialog" aria-labelledby="search-title"><form @submit.prevent="searchDialog?.close()"><div class="info-heading"><h2 id="search-title">搜索当前目录</h2><button type="button" class="icon-button" aria-label="关闭搜索" @click="searchDialog?.close()"><Close/></button></div><p class="search-scope" :title="path">{{path||'当前目录'}} · 不含子目录</p><label class="dialog-search-field"><Search/><input ref="searchInput" v-model="search" type="search" placeholder="输入文件或文件夹名称" aria-label="搜索文件或文件夹名称"/></label><p role="status" aria-live="polite">{{search?`找到 ${visible.length} 个项目`:'按名称搜索当前目录的文件和文件夹'}}</p><div class="dialog-actions"><button type="button" class="button" :disabled="!search" @click="search='';searchInput?.focus()">清除搜索</button><button type="submit" class="button primary">查看结果</button></div></form></dialog>
  <dialog ref="logDeleteDialog" class="confirm-dialog" aria-labelledby="log-delete-title" @cancel="deletingLogs&&$event.preventDefault()" @close="pendingLogDelete=null">
   <h2 id="log-delete-title">{{pendingLogDelete===''?'清空全部记录？':'删除这条记录？'}}</h2>
   <p>仅删除日志和报告，不影响已操作的文件。删除后无法再通过这些记录查看详情或恢复任务。</p>
   <div class="dialog-actions"><button class="button" autofocus :disabled="deletingLogs" @click="logDeleteDialog?.close()">取消</button><button class="button danger-button" :disabled="deletingLogs||running" @click="deleteLogs">{{deletingLogs?'正在删除…':pendingLogDelete===''?'清空全部记录':'删除记录'}}</button></div>
  </dialog>
  <dialog ref="operationDialog" class="confirm-dialog operation-dialog" aria-labelledby="operation-title" @close="operationTask=null">
   <template v-if="operationTask">
    <div class="info-heading"><h2 id="operation-title">{{taskName(operationTask)}}结果</h2><button class="icon-button" aria-label="关闭操作结果" @click="operationDialog?.close()"><Close/></button></div>
    <p>{{taskStatus(operationTask)}} · {{operationTask.moved}} / {{operationTask.total}} 个项目已完成</p>
    <p class="operation-time">{{new Date(operationTask.started).toLocaleString('zh-CN')}}</p>
    <p v-if="operationTask.error" class="error-text" role="alert">{{operationTask.error}}</p>
    <article v-for="(item,index) in operationTask.items" :key="index" class="entry-info operation-item">
     <div class="operation-item-heading"><h3>{{base(item.source)||base(item.target)}}</h3><span :class="item.error?'error-text':'muted'">{{operationTask.unknown?'待核对':item.error?'失败':'已完成'}}</span></div>
     <dl><dt>来源</dt><dd><button class="info-path" @click="copyPath(item.source)">{{item.source||'—'}}</button></dd><dt>目标</dt><dd><button v-if="item.target" class="info-path" @click="copyPath(item.target)">{{item.target}}</button><span v-else>{{operationTask.operation==='delete'&&!item.error&&!operationTask.unknown?'系统废纸篓或回收站':'—'}}</span></dd><template v-if="item.error"><dt>错误</dt><dd class="error-text">{{item.error}}</dd></template></dl>
    </article>
    <div class="dialog-actions"><button class="button" @click="openReports(operationTask.id)">打开日志</button><button class="button" @click="exportReports(operationTask.id)">导出报告</button></div>
   </template>
  </dialog>
  <dialog ref="infoDialog" class="confirm-dialog info-dialog" aria-label="显示简介" @close="infoEntries=[]">
   <div class="info-heading"><h2>显示简介</h2><button class="icon-button" aria-label="关闭简介" @click="infoDialog?.close()"><Close/></button></div>
   <article v-if="infoEntries.length>1" class="entry-info"><h3>{{infoEntries.length}} 个项目</h3><dl><dt>大小</dt><dd><DirectoryCount :entries="infoEntries" metric="size"/></dd><dt>文件数量</dt><dd><DirectoryCount :entries="infoEntries"/></dd></dl></article>
   <template v-else><article v-for="entry in infoEntries" :key="entry.path" class="entry-info"><h3><FileIcon :name="entry.name" :dir="entry.dir"/>{{entry.name}}</h3><dl><dt>位置</dt><dd><button class="info-path" title="单击复制路径" @click="copyPath(entry.path)">{{entry.path}}</button></dd><dt>类型</dt><dd>{{entryType(entry)}}</dd><dt>修改日期</dt><dd>{{new Date(entry.modified).toLocaleString('zh-CN')}}</dd><dt>大小</dt><dd><DirectoryCount v-if="entry.dir&&!entry.link" :entries="[entry]" metric="size"/><template v-else>{{size(entry.size)}}</template></dd><template v-if="entry.dir&&!entry.link"><dt>文件数量</dt><dd><DirectoryCount :entries="[entry]"/></dd></template></dl></article></template>
  </dialog>
  <dialog ref="confirmDialog" class="confirm-dialog"><div class="dialog-icon"><Operation/></div><h2>确认按当前预览移动？</h2><p>将 {{plan?.fileCount}} 个文件{{config.mode==='extract'?'提取到目标目录':'归并到 '+(plan?.groupCount??0)+' 个目标分组'}}，重名改名 {{plan?.renamedCount}} 个。</p><div class="confirm-target">{{config.target}}</div><p class="muted">执行前会重新核对来源与目标。文件将离开原位置，目标已有文件保留，原目录中的空文件夹会保留。</p><div class="dialog-actions"><button class="button" @click="confirmDialog?.close()">返回调整</button><button class="button primary" :disabled="!canExecute" @click="start">确认移动</button></div></dialog>
 </div>
</template>
