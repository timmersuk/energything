async page => {
 const credentials=__PRIVATE_AUTH__;
 await page.context().setHTTPCredentials(credentials);
 await page.goto('http://localhost:8080/');
 const status=await (await page.context().request.get('http://localhost:8080/api/v1/status')).json();
 if(status.build_id!=='task05-growatt')throw new Error('Preview build mismatch');
 await page.goto('http://localhost:8080/records/growatt?view=quality');
 const text=await page.locator('main').innerText();
 if(!text.includes('provider counters')||!text.includes('Growatt stored samples'))throw new Error('Updated preview UI missing');
 await page.goto('http://localhost:8080/records/octopus');
 if(!(await page.locator('main').innerText()).includes('Octopus'))throw new Error('Octopus browsing unavailable');
 return {previewBuildVerified:true,changedUIVerified:true,octopusBrowsing:true};
}
