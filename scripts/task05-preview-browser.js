async page => {
 const credentials=__PRIVATE_AUTH__;
 await page.context().setHTTPCredentials(credentials);
 await page.goto('http://localhost:8080/');
 const status=await (await page.context().request.get('http://localhost:8080/api/v1/status')).json();
 if(status.build_id!=='task05-growatt-ui')throw new Error('Preview build mismatch');
 await page.goto('http://localhost:8080/records/growatt?view=quality');
 const text=await page.locator('main').innerText();
 if(!text.includes('energy values are counters')||!text.includes('Growatt stored samples'))throw new Error('Updated preview UI missing');
 const region=page.getByRole('region',{name:'Growatt samples'});
 const height=await page.locator('.growatt-table tbody tr').first().boundingBox();
 if(height.height>140)throw new Error('Preview rows are not compact');
 await page.getByRole('slider',{name:'Horizontal table position'}).fill('1000');
 if(!(await region.evaluate(el=>el.scrollLeft>0)))throw new Error('Horizontal slider unavailable');
 await page.getByRole('slider',{name:'Vertical table position'}).fill('1000');
 if(!(await region.evaluate(el=>el.scrollTop>0)))throw new Error('Vertical slider unavailable');
 await page.goto('http://localhost:8080/records/octopus');
 if(!(await page.locator('main').innerText()).includes('Octopus'))throw new Error('Octopus browsing unavailable');
 return {previewBuildVerified:true,changedUIVerified:true,octopusBrowsing:true};
}
