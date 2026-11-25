const { withRetry } = require('./retry');

function msUntilSix(now = new Date()) {
  const next = new Date(now);
  next.setHours(6, 0, 0, 0);
  if (next <= now) next.setDate(next.getDate() + 1);
  return next - now;
}

function start(notes) {
  const scheduleNext = () => {
    setTimeout(async () => {
      try {
        const text = notes.map((n) => n.text).join('\n');
        await withRetry(() => fetch(process.env.DIGEST_HOOK_URL, { method: 'POST', body: text }));
      } finally {
        scheduleNext();
      }
    }, msUntilSix());
  };
  scheduleNext();
}

module.exports = { start };
