// ── Tab switching ──────────────────────────────────────────────────────────
const tabBtns    = document.querySelectorAll('.tab-btn');
const tabPanes   = document.querySelectorAll('.tab-pane');
let activeTab    = 'console';

tabBtns.forEach(btn => {
  btn.addEventListener('click', () => {
    const tab = btn.dataset.tab;
    tabBtns.forEach(b => b.classList.toggle('active', b.dataset.tab === tab));
    tabPanes.forEach(p => p.classList.toggle('active', p.id === 'tab-' + tab));
    activeTab = tab;
    if (tab === 'files' && !editorPath) { currentFMPath = ''; loadFiles(''); }
    if (tab === 'plugins') loadPlugins();
    if (tab === 'settings') loadSettings();
  });
});

// ── Topbar: server status + actions ───────────────────────────────────────
const topbarActions  = document.getElementById('topbar-actions');
const serverSubtitle = document.getElementById('server-subtitle');

async function loadServerStatus() {
  try {
    const r = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/status`);
    if (!r.ok) return;
    const s = await r.json();
    const status = s.status || 'OFFLINE';
    serverSubtitle.innerHTML = `<span class="badge badge-${status.toLowerCase()}">${status}</span>${s.uptime ? ' · ' + s.uptime : ''}${s.ram ? ' · ' + s.ram : ''}${s.cpu ? ' · ' + s.cpu + '% CPU' : ''}`;
    renderTopbarActions(status);

    // Update left sidebar dot
    const navItem = document.querySelector(`.sidebar-nav a[href="/servers/${encodeURIComponent(SERVER_NAME)}"]`);
    if (navItem) {
      navItem.className = navItem.className.replace(/status-\w+/g, '').trim() + ' status-' + status.toLowerCase();
    }
  } catch(e) {
    serverSubtitle.textContent = 'Could not load status';
  }
}

function renderTopbarActions(status) {
  topbarActions.innerHTML = '';
  if (status === 'ONLINE') {
    topbarActions.appendChild(actionBtn('Restart', 'restart', 'secondary',
      '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 .49-3.13"/></svg>'));
    topbarActions.appendChild(actionBtn('Stop', 'stop', 'danger', ''));
    topbarActions.appendChild(actionBtn('Kill', 'kill', 'danger', ''));
  } else {
    topbarActions.appendChild(actionBtn('Start', 'start', 'success',
      '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3"/></svg>'));
  }
}

function actionBtn(label, action, style, icon) {
  const btn = document.createElement('button');
  btn.className = `btn btn-${style} btn-sm`;
  btn.innerHTML = `${icon} ${label}`;
  btn.addEventListener('click', () => serverAction(action, btn, label));
  return btn;
}

async function serverAction(action, btn, origLabel) {
  const orig = btn.innerHTML;
  btn.disabled = true;
  btn.innerHTML = `<span class="spinner"></span> ${origLabel}…`;
  const r = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/${action}`, {
    method: 'POST', headers: {'X-CSRF-Token': CSRF}
  });
  const data = await r.json().catch(()=>({}));
  if (!r.ok || data.error) {
    toast(data.error || 'Action failed', 'error');
    btn.disabled = false;
    btn.innerHTML = orig;
  } else {
    toast(`${SERVER_NAME}: ${action} issued`, 'success');
    setTimeout(loadServerStatus, 1500);
  }
}

