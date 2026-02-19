function safeCall(fn) {
  try {
    return fn();
  } catch (e) {
    return "error";
  } finally {
    cleanup();
  }
}
