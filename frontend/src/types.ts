export interface Entry {path:string;name:string;dir:boolean;size:number;modified:number;link:boolean}
export interface Source {path:string;depth:number}
export interface Config {mode?:string;duplicate?:string;sources:Source[];target:string;prefix:boolean;rule:string;separator:string;length:number;pattern:string;unmatched:string;strict:boolean}
export interface Move {source:string;target:string;relative:string;size:number;modified:number;renamed:boolean}
export interface Group {source:string;original:string;name:string;matched:boolean;count:number}
export interface Plan {id:string;revision:number;created:string;config:Config;files:Move[];groups:Group[];existing:Entry[];issues:string[];excluded:string[];bytes:number;groupCount:number;fileCount:number;existingCount:number;renamedCount:number;recoveryFrom?:string}
export interface FileState {source:string;target:string;sha256:string;state:string;error:string;temp:string}
export interface Task {id:string;operation?:string;items?:OperationItem[];status:string;phase:string;current:string;error:string;total:number;moved:number;remaining:number;retained:number;unknown:number;before:number;after:number;bytes:number;doneBytes:number;copyBytes:number;directory:string;states:FileState[];started:string;finished:string}
export interface TreeNode {key:string;name:string;dir:boolean;existing:boolean;renamed:boolean;sources:string[];children:TreeNode[];group?:Group[];size?:number}

export interface OperationItem {source:string;target:string;error:string}
export interface OperationResult {items:OperationItem[]}
export interface RenameConfig {mode:'single'|'replace'|'add'|'format';name:string;find:string;replace:string;text:string;format:string;position:'before'|'after';start:number}
export interface RenamePreview {items:OperationItem[];total:number;changed:number;issues:string[]}
