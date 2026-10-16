document.addEventListener('click', async (event) => {
 const button = event.target.closest('button[data-evidence]');
 if (!button) return;
 const output = button.parentElement.querySelector('pre');
 output.hidden = false;
 output.textContent = 'Loading retained evidence…';
 button.disabled = true;
 try {
  const response = await fetch(button.dataset.evidence, {credentials: 'same-origin', cache: 'no-store'});
  if (!response.ok) throw new Error('Evidence unavailable (' + response.status + ')');
  output.textContent = JSON.stringify(await response.json(), null, 2);
 } catch (error) { output.textContent = error.message; }
 finally { button.disabled = false; }
});
