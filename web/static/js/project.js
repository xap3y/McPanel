// ── localStorage persistence keys ─────────────────────────────────────────
const lsKey = (suffix) => `mcpanel.project.${PROJECT_NAME}.${suffix}`;

function lsSave(key, value) {
  try { localStorage.setItem(lsKey(key), JSON.stringify(value)); } catch(_) {}
}
function lsLoad(key, fallback) {
  try { return JSON.parse(localStorage.getItem(lsKey(key))) ?? fallback; } catch(_) { return fallback; }
}

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
let projectButtonSnapshot = null;

function lockProjectButtons() {
  if (projectButtonSnapshot) return;
  const buttons = [...document.querySelectorAll('.main button')];
  projectButtonSnapshot = buttons.map(button => ({button, disabled: button.disabled}));
  buttons.forEach(button => button.disabled = true);
  document.querySelector('.project-page')?.setAttribute('aria-busy', 'true');
}

function unlockProjectButtons() {
  if (!projectButtonSnapshot) return;
  projectButtonSnapshot.forEach(({button, disabled}) => button.disabled = disabled);
  projectButtonSnapshot = null;
  document.querySelector('.project-page')?.removeAttribute('aria-busy');
}

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

  buildWS.onmessage = async (evt) => {
    try {
      const msg = JSON.parse(evt.data);
      if (msg.type === 'line') appendBuildLine(msg.data);
      if (msg.type === 'status') {
        if (msg.data === 'running') {
          setBuildRunning(true);
          buildStatusTx.textContent = 'Building…';
          setBadge('running', 'BUILDING');
        } else if (msg.data === 'finished') {
          const lines = buildConsole.innerText;
          const success = /BUILD SUCCESSFUL/i.test(lines);
          setBuildRunning(false);
          buildStatusTx.textContent = success ? 'Build successful' : 'Build failed';
          setBadge(success ? 'success' : 'failed', success ? 'SUCCESS' : 'FAILED');
          refreshHistory();
          if (success) loadArtifacts();
          if (success && onFinished) await onFinished(success);
          else {
            unlockProjectButtons();
            if (onFinished) await onFinished(success);
          }
        }
      }
    } catch(e) {}
  };

  buildWS.onclose = () => {
    if (buildRunning) {
      setBuildRunning(false);
      buildStatusTx.textContent = 'Disconnected';
      unlockProjectButtons();
    }
  };
}

function setBadge(type, label) {
  buildStatusBg.innerHTML = `<span class="badge badge-${type}">${label}</span>`;
}

async function startBuild(profile, onFinished = null) {
  lockProjectButtons();
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
      unlockProjectButtons();
      return false;
    }
    setBuildRunning(true);
    connectBuildWS(onFinished);
    return true;
  } catch (err) {
    console.error('Could not start build', err);
    toast('Could not reach the build API. Reload the page and try again.', 'error');
    buildStatusTx.textContent = 'Build request failed';
    setBadge('failed','FAILED');
    unlockProjectButtons();
    return false;
  }
}

async function doDeploy(restart) {
  const targets = selectedTargets();
  if (targets.length === 0) { toast('Select at least one deploy target', 'error'); return false; }
  buildStatusTx.textContent = restart ? 'Deploying + restarting…' : 'Deploying…';
  const artifact = document.getElementById('artifact-select').value;
  const url = `/api/projects/${PROJECT_NAME}/deploy${restart ? '?restart=true' : ''}`;
  const loadingToast = toast(
    restart ? `Deploying and restarting ${targets.join(', ')}…` : `Deploying to ${targets.join(', ')}…`,
    'loading',
    {persistent:true}
  );
  lockProjectButtons();

  try {
    const r = await projectAPI(url, 'POST', { targets, artifact });
    const data = await r.json().catch(()=>({}));
    if (!r.ok) throw new Error(data.error || 'Deployment failed');
    const results = Object.entries(data);
    const allOk = results.length > 0 && results.every(([, message]) =>
      restart ? message === 'deployed, restarted' : message === 'deployed'
    );
    if (allOk) {
      updateToast(
        loadingToast,
        restart ? `${targets.join(', ')}: deployed and restarted` : `${targets.join(', ')}: deployed`,
        'success'
      );
    } else {
      const failures = results
        .filter(([, message]) => restart ? message !== 'deployed, restarted' : message !== 'deployed')
        .map(([server, message]) => `${server}: ${message}`);
      updateToast(loadingToast, failures.join(' · ') || 'Deployment failed', 'error', 7000);
    }
    buildStatusTx.textContent = allOk ? (restart ? 'Deployed + restarted' : 'Deployed') : 'Partial deploy';
    refreshHistory();
    return allOk;
  } catch (error) {
    updateToast(loadingToast, error.message || 'Deployment failed', 'error', 7000);
    buildStatusTx.textContent = 'Deployment failed';
    return false;
  } finally {
    unlockProjectButtons();
  }
}

