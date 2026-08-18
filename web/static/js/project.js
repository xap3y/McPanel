// ── Build infrastructure ──────────────────────────────────────────────────
const buildConsole  = document.getElementById('build-console');
const buildStatusTx = document.getElementById('build-status-text');
const buildStatusBg = document.getElementById('build-status-badge');
const buildTimer    = document.getElementById('build-timer');
const buildBtn      = document.getElementById('build-btn');
const cancelBtn     = document.getElementById('cancel-btn');

async function projectAPI(url, method = 'GET', body = null) {
  const options = {method, headers: {}};
  if (body !== null) {
    options.headers['Content-Type'] = 'application/json';
    options.body = JSON.stringify(body);
  }
  return fetch(url, options);
}

let buildWS      = null;
let buildRunning = false;
let buildStart   = null;
let timerInt     = null;

function setBuildRunning(running) {
  buildRunning = running;
  buildBtn.disabled = running;
  cancelBtn.style.display = running ? 'inline-flex' : 'none';
  if (running) {
    buildStart = Date.now();
    timerInt = setInterval(() => {
      const s = Math.floor((Date.now() - buildStart) / 1000);
      buildTimer.textContent = `${Math.floor(s/60)}:${String(s%60).padStart(2,'0')}`;
    }, 500);
  } else {
    clearInterval(timerInt);
  }
}

