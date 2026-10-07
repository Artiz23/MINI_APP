const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
const path=require('node:path');
const {chromium}=require('playwright');

async function fixture(viewport,admin=false){
 const browser=await chromium.launch({headless:true,...(process.env.BROWSER_EXECUTABLE?{executablePath:process.env.BROWSER_EXECUTABLE}:{})});
 const page=await browser.newPage({viewport});page.setDefaultTimeout(10000);
 const errors=[];page.on('pageerror',e=>errors.push(e.message));
 let deleted=0;const writes=[];
 const rows=[{id:1,uid:'REQ-1',status:'in_progress',workflow_stage:'approval',created_by:7,comment:'Комментарий'},{id:2,uid:'REQ-2',status:'open',workflow_stage:'approval',created_by:7},{id:3,uid:'FOREIGN',status:'in_progress',workflow_stage:'payment',created_by:8},{id:4,uid:'REQ-4',status:'failed',workflow_stage:'complete',created_by:7,close_reason:'Причина'}];
 const decorate=r=>({...r,can_view:true,can_edit:admin||r.created_by===7,can_delete:admin||r.created_by===7,is_mine:r.created_by===7,created_at:'2026-10-07T10:00:00+03:00',files:[],payments:[],appeals:[],approvals:[],economics:{}});
 await page.route('**/*',async route=>{
  const req=route.request(),url=new URL(req.url()),method=req.method();
  if(url.hostname!=='miniapp.test')return route.fulfill({body:'',contentType:'text/javascript'});
  if(!url.pathname.startsWith('/api/')){
   const name=url.pathname==='/'?'index.html':path.basename(url.pathname);
   try{return route.fulfill({body:await fs.readFile(path.join(__dirname,'..','web',name)),contentType:name.endsWith('.js')?'text/javascript':name.endsWith('.css')?'text/css':'text/html'});}catch{return route.fulfill({status:404,body:''});}
  }
  let data={};const body=method==='POST'?req.postDataJSON():{};
  if(method!=='GET')writes.push({url:url.pathname,method,body});
  if(url.pathname==='/api/me')data={id:7,name:'Test',owner:admin,admin,employee:{id:1,name:'Test'},sections:['requests','tasks','rates','directory','documents','meetings','analytics','payments','approvals'],unread:{}};
  else if(url.pathname==='/api/directory')data={managers:[{id:10,name:'Manager'}],clients:[{id:20,name:'Client'}],counterparties:[{id:30,name:'CP'}],employees:[],positions:[],companies:[]};
  else if(url.pathname==='/api/requests'){
   if(method==='DELETE'){deleted++;const i=rows.findIndex(r=>r.id===Number(url.searchParams.get('id')));rows.splice(i,1);data={ok:true};}
   else if(method==='POST'){
    if(body.id){const r=rows.find(r=>r.id===body.id);if(body.action==='edit_comment')r.comment=body.comment;else if(body.status){r.status=body.status;r.close_reason=body.close_reason;}data=decorate(r);}
    else{const r={id:10,uid:'NEW',status:'open',workflow_stage:'approval',created_by:7,comment:body.title};rows.push(r);data=decorate(r);}
   }else data=url.searchParams.has('id')?decorate(rows.find(r=>r.id===Number(url.searchParams.get('id')))):rows.filter(r=>url.searchParams.get('scope')==='all'||r.created_by===7).map(decorate);
  }
  else if(url.pathname==='/api/rates')data={rapira:100,cbr:[{code:'USD',per_unit:90}],rub:[{pair:'USD/RUB',bid:91}],forex:[{pair:'EUR/USD',bid:1.1}],xe_eurusd:1.1,investing:92,custom_slots:[{label:'Мой курс',value:100}]};
  else if(url.pathname==='/api/meetings')data=[];
  else if(url.pathname==='/api/agents')data=[];
  else if(url.pathname==='/api/analytics')data={rows:[],period_basis:'Заявки за период',currency_policy:'По валютам',profit_available:true,profit_note:'Прибыль указана сотрудниками вручную'};
  else if(url.pathname==='/api/app-settings')data={approval_chat_id:-100,payment_chat_id:-200,custom_rate_slots:[{},{},{}]};
  else if(url.pathname==='/api/chats')data={items:[{id:1,name:'Согласование',chat_id:-100},{id:2,name:'Оплата',chat_id:-200},{id:3,name:'Новая группа',chat_id:-300}]};
  else if(url.pathname==='/api/archive-settings')data={settings:{day:31,time_hhmm:'23:00',sections:['requests']},sections:['requests','meetings']};
  else if(url.pathname==='/api/archive-now')data={id:99,name:'miniapp.zip'};
  else if(url.pathname==='/api/attach')data={waiting:false};
  else if(url.pathname==='/api/documents')data={companies:[],counterparties:[],files:[],tabs:[],can_edit:admin};
  return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
 });
 return {browser,page,errors,writes,deleted:()=>deleted};
}
for(const viewport of [{width:390,height:844},{width:1280,height:900}]){
 test(`request access, filters and actions ${viewport.width}px`,async()=>{
  const f=await fixture(viewport);const {page}=f;
  try{
   await page.goto('http://miniapp.test/?go=requests');await page.locator('[data-openreq="1"]').waitFor();
   assert.equal(await page.locator('[data-request-card]').count(),3);
   assert.equal(await page.locator('[data-reqfilter]').count(),3);
   assert.equal(await page.locator('[data-request-card] button:not([data-openreq])').count(),0);
   await page.locator('#reqTitle').fill('Черновик');await page.locator('[data-reqfilter="in_progress"]').uncheck();
   assert.equal(await page.locator('#reqTitle').inputValue(),'Черновик');assert.equal(await page.locator('[data-openreq="1"]').count(),0);
   await page.locator('[data-reqfilter="in_progress"]').check();
   await page.locator('[data-reqscope="all"]').click();await page.locator('[data-openreq="3"]').click();
   await page.getByText('Только просмотр',{exact:true}).waitFor();
   assert.equal(await page.locator('#rqPay,#rqApr,#rqTask,#rqAppeal,[data-delreq],[data-st]').count(),0);
   await page.locator('#rqDocs').click();await page.getByRole('heading',{name:/Файлы сделки/}).waitFor();
   assert.equal(await page.locator('#docTg,#docMsgBtn,[data-delfile]').count(),0);
   await page.goto('http://miniapp.test/?go=requests');await page.locator('[data-openreq="1"]').click();
   await page.locator('#reqEditComment').waitFor();assert.equal(await page.locator('#rqSaldo').count(),0);
   assert.equal(await page.locator('#rqPay').isDisabled(),false);assert.equal(await page.locator('#rqApr').isDisabled(),false);
   await page.locator('#rqPay').click();await page.getByText('Сейчас нужно завершить согласование',{exact:true}).waitFor();
   await page.locator('#reqEditComment').click();await page.locator('#requestComment').fill('Новый комментарий');await page.locator('dialog').getByRole('button',{name:'Сохранить',exact:true}).click();
   await page.getByText('Новый комментарий',{exact:true}).waitFor();
   page.once('dialog',d=>d.dismiss());await page.locator('[data-delreq]').click();assert.equal(f.deleted(),0);
   page.once('dialog',d=>d.accept());await page.locator('[data-delreq]').click();await page.locator('#reqCreate').waitFor();assert.equal(f.deleted(),1);
   await page.locator('[data-reqfilter="open"]').uncheck();await page.locator('#reqTitle').fill('Новая');await page.locator('#reqMgr').selectOption('10');await page.locator('#reqClient').selectOption('20');await page.locator('#reqCP').selectOption('30');await page.locator('#reqCreate').click();await page.locator('[data-openreq="10"]').waitFor();
   await page.locator('[data-openreq="10"]').click();await page.locator('#rqApr').click();await page.waitForTimeout(50);
   assert.ok(f.writes.some(x=>x.url==='/api/requests'&&x.body.id===10&&x.body.status==='in_progress'));
   assert.ok(f.writes.some(x=>x.url==='/api/approvals'&&x.body.request_id===10));
   assert.equal(await page.locator('[data-dock="rates"]').count(),1);assert.equal(await page.locator('[data-dock="tasks"]').count(),0);
   await page.locator('[data-dock="rates"]').click();await page.getByRole('heading',{name:'Мой курс',exact:true}).waitFor();
   assert.equal(await page.locator('.card').count(),7);assert.equal(await page.locator('#rateSettings').count(),0);
   await page.locator('#menuBtn').click();await page.locator('[data-go="tasks"]').waitFor();await page.locator('[data-go="meetings"]').click();await page.locator('#meetingCreate').click();
   await page.locator('dialog input[name="title"]').fill('Встреча');await page.locator('dialog input[name="url"]').fill('https://example.com');await page.locator('dialog input[name="starts"]').fill('2026-10-08T14:00');await page.locator('dialog').getByRole('button',{name:'Сохранить',exact:true}).click();await page.locator('dialog').waitFor({state:'detached'});
   assert.equal(f.writes.find(x=>x.url==='/api/meetings').body.starts_at,'2026-10-08T14:00:00+03:00');
   if(process.env.SCREENSHOT_DIR)await page.screenshot({path:path.join(process.env.SCREENSHOT_DIR,`meetings-${viewport.width}.png`)});
   assert.deepEqual(f.errors,[]);
  }finally{await f.browser.close();}
 });
}
test('admin archive settings and chat settings',async()=>{
 const f=await fixture({width:390,height:844},true);const {page}=f;
 try{
  await page.goto('http://miniapp.test/?go=requests');await page.getByRole('button',{name:'⚙ Настройки чата',exact:true}).click();await page.locator('#sectionChat').selectOption('-300');await page.locator('dialog').getByRole('button',{name:'Сохранить',exact:true}).click();await page.locator('dialog').waitFor({state:'detached'});
  assert.deepEqual(f.writes.find(x=>x.url==='/api/app-settings').body,{scope:'approval',chat_id:-300});
  await page.goto('http://miniapp.test/?go=documents');await page.getByRole('button',{name:'⚙ Автосохранение Mini App',exact:true}).click();await page.locator('#snapshotNow').click();await page.getByText('Сохранено: miniapp.zip',{exact:true}).waitFor();
  assert.ok(f.writes.some(x=>x.url==='/api/archive-now'));assert.deepEqual(f.errors,[]);
 }finally{await f.browser.close();}
});
