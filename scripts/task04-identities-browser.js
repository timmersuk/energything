// Actual image on isolated synthetic mounts: Octopus 8083, Growatt 8087.
async page => {
 const assert=(ok,message)=>{if(!ok)throw new Error(message)};
 await page.context().setHTTPCredentials({username:'synthetic',password:'synthetic'});
 await page.goto('http://127.0.0.1:8087/');
 let text=await page.locator('main').innerText();
 assert(text.includes('plant synthetic-plant')&&text.includes('device synthetic-inverter'),'Growatt status identifiers');
 assert(!text.includes('(masked)')&&!text.includes('synthetic-private-device'),'no opaque identity');
 await page.goto('http://127.0.0.1:8087/records/growatt?from=2026-03-29&to=2026-03-29');
 text=await page.locator('main').innerText();assert(text.includes('synthetic-plant')&&text.includes('synthetic-inverter'),'Growatt records identifiers');
 assert(await page.locator('script').count()===0,'provider text remains escaped');
 const samples=await(await page.context().request.get('http://127.0.0.1:8087/api/v1/growatt/samples?from=2026-03-29&to=2026-03-29')).json();
 assert(samples.records.length>0&&samples.records.every(r=>r.plant_id==='synthetic-plant'&&r.serial==='synthetic-inverter'),'Growatt JSON identities');
 await page.screenshot({path:'.cache/task04-identities-growatt.png',fullPage:true});
 await page.goto('http://127.0.0.1:8083/records/octopus?from=2026-03-29&to=2026-03-29');
 text=await page.locator('main').innerText();assert(text.includes('MPAN synthetic-import')&&text.includes('MPAN synthetic-export'),'Octopus identifiers retained');
 const browser=page.context().browser();const unauth=await browser.newContext();
 for(const path of ['/','/records/growatt','/api/v1/growatt/samples'])assert((await unauth.request.get('http://127.0.0.1:8087'+path)).status()===401,'auth');
 await unauth.close();
 assert((await page.context().request.get('http://127.0.0.1:8087/api/v1/audit')).status()===404,'audit task remains separate');
 return {passed:true,fullGrowattIdentifiers:true,fullOctopusIdentifiers:true,authenticated:true};
}
