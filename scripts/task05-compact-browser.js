async page => {
 await page.context().setHTTPCredentials({username:'synthetic',password:'synthetic'});
 const base='http://127.0.0.1:8087';
 await page.goto(base+'/records/growatt?view=quality&provider_date=2026-10-13');
 const assert=(x,m)=>{if(!x)throw new Error(m)};
 const region=page.getByRole('region',{name:'Growatt samples'});
 assert(await page.locator('.sample-details[open]').count()===0,'details should start closed');
 const row=await page.locator('.growatt-table tbody tr').first().boundingBox();assert(row.height<140,'compact row height');
 const scroll=await region.evaluate(el=>({overflow:getComputedStyle(el).overflowX,wide:el.scrollWidth>el.clientWidth,maxHeight:getComputedStyle(el).maxHeight}));assert(scroll.overflow==='scroll'&&scroll.wide,'horizontal scroll container');
 assert(!(await page.locator('main').innerText()).includes('Original timestamp evidence'),'raw evidence initially hidden');
 await page.locator('.sample-details summary').first().click();
 assert((await page.locator('.sample-detail-content').first().innerText()).includes('123.4567890123456789'),'exact decimal detail');
 await page.locator('.sample-details summary').first().click();
 await page.getByRole('slider',{name:'Horizontal table position'}).fill('1000');
 assert(await region.evaluate(el=>el.scrollLeft>0),'visible slider moves table');
 await page.getByRole('slider',{name:'Horizontal table position'}).fill('0');
 await page.screenshot({path:'.cache/task05-growatt-compact.png',fullPage:true});
 return {compactRows:true,rowHeight:row.height,visibleScrollbarTrack:true,detailsExpandable:true,exactValuesRetained:true};
}