// Load screen session name for attach command
async function loadScreenName() {
  try {
    const r = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/screen-name`);
    if (!r.ok) return;
    const data = await r.json();
    const bar  = document.getElementById('console-attach-bar');
    const cmd  = document.getElementById('console-attach-cmd');
    const copyBtn = document.getElementById('copy-attach-btn');
    if (data.screenName && bar && cmd) {
      const attachCmd = `screen -r ${data.screenName}`;
      cmd.textContent = attachCmd;
      bar.style.display = 'flex';
      bar.style.alignItems = 'center';
      bar.style.gap = '8px';
      bar.style.padding = '6px 16px';
      bar.style.borderTop = '1px solid var(--border-subtle)';
      const doCopy = () => {
        navigator.clipboard.writeText(attachCmd).then(() => toast('Copied!', 'success'));
      };
      cmd.addEventListener('click', doCopy);
      copyBtn?.addEventListener('click', doCopy);
    }
  } catch(_) {}
}

loadServerStatus();
setInterval(loadServerStatus, 2000);
loadScreenName();

// ── File Manager ──────────────────────────────────────────────────────────
let currentFMPath = '';
let editorPath = null;
let editorInstance = null;
let editorModel = null;
let editorOriginalContent = '';
let editorDirty = false;
let monacoLoadPromise = null;
let monacoAPI = null;

const fmList       = document.getElementById('fm-list');
const fmBreadcrumb = document.getElementById('fm-breadcrumb');
const fmUpBtn      = document.getElementById('fm-up-btn');
const fmFileInput  = document.getElementById('fm-file-input');

fmUpBtn.addEventListener('click', () => {
  const parts = currentFMPath.split('/').filter(Boolean);
  parts.pop();
  loadFiles(parts.join('/'));
});

document.getElementById('fm-upload-btn').addEventListener('click', () => fmFileInput.click());
fmFileInput.addEventListener('change', async () => {
  for (const file of fmFileInput.files) {
    const fd = new FormData();
    fd.append('file', file);
    const r = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/files/upload?path=${encodeURIComponent(currentFMPath)}`, {
      method: 'POST', headers: {'X-CSRF-Token': CSRF}, body: fd
    });
    if (r.ok) toast(`Uploaded ${file.name}`, 'success');
    else toast(`Failed to upload ${file.name}`, 'error');
  }
  fmFileInput.value = '';
  loadFiles(currentFMPath);
});

document.getElementById('fm-mkdir-btn').addEventListener('click', async () => {
  const name = prompt('New folder name:');
  if (!name) return;
  const r = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/files/mkdir`, {
    method: 'POST', headers: {'Content-Type':'application/json','X-CSRF-Token':CSRF},
    body: JSON.stringify({path: currentFMPath ? currentFMPath + '/' + name : name})
  });
  if (r.ok) { toast('Folder created', 'success'); loadFiles(currentFMPath); }
  else toast('Failed to create folder', 'error');
});

document.getElementById('fm-newfile-btn').addEventListener('click', async () => {
  const name = prompt('New file name:');
  if (!name) return;
  const path = currentFMPath ? currentFMPath + '/' + name : name;
  const r = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/files/create`, {
    method: 'POST', headers: {'Content-Type':'application/json','X-CSRF-Token':CSRF},
    body: JSON.stringify({path})
  });
  if (r.ok) { toast('File created', 'success'); loadFiles(currentFMPath); }
  else toast('Failed to create file', 'error');
});

function fmIcon(isDir, name, isPluginFolder = false) {
  if (isDir) {
    if (isPluginFolder) {
      return `<div class="fm-row-icon dir plugin-dir-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M8.5 3.5h7v4h3v4h-3v3.5a3.5 3.5 0 0 1-7 0v-3.5h-3v-4h3z"/><path d="M10.5 18.5V22M13.5 18.5V22"/></svg></div>`;
    }
    return `<div class="fm-row-icon dir"><svg viewBox="0 0 24 24" fill="currentColor" style="color:hsl(42,90%,62%)"><path d="M20 18a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h5.17a2 2 0 0 1 1.41.59L14 6h4a2 2 0 0 1 2 2v10z" opacity=".9"/></svg></div>`;
  }
  // pick icon by extension
  const ext = name.split('.').pop().toLowerCase();
  const colors = {yml:'#6EE7B7',yaml:'#6EE7B7',properties:'#93C5FD',json:'#FCD34D',toml:'#FCA5A5',xml:'#FCA5A5',jar:'#F9A8D4',log:'#9CA3AF',txt:'#D1D5DB',md:'#C4B5FD'};
  const c = colors[ext] || '#6B7280';
  return `<div class="fm-row-icon file"><svg viewBox="0 0 24 24" fill="none" stroke="${c}" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg></div>`;
}

function fmSize(bytes) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B','KB','MB','GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return (bytes / Math.pow(k, i)).toFixed(i === 0 ? 0 : 1) + ' ' + sizes[i];
}

function fmTime(unix) {
  if (!unix) return '';
  const d = new Date(unix * 1000);
  return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'});
}

const TEXT_EXTS = new Set(['yml','yaml','properties','json','toml','conf','txt','md','log','xml','cfg','ini','sh','bat','py','js','ts','java','gradle','kts']);

function isTextFile(name) {
  const ext = name.split('.').pop().toLowerCase();
  return TEXT_EXTS.has(ext);
}

