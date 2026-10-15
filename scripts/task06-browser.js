async page => {
 const assert=(ok,message)=>{if(!ok)throw new Error(message)};
 await page.context().setHTTPCredentials({username:'synthetic',password:'synthetic'});
 const base='http://127.0.0.1:8087';let external=0;
 page.on('request',r=>{if(!r.url().startsWith(base))external++});
 await page.goto(base+'/');
 assert((await page.locator('main').innerText()).includes('task06-scheduling'),'actual image build');
 assert(await page.getByRole('button',{name:'Refresh recent octopus'}).count()===1,'refresh UI');
 await page.getByRole('button',{name:'Refresh recent octopus'}).click();
 await page.getByRole('status').filter({hasText:'Routine work queued'}).waitFor();
 const status=await(await page.context().request.get(base+'/api/v1/status')).json();
 assert(status.schedules.some(p=>p.policy==='routine'),'durable local queue');
 assert(status.schedules.some(p=>p.search_policy==='calendar'),'continuous archive policy retained');
 assert(status.providers.some(p=>p.streams?.some(s=>s.state==='failed')),'provider outage visible');
 const rejected=await page.context().request.post(base+'/api/v1/refresh',{data:{source:'octopus'}});
 assert(rejected.status()===403,'missing CSRF rejected');
 await page.goto(base+'/records/octopus?from=2026-03-29&to=2026-03-29');
 assert((await page.locator('main').innerText()).includes('0.100000000000000001'),'browsing during source outage');
 await page.goto(base+'/records/growatt/aggregates?limit=2');
 assert((await page.locator('main').innerText()).includes('1.234567890123456789'),'independent aggregates retained');
 await page.setViewportSize({width:390,height:844});
 await page.goto(base+'/');assert(await page.getByRole('heading',{name:'Automatic progress'}).count()===1,'mobile status');
 assert((await page.locator('main').innerText()).includes('extends automatically; no lookback stop'),'calendar target is not an archive cap');
 await page.screenshot({path:'.cache/task06-browser.png',fullPage:true});
 assert(external===0,'browser stayed local');
 return {passed:true,actualImage:true,refresh:true,csrf:true,providerOutage:true,storedBrowsing:true,independentGeneration:true,external};
}
