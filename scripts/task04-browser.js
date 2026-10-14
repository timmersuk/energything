// Actual task04 image, isolated synthetic mounts on 8083/8084/8085 only.
async page => {
 const assert=(ok,message)=>{if(!ok)throw new Error(message)};
 await page.context().setHTTPCredentials({username:'synthetic',password:'synthetic'});
 let outbound=0;page.on('request',r=>{if(!/^http:\/\/127\.0\.0\.1:808[345]\//.test(r.url()))outbound++});
 for(const [port,expected] of [[8083,['selected','selected']],[8084,['selected','selection_required']]]){
  const base='http://127.0.0.1:'+port;
  await page.goto(base+'/');
  const status=await (await page.context().request.get(base+'/api/v1/status')).json();
  assert(status.build_id==='task04-octopus','actual task build');
  const streams=status.providers[0].streams;
  assert(streams.map(x=>x.state).join(',')===expected.join(','),'independent discovery');
  const text=await page.locator('main').innerText();
  assert(text.includes('export discovery: selected'),'export visible');
  assert(text.includes(port===8083?'import discovery: selected':'import discovery: selection_required'),'import visible');
  assert(text.includes('MPAN synthetic-import') && text.includes('MPAN synthetic-export'),'full meter identifiers');
  assert(text.includes('expected 46')&&text.includes('missing 45'),'latest coverage');
  await page.screenshot({path:'.cache/task04-status-'+port+'.png',fullPage:true});
  await page.getByRole('link',{name:'Octopus records',exact:true}).click();
  await page.getByLabel('From',{exact:true}).fill('2026-03-29');await page.getByLabel('To',{exact:true}).fill('2026-03-29');
  await page.getByRole('button',{name:'Apply filters'}).click();
  assert((await page.locator('main').innerText()).includes('0.100000000000000001'),'exact decimal visible');
  const response=await page.context().request.get(base+'/api/v1/octopus/intervals?from=2026-03-29&to=2026-03-29');const records=await response.json();
  assert(records.records.length===(port===8083?2:1),'independent retained rows');
  assert(records.records.every(r=>typeof r.kwh==='string'),'decimal JSON');
  assert(records.records.every(r=>r.mpan && r.serial && r.identity.includes(r.mpan)),'explicit MPAN and serial JSON');
  await page.screenshot({path:'.cache/task04-records-'+port+'.png',fullPage:true});
  for(const path of ['/audit','/api/v1/audit','/api/v1/refresh'])assert((await page.context().request.get(base+path)).status()===404,'task boundary');
 }
 await page.goto('http://127.0.0.1:8085/');assert((await page.locator('main').innerText()).includes('No stored identities'),'empty');
 assert(outbound===0,'browsing outbound');
 return {passed:true,build:'task04-octopus',independentDiscovery:true,exactDecimals:true,calendarSlots:46,outboundRequests:outbound};
}