async function loadFiles(path) {
  currentFMPath = path;
  fmList.innerHTML = '<div style="padding:20px 14px;color:var(--text-muted);font-size:13px">Loading…</div>';

  // Update breadcrumb
  const parts = path.split('/').filter(Boolean);
  let crumbHtml = `<span class="fm-breadcrumb-item" data-path="">root</span>`;
  let accumulated = '';
  parts.forEach((part, i) => {
    accumulated = accumulated ? accumulated + '/' + part : part;
    const p = accumulated;
    crumbHtml += `<span class="fm-breadcrumb-sep">/</span><span class="fm-breadcrumb-item" data-path="${p}">${part}</span>`;
  });
  fmBreadcrumb.innerHTML = crumbHtml;
  fmBreadcrumb.querySelectorAll('.fm-breadcrumb-item').forEach(el => {
    el.addEventListener('click', () => loadFiles(el.dataset.path));
  });

  const r = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/files?path=${encodeURIComponent(path)}`);
  if (!r.ok) { fmList.innerHTML = `<div class="empty-state"><p>Error loading directory</p></div>`; return; }
  const entries = await r.json() || [];

  fmList.innerHTML = '';

  // Back row
  if (path) {
    const row = document.createElement('div');
    row.className = 'fm-row';
    row.innerHTML = `
      <div class="fm-row-icon dir"><svg viewBox="0 0 24 24" fill="none" stroke="hsl(42,90%,62%)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg></div>
      <span class="fm-row-name" style="color:var(--text-muted)">.. (parent folder)</span>`;
    row.addEventListener('click', () => {
      const pp = path.split('/').slice(0,-1).join('/');
      loadFiles(pp);
    });
    fmList.appendChild(row);
  }

  // Sort: dirs first
  const dirs  = entries.filter(e => e.isDir).sort((a,b) => a.name.localeCompare(b.name));
  const files = entries.filter(e => !e.isDir).sort((a,b) => a.name.localeCompare(b.name));

  [...dirs, ...files].forEach(e => {
    const row = document.createElement('div');
    const isPluginFolder = !path && e.isDir && e.name.toLowerCase() === 'plugins';
    row.className = `fm-row${isPluginFolder ? ' plugin-folder-row' : ''}`;
    const filePath = path ? path + '/' + e.name : e.name;
    row.innerHTML = `
      ${fmIcon(e.isDir, e.name, isPluginFolder)}
      <span class="fm-row-name">${escHtml(e.name)}</span>
      <span class="fm-row-size">${e.isDir ? '' : fmSize(e.size)}</span>
      <span class="fm-row-time">${fmTime(e.modTime)}</span>
      <div class="fm-row-actions">
        ${!e.isDir && isTextFile(e.name) ? `<button class="btn-icon" data-action="edit" title="Edit"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg></button>` : ''}
        ${!e.isDir ? `<a class="btn-icon" href="/api/servers/${encodeURIComponent(SERVER_NAME)}/files/download?path=${encodeURIComponent(filePath)}" download title="Download"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg></a>` : ''}
        <button class="btn-icon" data-action="rename" title="Rename"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"/></svg></button>
        <button class="btn-icon danger" data-action="delete" title="Delete"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/><path d="M10 11v6M14 11v6"/><path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg></button>
      </div>`;

    // Click row: open dir or edit file
    row.addEventListener('click', (evt) => {
      if (evt.target.closest('.fm-row-actions')) return;
      if (e.isDir) {
        loadFiles(filePath);
      } else if (isTextFile(e.name)) {
        openFileEditor(filePath, e.name);
      }
    });

    // Action buttons
    row.querySelector('[data-action="edit"]')?.addEventListener('click', (e2) => {
      e2.stopPropagation();
      openFileEditor(filePath, e.name);
    });

    row.querySelector('[data-action="rename"]')?.addEventListener('click', async (e2) => {
      e2.stopPropagation();
      const newName = prompt('Rename to:', e.name);
      if (!newName || newName === e.name) return;
      const r2 = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/files/rename`, {
        method:'POST', headers:{'Content-Type':'application/json','X-CSRF-Token':CSRF},
        body:JSON.stringify({path:filePath, newName})
      });
      if (r2.ok) { toast('Renamed', 'success'); loadFiles(currentFMPath); }
      else toast('Rename failed: ' + await r2.text(), 'error');
    });

    row.querySelector('[data-action="delete"]')?.addEventListener('click', async (e2) => {
      e2.stopPropagation();
      if (!confirm(`Delete ${e.name}?`)) return;
      const r2 = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/files/delete`, {
        method:'POST', headers:{'Content-Type':'application/json','X-CSRF-Token':CSRF},
        body:JSON.stringify({path:filePath})
      });
      if (r2.ok) { toast('Deleted', 'success'); loadFiles(currentFMPath); }
      else toast('Delete failed', 'error');
    });

    fmList.appendChild(row);
  });

  if (dirs.length === 0 && files.length === 0 && !path) {
    fmList.innerHTML = '<div class="empty-state"><p>Empty directory</p></div>';
  }
}

async function openFileEditor(path, filename) {
  const r = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/files/read?path=${encodeURIComponent(path)}`);
  if (!r.ok) { toast('Cannot open file (too large or binary)', 'error'); return; }
  const content = await r.text();
  editorPath = path;
  editorOriginalContent = content;
  editorDirty = false;
  document.getElementById('editor-filename').textContent = path;
  document.getElementById('file-manager').hidden = true;
  document.getElementById('file-editor-workspace').hidden = false;
  document.body.classList.add('file-editor-open');
  updateEditorDirtyStatus();
  await mountMonacoEditor(content, filename || path.split('/').pop());
}

