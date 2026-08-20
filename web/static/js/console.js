// ── ANSI escape code renderer ──────────────────────────────────────────────
const ANSI_COLORS = {
  30: 'style="color:#4d4d4d"', 31: 'class="ansi-red"',    32: 'class="ansi-green"',
  33: 'class="ansi-yellow"',  34: 'class="ansi-blue"',   35: 'class="ansi-magenta"',
  36: 'class="ansi-cyan"',    37: 'class="ansi-white"',
  90: 'style="color:#808080"',91: 'class="ansi-bright-red"',   92: 'class="ansi-bright-green"',
  93: 'class="ansi-bright-yellow"',
  94: 'style="color:#60a5fa"', 95: 'style="color:#e879f9"', 96: 'style="color:#2dd4bf"',
  97: 'style="color:#ffffff"',
};

function ansiToHtml(text) {
  // Convert Minecraft legacy & codes to ANSI
  const mcColors = {
    '0':'30', '1':'34', '2':'32', '3':'36', '4':'31', '5':'35', '6':'33', '7':'37',
    '8':'90', '9':'94', 'a':'92', 'b':'96', 'c':'91', 'd':'95', 'e':'93', 'f':'97',
    'l':'1', 'r':'0'
  };
  text = text.replace(/&([0-9a-fA-Flr])/g, (m, c) => `\x1b[${mcColors[c.toLowerCase()]}m`);

  let result = '';
  let openSpan = false;
  const parts = text.split(/\x1b\[([0-9;]*)m/);
  
  let currentClasses = new Set();
  let currentStyles = new Set();

  for (let i = 0; i < parts.length; i++) {
    if (i % 2 === 0) {
      if (parts[i]) {
        if (!openSpan && (currentClasses.size > 0 || currentStyles.size > 0)) {
           let attr = '';
           if (currentClasses.size > 0) attr += ` class="${Array.from(currentClasses).join(' ')}"`;
           if (currentStyles.size > 0) attr += ` style="${Array.from(currentStyles).join(';')}"`;
           result += `<span${attr}>`;
           openSpan = true;
        }
        result += escHtml(parts[i]);
      }
    } else {
      const codes = parts[i] === '' ? [0] : parts[i].split(';').map(Number);
      let changed = false;
      for (let j = 0; j < codes.length; j++) {
        const c = codes[j];
        if (c === 0 || c === 39) {
          currentClasses.clear();
          currentStyles.clear();
          changed = true;
        } else if (c === 38 || c === 48) {
          if (j + 2 < codes.length && codes[j+1] === 5) {
            j += 2;
          } else if (j + 4 < codes.length && codes[j+1] === 2) {
            if (c === 38) {
              for (let cls of currentClasses) {
                if (cls.startsWith('ansi-') && !cls.includes('bold') && !cls.includes('dim')) currentClasses.delete(cls);
              }
              for (let sty of currentStyles) {
                if (sty.startsWith('color:')) currentStyles.delete(sty);
              }
              currentStyles.add(`color:rgb(${codes[j+2]},${codes[j+3]},${codes[j+4]})`);
              changed = true;
            } else if (c === 48) {
              for (let sty of currentStyles) {
                if (sty.startsWith('background-color:')) currentStyles.delete(sty);
              }
              currentStyles.add(`background-color:rgb(${codes[j+2]},${codes[j+3]},${codes[j+4]})`);
              changed = true;
            }
            j += 4;
          }
        } else if (ANSI_COLORS[c]) {
          const val = ANSI_COLORS[c];
          if (val.startsWith('class=')) {
            // Remove previous color classes
            for (let cls of currentClasses) {
              if (cls.startsWith('ansi-') && !cls.includes('bold') && !cls.includes('dim')) currentClasses.delete(cls);
            }
            const m = val.match(/"([^"]+)"/);
            if (m) currentClasses.add(m[1]);
          } else {
            for (let sty of currentStyles) {
              if (sty.startsWith('color:')) currentStyles.delete(sty);
            }
            const m = val.match(/"([^"]+)"/);
            if (m) currentStyles.add(m[1]);
          }
          changed = true;
        } else if (c === 1) {
          currentClasses.add('ansi-bold');
          changed = true;
        } else if (c === 2) {
          currentClasses.add('ansi-dim');
          changed = true;
        }
      }
      if (changed && openSpan) {
        result += '</span>';
        openSpan = false;
      }
    }
  }
  if (openSpan) result += '</span>';
  return result;
}

