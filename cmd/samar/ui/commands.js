// One in-flight engine command; rapid input keeps the latest desired state.
// Shared by the shipped WebView and the dependency-free regression tests.
(function (root) {
  function createCommands(initial, paint, send) {
    let confirmed = { ...initial }, desired = { ...initial }, pending = false;
    let pendingField = null, pendingValue;
    const fields = ['style', 'rounded', 'startup'];
    const dirty = new Set(), actions = [];
    function flush() {
      if (pending) return;
      let command;
      pendingField = null;
      for (const field of fields) {
        if (desired[field] !== confirmed[field]) {
          command = field === 'style' ? desired.style + 99 : field === 'rounded' ? 105 : 110;
          pendingField = field; pendingValue = desired[field];
          break;
        }
      }
      if (command === undefined) command = actions.shift();
      if (command === undefined) return;
      pending = true;
      paint({ ...desired }, '', pending);
      send(command);
    }
    return {
      choose(field, value) {
        if (!fields.includes(field)) return;
        desired[field] = value;
        dirty.add(field);
        paint({ ...desired }, '', pending);
        flush();
      },
      action(command) {
        if (command === 140) { send(command); return; }
        if (!actions.includes(command)) actions.push(command);
        flush();
      },
      acknowledge(next, error = '') {
        if (pendingField && next[pendingField] !== pendingValue && next[pendingField] !== desired[pendingField]) {
          error ||= 'Efek belum diterapkan. Coba lagi.';
        }
        confirmed = { ...next };
        pending = false;
        pendingField = null;
        for (const field of fields) {
          if (!dirty.has(field) || error) desired[field] = next[field];
          if (desired[field] === next[field]) dirty.delete(field);
        }
        if (error) { dirty.clear(); actions.length = 0; }
        paint({ ...desired }, error, pending);
        if (!error) flush();
      },
      reset(next) {
        confirmed = { ...next }; desired = { ...next }; pending = false;
        pendingField = null;
        dirty.clear(); actions.length = 0;
        paint({ ...desired }, '', pending);
      }
    };
  }
  if (typeof module !== 'undefined') module.exports = createCommands;
  else root.createSenjaCommands = createCommands;
})(typeof window === 'undefined' ? globalThis : window);
