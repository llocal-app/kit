function withRetry(call, tries = 3) {
  return call().catch((err) => {
    if (tries <= 1) throw err;
    return new Promise((resolve) => setTimeout(() => resolve(withRetry(call, tries - 1)), 1000));
  });
}

module.exports = { withRetry };