function escHtml(s) {
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

// ── Console WebSocket ──────────────────────────────────────────────────────
const consoleEl   = document.getElementById('console-output');
const cmdInput    = document.getElementById('cmd-input');
const scrollBtn   = document.getElementById('autoscroll-btn');

let autoscroll = true;
let cmdHistory = [];
let histIdx    = -1;
let ws         = null;

function updateScrollBtn() {
  scrollBtn.textContent = autoscroll ? '⬇ Auto' : '⬆ Paused';
  scrollBtn.style.color = autoscroll ? '' : 'var(--yellow)';
}

consoleEl.addEventListener('scroll', () => {
  const atBottom = consoleEl.scrollHeight - consoleEl.scrollTop - consoleEl.clientHeight < 40;
  if (autoscroll !== atBottom) {
    autoscroll = atBottom;
    updateScrollBtn();
  }
});

scrollBtn.addEventListener('click', () => {
  autoscroll = !autoscroll;
  updateScrollBtn();
  if (autoscroll) consoleEl.scrollTop = consoleEl.scrollHeight;
});

function appendLine(raw) {
  const span = document.createElement('span');
  span.className = 'console-line';
  span.innerHTML = ansiToHtml(raw) + '\n';
  consoleEl.appendChild(span);
  if (autoscroll) consoleEl.scrollTop = consoleEl.scrollHeight;
  // Trim old lines
  while (consoleEl.childElementCount > 3000) {
    consoleEl.removeChild(consoleEl.firstChild);
  }
}

function connect() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  ws = new WebSocket(`${proto}://${location.host}/ws/console/${window.SERVER_NAME}`);

  ws.onopen = () => {
    appendLine('\x1b[2m[panel] Connected to console\x1b[0m');
  };

  ws.onmessage = (evt) => {
    try {
      const msg = JSON.parse(evt.data);
      if (msg.type === 'line') appendLine(msg.data);
    } catch(e) { appendLine(evt.data); }
  };

  ws.onclose = () => {
    appendLine('\x1b[2m[panel] Connection closed – reconnecting in 2s…\x1b[0m');
    setTimeout(connect, 2000);
  };

  ws.onerror = () => ws.close();
}

connect();

// ── Command input ──────────────────────────────────────────────────────────
cmdInput.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') {
    const cmd = cmdInput.value.trim();
    if (!cmd) return;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({type: 'command', data: cmd}));
      appendLine('\x1b[2m> ' + cmd + '\x1b[0m');
    } else {
      appendLine('\x1b[31m[panel] Not connected\x1b[0m');
    }
    cmdHistory.push(cmd);
    histIdx = cmdHistory.length;
    cmdInput.value = '';
  } else if (e.key === 'ArrowUp') {
    e.preventDefault();
    if (histIdx > 0) histIdx--;
    cmdInput.value = cmdHistory[histIdx] || '';
  } else if (e.key === 'ArrowDown') {
    e.preventDefault();
    if (histIdx < cmdHistory.length - 1) { histIdx++; cmdInput.value = cmdHistory[histIdx]; }
    else { histIdx = cmdHistory.length; cmdInput.value = ''; }
  }
});

// ── Ctrl-C button ──────────────────────────────────────────────────────────
const ctrlcBtn = document.getElementById('ctrlc-btn');
if (ctrlcBtn) {
  ctrlcBtn.addEventListener('click', () => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({type: 'ctrlc'}));
      appendLine('\x1b[2m[panel] Sent ^C\x1b[0m');
    } else {
      appendLine('\x1b[31m[panel] Not connected\x1b[0m');
    }
  });
}