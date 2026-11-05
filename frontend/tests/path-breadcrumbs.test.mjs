import assert from 'node:assert/strict'
import test from 'node:test'
import { pathBreadcrumbs } from '../src/format.ts'

test('breadcrumbs retain navigable roots, spaces and native separators',()=>{
 for(const [path,expected] of [
  ['',[]],['/',['/']],
  ['/Users/mico/Library/Application Support/apifox/',['/','/Users','/Users/mico','/Users/mico/Library','/Users/mico/Library/Application Support','/Users/mico/Library/Application Support/apifox']],
  ['C:\\Users\\mico',['C:\\','C:\\Users','C:\\Users\\mico']],
  ['C:/Users/mico',['C:/','C:/Users','C:/Users/mico']],
  ['\\\\server\\share\\folder',['\\\\server\\share\\','\\\\server\\share\\folder']],
  ['//server/share/folder',['//server/share/','//server/share/folder']],
 ])assert.deepEqual(pathBreadcrumbs(path).map(c=>c.path),expected,path)
 assert.equal(pathBreadcrumbs('/Library/Application Support').at(-1).name,'Application Support')
})