function selectedTargets() {
  return Array.from(document.querySelectorAll('#deploy-targets .target-cb:checked')).map(cb => cb.value);
}

// Keep deploy targets navigable when servers are arranged in group folders.
// Direct children of the Minecraft root appear under an explicit Root group.
function groupDeployTargets() {
  const container = document.getElementById('deploy-targets');
  if (!container) return;
  const targets = [...container.querySelectorAll('.deploy-target-item')];
  if (targets.length === 0) return;

  const groups = new Map();
  targets.forEach(target => {
    const name = target.querySelector('.target-cb').value;
    const parts = name.split('/');
    const group = parts.length > 1 ? parts.shift() : 'Root';
    target.querySelector('.deploy-target-name').textContent = parts.join('/') || name;
    if (!groups.has(group)) groups.set(group, []);
    groups.get(group).push(target);
  });

  const fragment = document.createDocumentFragment();
  for (const [group, items] of groups) {
    const wrapper = document.createElement('div');
    wrapper.className = 'deploy-target-group';
    const header = document.createElement('button');
    header.type = 'button';
    header.className = 'deploy-target-group-header';
    header.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"></polyline></svg><span class="deploy-target-group-name"></span><span class="deploy-target-group-count"></span>';
    header.querySelector('.deploy-target-group-name').textContent = group;
    const content = document.createElement('div');
    content.className = 'deploy-target-group-content';
    items.forEach(item => content.appendChild(item));
    const stateKey = 'deploy_target_group_open_' + group;
    const savedState = localStorage.getItem(stateKey);
    if (savedState === null ? items.some(item => item.querySelector('.target-cb').checked) : savedState === 'true') wrapper.classList.add('open');
    header.setAttribute('aria-expanded', wrapper.classList.contains('open'));
    header.addEventListener('click', () => {
      const open = wrapper.classList.toggle('open');
      header.setAttribute('aria-expanded', open);
      localStorage.setItem(stateKey, open);
    });
    wrapper.append(header, content);
    fragment.appendChild(wrapper);
  }
  container.replaceChildren(fragment);
}

function updateTargetGroupCounts() {
  document.querySelectorAll('.deploy-target-group').forEach(group => {
    const boxes = [...group.querySelectorAll('.target-cb')];
    const selected = boxes.filter(box => box.checked).length;
    const count = group.querySelector('.deploy-target-group-count');
    if (count) count.textContent = selected ? `${selected}/${boxes.length}` : String(boxes.length);
  });
}

groupDeployTargets();

// Deploy target checkboxes + localStorage persistence
document.querySelectorAll('#deploy-targets .deploy-target-item').forEach(item => {
  const cb = item.querySelector('input[type=checkbox]');
  cb.addEventListener('change', () => {
    item.classList.toggle('checked', cb.checked);
    lsSave('targets', selectedTargets());
    updateTargetGroupCounts();
  });
});

// Restore persisted target checkboxes (overrides server defaults if saved)
const savedTargets = lsLoad('targets', null);
if (savedTargets !== null) {
  document.querySelectorAll('#deploy-targets .target-cb').forEach(cb => {
    const checked = savedTargets.includes(cb.value);
    cb.checked = checked;
    cb.closest('.deploy-target-item')?.classList.toggle('checked', checked);
  });
}
updateTargetGroupCounts();

