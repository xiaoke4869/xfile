import assert from 'node:assert/strict'
import test from 'node:test'
import { sortEntries } from '../src/format.ts'

const entries=[
 {name:'10.txt',size:2,modified:1000,dir:false,link:false},
 {name:'2.png',size:10,modified:3000,dir:false,link:false},
 {name:'Folder',size:0,modified:2000,dir:true,link:false},
 {name:'A Folder',size:0,modified:4000,dir:true,link:false},
].map(entry=>({...entry,path:'/tmp/'+entry.name}))

test('folders stay at the top in name order while each column sorts files',()=>{
 for(const [field,expected] of [
  ['name',['2.png','10.txt']],
  ['size',['10.txt','2.png']],
  ['type',['2.png','10.txt']],
  ['modified',['10.txt','2.png']],
 ]){
  assert.deepEqual(sortEntries(entries,field,'asc').map(entry=>entry.name),['A Folder','Folder',...expected])
  assert.deepEqual(sortEntries(entries,field,'desc').map(entry=>entry.name),['A Folder','Folder',...[...expected].reverse()])
 }
 assert.deepEqual(entries.map(entry=>entry.name),['10.txt','2.png','Folder','A Folder'])
 const many=Array.from({length:205},(_,i)=>({...entries[0],name:`${205-i}.txt`,size:205-i}))
 assert.equal(sortEntries(many,'size','asc').slice(0,200)[0].size,1)
 assert.equal(sortEntries(many,'size','asc').slice(200)[0].size,201)
})

test('calculated directory sizes sort in both directions before unknown sizes and pagination',()=>{
 const folder=name=>({...entries[2],name,path:'/tmp/'+name,size:9999})
 const folders=['unknown','large','empty','small','small2'].map(folder)
 const sizes={'/tmp/large':2048,'/tmp/empty':0,'/tmp/small':20,'/tmp/small2':20}
 assert.deepEqual(sortEntries([...folders,entries[0]],'size','asc',sizes).map(e=>e.name),['empty','small','small2','large','unknown','10.txt'])
 assert.deepEqual(sortEntries([...folders,entries[0]],'size','desc',sizes).map(e=>e.name),['large','small2','small','empty','unknown','10.txt'])
 assert.deepEqual(sortEntries(folders,'name','asc',sizes).map(e=>e.name),['empty','large','small','small2','unknown'])
 assert.deepEqual(sortEntries(folders,'size','asc',{}).map(e=>e.name),['empty','large','small','small2','unknown'])
 const many=Array.from({length:205},(_,i)=>folder(`folder${i}`))
 const calculated=Object.fromEntries(many.map((e,i)=>[e.path,204-i]))
 assert.equal(sortEntries(many,'size','asc',calculated)[0].path,'/tmp/folder204')
 assert.equal(sortEntries(many,'size','asc',calculated).slice(200)[0].path,'/tmp/folder4')
 assert.equal(sortEntries(many,'size','desc',calculated)[0].path,'/tmp/folder0')
})
