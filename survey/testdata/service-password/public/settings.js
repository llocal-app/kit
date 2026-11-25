const form = document.getElementById('mail');

fetch('/api/mail-settings').then((r) => r.json()).then((s) => {
  for (const [name, value] of Object.entries(s)) form.elements[name].value = value;
});

form.addEventListener('submit', async (e) => {
  e.preventDefault();
  await fetch('/api/mail-settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(Object.fromEntries(new FormData(form))),
  });
});
