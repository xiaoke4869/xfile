// Run with a Playwright Page and a running Vite server. Uses simulated Wails calls.
export default async function(page,origin='http://127.0.0.1:5173'){
 const assert=(value,message)=>{if(!value)throw new Error(message)}
 const checks=[]
 const frame=()=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))))
 const row=path=>page.locator('tbody tr').filter({has:page.locator(`.file-label[title="${path}"]`)})
 const point=async(path,icon=false)=>{
  const locator=icon?row(path).locator('.file-icon'):row(path).locator('.file-label')
  const box=await locator.boundingBox();assert(box,'找不到行 '+path)
  return {x:box.x+Math.min(8,box.width/2),y:box.y+box.height/2}
 }
 const choose=async paths=>{await page.evaluate(paths=>{const f=window.dragFixture;f.state.selected=paths.map(path=>f.initial.find(e=>e.path===path))},paths);await frame()}
 const reset=async()=>{await page.evaluate(()=>window.dragFixture.reset());await frame()}
 const begin=async(source,target,icon=false)=>{
  const from=await point(source,icon),to=await point(target)
  await page.mouse.move(from.x,from.y);await page.mouse.down();await page.mouse.move(to.x,to.y,{steps:6});await frame()
 }
 const release=async()=>{await page.mouse.up();await frame()}
 const finish=async(failure='')=>{const result=await page.evaluate(failure=>window.dragFixture.finish(failure),failure);await frame();return result}
 const request=()=>page.evaluate(()=>{const r=window.dragFixture.request;return r?{name:r.name,args:r.args,paths:r.paths}:null})
 const noDrag=async()=>assert(await page.locator('.file-drag-preview,.drop-folder,.drop-group,.drop-favorite,.drop-current-directory,.drag-source,.selection-marquee').count()===0,'拖拽状态未清理')
 const a='/drag/10-a.txt',b='/drag/11-b.txt',c='/drag/12-c.txt',folder='/drag/00-target',source='/drag/01-source',other='/drag/02-other'
 await page.setViewportSize({width:1100,height:760})
 await page.goto(origin+'/tests/file-drag.html');await page.waitForFunction(()=>window.dragFixture);await frame()

 for(const [paths,icon] of [[[a],false],[[a,b],true],[[source],true],[[source,other],false],[[a,source,other],false]]){
  await reset();await choose(paths);await begin(paths[0],folder,icon)
  assert(await row(folder).evaluate(el=>el.classList.contains('drop-folder')),'文件夹未高亮')
  assert((await page.locator('.file-drag-preview').textContent()).includes(`移动 ${paths.length} 个项目`),'数量提示错误')
  assert(!await page.locator('.selection-marquee').count(),'文件拖拽触发了框选')
  await release();const job=await request()
  assert(job?.name==='TransferEntries'&&job.args[1]===folder&&job.args[2]===true&&JSON.stringify(job.args[0])===JSON.stringify(paths),'移动参数错误')
  assert(!await page.locator('dialog[open]').count(),'拖拽移动出现确认弹窗')
  assert(await page.evaluate(()=>window.dragFixture.state.fileBusy&&window.dragFixture.state.centerOperation.status==='running'),'任务中心或互斥缺失')
  const writes=await page.evaluate(()=>window.dragFixture.calls.filter(c=>['TransferEntries','GroupFiles'].includes(c.name)).length)
  await begin(b,folder);await release()
  assert(await page.evaluate(()=>window.dragFixture.calls.filter(c=>['TransferEntries','GroupFiles'].includes(c.name)).length)===writes,'忙碌时仍可拖拽执行')
  await finish();await noDrag()
  assert(await page.evaluate(()=>window.dragFixture.state.centerOperation.status==='succeeded'&&window.dragFixture.state.tasks[0].items.length>0),'完成结果/日志缺失')
 }
 checks.push('图标/文件名、单文件、多文件、单目录、多目录、混选移动及任务中心互斥')

 await reset();await choose([b,c]);await begin(a,folder);await release()
 assert(JSON.stringify((await request()).paths)===JSON.stringify([a]),'未选起点携带了原选择');await finish()
 for(const paths of [[a],[a,b]]){
  await reset();await choose(paths);await begin(a,c)
  assert(await row(c).evaluate(el=>el.classList.contains('drop-group')),'合成目标无效果')
  const style=await row(c).locator('.file-drag-icon').evaluate(el=>({opacity:getComputedStyle(el.querySelector('img')).opacity,transform:getComputedStyle(el.querySelector('img')).transform,before:getComputedStyle(el,'::before').content}))
  assert(Number(style.opacity)<1&&style.transform!=='none'&&style.before==='""','半透明、旋转、叠放效果缺失')
  assert((await page.locator('.file-drag-preview').textContent()).includes('合成文件夹'),'合成提示缺失')
  await release();const job=await request()
  assert(job?.name==='GroupFiles'&&job.args[1]===c&&job.paths.length===paths.length+1,'合成未包含目标文件')
  const result=await finish()
  assert(await page.evaluate(result=>{const f=window.dragFixture;return f.state.path==='/drag'&&f.state.selected[0]?.path===result&&f.logs[0].items.every(item=>item.target.startsWith(result+'/'))},result),'合成后未原地选中新目录或目标错误')
  await noDrag()
 }
 checks.push('拖动未选项目只移动自身；单/多文件合成含目标文件、半透明旋转叠放、刷新选中')

 await reset()
 await page.getByRole('button',{name:'展开 00-target',exact:true}).click()
 await page.getByRole('button',{name:'展开 sub',exact:true}).click();await frame()
 await choose([a,b]);await begin(a,folder+'/sub/deep.txt');await release()
 let nested=await finish()
 await page.waitForFunction(path=>document.querySelector('tr.selected .file-label')?.getAttribute('title')===path,nested)
 assert(nested===folder+'/sub/合成的文件夹','新目录名称或目标文件父目录不正确')
 assert(await row(nested).isVisible(),'嵌套新目录未展开显示')
 await reset();await page.getByRole('button',{name:'展开 01-source',exact:true}).click();await frame()
 await choose([source,source+'/child.txt']);await begin(source,folder);await release()
 assert(JSON.stringify((await request()).paths)===JSON.stringify([source]),'父子混选未按父目录去重');await finish()
 assert(await page.evaluate(()=>window.dragFixture.logs[0].items.length===1),'父子混选重复移动')
 checks.push('展开层级目标、嵌套新目录自动展示并选中、父子混选')

 const moveTo=async(source,to)=>{const from=await point(source);await page.mouse.move(from.x,from.y);await page.mouse.down();await page.mouse.move(to.x,to.y,{steps:6});await frame()}
 const blankPoint=async()=>{const box=await page.locator('.table-scroll').boundingBox();return {x:box.x+box.width*.6,y:box.y+box.height-40}}
 const compact=async()=>{await reset();await page.evaluate(()=>window.dragFixture.compact());await page.getByRole('button',{name:'展开 00-target',exact:true}).click();await frame()}
 const receiver=folder+'/receiver.txt',sub=folder+'/sub'
 for(const paths of [[receiver],[receiver,sub],[receiver,sub,a]]){
  await compact();await choose(paths);await moveTo(paths[0],await blankPoint())
  const expected=paths.filter(path=>path!==a)
  assert(await page.locator('.drop-current-directory').count()===1,'当前目录空白未高亮')
  assert((await page.locator('.file-drag-preview').textContent()).includes(`移动 ${expected.length} 个项目`),'空白落点未跳过已在当前目录的项目')
  await release();const job=await request()
  assert(job?.name==='TransferEntries'&&job.args[1]==='/drag'&&JSON.stringify(job.paths)===JSON.stringify(expected),'空白落点移动参数错误')
  await finish();await noDrag()
  assert(await page.evaluate(()=>window.dragFixture.state.path==='/drag'&&window.dragFixture.logs[0].operation==='move'),'空白移动改变浏览位置或无日志')
 }
 await compact();await choose([a]);await moveTo(a,await blankPoint());await release();assert(!await request(),'根层项目空白放置产生操作');await noDrag()
 await choose([folder,receiver]);await moveTo(receiver,await blankPoint());await release();assert(!await request(),'跳过根层父目录时把选中子项单独移出了父目录');await noDrag()
 await choose([receiver]);const header=await page.locator('thead').boundingBox();await moveTo(receiver,{x:header.x+header.width*.7,y:header.y+header.height/2});await release();assert(!await request(),'表头被识别为空白')
 await choose([receiver]);await moveTo(receiver,await blankPoint());await page.keyboard.press('Escape');await release();assert(!await request(),'空白落点取消后仍移动');await noDrag()
 checks.push('展开内容拖至当前目录空白、混合多选、跳过根层项目及数量、表头排除、取消和日志')

 const favorites=async()=>{await page.evaluate(()=>{window.dragFixture.state.favorites=['/saved','/drag','/drag/00-target','/drag/00-target/sub','/missing']});await frame()}
 const favorite=path=>page.locator(`[data-drop-directory="${path}"]`)
 const center=async locator=>{const box=await locator.boundingBox();return {x:box.x+12,y:box.y+box.height/2}}
 for(const paths of [[a],[a,b],[source,other],[a,source]]){
  await reset();await favorites();await choose(paths);await moveTo(paths[0],await center(favorite('/saved')))
  assert(await favorite('/saved').evaluate(el=>el.classList.contains('drop-favorite')),'收藏目标未高亮')
  assert((await page.locator('.file-drag-preview').textContent()).includes(`移动 ${paths.length} 个项目`),'收藏移动数量错误')
  await release();const job=await request();assert(job?.args[1]==='/saved'&&JSON.stringify(job.paths)===JSON.stringify(paths),'收藏移动参数错误')
  await finish();await noDrag();assert(await page.evaluate(()=>window.dragFixture.state.path==='/drag'),'拖到收藏后自动导航')
 }
 await compact();await favorites();await choose([receiver,a]);await moveTo(receiver,await center(favorite('/drag')));await release()
 assert(JSON.stringify((await request()).paths)===JSON.stringify([receiver]),'收藏当前目录未跳过已有项目');await finish()
 for(const [paths,target] of [[[a],'/drag'],[[folder],folder],[[folder],sub]]){
  await reset();await favorites();await choose(paths);await moveTo(paths[0],await center(favorite(target)));await release();assert(!await request(),'收藏自身/子目录/原目录触发了移动');await noDrag()
 }
 await reset();await favorites();await choose([a])
 for(const locator of [page.getByRole('button',{name:'取消收藏 saved',exact:true}),page.getByRole('button',{name:'系统目录',exact:true})]){
  await moveTo(a,await center(locator));await release();assert(!await request(),'收藏取消按钮或系统常用位置接收了拖拽')
 }
 await moveTo(a,await center(favorite('/saved')));await page.keyboard.press('Escape');await release();assert(!await request(),'收藏拖拽取消后仍移动');await noDrag()
 await moveTo(a,await center(favorite('/missing')));await release();await finish('missing')
 assert(await page.evaluate(()=>window.dragFixture.listing.some(e=>e.path==='/drag/10-a.txt')&&window.dragFixture.logs[0].status==='failed'),'失效收藏未保留来源或失败日志')
 await reset();await favorites();await choose([a,b]);await moveTo(a,await center(favorite('/saved')));await release()
 const favoriteWrites=await page.evaluate(()=>window.dragFixture.calls.filter(c=>c.name==='TransferEntries').length)
 await moveTo(b,await center(favorite('/saved')));await release()
 assert(await page.evaluate(()=>window.dragFixture.calls.filter(c=>c.name==='TransferEntries').length)===favoriteWrites,'忙碌时重复向收藏移动')
 await finish(b);assert(await page.evaluate(()=>window.dragFixture.listing.some(e=>e.path==='/drag/11-b.txt')&&window.dragFixture.logs[0].status==='partial_failed'),'收藏部分失败未保留来源或日志')
 checks.push('收藏目录单/多文件、目录和混选移动，原目录跳过、自身/子目录及非落点保护、取消和失效目标日志')

 const invalid=async(paths,from,to)=>{await reset();await choose(paths);await begin(from,to);assert(!await page.locator('.drop-folder,.drop-group').count(),'非法目标高亮');await release();assert(!await request(),'非法拖放执行了操作');await noDrag()}
 await invalid([a],a,a)
 await invalid([a,b],a,b)
 await invalid([source],source,a)
 await invalid([source,a],a,c)
 await invalid([a],a,'/drag/13-link')
 await invalid([], '/drag/13-link',folder)
 await reset();await page.getByRole('button',{name:'展开 00-target',exact:true}).click();await frame();await choose([folder]);await begin(folder,folder+'/sub');await release();assert(!await request(),'目录移动到了自身子目录')
 await reset();await choose([a]);await begin(a,b);const panel=await page.locator('.file-panel').boundingBox();await page.mouse.move(panel.x-12,panel.y+60);await release();assert(!await request(),'列表外松手执行了操作')
 checks.push('自身、所选目标、目录向文件、混选向文件、链接、自身子目录、列表外保护')

 for(const cancel of ['Escape','blur','pointercancel','lostpointercapture','view','search','page','sort','navigation']){
  await reset();await choose([b]);await begin(a,c)
  if(cancel==='Escape')await page.keyboard.press('Escape')
  else await page.evaluate(async cancel=>{
   const f=window.dragFixture
   if(cancel==='blur')window.dispatchEvent(new Event('blur'))
   if(cancel==='pointercancel')window.dispatchEvent(new PointerEvent('pointercancel',{pointerId:1}))
   if(cancel==='lostpointercapture')document.querySelector('.table-scroll').dispatchEvent(new PointerEvent('lostpointercapture',{pointerId:1}))
   if(cancel==='view')f.state.view='tasks'
   if(cancel==='search')f.state.search='10-a'
   if(cancel==='page')f.state.page=1
   if(cancel==='sort')f.state.changeSort('size')
   if(cancel==='navigation')await f.state.navigate('/elsewhere')
  },cancel)
  await release();assert(!await request(),cancel+' 后执行了操作');await noDrag()
  if(!['navigation','search','page'].includes(cancel))assert(await page.evaluate(()=>window.dragFixture.state.selected[0]?.path==='/drag/11-b.txt'),cancel+' 取消未恢复选择')
  await page.evaluate(()=>window.dragFixture.state.view='files')
 }
 await reset();await choose([a]);const from=await point(a),viewport=await page.locator('.table-scroll').boundingBox()
 await page.mouse.move(from.x,from.y);await page.mouse.down();await page.mouse.move(from.x,viewport.y+viewport.height-3,{steps:5})
 await page.waitForFunction(()=>document.querySelector('.table-scroll').scrollTop>160)
 await page.keyboard.press('Escape');await release();await noDrag()
 checks.push('Esc/失焦/指针丢失/视图过滤分页排序导航取消与边缘自动滚动')

 await reset();await choose([a,b]);await begin(a,c);await release();nested=await finish(b)
 assert(await page.evaluate(b=>window.dragFixture.listing.some(e=>e.path===b)&&window.dragFixture.state.centerOperation.status==='partial_failed',b),'部分失败未保留来源或状态')
 await page.evaluate(()=>window.dragFixture.state.inspectTask(window.dragFixture.state.centerOperation));await frame()
 assert((await page.locator('.operation-dialog').textContent()).includes('permission denied'),'失败详情缺失')
 await page.getByRole('button',{name:'关闭操作结果',exact:true}).click()
 await reset();await choose([a]);await begin(a,c);await release();await finish('create')
 assert(await page.evaluate(()=>window.dragFixture.listing.some(e=>e.path==='/drag/10-a.txt')&&window.dragFixture.listing.some(e=>e.path==='/drag/12-c.txt')),'创建失败改变来源')
 await reset();await choose([a]);await begin(a,c);await release();await page.evaluate(()=>window.dragFixture.state.navigate('/elsewhere'));await finish()
 assert(await page.evaluate(()=>window.dragFixture.state.path==='/elsewhere'&&window.dragFixture.state.selected.length===0),'完成后导航被拉回')
 checks.push('部分失败详情、创建失败保留、执行期间导航不被拉回')

 await reset();await row(a).locator('.file-label').click();await frame();assert(await page.evaluate(()=>window.dragFixture.state.selected[0]?.path==='/drag/10-a.txt'),'文件名单击失效')
 await row(b).locator('.file-label').click({modifiers:['Meta']});await frame();assert(await page.evaluate(()=>window.dragFixture.state.selected.length===2),'修饰键多选失效')
 await row(c).locator('input').click();await frame();assert(await page.evaluate(()=>window.dragFixture.state.selected.length===3),'复选框多选失效')
 await row(a).locator('.file-label').dblclick();assert(await page.evaluate(()=>window.dragFixture.calls.some(c=>c.name==='OpenFile')),'双击打开失效')
 assert(await page.evaluate(()=>window.dragFixture.unhandled.length===0),'出现未处理的异常')
 checks.push('单击、修饰键多选、复选框、双击回归，无未处理异常')
 await page.setViewportSize({width:1000,height:680});await page.reload();await page.waitForFunction(()=>window.dragFixture);await frame();await choose([a]);await begin(a,b)
 await page.screenshot({path:'/private/tmp/xfile-drag-group-preview.png'})
 await page.keyboard.press('Escape');await release()
 return {passed:true,checks}
}
