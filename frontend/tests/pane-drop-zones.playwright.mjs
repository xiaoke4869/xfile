// Real pointer checks for compact directory headers and dense-list drop targets.
export default async function(page,origin='http://127.0.0.1:5173',screenshots=''){
 const assert=(value,message)=>{if(!value)throw new Error(message)}
 const frame=()=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))))
 const panel=index=>page.locator('.file-panel').nth(index)
 const row=(index,path)=>panel(index).locator('tbody tr').filter({has:page.locator(`.file-label[title="${path}"]`)})
 const center=async locator=>{const r=await locator.boundingBox();assert(r,'找不到落点');return {x:r.x+r.width/2,y:r.y+r.height/2}}
 const name=async(index,path)=>{const r=await row(index,path).locator('.file-label').boundingBox();return {x:r.x+Math.min(8,r.width/2),y:r.y+r.height/2}}
 const gap=async(index,path)=>{const icon=await row(index,path).locator('.file-drag-icon').boundingBox(),text=await row(index,path).locator('.file-label').boundingBox();return {x:(icon.x+icon.width+text.x)/2,y:icon.y+icon.height/2}}
 const choose=async paths=>{await page.evaluate(paths=>{const f=window.dragFixture;f.state.panes[0].selected=paths.map(path=>f.listing.find(e=>e.path===path))},paths);await frame()}
 const begin=async(from,to)=>{await page.mouse.move(from.x,from.y);await page.mouse.down();await page.mouse.move(to.x,to.y,{steps:8});await frame()}
 const release=async()=>{await page.mouse.up();await frame()}
 const request=()=>page.evaluate(()=>{const r=window.dragFixture.request;return r?{name:r.name,args:r.args,paths:r.paths}:null})
 const finish=async()=>{const value=await page.evaluate(()=>window.dragFixture.finish());await frame();return value}
 const cancel=async()=>{await page.keyboard.press('Escape');await release();assert(!await request(),'取消仍然提交操作')}
 const colors=locator=>locator.locator('td').evaluateAll(cells=>cells.map(td=>getComputedStyle(td).backgroundColor))
 const a='/drag/10-a.txt',b='/drag/11-b.txt',target='/drag/00-target',sub=target+'/sub',receiver=target+'/receiver.txt',source='/drag/01-source'
 const checks=[]
 const fresh=async()=>{
  await page.goto(origin+'/tests/file-drag.html');await page.waitForFunction(()=>window.dragFixture?.state.path==='/drag')
  await page.evaluate(target=>{const f=window.dragFixture;const template=f.initial.find(e=>!e.dir&&!e.link);f.setListing([...f.initial,{...template,path:target+'/01-folder',name:'01-folder',dir:true},...Array.from({length:85},(_,i)=>({...template,path:target+`/zz-dense-${i}.txt`,name:`zz-dense-${i}.txt`}))])},target)
  await page.getByRole('button',{name:'分屏',exact:true}).click()
  await page.evaluate(async target=>{const s=window.dragFixture.state;await s.navigate(target,true,true,s.panes[1])},target);await frame()
  assert(await panel(1).locator('.table-scroll').evaluate(el=>el.scrollHeight>el.clientHeight),'目标屏没有撑满滚动区域')
 }
 await page.setViewportSize({width:1440,height:800})

 await fresh()
 assert((await panel(1).locator('.pane-path').textContent()).trim()==='00-target','分屏顶部仍显示完整路径')
 assert(await panel(1).locator('.pane-path').getAttribute('title')===target,'没有保留完整路径提示')
 assert(await page.locator('.directory-title').getAttribute('title')===target,'主工具栏没有完整路径提示')
 await panel(1).getByRole('button',{name:'切换分屏 2 目录',exact:true}).click()
 let picker=page.locator('.directory-picker[open]')
 await picker.waitFor();assert(await picker.getByRole('textbox',{name:'目标选择路径'}).inputValue()===target,'选择器没有从对应屏路径开始')
 await picker.getByRole('button',{name:'选择目录 sub',exact:true}).click()
 await picker.getByRole('button',{name:'打开此目录',exact:true}).click();await frame()
 assert(await page.evaluate(sub=>{const p=window.dragFixture.state.panes;return p[0].path==='/drag'&&p[1].path===sub},sub),'切换目录影响其他屏或未导航')
 await panel(1).getByRole('button',{name:'切换分屏 2 目录',exact:true}).click()
 await picker.getByRole('button',{name:'取消',exact:true}).click();await frame()
 assert(await page.evaluate(sub=>window.dragFixture.state.panes[1].path===sub,sub),'取消选择改变了目录')
 // The picker must remember its owning pane even if another pane is activated.
 await panel(1).getByRole('button',{name:'切换分屏 2 目录',exact:true}).click()
 await picker.getByRole('button',{name:'返回上一级目录',exact:true}).click()
 await page.evaluate(()=>{const s=window.dragFixture.state;s.activatePane(s.panes[0])})
 await picker.getByRole('button',{name:'打开此目录',exact:true}).click();await frame()
 assert(await page.evaluate(target=>{const p=window.dragFixture.state.panes;return p[0].path==='/drag'&&p[1].path===target},target),'选择器使用了后来激活的屏')
 await panel(1).getByRole('button',{name:'切换分屏 2 目录',exact:true}).click()
 await picker.getByRole('button',{name:'选择目录 sub',exact:true}).click()
 await page.evaluate(()=>{const s=window.dragFixture.state;s.closePane(s.panes[1])})
 await picker.getByRole('button',{name:'打开此目录',exact:true}).click();await frame()
 assert(await page.evaluate(()=>window.dragFixture.state.panes[0].path==='/drag'),'已关闭屏的目录选择污染其他屏')
 await page.getByRole('button',{name:'切换当前目录',exact:true}).click()
 assert(await picker.getByRole('textbox',{name:'目标选择路径'}).inputValue()==='/drag','单屏切换没有从当前目录开始')
 await picker.getByRole('button',{name:'选择目录 00-target',exact:true}).click();await picker.getByRole('button',{name:'打开此目录',exact:true}).click();await frame()
 assert(await page.evaluate(target=>window.dragFixture.state.path===target,target),'单屏目录切换失败')
 checks.push('目录名/完整路径提示、封装选择器、取消、单屏及选择器归属屏保护')

 for(const column of [0,1,2,3]){
  await fresh();const cell=row(1,receiver).locator('td').nth(column)
  const to=column===0?await cell.evaluate(el=>{const r=el.getBoundingClientRect();return {x:r.right-8,y:r.top+r.height/2}}):await center(cell)
  await begin(await name(0,a),to)
  assert(await panel(1).locator('.drop-current-directory').count()===1,'列表非交互列未识别为当前目录')
  assert(await page.locator('.drop-group,.drop-folder').count()===0,'非图标文字区域仍触发条目操作')
  assert((await page.locator('.file-drag-preview').textContent()).includes('移动 1 个项目'),'当前目录数量提示错误')
  const backgrounds=await panel(1).locator('tbody tr').evaluateAll(rows=>rows.slice(0,8).flatMap(row=>[...row.children].map(td=>getComputedStyle(td).backgroundColor)))
  assert(backgrounds.every(color=>color==='rgb(244, 248, 255)'),'奇偶行或悬停遮断当前目录高亮')
  assert(await panel(1).locator('.directory-drop-outline').evaluate(el=>{const s=getComputedStyle(el),r=el.getBoundingClientRect(),list=el.previousElementSibling.getBoundingClientRect();return s.borderTopWidth==='2px'&&s.borderBottomWidth==='2px'&&s.pointerEvents==='none'&&r.top===list.top&&r.bottom===list.bottom}),'当前目录缺少独立于滚动内容的连续边框')
  if(screenshots&&column===0)await page.screenshot({path:screenshots+'/current-directory.png'})
  await release();const job=await request()
  assert(job?.name==='TransferEntries'&&job.args[1]===target&&job.paths[0]===a,'满屏列表没有移动到目标屏当前目录')
  await finish();assert(await row(1,target+'/10-a.txt').count()===1&&await row(0,a).count()===0,'来源或目标未刷新')
 }
 checks.push('满屏滚动列表：名称右侧空白与大小/类型/日期列均移入当前目录，整屏高亮连续')

 await fresh();await choose([a,source]);await begin(await name(0,a),await center(row(1,receiver).locator('td').nth(2)));await release()
 assert((await request()).paths.length===2&&(await request()).name==='TransferEntries','混选不能通过非交互区域移动');await finish()
 await fresh();await begin(await name(0,source),await center(row(1,sub).locator('td').nth(2)));await release()
 assert((await request()).args[1]===target,'文件夹行的类型列误移入子文件夹');await finish()
 await fresh();await panel(0).getByRole('button',{name:'展开 00-target',exact:true}).click();await frame()
 await begin(await name(0,receiver),await center(row(0,receiver).locator('td').nth(3)));await release()
 assert((await request()).args[1]==='/drag'&&(await request()).paths[0]===receiver,'同屏展开项目非交互落点未移入当前目录');await finish()
 checks.push('目录、混选、文件夹元数据列与同屏展开内容的当前目录移动')

 for(const [path,kind,selected] of [[sub,'folder',false],[target+'/01-folder','folder',true],[receiver,'group',false],[receiver,'group',true]]){
  await fresh()
  if(selected){await page.evaluate(path=>{const f=window.dragFixture;f.state.panes[1].selected=[f.listing.find(e=>e.path===path)]},path);await frame()}
  await begin(await name(0,a),await name(1,path))
  assert(await row(1,path).evaluate((el,kind)=>el.classList.contains('drop-'+kind),kind),'条目名称落点未识别')
  assert((await colors(row(1,path))).every(color=>color==='rgb(234, 242, 255)'),'奇偶/选中/悬停行覆盖条目蓝色高亮')
  if(screenshots&&kind==='group'&&selected)await page.screenshot({path:screenshots+'/group-highlight.png'})
  await cancel()
  assert(!(await colors(row(1,path))).includes('rgb(234, 242, 255)'),'取消后条目高亮残留')
 }
 for(const icon of [true,false]){
  await fresh();await choose([a,b]);await begin(await name(0,a),icon?await center(row(1,receiver).locator('.file-icon')):await gap(1,receiver))
  await release();assert((await request())?.name==='GroupFiles'&&(await request()).paths.length===3,'图标或名称间隔合成没有包含目标文件');await finish()
 }
 await fresh();await begin(await gap(0,a),await name(1,sub));assert(await page.locator('.selection-marquee').count()===0,'图标名称间隔拖拽误触发框选');await release();assert((await request())?.args[1]===sub,'图标名称间隔不能开始拖拽');await finish()
 checks.push('图标/名称/两者间隔落点、合成与目录移动、奇偶/选中/悬停行高亮及取消恢复')

 await fresh()
 for(const locator of [row(1,receiver).locator('input'),row(1,sub).locator('.disclosure'),row(1,sub).locator('.folder-size-button'),panel(1).locator('thead'),panel(1).locator('.pane-header')]){
  await begin(await name(0,a),await center(locator));assert(await page.evaluate(()=>!window.dragFixture.state.dragPreview?.target),'控件仍被当作落点');await release();assert(!await request(),'控件落点产生了操作')
 }
 await page.evaluate(async target=>{const s=window.dragFixture.state;await s.navigate(target,true,true,s.panes[1])},'/drag');await frame()
 await begin(await name(0,a),await name(1,'/drag/13-link'));assert(await page.evaluate(()=>!window.dragFixture.state.dragPreview?.target),'链接名称回退为当前目录');await release();assert(!await request(),'链接名称触发操作')
 await fresh();await begin(await name(0,source),await name(1,receiver));assert(await page.evaluate(()=>!window.dragFixture.state.dragPreview?.target),'目录向文件时回退成当前目录移动');await release();assert(!await request(),'无效合成仍移动目录')
 await begin(await name(0,target),await name(1,sub));assert(await page.evaluate(()=>!window.dragFixture.state.dragPreview?.target),'子目录名称无效后回退当前目录');await release();assert(!await request(),'目录移入自身或子目录')
 await begin(await name(0,a),await center(row(1,receiver).locator('td').nth(2)));await cancel()
 assert(await page.locator('.drop-current-directory,.drop-folder,.drop-group,.file-drag-preview').count()===0,'取消后高亮残留')
 assert(await page.evaluate(()=>window.dragFixture.unhandled.length===0),'存在未处理异步异常')
 checks.push('复选框/展开/计算/表头/路径栏不接收，链接与无效图标名称不回退，取消无残留')
 return {passed:true,checks}
}
