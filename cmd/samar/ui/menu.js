(() => {
  const state = window.__SENJA_STATE__;
  // The user explicitly enabled full motion in Senja, independently of
  // Windows' disabled animation setting. Leave the system setting untouched.
  const reducedMotion = { matches: false };
  const grid = document.querySelector('.style-grid');
  const plate = document.createElement('i');
  plate.className = 'selection-plate';
  plate.setAttribute('aria-hidden', 'true');
  grid.prepend(plate);
  let platePlaced = false;
  let plateGeometry = "";
  function positionSelection() {
    const selected = grid.querySelector(`[data-style="${state.style}"]`);
    if (!selected) return;
    const geometry = [selected.offsetLeft, selected.offsetTop, selected.offsetWidth, selected.offsetHeight];
    if (geometry.join(',') === plateGeometry) return;
    plateGeometry = geometry.join(',');
    if (!platePlaced) plate.style.transition = 'none';
    plate.style.width = `${geometry[2]}px`;
    plate.style.height = `${geometry[3]}px`;
    plate.style.transform = `translate(${geometry[0]}px, ${geometry[1]}px)`;
    if (!platePlaced) {
      platePlaced = true;
      requestAnimationFrame(() => requestAnimationFrame(() => plate.style.removeProperty('transition')));
    }
  }
  new ResizeObserver(positionSelection).observe(grid);
  const panel = document.querySelector('.panel');
  let revealed = false;
  let dismissing = false;
  window.senjaReveal = () => {
    if (revealed || dismissing) return;
    revealed = true;
    requestAnimationFrame(() => {
      if (dismissing) return;
      panel.classList.add('entering');
      send('motion');
    });
  };
  window.senjaDismiss = () => {
    if (dismissing) return;
    dismissing = true;
    // The native compositor moves and fades the whole window as one unit.
    // Scaling HTML separately from its live backdrop creates edge judder.
    panel.style.pointerEvents = 'none';
    send('motion');
  };
  window.senjaReset = next => {
    revealed = false;
    dismissing = false;
    panel.style.pointerEvents = '';
    panel.classList.remove('entering');
    commands.reset(next);
    window.senjaReveal();
  };
  function animatePress(button, event) {
    if (button.getAttribute('role') === 'switch') return;
    if (reducedMotion.matches) return;
    button.animate([
      { transform: 'scale(.98)', offset: 0 },
      { transform: 'scale(1)', offset: 1 }
    ], { duration: 140, easing: 'cubic-bezier(.215,.61,.355,1)' });
    if (button.dataset.command === '111') {
      button.querySelector('svg')?.animate([{ transform: 'rotate(0deg)' }, { transform: 'rotate(360deg)' }], { duration: 350, easing: 'cubic-bezier(.16,1,.3,1)' });
    }
  }
  function send(type, command) {
    window.chrome?.webview?.postMessage(JSON.stringify({ type, command, reducedMotion: reducedMotion.matches }));
  }
  function render(next, message = '', pending = false) {
    Object.assign(state, next);
    positionSelection();
    for (const el of document.querySelectorAll('[data-style]')) {
      const active = Number(el.dataset.style) === state.style;
      el.setAttribute('aria-checked', String(active));
      el.tabIndex = active ? 0 : -1;
    }
    document.querySelector('#rounded').setAttribute('aria-checked', String(state.rounded));
    document.querySelector('#startup').setAttribute('aria-checked', String(state.startup));
    panel.setAttribute('aria-busy', String(pending));
    const status = document.querySelector('#status');
    status.textContent = message || (pending ? 'Menerapkan…' : '');
    status.classList.toggle('error', Boolean(message));
  }
  const commands = window.createSenjaCommands(state, render, command => send('command', command));
  window.senjaUpdate = commands.acknowledge;
  document.addEventListener('click', event => {
    const action = event.target.closest('button');
    if (!action) return;
    if (action.dataset.action === 'close') { send('close'); return; }
    if (!action.dataset.command || dismissing) return;
    animatePress(action, event);
    const command = Number(action.dataset.command);
    if (command >= 100 && command <= 103) commands.choose('style', command - 99);
    else if (command === 105) commands.choose('rounded', !state.rounded);
    else if (command === 110) commands.choose('startup', !state.startup);
    else commands.action(command);
  });
  document.addEventListener('keydown', event => {
    if (event.key === 'Escape') { event.preventDefault(); send('close'); }
    if (event.key === 'F5' || (event.ctrlKey && event.key.toLowerCase() === 'r')) { event.preventDefault(); send('reload'); }
    const radio = event.target.closest('[role="radio"]');
    if (radio && ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) {
      event.preventDefault();
      const options = [...document.querySelectorAll('[role="radio"]')];
      const index = options.indexOf(radio);
      let next = event.key === 'Home' ? 0 : event.key === 'End' ? options.length - 1 : (index + (['ArrowLeft', 'ArrowUp'].includes(event.key) ? -1 : 1) + options.length) % options.length;
      options[next].focus(); options[next].click();
    }
  });
  render(state);
  send('ready');
})();