function editorLanguage(filename) {
  const ext = filename.split('.').pop().toLowerCase();
  return ({
    yml:'yaml', yaml:'yaml', json:'json', properties:'ini', ini:'ini', cfg:'ini', conf:'ini', toml:'ini',
    xml:'xml', html:'html', htm:'html', css:'css', js:'javascript', ts:'typescript', java:'java',
    py:'python', sh:'shell', bat:'bat', md:'markdown', sql:'sql', go:'go', txt:'plaintext', log:'plaintext'
  })[ext] || 'plaintext';
}

function loadMonaco() {
  if (monacoAPI) return Promise.resolve(monacoAPI);
  if (monacoLoadPromise) return monacoLoadPromise;
  monacoLoadPromise = new Promise((resolve, reject) => {
    const base = '/static/vendor/monaco/';
    let settled = false;
    const finish = (callback, value) => {
      if (settled) return;
      settled = true;
      clearTimeout(timeout);
      callback(value);
    };
    const timeout = setTimeout(() => {
      finish(reject, new Error('Monaco Editor initialization timed out'));
    }, 12000);

    const initialize = () => {
      try {
        if (typeof window.require !== 'function' || typeof window.require.config !== 'function') {
          throw new Error('Monaco Editor loader is unavailable');
        }
        window.require.config({paths: {vs: base + 'vs'}});
        window.require(['vs/editor/editor.main'], api => {
          if (!api?.editor?.create) {
            finish(reject, new Error('Monaco Editor returned an invalid API'));
            return;
          }
          monacoAPI = api;
          finish(resolve, api);
        }, error => finish(reject, error instanceof Error ? error : new Error(String(error))));
      } catch (error) {
        finish(reject, error);
      }
    };

    if (typeof window.require === 'function' && typeof window.require.config === 'function') {
      initialize();
      return;
    }
    const script = document.createElement('script');
    script.src = base + 'vs/loader.js';
    script.onload = initialize;
    script.onerror = () => finish(reject, new Error('Monaco Editor assets could not be loaded'));
    document.head.appendChild(script);
  });
  return monacoLoadPromise;
}

async function mountMonacoEditor(content, filename) {
  const loading = document.getElementById('editor-loading');
  const fallback = document.getElementById('editor-fallback');
  loading.hidden = false;
  loading.style.display = 'flex';
  fallback.hidden = true;
  fallback.style.display = 'none';
  try {
    const monaco = await loadMonaco();
    editorModel?.dispose();
    editorModel = monaco.editor.createModel(content, editorLanguage(filename));
    if (!editorInstance) {
      editorInstance = monaco.editor.create(document.getElementById('monaco-editor'), {
        model: editorModel,
        theme: 'vs-dark',
        automaticLayout: true,
        fontFamily: 'JetBrains Mono, SFMono-Regular, Consolas, monospace',
        fontSize: 13,
        lineHeight: 21,
        minimap: {enabled: true},
        scrollBeyondLastLine: false,
        smoothScrolling: true,
        wordWrap: 'off',
        padding: {top: 10, bottom: 10},
      });
      editorInstance.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, saveEditorFile);
      editorInstance.addCommand(monaco.KeyCode.Escape, closeFileEditor);
    } else {
      editorInstance.setModel(editorModel);
    }
    editorModel.onDidChangeContent(() => {
      editorDirty = editorModel.getValue() !== editorOriginalContent;
      updateEditorDirtyStatus();
    });
    loading.hidden = true;
    loading.style.display = 'none';
    requestAnimationFrame(() => editorInstance.focus());
  } catch (error) {
    monacoLoadPromise = null;
    editorModel?.dispose();
    editorModel = null;
    loading.hidden = true;
    loading.style.display = 'none';
    fallback.value = content;
    fallback.hidden = false;
    fallback.style.display = 'block';
    fallback.focus();
    console.error('Monaco Editor failed to initialize:', error);
    toast(error.message + '; using basic editor', 'error');
  }
}

