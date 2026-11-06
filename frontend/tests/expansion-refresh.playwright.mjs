// Run with a Playwright Page and Vite. The fixture simulates Wails/filesystem results.
export default async function(page,origin='http://127.0.0.1:5173'){
 const assert=(condition,message)=>{if(!condition)throw new Error(message)}
 const frame=()=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))))
 const row=path=>page.locator('tbody tr').filter({has:page.locator(`.file-label[title="${path}"]`)})
 const exists=async path=>await row(path).count()>0
 const toggle=(name,open=true)=>page.getByRole('button',{name:`${open?'展开':'收起'} ${name}`,exact:true}).click()
 const expanded=()=>page.evaluate(()=>Object.keys(window.dragFixture.state.expandedDirectories).sort())
 const check=async(expected,message)=>{await frame();assert(JSON.stringify(await expanded())===JSON.stringify([...expected].sort()),message+': '+JSON.stringify(await expanded()))}
 const target='/drag/00-target',sub=target+'/sub',source='/drag/01-source',other='/drag/02-other',receiver=target+'/receiver.txt',deep=sub+'/deep.txt',child=source+'/child.txt'
 const base=[target,sub,source],checks=[]
 const reset=async()=>{await page.evaluate(async()=>{await window.dragFixture.reset();await window.dragFixture.compact()});await frame()}
 const openBase=async()=>{await toggle('00-target');await toggle('sub');await toggle('01-source');await frame()}
 const refresh=async()=>{await page.getByRole('button',{name:'刷新当前目录',exact:true}).click();await page.waitForFunction(()=>!window.dragFixture.state.browserLoading);await frame()}
 const drag=async(from,to)=>{
  const a=await row(from).locator('.file-label').boundingBox(),b=await row(to).locator('.file-label').boundingBox()
  await page.mouse.move(a.x+6,a.y+a.height/2);await page.mouse.down();await page.mouse.move(b.x+6,b.y+b.height/2,{steps:6});await page.mouse.up()
  await page.waitForFunction(()=>window.dragFixture.request)
 }
 const finish=async()=>{const folder=await page.evaluate(()=>window.dragFixture.finish());await frame();return folder}
 await page.setViewportSize({width:1100,height:760});await page.goto(origin+'/tests/file-drag.html');await page.waitForFunction(()=>window.dragFixture);await page.evaluate(()=>window.dragFixture.compact());await frame()
 await openBase();await toggle('02-other');await toggle('02-other',false);await check(base,'初始展开状态错误')
 const calls=await page.evaluate(()=>window.dragFixture.calls.filter(c=>c.name==='List').length)
 await page.evaluate(()=>{
  const f=window.dragFixture;f.setListing([...f.listing.filter(e=>e.path!=='/drag/00-target/receiver.txt'),{...f.initial.find(e=>e.path==='/drag/00-target/receiver.txt'),path:'/drag/00-target/fresh.txt',name:'fresh.txt'}])
 })
 await refresh();await check(base,'工具栏刷新丢失展开状态')
 assert(await exists(target+'/fresh.txt')&&await exists(deep)&&await exists(child)&&!await exists(receiver),'刷新没有重新读取展开内容')
 assert(await page.getByRole('button',{name:'展开 02-other',exact:true}).count()===1,'已收起分支被自动展开')
 assert(await page.evaluate(()=>window.dragFixture.calls.filter(c=>c.name==='List').length)===calls+4,'刷新未逐层重读或读取了已收起目录')
 assert(await page.evaluate(()=>window.dragFixture.logs.length===0),'只读刷新写入了日志')
 await toggle('00-target',false);await check([source],'手动收起未清理其子层展开')
 await toggle('00-target');await check([target,source],'手动重开改变了原有子层折叠行为')
 assert(!await exists(deep),'手动重开父目录意外展开子层');await toggle('sub')
 await toggle('sub',false)
 await page.locator('.file-panel').click({button:'right',position:{x:700,y:500}})
 await page.getByRole('menuitem',{name:'刷新',exact:true}).click();await frame()
 await check([target,source],'右键刷新丢失展开状态或展开了收起分支');assert(!await exists(deep),'右键刷新展开了收起的子目录')
 checks.push('工具栏/右键刷新保留多层展开与收起；展开内容重新读取，无额外日志')

 await reset();await openBase();await drag(receiver,other);await finish();await check(base,'移动文件后关闭了展开分支')
 assert(!await exists(receiver)&&await exists(deep)&&await exists(child),'移动刷新后来源陈旧或其他展开内容消失')
 await toggle('02-other');assert(await exists(other+'/receiver.txt'),'移动后的实际目标内容未刷新')
 await reset();await openBase();await drag('/drag/10-a.txt','/drag/11-b.txt');const grouped=await finish()
 await check(base,'根层合成关闭了其他展开分支')
 assert(await page.evaluate(folder=>window.dragFixture.state.selected[0]?.path===folder,grouped),'根层合成未选中新目录')
 await reset();await openBase();await drag('/drag/10-a.txt',deep);const nested=await finish()
 await page.waitForFunction(path=>document.querySelector('tr.selected .file-label')?.getAttribute('title')===path,nested)
 await check(base,'嵌套合成关闭了原有分支')
 assert(await exists(nested)&&!await exists(deep)&&await exists(child),'嵌套合成新目录不可见或旧文件残留')
 checks.push('文件移动、根层及嵌套合成保留其他分支，内容更新且新目录选中可见')

 await reset();await openBase();await drag(source,other);await finish();await check([target,sub],'移走的展开目录状态未清理')
 assert(!await exists(source)&&await exists(deep),'移走目录未消失或影响其他分支')
 await reset();await openBase();await toggle('02-other')
 await page.evaluate(()=>{const f=window.dragFixture;f.setListing(f.listing.filter(e=>!e.path.startsWith('/drag/00-target/sub')&&!e.path.startsWith('/drag/01-source')))})
 await refresh();await check([target,other],'删除根层/嵌套目录后状态未清理')
 await page.evaluate(()=>{const f=window.dragFixture;f.setListing([...f.initial])});await refresh();await check([target,other],'已删除路径重新出现后错误恢复旧状态')
 await toggle('sub');await toggle('01-source');await page.evaluate(()=>{
  const f=window.dragFixture;f.setListing(f.listing.map(e=>e.path==='/drag/00-target/sub'?{...e,dir:false}:e.path==='/drag/01-source'?{...e,link:true}:e))
 });await refresh();await check([target,other],'变为文件/链接的目录状态未清理')
 checks.push('已移动、删除、变为文件或链接的失效目录清理，其他展开分支保留')

 await reset();await openBase();await page.evaluate(()=>window.dragFixture.deniedLists.add('/drag/01-source'));await refresh()
 await check([target,sub],'展开目录读取失败没有仅清理失败分支');assert(!await exists(child)&&await exists(deep),'读取失败后残留旧内容或影响其他分支')
 await page.evaluate(()=>window.dragFixture.deniedLists.add('/drag'));await refresh();await check([target,sub],'根目录刷新失败清空了原有展开')
 assert(await exists(deep),'根目录刷新失败移除了原内容')
 await page.evaluate(()=>window.dragFixture.deniedLists.clear());await refresh();await check([target,sub],'刷新重试重新展开了已清理的分支')
 checks.push('展开目录读取失败仅清理失败分支；根刷新失败保留原内容，重试不复活失效状态')

 await reset();await page.evaluate(()=>window.dragFixture.deferList('/drag/00-target'));await toggle('00-target')
 await page.evaluate(()=>{
  const f=window.dragFixture;f.setListing([...f.listing.filter(e=>e.path!=='/drag/00-target/receiver.txt'),{...f.initial.find(e=>e.path==='/drag/00-target/receiver.txt'),path:'/drag/00-target/latest.txt',name:'latest.txt'}])
 });await refresh();assert(await exists(target+'/latest.txt')&&!await exists(receiver),'刷新未显示最新展开内容')
 await page.evaluate(()=>window.dragFixture.releaseList());await frame();assert(await exists(target+'/latest.txt')&&!await exists(receiver),'刷新前旧请求覆盖了新内容')
 await reset();await page.evaluate(()=>window.dragFixture.deferList('/drag/00-target'));await toggle('00-target')
 await page.evaluate(()=>window.dragFixture.state.navigate('/elsewhere'));await page.evaluate(()=>window.dragFixture.releaseList());await check([],'旧展开请求污染了新目录')
 await page.evaluate(()=>window.dragFixture.state.navigate('/drag'));await check([],'跨目录返回恢复了旧展开状态')
 assert(!await exists(receiver),'跨目录返回自动展开旧分支')
 await page.evaluate(()=>window.dragFixture.deferList('/drag/00-target'));await toggle('00-target')
 await page.evaluate(()=>window.dragFixture.state.resetDirectorySizes());await frame();await page.evaluate(()=>window.dragFixture.releaseList())
 await page.waitForFunction(()=>[...document.querySelectorAll('.file-label')].some(e=>e.title==='/drag/00-target/receiver.txt'))
 await check([target],'会话变化后展开请求未恢复')
 checks.push('刷新/导航后的旧请求不污染新内容，目录切换清空；大小计算会话变化后展开读取可恢复')

 await page.evaluate(()=>window.dragFixture.state.view='tasks');await frame();await page.evaluate(()=>window.dragFixture.state.view='files');await frame()
 await check([target],'同目录切换视图丢失展开状态');assert(await exists(receiver),'同目录返回列表未恢复内容')
 assert(await page.evaluate(()=>window.dragFixture.unhandled.length===0),'存在未处理异常')
 checks.push('同目录视图切换保留状态，无未处理异常')
 return {passed:true,checks}
}
