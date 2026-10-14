async page => {
 const assert=(ok,msg)=>{if(!ok)throw new Error(msg)};
 await page.context().setHTTPCredentials({username:'synthetic',password:'synthetic'});
 const base='http://127.0.0.1:8087';let outbound=0;
 page.on('request',r=>{if(!r.url().startsWith(base))outbound++});
 await page.goto(base+'/');
 const status=await (await page.context().request.get(base+'/api/v1/status')).json();
 assert(status.build_id==='task05-growatt','task image build');
 const source=status.providers.find(p=>p.source==='growatt').sources.find(s=>s.serial==='SYN-A');
 assert(source.plant_id==='123'&&source.last_attempt.outcome==='failed'&&source.last_accepted_attempt.outcome==='success','failure preserves acceptance');
 await page.goto(base+'/records/growatt?view=quality&provider_date=2026-10-13');
 const body=await page.locator('main').innerText();
 assert(body.includes('plant 123')&&body.includes('device SYN-A'),'identifiers');
 assert(body.includes('123.4567890123456789')&&body.includes('bmsSOC: 12')&&body.includes('unit unresolved'),'exact and qualified fields');
 assert(body.includes('soc: 0')&&body.includes('Unresolved')&&body.includes('provider counters'),'zero and unresolved');
 const records=await (await page.context().request.get(base+'/api/v1/growatt/samples?view=quality&provider_date=2026-10-13')).json();
 const sample=records.records.find(s=>s.serial==='SYN-A');
 assert(sample.measured_at===null&&sample.values.soc.value==='0'&&sample.values.bmsSOC.unit===null,'API exact null contract');
 await page.screenshot({path:'.cache/task05-growatt-browser.png',fullPage:true});
 await page.goto(base+'/records/octopus?from=2026-03-29&to=2026-03-29');assert((await page.locator('main').innerText()).includes('0.100000000000000001'),'Octopus browsing during outage');
 assert(outbound===0,'cloud calls from browsing');
 return {passed:true,build:'task05-growatt',outbound,exactValues:true,qualifiedTime:true,priorAcceptancePreserved:true,octopusUsable:true};
}