// ── Button handlers ───────────────────────────────────────────────────────
const profileSelect = document.getElementById('profile-select');

// Restore persisted profile selection
const savedProfile = lsLoad('profile', null);
if (savedProfile && [...profileSelect.options].some(o => o.value === savedProfile)) {
  profileSelect.value = savedProfile;
}
profileSelect.addEventListener('change', () => lsSave('profile', profileSelect.value));

buildBtn.addEventListener('click', () => {
  const profile = profileSelect.value;
  startBuild(profile);
});

cancelBtn.addEventListener('click', async () => {
  await fetch(`/api/projects/${PROJECT_NAME}/cancel`, {method:'POST',headers:{'X-CSRF-Token':CSRF}});
  toast('Build cancelled', 'info');
  setBuildRunning(false);
  unlockProjectButtons();
});

document.getElementById('deploy-btn').addEventListener('click', () => doDeploy(false));
document.getElementById('deploy-restart-btn').addEventListener('click', () => doDeploy(true));

document.getElementById('build-deploy-btn').addEventListener('click', async () => {
  const profile = profileSelect.value;
  await startBuild(profile, async (success) => { if (success) await doDeploy(false); });
});

document.getElementById('bdr-btn').addEventListener('click', async () => {
  const profile = profileSelect.value;
  await startBuild(profile, async (success) => { if (success) await doDeploy(true); });
});

