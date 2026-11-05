import type { Entry,Task } from './types'
export function size(n:number){if(n<1024)return `${n} B`;const u=['KB','MB','GB','TB'];let i=-1;do{n/=1024;i++}while(n>=1024&&i<3);return `${n.toFixed(n<10?1:0)} ${u[i]}`}
export function base(p:string){return p.replace(/[\\/]+$/,'').split(/[\\/]/).pop()||p}
export function pathKey(p:string){const normalized=p.replace(/\\/g,'/').replace(/\/+$/,'')||'/';return /^(?:[A-Za-z]:|\/\/)/.test(normalized)?normalized.toLowerCase():normalized}
export function parent(p:string){const n=p.replace(/[\\/]+$/,'');const i=Math.max(n.lastIndexOf('/'),n.lastIndexOf('\\'));return i===0?'/':i===2&&n[1]===':'?n.slice(0,3):n.slice(0,i)||p}
export function pathBreadcrumbs(path:string){
 if(!path)return []
 const root=path.match(/^[A-Za-z]:[\\/]/)?.[0]||path.match(/^[\\/]{2}[^\\/]+[\\/][^\\/]+[\\/]?/)?.[0]||(path.startsWith('/')?'/':'')
 const separator=path.includes('\\')?'\\':'/'
 let current=root
 const crumbs=root?[{name:root==='/'?'磁盘根目录':root.replace(/[\\/]+$/,''),path:root}]:[]
 for(const name of path.slice(root.length).split(/[\\/]+/).filter(Boolean)){
  current+=(current&&!/[\\/]$/.test(current)?separator:'')+name
  crumbs.push({name,path:current})
 }
 return crumbs
}
export const statuses:Record<string,string>={running:'正在处理',succeeded:'校验通过',failed:'部分失败',needs_review:'需要核对',cancelled:'已取消',stale_plan:'计划已过期',recovery_required:'任务中断，需核对'}
export function taskName(task:Task){return task.operation?({copy:'复制',move:'移动',rename:'重命名',delete:'删除',create_folder:'新建文件夹',export_directory:'导出目录结构'} as Record<string,string>)[task.operation]||task.operation:'归并 / 提取'}
export function taskStatus(task:Task){return task.operation?({succeeded:'已完成',failed:'失败',partial_failed:'部分完成'} as Record<string,string>)[task.status]||statuses[task.status]||task.status:statuses[task.status]||task.status}

export function extractionTarget(source:string){
 const now=new Date(),date=`${now.getFullYear()}${String(now.getMonth()+1).padStart(2,'0')}${String(now.getDate()).padStart(2,'0')}`
 return source.replace(/[\\/]+$/,'')+(source.includes('\\')?'\\':'/')+`result-${date}`
}

export type SortField='name'|'size'|'type'|'modified'
export function entryType(entry:Entry){return entry.link?'链接':entry.dir?'文件夹':entry.name.includes('.')?entry.name.split('.').pop()?.toUpperCase()+' 文件':'文件'}
// Sort a whole directory before pagination; expanded directories use the same order.
export function sortEntries(entries:Entry[],field:SortField,direction:'asc'|'desc',directorySizes:Record<string,number>={}){
 return [...entries].sort((a,b)=>{
  if(a.dir!==b.dir)return a.dir?-1:1
  if(a.dir&&field==='size'){
   const left=directorySizes[a.path],right=directorySizes[b.path]
   if(left===undefined||right===undefined){
    if(left!==right)return left===undefined?1:-1
   }else return (left-right||a.name.localeCompare(b.name,'zh-CN',{numeric:true}))*(direction==='asc'?1:-1)
  }
  if(a.dir)return a.name.localeCompare(b.name,'zh-CN',{numeric:true,sensitivity:'base'})
  const value=field==='size'||field==='modified'?a[field]-b[field]:(field==='type'?entryType(a):a.name).localeCompare(field==='type'?entryType(b):b.name,'zh-CN',{numeric:true,sensitivity:'base'})
  return (value||a.name.localeCompare(b.name,'zh-CN',{numeric:true}))*(direction==='asc'?1:-1)
 })
}