function currentEditorContent() {
  return editorModel && editorInstance ? editorModel.getValue() : document.getElementById('editor-fallback').value;
}

function updateEditorDirtyStatus() {
  const status = document.getElementById('editor-dirty-status');
  status.textContent = editorDirty ? 'Modified' : 'Saved';
  status.classList.toggle('modified', editorDirty);
}

async function saveEditorFile() {
  if (!editorPath) return;
  const saveButton = document.getElementById('editor-save-btn');
  const original = saveButton.textContent;
  saveButton.disabled = true;
  saveButton.textContent = 'Saving…';
  const content = currentEditorContent();
  const r = await api(`/api/servers/${encodeURIComponent(SERVER_NAME)}/files/write`, 'POST', {path: editorPath, content});
  if (r.ok) {
    editorOriginalContent = content;
    editorDirty = false;
    updateEditorDirtyStatus();
    toast('File saved', 'success');
  } else {
    toast('Save failed: ' + await r.text(), 'error');
  }
  saveButton.disabled = false;
  saveButton.textContent = original;
}

function closeFileEditor() {
  if (editorDirty && !confirm('Discard unsaved changes?')) return;
  editorPath = null;
  editorDirty = false;
  document.getElementById('file-editor-workspace').hidden = true;
  document.getElementById('file-manager').hidden = false;
  document.body.classList.remove('file-editor-open');
  loadFiles(currentFMPath);
}

document.getElementById('editor-save-btn').addEventListener('click', saveEditorFile);
document.getElementById('editor-back-btn').addEventListener('click', closeFileEditor);
document.getElementById('editor-fallback').addEventListener('input', event => {
  editorDirty = event.target.value !== editorOriginalContent;
  updateEditorDirtyStatus();
});
document.getElementById('editor-fallback').addEventListener('keydown', event => {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
    event.preventDefault();
    saveEditorFile();
  }
  if (event.key === 'Escape') {
    event.preventDefault();
    closeFileEditor();
  }
});
window.addEventListener('beforeunload', event => {
  if (!editorDirty) return;
  event.preventDefault();
  event.returnValue = '';
});