// ── Build Profiles ────────────────────────────────────────────────────────
async function loadProfiles() {
  const r = await fetch(`/api/projects/${PROJECT_NAME}/profiles`);
  const profiles = await r.json() || [];
  const profilesList  = document.getElementById('profiles-list');

  // Update select options
  const currentVal = profileSelect.value;
  profileSelect.innerHTML = profiles.map(p => `<option value="${esc(p.ProfileName)}" ${p.IsDefault?'selected':''}>${esc(p.ProfileName)}</option>`).join('');
  // Restore selection
  const toRestore = lsLoad('profile', null);
  if (toRestore && [...profileSelect.options].some(o => o.value === toRestore)) {
    profileSelect.value = toRestore;
  } else if (currentVal && [...profileSelect.options].some(o => o.value === currentVal)) {
    profileSelect.value = currentVal;
  }

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
    row.querySelector('[data-paction="edit"]')?.addEventListener('click', () => openProfileModal(prof));
    row.querySelector('[data-paction="delete"]')?.addEventListener('click', () => openDeleteModal(prof));
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

// ── Profile Modal ─────────────────────────────────────────────────────────
const profileModal = document.getElementById('profile-modal');
const pmId   = document.getElementById('pm-id');
const pmName = document.getElementById('pm-name');
const pmCmd  = document.getElementById('pm-cmd');
const pmDefault = document.getElementById('pm-default');
const pmDefaultLabel = document.getElementById('pm-default-label');

function openProfileModal(existing) {
  document.getElementById('profile-modal-title').textContent = existing ? 'Edit Build Profile' : 'Add Build Profile';
  pmId.value = existing?.ID || 0;
  pmName.value = existing?.ProfileName || '';
  pmCmd.value  = existing?.Command || (existing?.Executable ? [existing.Executable, ...(existing.Args||[])].join(' ') : './gradlew clean build');
  pmDefault.checked = existing?.IsDefault || false;
  pmDefaultLabel.classList.toggle('checked', existing?.IsDefault || false);
  profileModal.style.display = 'flex';
  pmName.focus();
}

pmDefault.addEventListener('change', () => pmDefaultLabel.classList.toggle('checked', pmDefault.checked));

document.getElementById('profile-modal-close').addEventListener('click', () => profileModal.style.display = 'none');
document.getElementById('profile-modal-cancel').addEventListener('click', () => profileModal.style.display = 'none');
profileModal.addEventListener('click', e => { if (e.target === profileModal) profileModal.style.display = 'none'; });

document.getElementById('profile-modal-save').addEventListener('click', async () => {
  const name = pmName.value.trim();
  const cmd  = pmCmd.value.trim();
  if (!name) { toast('Profile name is required', 'error'); return; }
  if (!cmd)  { toast('Build command is required', 'error'); return; }
  const btn = document.getElementById('profile-modal-save');
  btn.disabled = true;
  await saveProfile({
    ID: parseInt(pmId.value) || 0,
    ProjectName: PROJECT_NAME,
    ProfileName: name,
    Command: cmd,
    IsDefault: pmDefault.checked,
  });
  btn.disabled = false;
  profileModal.style.display = 'none';
  loadProfiles();
  toast('Profile saved', 'success');
});

// ── Delete Profile Modal ──────────────────────────────────────────────────
const deleteModal = document.getElementById('delete-profile-modal');
let _deleteTarget = null;

function openDeleteModal(prof) {
  _deleteTarget = prof;
  document.getElementById('delete-profile-name').textContent = prof.ProfileName;
  deleteModal.style.display = 'flex';
}

document.getElementById('delete-profile-close').addEventListener('click', () => deleteModal.style.display = 'none');
document.getElementById('delete-profile-cancel').addEventListener('click', () => deleteModal.style.display = 'none');
deleteModal.addEventListener('click', e => { if (e.target === deleteModal) deleteModal.style.display = 'none'; });

document.getElementById('delete-profile-confirm').addEventListener('click', async () => {
  if (!_deleteTarget) return;
  const btn = document.getElementById('delete-profile-confirm');
  btn.disabled = true;
  await fetch(`/api/projects/${PROJECT_NAME}/profiles?profile=${encodeURIComponent(_deleteTarget.ProfileName)}`, {
    method:'DELETE', headers:{'X-CSRF-Token':CSRF}
  });
  btn.disabled = false;
  deleteModal.style.display = 'none';
  loadProfiles();
  toast('Profile deleted', 'success');
});

document.getElementById('add-profile-btn').addEventListener('click', () => openProfileModal(null));

// ── Artifacts ─────────────────────────────────────────────────────────────
async function loadArtifacts() {
  const selectedInput = document.getElementById('artifact-select');
  const hint = document.getElementById('artifact-picker-hint');

  // Restore persisted artifact selection
  const savedArtifact = lsLoad('artifact', '');
  const current = savedArtifact || selectedInput.value;

  try {
    const r = await projectAPI(`/api/projects/${PROJECT_NAME}/artifacts`);
    if (!r.ok) throw new Error(await r.text());
    const jars = await r.json() || [];
    const selected = jars.includes(current) ? current : '';
    selectedInput.value = selected;

    selectedInput.innerHTML = '<option value="">Latest build (newest JAR)</option>' + jars.map(j =>
      `<option value="${esc(j)}">${esc(basename(j))}</option>`
    ).join('');
    selectedInput.value = selected;
    hint.textContent = jars.length === 0
      ? 'No JARs found in build/libs, target, or deploy.'
      : `${jars.length} deployable JAR${jars.length === 1 ? '' : 's'} found.`;
    selectedInput.onchange = () => lsSave('artifact', selectedInput.value);
  } catch (err) {
    selectedInput.innerHTML = '<option value="">Could not load artifacts</option>';
    selectedInput.disabled = true;
    hint.textContent = String(err.message || err);
  }
}

// ── Project settings ───────────────────────────────────────────────────────
// Load saved build args
async function loadProjectSettingsUI() {
  try {
    const r = await projectAPI(`/api/projects/${PROJECT_NAME}/settings`);
    if (!r.ok) return;
    const ps = await r.json();
    const baEl = document.getElementById('ps-buildargs');
    if (baEl && ps.BuildArgs) baEl.value = ps.BuildArgs;
  } catch(_) {}
}

document.getElementById('project-settings-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const f = e.target;
  const targets = selectedTargets();
  const buildArgs = f.BuildArgs?.value || '';
  const r = await projectAPI(`/api/projects/${PROJECT_NAME}/settings`, 'POST', {
    ArtifactGlob: "*.jar",
    DeploymentFilename: f.DeploymentFilename.value,
    DeployTargets: targets,
    BuildArgs: buildArgs,
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
loadProjectSettingsUI();

// Reconnect WS if build is already running
connectBuildWS();
