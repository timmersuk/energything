async page => {
 const assert=(ok,msg)=>{if(!ok)throw new Error(msg)};
 await page.context().setHTTPCredentials({username:'synthetic',password:'synthetic'});
 const base='http://127.0.0.1:8087';let outbound=0;
 page.on('request',r=>{if(!r.url().startsWith(base))outbound++});
 const status=await(await page.context().request.get(base+'/api/v1/status')).json();
 assert(status.build_id==='task05a-growatt-aggregates','actual task build');
 await page.goto(base+'/records/growatt/aggregates?limit=2');
 let text=await page.locator('main').innerText();
 assert(text.includes('SYN-PLANT-A')&&text.includes('1.234567890123456789'),'full identity and exact value');
 assert(await page.getByRole('slider').count()===0,'no custom scroll control');
 const row=await page.locator('tbody tr').first().boundingBox();assert(row.height<140,'compact rows');
 await page.locator('details').first().locator('summary').click();assert((await page.locator('details').first().innerText()).includes('retrieval'),'expand provenance');
 await page.getByRole('link',{name:'Next page'}).click();text=await page.locator('main').innerText();assert(text.includes('reported_zero_availability_unknown'),'zero qualification');
 await page.getByRole('link',{name:'Growatt samples'}).click();assert((await page.locator('main').innerText()).includes('Growatt'),'sample navigation');
 await page.goto(base+'/records/growatt/aggregates');await page.getByLabel('Plant',{exact:true}).fill('SYN-PLANT-B');await page.getByRole('button',{name:'Filter',exact:true}).click();text=await page.locator('main').innerText();assert(text.includes('SYN-PLANT-B')&&!text.includes('SYN-PLANT-A'),'plant filter');
 await page.screenshot({path:'.cache/task05a-browser.png',fullPage:true});
 await page.goto(base+'/records/octopus?from=2026-03-29&to=2026-03-29');assert((await page.locator('main').innerText()).includes('0.100000000000000001'),'Octopus retained');
 const source=status.providers.find(p=>p.source==='growatt').sources.find(s=>s.plant_id==='SYN-PLANT-A');assert(source.last_attempt.outcome==='rejected'&&source.last_accepted_attempt.outcome==='success','rejection preserves acceptance');
 assert(outbound===0,'local browsing');return {passed:true,build:status.build_id,exact:true,compact:true,paging:true,qualifiedZero:true,rejectionPreserved:true,outbound};
}
