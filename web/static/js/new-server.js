(() => {
  const form = document.getElementById('new-server-form');
  const submitButton = document.getElementById('new-server-submit-btn');
  const progress = document.getElementById('new-server-progress');

  function addConfigRow(kind, key = '', value = '') {
    const container = document.getElementById(kind === 'bukkit' ? 'custom-bukkit' : 'custom-properties');
    const row = document.createElement('div');
    row.className = 'custom-config-row';
    row.innerHTML = `
      <input type="text" class="custom-config-key" placeholder="${kind === 'bukkit' ? 'settings.key' : 'property-key'}" aria-label="Configuration key">
      <input type="text" class="custom-config-value" placeholder="value" aria-label="Configuration value">
      <button type="button" class="btn-icon danger" title="Remove value" aria-label="Remove value">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
      </button>`;
    row.querySelector('.custom-config-key').value = key;
    row.querySelector('.custom-config-value').value = value;
    row.querySelector('button').addEventListener('click', () => row.remove());
    container.appendChild(row);
    row.querySelector('.custom-config-key').focus();
  }

  document.querySelectorAll('[data-add-config]').forEach(button => {
    button.addEventListener('click', () => addConfigRow(button.dataset.addConfig));
  });

  function collectCustom(kind) {
    const container = document.getElementById(kind === 'bukkit' ? 'custom-bukkit' : 'custom-properties');
    const values = {};
    for (const row of container.querySelectorAll('.custom-config-row')) {
      const key = row.querySelector('.custom-config-key').value.trim();
      const value = row.querySelector('.custom-config-value').value;
      if (!key) throw new Error('Custom configuration keys cannot be empty');
      values[key] = value;
    }
    return values;
  }

  form.addEventListener('submit', async event => {
    event.preventDefault();
    if (!form.reportValidity()) return;

    let serverProperties;
    let bukkitSettings;
    try {
      serverProperties = collectCustom('properties');
      bukkitSettings = collectCustom('bukkit');
    } catch (error) {
      toast(error.message, 'error');
      return;
    }

    document.querySelectorAll('.config-preset > input[type=checkbox]:checked').forEach(checkbox => {
      const value = checkbox.closest('.config-preset').querySelector('.preset-value').value;
      const destination = checkbox.dataset.config === 'bukkit' ? bukkitSettings : serverProperties;
      if (!(checkbox.dataset.key in destination)) destination[checkbox.dataset.key] = value;
    });

    const payload = {
      name: document.getElementById('new-server-name').value.trim(),
      group: document.getElementById('new-server-group').value,
      jarUrl: document.getElementById('new-server-url').value.trim(),
      jarFilename: document.getElementById('new-server-jar-name').value.trim(),
      acceptEula: document.getElementById('new-server-eula').checked,
      serverProperties,
      bukkitSettings,
    };

    const original = submitButton.innerHTML;
    submitButton.disabled = true;
    submitButton.innerHTML = '<span class="spinner"></span> Creating…';
    progress.textContent = 'Downloading and validating the server JAR…';
    try {
      const response = await fetch('/api/servers', {
        method: 'POST',
        headers: {'Content-Type': 'application/json', 'X-CSRF-Token': window.CSRF},
        body: JSON.stringify(payload),
      });
      if (!response.ok) throw new Error((await response.text()).trim() || 'Server creation failed');
      const result = await response.json();
      progress.textContent = 'Server created. Opening it now…';
      toast(`Created ${result.name}`, 'success');
      window.location.assign(`/servers/${encodeURIComponent(result.name)}`);
    } catch (error) {
      progress.textContent = '';
      toast(error.message, 'error');
      submitButton.disabled = false;
      submitButton.innerHTML = original;
    }
  });
})();
