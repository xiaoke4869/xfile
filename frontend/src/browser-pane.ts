import { sortEntries,type SortField } from './format'
import type { Entry } from './types'

export function createBrowserPane(id:number,path=''){
 return {
  id,path,entries:[] as Entry[],selected:[] as Entry[],search:'',showHidden:false,
  expandedDirectories:{} as Record<string,boolean>,history:path?[path]:[],historyIndex:path?0:-1,
  page:0,browserLoading:false,browserVersion:0,navRevision:0,navigationError:'',refreshPending:false,
  sortField:'name' as SortField,sortDirection:'asc' as 'asc'|'desc',
  directorySizes:{} as Record<string,number>,sizeLoading:{} as Record<string,boolean>,revealFolder:'',
 }
}
export type BrowserPane=ReturnType<typeof createBrowserPane>
export function paneEntries(pane:BrowserPane){
 return sortEntries(pane.entries.filter(entry=>(pane.showHidden||!entry.name.startsWith('.'))&&entry.name.toLowerCase().includes(pane.search.toLowerCase())),pane.sortField,pane.sortDirection,pane.directorySizes)
}