function escHtml(s) {
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

// ── Plugins ────────────────────────────────────────────────────────────────
async function loadPlugins() {
  const r = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/plugins`);
  if (!r.ok) return;
  const plugins = await r.json() || [];
  const list = document.getElementById('plugin-list');
  const count = document.getElementById('plugin-count');

  count.textContent = `${plugins.filter(p=>p.enabled).length} enabled · ${plugins.filter(p=>!p.enabled).length} disabled`;

  if (plugins.length === 0) {
    list.innerHTML = '<div class="empty-state"><p>No plugins found in plugins/</p></div>';
    return;
  }

  list.innerHTML = '';
  const sorted = [...plugins].sort((a,b) => {
    if (a.enabled !== b.enabled) return a.enabled ? -1 : 1;
    return a.name.localeCompare(b.name);
  });

  sorted.forEach(p => {
    const row = document.createElement('div');
    row.className = 'plugin-row';
    row.innerHTML = `
      <span class="badge ${p.enabled ? 'badge-online' : 'badge-offline'}">${p.enabled ? 'ENABLED' : 'DISABLED'}</span>
      <span class="plugin-name ${p.enabled ? '' : 'disabled'}">${escHtml(p.name)}</span>
      <span class="plugin-note text-mono">*.jar${p.enabled?'':'.d'}</span>
      <div class="plugin-actions">
        <button class="btn ${p.enabled ? 'btn-danger' : 'btn-success'} btn-sm" data-plugin-action="toggle">
          ${p.enabled ? 'Disable' : 'Enable'}
        </button>
        <button class="btn btn-secondary btn-sm" data-plugin-action="restart">
          ${p.enabled ? 'Disable' : 'Enable'} + Restart
        </button>
        <button class="btn-icon danger" data-plugin-action="delete" title="Delete plugin">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/><path d="M10 11v6M14 11v6"/><path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg>
        </button>
      </div>`;

    const toggleBtn = row.querySelector('[data-plugin-action="toggle"]');
    const restartBtn = row.querySelector('[data-plugin-action="restart"]');
    const deleteBtn = row.querySelector('[data-plugin-action="delete"]');

    async function doToggle(restart) {
      const r2 = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/plugins/${encodeURIComponent(p.name)}/toggle`, {
        method:'POST', headers:{'Content-Type':'application/json','X-CSRF-Token':CSRF},
        body:JSON.stringify({enable: !p.enabled})
      });
      if (r2.ok) {
        toast(`Plugin ${p.name} ${!p.enabled ? 'enabled' : 'disabled'}`, 'success');
        if (restart) {
          await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/restart`, {method:'POST',headers:{'X-CSRF-Token':CSRF}});
          toast('Restart issued', 'info');
        }
        loadPlugins();
      } else {
        toast('Toggle failed', 'error');
      }
    }

    toggleBtn.addEventListener('click', () => doToggle(false));
    restartBtn.addEventListener('click', () => doToggle(true));
    deleteBtn.addEventListener('click', async () => {
      if (!confirm(`Permanently delete plugin "${p.name}"?\n\nThis removes the JAR from this server.`)) return;
      deleteBtn.disabled = true;
      const r2 = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/plugins/${encodeURIComponent(p.name)}/delete`, {
        method:'POST', headers:{'Content-Type':'application/json','X-CSRF-Token':CSRF},
        body:JSON.stringify({enabled:p.enabled})
      });
      if (r2.ok) {
        toast(`Deleted plugin ${p.name}`, 'success');
        loadPlugins();
      } else {
        toast('Delete failed: ' + await r2.text(), 'error');
        deleteBtn.disabled = false;
      }
    });

    list.appendChild(row);
  });
}

// ── Settings ───────────────────────────────────────────────────────────────
async function loadSettings() {
  const r = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/settings`);
  if (!r.ok) return;
  const s = await r.json();
  document.getElementById('s-java').value = s.JavaPath || '';
  document.getElementById('s-xms').value  = s.Xms || '';
  document.getElementById('s-xmx').value  = s.Xmx || '';
  document.getElementById('s-jvmargs').value    = (s.JVMArgs || []).join('\n');
  document.getElementById('s-jar').value        = s.ServerJar || '';
  document.getElementById('s-serverargs').value = (s.ServerArgs || []).join('\n');

  const vtoggle = document.getElementById('velocity-toggle');
  vtoggle.checked = !!s.VelocityEnabled;
  document.getElementById('velocity-status-text').textContent = s.VelocityEnabled ? 'Enabled' : 'Disabled';
}

document.getElementById('settings-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const f = e.target;
  const body = {
    JavaPath: f.JavaPath.value, Xms: f.Xms.value, Xmx: f.Xmx.value,
    JVMArgs:  f.JVMArgsText.value.split('\n').filter(Boolean),
    ServerJar: f.ServerJar.value,
    ServerArgs: f.ServerArgsText.value.split('\n').filter(Boolean),
  };
  const r = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/settings`, {
    method:'POST', headers:{'Content-Type':'application/json','X-CSRF-Token':CSRF},
    body: JSON.stringify(body)
  });
  if (r.ok) toast('Settings saved — restart for changes to apply', 'success');
  else toast('Save failed', 'error');
});

document.getElementById('velocity-toggle').addEventListener('change', async (e) => {
  const enable = e.target.checked;
  const r = await fetch(`/api/servers/${encodeURIComponent(SERVER_NAME)}/velocity`, {
    method:'POST', headers:{'Content-Type':'application/json','X-CSRF-Token':CSRF},
    body:JSON.stringify({enable})
  });
  if (r.ok) {
    document.getElementById('velocity-status-text').textContent = enable ? 'Enabled' : 'Disabled';
    document.getElementById('velocity-restart-hint').style.display = 'block';
    toast(`Velocity ${enable?'enabled':'disabled'} — restart to apply`, 'info');
  } else {
    e.target.checked = !enable; // revert
    toast('Failed to toggle Velocity', 'error');
  }
});
