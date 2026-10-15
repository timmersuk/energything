async page => {
 await page.context().setHTTPCredentials(__PRIVATE_AUTH__);
 const base='http://localhost:8080';
 const status=await(await page.context().request.get(base+'/api/v1/status')).json();
 if(status.build_id!=='task05a-growatt-aggregates')throw new Error('Build mismatch');
 await page.goto(base+'/records/growatt/aggregates?from=2026-01-15&to=2026-01-15');
 const rows=page.locator('tbody tr');if(await rows.count()!==1)throw new Error('Older row unavailable');
 const api=await(await page.context().request.get(base+'/api/v1/growatt/aggregates?from=2026-01-15&to=2026-01-15')).json();
 const window=await(await page.context().request.get(base+'/api/v1/growatt/aggregates?from=2026-01-12&to=2026-01-18')).json();
 if(window.records.length!==7||window.records[0].period!=='2026-01-12'||window.records[6].period!=='2026-01-18')throw new Error('Seven-date window unavailable');
 const record=api.records[0];const text=await page.locator('main').innerText();
 if(!record||!text.includes(record.plant_id)||!text.includes(record.value)||record.unit!=='kWh'||record.period_start!==null||record.calendar!=='provider_date_zone_unverified')throw new Error('Private aggregate view mismatch');
 await page.locator('details summary').click();if(!(await page.locator('details').innerText()).includes('retrieval'))throw new Error('Provenance missing');
 await page.goto(base+'/records/octopus');if(!(await page.locator('main').innerText()).includes('Octopus'))throw new Error('Octopus unavailable');
 await page.goto(base+'/records/growatt?view=quality');if(!(await page.locator('main').innerText()).includes('Growatt'))throw new Error('Detailed records unavailable');
 return {previewBuildVerified:true,olderAggregateVerified:true,existingBrowsingVerified:true};
}
