// Local table navigation only; no network requests or stored-data changes.
const panel = document.querySelector('.growatt-scroll');
const controls = document.querySelector('.growatt-scroll-controls');
if (panel && controls) {
  const across = controls.querySelector('[data-axis="x"]');
  const down = controls.querySelector('[data-axis="y"]');
  const update = () => {
    for (const [input, max, position] of [[across, panel.scrollWidth-panel.clientWidth, panel.scrollLeft], [down, panel.scrollHeight-panel.clientHeight, panel.scrollTop]]) {
      input.disabled = max <= 0;
      input.value = max > 0 ? Math.round(position/max*1000) : 0;
    }
  };
  across.addEventListener('input', () => { panel.scrollLeft = Number(across.value)/1000*(panel.scrollWidth-panel.clientWidth); });
  down.addEventListener('input', () => { panel.scrollTop = Number(down.value)/1000*(panel.scrollHeight-panel.clientHeight); });
  panel.addEventListener('scroll', update, {passive:true});
  panel.addEventListener('toggle', update, true);
  new ResizeObserver(update).observe(panel.querySelector('table'));
  new ResizeObserver(update).observe(panel);
  controls.hidden = false;
  update();
}
