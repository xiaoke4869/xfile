// Run with an existing Playwright Page, with the Vite server at 127.0.0.1:5173.
export default async function(page){
 const assert=(condition,message)=>{if(!condition)throw new Error(message)}
 const frame=()=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))))
 const snapshot=()=>page.evaluate(()=>({paths:window.marqueeFixture.state.selected.map(e=>e.path),checked:document.querySelectorAll('tbody input:checked').length,box:!!document.querySelector('.selection-marquee'),scroll:document.querySelector('.table-scroll').scrollTop}))
 const row=index=>page.locator('.file-table tbody tr').nth(index)
 const point=async(index)=>{const b=await row(index).boundingBox();return {x:b.x+b.width*.42,y:b.y+b.height/2}}
 const begin=async(a,b)=>{await page.mouse.move(a.x,a.y);await page.mouse.down();await page.mouse.move(b.x,b.y,{steps:5});await frame()}
 const end=async()=>{await page.mouse.up();await frame()}
 const drag=async(a,b)=>{await begin(a,b);await end()}
 const clear=async()=>{await page.evaluate(()=>{window.marqueeFixture.state.selected=[];document.querySelector('.table-scroll').scrollTop=0});await frame()}
 const checkPaths=async(expected,message)=>{const actual=(await snapshot()).paths;assert(actual.length===expected.length&&expected.every(path=>actual.includes(path)),message+': '+JSON.stringify(actual))}
 const checks=[]
 await page.setViewportSize({width:1100,height:760})
 await page.goto('http://127.0.0.1:5173/tests/marquee-selection.html')
 await page.waitForFunction(()=>window.marqueeFixture)
 await frame()
 const roots=await page.evaluate(()=>window.marqueeFixture.roots.map(e=>e.path))
 let p0=await point(0),p4=await point(4)
 await begin(p0,{x:p4.x+80,y:p4.y})
 let current=await snapshot()
 const style=await page.locator('.selection-marquee').evaluate(el=>({border:getComputedStyle(el).borderStyle,color:getComputedStyle(el).borderColor,background:getComputedStyle(el).backgroundColor}))
 assert(current.box&&current.checked===4&&style.border==='dashed'&&style.color==='rgb(59, 130, 246)'&&style.background==='rgba(59, 130, 246, 0.12)','选框外观或实时勾选不正确')
 await end();await checkPaths([roots[0],roots[1],roots[2],roots[4]],'正向框选或跳过链接失败')
 assert(!(await snapshot()).box,'松开后选框未消失')
 checks.push('正向框选、样式、跳过链接、松开保留且未误点击')

 await drag({x:p4.x+80,y:p4.y},{x:p0.x,y:p0.y})
 await checkPaths([roots[0],roots[1],roots[2],roots[4]],'反向框选失败')
 await begin(p0,{x:p4.x+80,y:p4.y})
 const p1=await point(1)
 await page.mouse.move(p1.x+80,p1.y,{steps:4});await frame()
 await checkPaths([roots[0],roots[1]],'缩小选框没有取消离开框的条目')
 await end()
 checks.push('反向拖动、缩小选框、替换原选择')

 for(const modifier of ['Control','Meta','Shift']){
  await page.evaluate(()=>{const f=window.marqueeFixture;f.state.selected=[f.roots[10]]});await frame()
  await page.keyboard.down(modifier);await drag(p0,{x:p1.x+80,y:p1.y});await page.keyboard.up(modifier)
  await checkPaths([roots[10],roots[0],roots[1]],modifier+' 追加失败')
 }
 await begin(p0,{x:p4.x+80,y:p4.y});await page.keyboard.press('Escape');await frame()
 await checkPaths([roots[10],roots[0],roots[1]],'Esc 未恢复原勾选');await end()
 assert(!(await snapshot()).box,'Esc 后选框残留')
 checks.push('Ctrl/Cmd/Shift 追加、Esc 恢复')

 await clear();await page.getByRole('button',{name:'展开 00-folder',exact:true}).click()
 await page.getByRole('button',{name:'展开 00-child',exact:true}).click();await frame()
 p0=await point(0);const nested=await point(3)
 await drag(p0,{x:nested.x+80,y:nested.y})
 const nestedPaths=await page.evaluate(()=>{const f=window.marqueeFixture;return [f.folder.path,f.child.path,f.grandchild.path,f.children[1].path]})
 await checkPaths(nestedPaths,'展开层级没有完整参与框选')
 await page.getByRole('button',{name:'收起 00-folder',exact:true}).click();await clear()
 p0=await point(0);p4=await point(4)
 await drag(p0,{x:p4.x+80,y:p4.y})
 assert((await snapshot()).paths.every(path=>!nestedPaths.slice(1).includes(path)),'收起后仍命中子行')
 checks.push('展开子目录、孙目录和收起后的行注销')

 await clear()
 const viewport=await page.locator('.table-scroll').boundingBox()
 await begin(await point(0),{x:viewport.x+viewport.width*.6,y:viewport.y+viewport.height-4})
 await page.waitForFunction(()=>document.querySelector('.table-scroll').scrollTop>320)
 current=await snapshot();assert(current.paths.length>16,'自动向下滚动未扩展勾选')
 await page.mouse.move(viewport.x+viewport.width*.6,viewport.y+36)
 await page.waitForFunction(()=>document.querySelector('.table-scroll').scrollTop<40)
 await end()
 const stopped=(await snapshot()).scroll
 await page.waitForTimeout(100)
 assert((await snapshot()).scroll===stopped,'松开后还在自动滚动')
 checks.push('上下边缘自动滚动及停止')

 await clear()
 await begin(await point(0),{x:viewport.x+viewport.width*.6,y:viewport.y+viewport.height-4})
 await page.waitForFunction(()=>document.querySelector('.table-scroll').scrollTop>80)
 await page.mouse.move(viewport.x+viewport.width+10,viewport.y+viewport.height+10)
 await end();assert(!(await snapshot()).box&&(await snapshot()).paths.length>0,'列表外松开未完成框选')
 await clear();await begin(await point(0),{x:p4.x+80,y:p4.y})
 await page.evaluate(()=>window.dispatchEvent(new Event('blur')));await end()
 assert(!(await snapshot()).box&&!(await snapshot()).paths.length,'失焦未取消框选')
 for(const event of ['pointercancel','lostpointercapture']){
  await begin(await point(0),{x:p4.x+80,y:p4.y})
  await page.locator('.table-scroll').dispatchEvent(event,{pointerId:1,pointerType:'mouse'});await end()
  assert(!(await snapshot()).box&&!(await snapshot()).paths.length,event+' 未取消框选')
 }
 assert(await page.evaluate(()=>!window.marqueeFixture.state.centerKind&&!window.marqueeFixture.state.tasks.length),'框选创建了任务中心记录或日志')
 checks.push('列表外松开、窗口失焦取消')

 for(const [field,value] of [['search','file-00'],['showHidden',true],['page',1],['busy',true],['browserLoading',true],['view','tasks']]){
  await clear();await begin(await point(0),{x:p4.x+80,y:p4.y})
  await page.evaluate(([field,value])=>{window.marqueeFixture.state[field]=value},[field,value]);await frame();await end()
  const state=await page.evaluate(()=>({box:!!document.querySelector('.selection-marquee'),count:window.marqueeFixture.state.selected.length}))
  assert(!state.box&&state.count===0,field+' 变化未取消框选')
  await page.evaluate(([field])=>{window.marqueeFixture.state[field]=field==='search'?'':field==='view'?'files':field==='page'?0:false},[field]);await frame()
 }
 await clear();await begin(await point(0),{x:p4.x+80,y:p4.y})
 await page.evaluate(()=>window.marqueeFixture.state.navigate('/empty'));await end()
 assert(!(await snapshot()).box&&!(await snapshot()).paths.length,'导航未清理框选')
 await page.evaluate(()=>window.marqueeFixture.state.navigate('/marquee'));await frame()
 checks.push('过滤、分页、忙碌、视图变化和导航取消')

 for(const busy of ['busy','browserLoading']){
  await page.evaluate(field=>{window.marqueeFixture.state[field]=true},busy);await frame()
  await drag(await point(0),{x:p4.x+80,y:p4.y})
  assert(!(await snapshot()).box&&!(await snapshot()).paths.length,busy+' 期间仍能框选')
  await page.evaluate(field=>{window.marqueeFixture.state[field]=false},busy);await frame()
 }
 await page.evaluate(()=>{window.marqueeFixture.state.page=1});await frame()
 const blank=await page.locator('.table-scroll').boundingBox(),lastPageFirst=await point(0)
 await drag({x:lastPageFirst.x,y:blank.y+blank.height-50},{x:lastPageFirst.x+80,y:lastPageFirst.y})
 assert((await snapshot()).paths.length===5&&(await snapshot()).paths.every(p=>roots.slice(200).includes(p)),'列表底部空白处向上框选失败')
 await drag(await point(0),{x:(await point(4)).x+80,y:(await point(4)).y})
 assert((await snapshot()).paths.length===5&&(await snapshot()).paths.every(p=>roots.slice(200).includes(p)),'框选跨越了分页')
 await page.evaluate(()=>{window.marqueeFixture.state.page=0});await clear()
 await begin(await point(0),{x:p4.x+80,y:p4.y});await page.keyboard.press('Escape');await end()
 await page.getByRole('checkbox',{name:'选择 00-folder',exact:true}).click();await frame()
 await checkPaths([roots[0]],'框选后复选框点击失效')
 await row(1).click({button:'right'})
 assert(await page.getByRole('menuitem',{name:'打开',exact:true}).isVisible(),'框选后右键打开菜单失效')
 await page.keyboard.press('Escape')
 await row(1).locator('.file-label').dblclick();await frame()
 assert(await page.evaluate(()=>window.marqueeFixture.calls.some(c=>c.name==='OpenFile')),'文件双击打开失效')
 await clear()
 const label=await row(0).locator('.file-label').boundingBox()
 await begin({x:label.x+2,y:label.y+label.height/2},{x:label.x+60,y:label.y+80})
 assert(!(await snapshot()).box,'文件名被用作框选起点');await end()
 const noWrites=await page.evaluate(()=>window.marqueeFixture.calls.every(c=>['Home','Tasks','List','OpenFile'].includes(c.name))&&!window.marqueeFixture.state.tasks.length)
 assert(noWrites,'框选产生了写入、任务或日志')
 await clear();await page.setViewportSize({width:1000,height:680});await frame()
 const small0=await point(0),small4=await point(4)
 await drag(small0,{x:small4.x+60,y:small4.y})
 await checkPaths([roots[0],roots[1],roots[2],roots[4]],'最小窗口框选失败')
 const firstBounds=await row(0).boundingBox()
 await row(0).dblclick({position:{x:firstBounds.width*.42,y:firstBounds.height/2}});await frame()
 assert(await page.evaluate(()=>window.marqueeFixture.state.path===window.marqueeFixture.folder.path),'行内空白处双击进入目录失效')
 checks.push('加载和操作时禁用、分页边界、复选框/右键/双击回归、无文件写入和任务')
 await page.evaluate(checks=>{window.marqueeSelectionResult={passed:true,checks};document.title='PASS · 鼠标框选检查'},checks)
 return {passed:true,checks}
}
