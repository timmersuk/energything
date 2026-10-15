document.querySelectorAll('[data-refresh]').forEach(button => {
  button.addEventListener('click', async () => {
    button.disabled = true;
    const message = document.querySelector('#refresh-result');
    try {
      const result = await fetch('/api/v1/refresh', {
        method: 'POST',
        headers: {'Content-Type': 'application/json', 'X-CSRF-Token': document.querySelector('meta[name="csrf-token"]').content},
        body: JSON.stringify({source: button.dataset.refresh})
      });
      message.textContent = result.ok ? 'Routine work queued. Reload status to inspect progress.' : 'Collection could not be queued. Inspect source status.';
    } catch { message.textContent = 'Status server is unavailable.'; }
    finally { button.disabled = false; }
  });
});
