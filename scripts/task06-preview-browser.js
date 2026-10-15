async page => {
 await page.context().setHTTPCredentials(__PRIVATE_AUTH__);
 const base='http://localhost:8080';
 await page.goto(base+'/');
 const text=await page.locator('main').innerText();
 if(!text.includes('task06-scheduling')||!text.includes('Automatic progress'))throw new Error('Preview build/status mismatch');
 await page.goto(base+'/records/growatt/aggregates');
 if(await page.locator('tbody tr').count()===0)throw new Error('Retained generation unavailable');
 await page.goto(base+'/records/octopus');
 if(await page.locator('tbody tr').count()===0)throw new Error('Retained Octopus unavailable');
 return {previewBuildVerified:true,retainedBrowsingVerified:true};
}
