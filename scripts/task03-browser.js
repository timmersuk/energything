// Run with playwright-cli run-code --filename=scripts/task03-browser.js.
// Requires isolated synthetic task containers on 8081 (fixtures), 8082 (empty).
async page => {
  const assert = (condition, message) => { if (!condition) throw new Error(message); };
  const fixture = 'http://127.0.0.1:8081';
  const empty = 'http://127.0.0.1:8082';
  const paths = ['/', '/records/octopus', '/records/growatt', '/api/v1/status', '/api/v1/octopus/intervals', '/api/v1/growatt/samples', '/static/style.css'];
  const browser = page.context().browser();
  let outbound = 0;
  page.on('request', request => { if (!request.url().startsWith(fixture) && !request.url().startsWith(empty)) outbound++; });
  for (const credentials of [undefined, {username: 'synthetic', password: 'wrong'}]) {
    const context = await browser.newContext({httpCredentials: credentials});
    for (const path of paths) {
      const response = await context.request.get(fixture + path);
      assert(response.status() === 401, 'authentication: ' + path);
      assert(response.headers()['www-authenticate'], 'missing auth challenge');
      assert(!(await response.text()).includes('records'), 'unauthenticated data');
    }
    await context.close();
  }
  await page.context().setHTTPCredentials({username: 'synthetic', password: 'synthetic'});
  const get = async path => {
    const response = await page.context().request.get(fixture + path);
    assert(response.status() === 200, path + ': ' + response.status());
    return response.json();
  };
  await page.goto(fixture + '/');
  const statusText = await page.locator('main').innerText();
  assert(statusText.includes('Latest retrieval') && statusText.includes('Newest measurement'), 'status times');
  assert(statusText.includes('2026-10-26T12:00:00Z') && statusText.includes('2026-10-25T01:30:00Z'), 'old measurement fetched later');
  assert(statusText.includes('missing 47') && statusText.includes('Selected: false'), 'retained partial coverage');
  assert(statusText.includes('Provider transport failed'), 'sanitized failure state');
  await page.screenshot({path: '.cache/task03-status.png', fullPage: true});
  await page.getByRole('link', {name: 'Octopus records', exact: true}).click();
  await page.getByLabel('From', {exact: true}).fill('2026-10-25');
  await page.getByLabel('To', {exact: true}).fill('2026-10-25');
  await page.getByLabel('Rows', {exact: true}).fill('3');
  await page.getByRole('combobox', {name: 'Direction', exact: true}).selectOption('import');
  await Promise.all([page.waitForNavigation(), page.getByRole('button', {name: 'Apply filters'}).click()]);
  assert(await page.locator('tbody tr').count() === 3, 'bounded HTML rows');
  const first = await page.locator('tbody').innerText();
  assert(first.includes('0.100000000000000001 kWh') && first.includes('0.000 kWh'), 'exact decimal HTML');
  await Promise.all([page.waitForNavigation(), page.getByRole('link', {name: 'Next page'}).click()]);
  assert(await page.locator('tbody tr').count() === 3 && await page.locator('tbody').innerText() !== first, 'HTML next page');
  await page.screenshot({path: '.cache/task03-octopus.png', fullPage: true});
  for (const [day, slots] of [['2026-03-29', 46], ['2026-10-25', 50]]) {
    const result = await get('/api/v1/octopus/intervals?from=' + day + '&to=' + day + '&direction=import&limit=500');
    assert(result.records.length === slots, day + ' slot count');
    assert((Date.parse(result.to) - Date.parse(result.from)) / 1800000 === slots, day + ' bounds');
  }
  let cursor = '', seen = new Set(), pages = 0;
  do {
    const result = await get('/api/v1/octopus/intervals?from=2026-10-25&to=2026-10-25&limit=3' + (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''));
    assert(result.records.length <= 3, 'API row bound');
    for (const row of result.records) {
      const key = row.identity_id + '/' + row.measured_at;
      assert(!seen.has(key), 'pagination duplicate'); seen.add(key);
      assert(typeof row.kwh === 'string' && row.unit === 'kWh', 'decimal string contract');
    }
    cursor = result.next_cursor || ''; pages++;
  } while (cursor);
  assert(seen.size === 100 && pages === 34, 'pagination omissions/equal timestamp tie-breaker');
  const resolved = await get('/api/v1/growatt/samples?from=2026-10-25&to=2026-10-25');
  assert(resolved.records.length === 2 && resolved.omitted_unresolved === 4, 'unresolved exclusion count');
  assert(resolved.records[0].values.soc.value === null && resolved.records[1].values.soc.value === '0', 'null versus zero');
  assert(resolved.records[1].values.counter.unit === null, 'null unit');
  assert(resolved.records[0].measured_at === '2026-10-25T00:30:00Z' && resolved.records[1].measured_at === '2026-10-25T01:30:00Z', 'repeated London hour distinct UTC');
  await page.goto(fixture + '/records/growatt?from=2026-10-25&to=2026-10-25');
  assert((await page.locator('tbody').innerText()).includes('null (missing)'), 'HTML null');
  assert((await page.locator('tbody').innerText()).includes('soc: 0 %'), 'HTML zero');
  assert(await page.locator('script').count() === 0 && !await page.evaluate(() => window.fixtureExecuted), 'provider script executed');
  assert((await page.locator('tbody').innerText()).includes('<script>'), 'provider text not visible escaped');
  await page.getByLabel('From', {exact: true}).fill('');
  await page.getByLabel('To', {exact: true}).fill('');
  await page.getByRole('combobox', {name: 'View', exact: true}).selectOption('quality');
  await page.getByLabel('Provider date (quality view only)', {exact: true}).fill('2026-10-25');
  await Promise.all([page.waitForNavigation(), page.getByRole('button', {name: 'Apply filters'}).click()]);
  assert(await page.locator('tbody tr').count() === 4, 'quality provider-date rows');
  assert((await page.locator('tbody').innerText()).includes('Unresolved'), 'unresolved label');
  await page.screenshot({path: '.cache/task03-growatt.png', fullPage: true});
  const quality = await get('/api/v1/growatt/samples?view=quality&from=2026-10-26&to=2026-10-26&quality=unresolved');
  assert(quality.records.length === 4 && quality.range_basis === 'retrieval', 'retrieval quality filter');
  for (const path of ['/api/v1/octopus/intervals?limit=501', '/records/octopus?cursor=malformed', '/api/v1/growatt/samples?from=2026-10-25', '/api/v1/octopus/intervals?from=2026-10-25&to=2026-10-26T00:00:00Z', '/api/v1/octopus/intervals?from=2026-10-25T00:00:00%2B24:00&to=2026-10-27T00:00:00Z']) {
    const response = await page.context().request.get(fixture + path);
    assert(response.status() === 400 && (await response.json()).error.code === 'invalid_query', 'safe error: ' + path);
  }
  for (const path of ['/audit', '/api/v1/audit', '/api/v1/refresh']) {
    assert((await page.context().request.get(fixture + path)).status() === 404, 'out of scope route');
  }
  for (const path of ['/', '/records/octopus', '/records/growatt']) {
    await page.goto(empty + path);
    assert((await page.locator('main').innerText()).includes('No stored'), 'empty state: ' + path);
  }
  await page.screenshot({path: '.cache/task03-empty.png', fullPage: true});
  assert(outbound === 0, 'external browser requests');
  return {passed: true, authenticatedRoutes: paths.length, calendarSlots: [46,50], paginatedRows: seen.size, pages, unresolvedExcluded: resolved.omitted_unresolved, outboundRequests: outbound};
}