function appendBuildLine(raw) {
  const line = document.createElement('div');
  const text = raw.replace(/\x1b\[[0-9;]*m/g,'');
  // Classify line
  if (/BUILD SUCCESSFUL/i.test(text)) line.className = 'success-line';
  else if (/BUILD FAILED|error:/i.test(text)) line.className = 'error-line';
  else if (/^> Task/i.test(text)) line.className = 'task-line';
  line.textContent = text + '\n';
  buildConsole.appendChild(line);
  buildConsole.scrollTop = buildConsole.scrollHeight;
  while (buildConsole.childElementCount > 2000) buildConsole.removeChild(buildConsole.firstChild);
}

function connectBuildWS(onFinished) {
  if (buildWS) { buildWS.close(); buildWS = null; }
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  buildWS = new WebSocket(`${proto}://${location.host}/ws/build/${PROJECT_NAME}`);

  buildWS.onmessage = (evt) => {
    try {
      const msg = JSON.parse(evt.data);
      if (msg.type === 'line') appendBuildLine(msg.data);
      if (msg.type === 'status') {
        if (msg.data === 'running') {
          setBuildRunning(true);
          buildStatusTx.textContent = 'Building…';
          setBadge('running', 'BUILDING');
        } else if (msg.data === 'finished') {
          // Check last line for success
          const lines = buildConsole.innerText;
          const success = /BUILD SUCCESSFUL/i.test(lines);
          setBuildRunning(false);
          buildStatusTx.textContent = success ? 'Build successful' : 'Build failed';
          setBadge(success ? 'success' : 'failed', success ? 'SUCCESS' : 'FAILED');
          refreshHistory();
          if (success) loadArtifacts();
          if (onFinished) onFinished(success);
        }
      }
    } catch(e) {}
  };

  buildWS.onclose = () => {
    if (buildRunning) {
      setBuildRunning(false);
      buildStatusTx.textContent = 'Disconnected';
    }
  };
}

function setBadge(type, label) {
  buildStatusBg.innerHTML = `<span class="badge badge-${type}">${label}</span>`;
}

async function startBuild(profile) {
  try {
    buildConsole.innerHTML = '';
    buildStatusTx.textContent = 'Starting build…';
    setBadge('running', 'BUILDING');
    const r = await fetch(`/api/projects/${PROJECT_NAME}/build?profile=${encodeURIComponent(profile)}`, {
      method:'POST', headers:{'X-CSRF-Token':CSRF}
    });
    if (!r.ok) {
      const txt = await r.text();
      toast('Could not start build: ' + txt, 'error');
      buildStatusTx.textContent = 'Failed to start';
      setBadge('failed','FAILED');
      return false;
    }
    setBuildRunning(true);
    connectBuildWS();
    return true;
  } catch (err) {
    console.error('Could not start build', err);
    toast('Could not reach the build API. Reload the page and try again.', 'error');
    buildStatusTx.textContent = 'Build request failed';
    setBadge('failed','FAILED');
    return false;
  }
}

async function doDeploy(restart) {
  const targets = selectedTargets();
  if (targets.length === 0) { toast('Select at least one deploy target', 'error'); return false; }
  buildStatusTx.textContent = 'Deploying…';
  const artifact = document.getElementById('artifact-select').value;
  const url = `/api/projects/${PROJECT_NAME}/deploy${restart ? '?restart=true' : ''}`;
  const r = await projectAPI(url, 'POST', { targets, artifact });
  const data = await r.json().catch(()=>({}));
  const results = Object.entries(data);
  const allOk = results.every(([,v]) => v.startsWith('deployed'));
  if (allOk) toast('Deployed to ' + targets.join(', '), 'success');
  else {
    results.forEach(([srv, msg]) => toast(`${srv}: ${msg}`, msg.startsWith('deployed') ? 'success' : 'error'));
  }
  buildStatusTx.textContent = allOk ? 'Deployed' : 'Partial deploy';
  refreshHistory();
  return allOk;
}

function selectedTargets() {
  return Array.from(document.querySelectorAll('.target-cb:checked')).map(cb => cb.value);
}

// Deploy target checkboxes. The label performs the native toggle; only the
// visuals are custom, so one click always means one state change.
document.querySelectorAll('.deploy-target-item').forEach(item => {
  const cb = item.querySelector('input[type=checkbox]');
  cb.addEventListener('change', () => item.classList.toggle('checked', cb.checked));
});

// ── Button handlers ───────────────────────────────────────────────────────
buildBtn.addEventListener('click', () => {
  const profile = document.getElementById('profile-select').value;
  startBuild(profile);
});

cancelBtn.addEventListener('click', async () => {
  await fetch(`/api/projects/${PROJECT_NAME}/cancel`, {method:'POST',headers:{'X-CSRF-Token':CSRF}});
  toast('Build cancelled', 'info');
  setBuildRunning(false);
});

document.getElementById('deploy-btn').addEventListener('click', () => doDeploy(false));

document.getElementById('build-deploy-btn').addEventListener('click', async () => {
  const profile = document.getElementById('profile-select').value;
  const ok = await startBuild(profile);
  if (!ok) return;
  connectBuildWS(async (success) => { if (success) await doDeploy(false); });
});

document.getElementById('bdr-btn').addEventListener('click', async () => {
  const profile = document.getElementById('profile-select').value;
  const ok = await startBuild(profile);
  if (!ok) return;
  connectBuildWS(async (success) => { if (success) await doDeploy(true); });
});

// ── Build Profiles ────────────────────────────────────────────────────────
async function loadProfiles() {
  const r = await fetch(`/api/projects/${PROJECT_NAME}/profiles`);
  const profiles = await r.json() || [];
  const profileSelect = document.getElementById('profile-select');
  const profilesList  = document.getElementById('profiles-list');

  // Update select options
  const current = profileSelect.value;
  profileSelect.innerHTML = profiles.map(p => `<option value="${esc(p.ProfileName)}" ${p.IsDefault?'selected':''}>${esc(p.ProfileName)}</option>`).join('');

  // Render list
  if (profiles.length === 0) {
    profilesList.innerHTML = '<div class="empty-state" style="padding:24px"><p>No profiles yet</p></div>';
    return;
  }

  profilesList.innerHTML = profiles.map(p => `
    <div class="plugin-row" style="border:none;border-bottom:1px solid var(--border-subtle)" data-profile-id="${p.ID}">
      <div style="flex:1">
        <div style="font-size:13px;font-weight:500">${esc(p.ProfileName)}${p.IsDefault?'<span class="profile-badge" style="margin-left:8px">default</span>':''}</div>
        <div class="text-mono text-muted">${esc(p.Command || [p.Executable, ...(p.Args||[])].join(' '))}</div>
      </div>
      <div class="flex gap-2">
        ${!p.IsDefault?`<button class="btn btn-ghost btn-sm" data-paction="default">Set default</button>`:''}
        <button class="btn btn-ghost btn-sm" data-paction="edit">Edit</button>
        <button class="btn-icon danger" data-paction="delete" title="Delete profile"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="width:14px;height:14px"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/></svg></button>
      </div>
    </div>`).join('');

  profilesList.querySelectorAll('[data-profile-id]').forEach(row => {
    const profId = parseInt(row.dataset.profileId);
    const prof = profiles.find(p => p.ID === profId);
    if (!prof) return;

    row.querySelector('[data-paction="default"]')?.addEventListener('click', async () => {
      await saveProfile({...prof, IsDefault:true});
      loadProfiles();
    });
    row.querySelector('[data-paction="edit"]')?.addEventListener('click', () => editProfileDialog(prof));
    row.querySelector('[data-paction="delete"]')?.addEventListener('click', async () => {
      if (!confirm(`Delete profile "${prof.ProfileName}"?`)) return;
      await fetch(`/api/projects/${PROJECT_NAME}/profiles?profile=${encodeURIComponent(prof.ProfileName)}`, {
        method:'DELETE', headers:{'X-CSRF-Token':CSRF}
      });
      loadProfiles();
    });
  });
}

function esc(s) {
  return String(s)
    .replace(/&/g,'&amp;')
    .replace(/</g,'&lt;')
    .replace(/>/g,'&gt;')
    .replace(/"/g,'&quot;')
    .replace(/'/g,'&#39;');
}

async function saveProfile(p) {
  await fetch(`/api/projects/${PROJECT_NAME}/profiles`, {
    method:'POST', headers:{'Content-Type':'application/json','X-CSRF-Token':CSRF},
    body:JSON.stringify(p)
  });
}

function editProfileDialog(existing) {
  const name = prompt('Profile name:', existing?.ProfileName || 'New Profile');
  if (!name) return;
  const defaultCmd = existing?.Command || (existing?.Executable ? [existing.Executable, ...(existing.Args||[])].join(' ') : './gradlew clean build');
  const cmd = prompt('Build command:', defaultCmd);
  if (!cmd) return;
  if (!cmd.trim()) return;
  const isDef = confirm('Set as default?');
  saveProfile({
    ID: existing?.ID || 0,
    ProjectName: PROJECT_NAME,
    ProfileName: name,
    Command: cmd.trim(),
    IsDefault: isDef,
  }).then(loadProfiles);
}

document.getElementById('add-profile-btn').addEventListener('click', () => editProfileDialog(null));

// ── Artifacts ─────────────────────────────────────────────────────────────
async function loadArtifacts() {
  const picker = document.getElementById('artifact-picker');
  const selectedInput = document.getElementById('artifact-select');
  const hint = document.getElementById('artifact-picker-hint');
  const current = selectedInput.value;
  try {
    const r = await projectAPI(`/api/projects/${PROJECT_NAME}/artifacts`);
    if (!r.ok) throw new Error(await r.text());
    const jars = await r.json() || [];
    const selected = jars.includes(current) ? current : '';
    selectedInput.value = selected;
    picker.innerHTML = `
      <button type="button" class="artifact-option ${selected === '' ? 'selected' : ''}" data-artifact="">
        <span class="artifact-radio"></span>
        <span><strong>Latest build</strong><small>Automatically use the newest JAR</small></span>
      </button>` + jars.map(j => `
      <button type="button" class="artifact-option ${selected === j ? 'selected' : ''}" data-artifact="${esc(j)}">
        <span class="artifact-radio"></span>
        <span><strong>${esc(basename(j))}</strong><small>${esc(j)}</small></span>
      </button>`).join('');
    hint.textContent = jars.length === 0
      ? 'No JARs found in build/libs, target, or deploy.'
      : `${jars.length} deployable JAR${jars.length === 1 ? '' : 's'} found.`;
    picker.querySelectorAll('.artifact-option').forEach(option => {
      option.addEventListener('click', () => {
        selectedInput.value = option.dataset.artifact;
        picker.querySelectorAll('.artifact-option').forEach(el => el.classList.toggle('selected', el === option));
      });
    });
  } catch (err) {
    picker.innerHTML = '<div class="artifact-empty">Could not load project artifacts.</div>';
    hint.textContent = String(err.message || err);
  }
}

// ── Project settings ───────────────────────────────────────────────────────
document.getElementById('project-settings-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const f = e.target;
  const targets = selectedTargets();
  const r = await projectAPI(`/api/projects/${PROJECT_NAME}/settings`, 'POST', {
    ArtifactGlob: "*.jar", // fallback to default
    DeploymentFilename: f.DeploymentFilename.value,
    DeployTargets: targets,
  });
  if (r.ok) toast('Settings saved', 'success');
  else toast('Save failed', 'error');
});

// ── Build/Deploy History ───────────────────────────────────────────────────
async function refreshHistory() {
  const panel = document.getElementById('history-panel');
  try {
    const [bRes, dRes] = await Promise.all([
      fetch(`/api/projects/${PROJECT_NAME}/history/builds?limit=10`),
      fetch(`/api/projects/${PROJECT_NAME}/history/deploys?limit=10`),
    ]);

    const builds  = bRes.ok  ? (await bRes.json()  || []) : [];
    const deploys = dRes.ok  ? (await dRes.json()  || []) : [];

    if (builds.length === 0 && deploys.length === 0) {
      panel.innerHTML = '<div class="empty-state" style="padding:32px"><p>No history yet</p></div>';
      return;
    }

    let html = '<table class="history-table"><thead><tr><th>Time</th><th>Event</th><th>Status</th></tr></thead><tbody>';

    // Merge and sort by time (most recent first)
    const events = [
      ...builds.map(b => ({time: b.StartedAt, type: 'build', data: b})),
      ...deploys.map(d => ({time: d.DeployedAt, type: 'deploy', data: d})),
    ].sort((a,b) => new Date(b.time) - new Date(a.time)).slice(0, 20);

    events.forEach(ev => {
      const timeStr = fmtTimeStr(ev.time);
      if (ev.type === 'build') {
        const b = ev.data;
        const dur = b.FinishedAt && b.StartedAt ?
          Math.round((new Date(b.FinishedAt) - new Date(b.StartedAt)) / 1000) + 's' : '—';
        html += `<tr>
          <td><span class="mono text-muted">${timeStr}</span></td>
          <td>
            <div>Build · <span class="mono" style="font-size:11px">${esc(b.ProfileName||'—')}</span></div>
            ${b.ArtifactPath ? `<div class="mono text-muted" style="font-size:11px">${esc(basename(b.ArtifactPath))}</div>` : ''}
          </td>
          <td><span class="badge badge-${(b.Status||'').toLowerCase()}">${b.Status}</span> <span class="text-muted" style="font-size:11px">${dur}</span></td>
        </tr>`;
      } else {
        const d = ev.data;
        html += `<tr>
          <td><span class="mono text-muted">${timeStr}</span></td>
          <td>
            <div>Deploy → <span class="mono" style="font-size:11px">${esc(d.ServerName||'—')}</span></div>
            ${d.ErrorMessage ? `<div class="text-muted" style="font-size:11px;color:var(--red)">${esc(d.ErrorMessage)}</div>` : ''}
          </td>
          <td><span class="badge badge-${d.Status==='SUCCESS'?'success':'failed'}">${d.Status}</span>${d.Restarted ? ' <span class="text-muted" style="font-size:11px">↺</span>' : ''}</td>
        </tr>`;
      }
    });

    html += '</tbody></table>';
    panel.innerHTML = html;
  } catch(e) {
    panel.innerHTML = '<div class="empty-state" style="padding:24px"><p>Could not load history</p></div>';
  }
}

function fmtTimeStr(iso) {
  if (!iso) return '—';
  const d = new Date(iso);
  return d.toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'});
}

function basename(p) {
  return p ? p.split('/').pop() : '';
}

document.getElementById('history-refresh-btn').addEventListener('click', refreshHistory);

// ── Init ───────────────────────────────────────────────────────────────────
loadProfiles();
loadArtifacts();
refreshHistory();

// Reconnect WS if build is already running
connectBuildWS();
