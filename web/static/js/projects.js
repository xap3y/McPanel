const projectGrid = document.getElementById('project-grid');
const projectColorMenu = document.getElementById('project-color-menu');
const projectOrderKey = 'mcpanel.projectOrder.v1';
const projectColorsKey = 'mcpanel.projectColors.v1';
let draggedProject = null;
let colorTargetProject = null;
let projectWasDragged = false;

function readProjectPreference(key, fallback) {
  try { return JSON.parse(localStorage.getItem(key)) || fallback; }
  catch (_) { return fallback; }
}

function saveProjectOrder() {
  if (!projectGrid) return;
  const order = [...projectGrid.querySelectorAll('.project-card')].map(card => card.dataset.project);
  localStorage.setItem(projectOrderKey, JSON.stringify(order));
}

function applySavedProjectOrder() {
  if (!projectGrid) return;
  const order = readProjectPreference(projectOrderKey, []);
  const cards = new Map([...projectGrid.querySelectorAll('.project-card')].map(card => [card.dataset.project, card]));
  order.forEach(name => { if (cards.has(name)) projectGrid.appendChild(cards.get(name)); });
}

function applySavedProjectColors() {
  const colors = readProjectPreference(projectColorsKey, {});
  document.querySelectorAll('.project-card').forEach(card => {
    if (colors[card.dataset.project]) card.style.setProperty('--project-color', colors[card.dataset.project]);
  });
}

function closeProjectColorMenu() {
  projectColorMenu?.classList.remove('open');
  projectColorMenu?.setAttribute('aria-hidden', 'true');
  colorTargetProject = null;
}

applySavedProjectOrder();
applySavedProjectColors();

document.querySelectorAll('.project-card').forEach(card => {
  card.addEventListener('click', () => {
    if (projectWasDragged) return;
    window.location.href = card.dataset.url;
  });
  card.addEventListener('keydown', event => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      window.location.href = card.dataset.url;
    }
  });
  card.addEventListener('dragstart', event => {
    draggedProject = card;
    projectWasDragged = true;
    card.classList.add('dragging');
    event.dataTransfer.effectAllowed = 'move';
    event.dataTransfer.setData('text/plain', card.dataset.project);
  });
  card.addEventListener('dragend', () => {
    card.classList.remove('dragging');
    draggedProject = null;
    saveProjectOrder();
    setTimeout(() => { projectWasDragged = false; }, 0);
  });
  card.addEventListener('contextmenu', event => {
    event.preventDefault();
    colorTargetProject = card;
    projectColorMenu.style.left = `${Math.min(event.clientX, window.innerWidth - 210)}px`;
    projectColorMenu.style.top = `${Math.min(event.clientY, window.innerHeight - 100)}px`;
    projectColorMenu.classList.add('open');
    projectColorMenu.setAttribute('aria-hidden', 'false');
  });
});

projectGrid?.addEventListener('dragover', event => {
  event.preventDefault();
  if (!draggedProject) return;
  const over = event.target.closest('.project-card');
  if (!over || over === draggedProject) return;
  const rect = over.getBoundingClientRect();
  const before = event.clientY < rect.top + rect.height / 2 ||
    (Math.abs(event.clientY - (rect.top + rect.height / 2)) < rect.height / 3 && event.clientX < rect.left + rect.width / 2);
  projectGrid.insertBefore(draggedProject, before ? over : over.nextSibling);
});

projectGrid?.addEventListener('drop', event => {
  event.preventDefault();
  saveProjectOrder();
});

projectColorMenu?.querySelectorAll('[data-color]').forEach(button => {
  button.addEventListener('click', event => {
    event.stopPropagation();
    if (!colorTargetProject) return;
    const colors = readProjectPreference(projectColorsKey, {});
    const color = button.dataset.color;
    if (color) {
      colors[colorTargetProject.dataset.project] = color;
      colorTargetProject.style.setProperty('--project-color', color);
    } else {
      delete colors[colorTargetProject.dataset.project];
      colorTargetProject.style.removeProperty('--project-color');
    }
    localStorage.setItem(projectColorsKey, JSON.stringify(colors));
    closeProjectColorMenu();
  });
});

document.addEventListener('click', event => {
  if (!event.target.closest('#project-color-menu')) closeProjectColorMenu();
});
window.addEventListener('blur', closeProjectColorMenu);
