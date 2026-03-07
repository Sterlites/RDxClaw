const API_BASE = '/v1';

// --- Matrix Rain Canvas ---
const canvas = document.getElementById('matrixCanvas');
const ctx = canvas.getContext('2d');

let width, height, columns;
const fontSize = 32; // Double size
const chars = 'ｱｲｳｴｵｶｷｸｹｺｻｼｽｾｿﾀﾁﾂﾃﾄﾅﾆﾇﾈﾉﾊﾋﾌﾍﾎﾏﾐﾑﾒﾓﾔﾕﾖﾗﾘﾙﾚﾛﾜﾝ1234567890ABCDEF'.split('');
let drops = [];

function initMatrix() {
  width = canvas.width = window.innerWidth;
  height = canvas.height = window.innerHeight;
  columns = Math.floor(width / fontSize);
  drops = [];
  for (let i = 0; i < columns; i++) {
    drops[i] = Math.random() * -100;
  }
}

function drawMatrix() {
  ctx.fillStyle = 'rgba(0, 0, 0, 0.1)'; // More persistent trail
  ctx.fillRect(0, 0, width, height);

  ctx.fillStyle = '#00ff41';
  ctx.font = fontSize + 'px "Share Tech Mono"';

  for (let i = 0; i < drops.length; i++) {
    const text = chars[Math.floor(Math.random() * chars.length)];
    
    // Depth effect: some characters are brighter and larger
    const isBright = Math.random() > 0.95;
    ctx.fillStyle = isBright ? '#39ff14' : '#00ff41';
    ctx.font = (isBright ? fontSize + 4 : fontSize) + 'px "Share Tech Mono"';
    if (isBright) ctx.shadowBlur = 15;
    
    ctx.fillText(text, i * fontSize, drops[i] * fontSize);
    ctx.shadowBlur = 0;

    if (drops[i] * fontSize > height && Math.random() > 0.975) {
      drops[i] = 0;
    }
    drops[i]++;
  }
}

let matrixInterval;
function startMatrix() {
  if (window.innerWidth < 768) return; // Performance on mobile
  initMatrix();
  matrixInterval = setInterval(drawMatrix, 50);
}

window.addEventListener('resize', initMatrix);

// --- Navigation State ---
document.querySelectorAll('.nav-item').forEach(el => {
  el.addEventListener('click', (e) => {
    document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
    el.classList.add('active');
    
    document.querySelectorAll('.section').forEach(s => s.classList.remove('active'));
    const target = document.getElementById(el.dataset.target);
    target.classList.add('active');
    document.getElementById('pageTitle').innerText = el.querySelector('span').innerText;

    // Load data based on tab
    if (el.dataset.target === 'dashboard') loadStatus();
    if (el.dataset.target === 'swarm') loadAgents();
    if (el.dataset.target === 'skills') loadSkills();
    if (el.dataset.target === 'docs') loadFiles();
  });
});

// --- Boot & Random Glitch ---
function triggerBoot() {
  setTimeout(() => {
    document.body.classList.remove('booting');
  }, 1000);
}

function randomGlitch() {
  const cards = document.querySelectorAll('.card');
  if (cards.length === 0) return;
  
  setInterval(() => {
    if (Math.random() > 0.8) {
      const card = cards[Math.floor(Math.random() * cards.length)];
      card.classList.add('glitch');
      setTimeout(() => card.classList.remove('glitch'), 150);
    }
  }, 5000 + Math.random() * 5000);
}

// Initialization
let refreshInterval;
document.addEventListener('DOMContentLoaded', () => {
  startMatrix();
  triggerBoot();
  randomGlitch();
  
  loadStatus();
  // Poll every 3 seconds
  refreshInterval = setInterval(() => {
    loadStatus();
    const activeSection = document.querySelector('.section.active');
    if (activeSection.id === 'swarm') loadAgents();
    if (activeSection.id === 'docs') loadFiles();
  }, 3000);
});

// Visibility API
document.addEventListener('visibilitychange', () => {
  if (document.hidden) {
    clearInterval(matrixInterval);
  } else {
    matrixInterval = setInterval(drawMatrix, 50);
  }
});


// --- API Calls ---

async function fetchJSON(endpoint, options = {}) {
  try {
    const res = await fetch(`${API_BASE}${endpoint}`, options);
    if (!res.ok) throw new Error(`HTTP error! status: ${res.status}`);
    return await res.json();
  } catch (err) {
    console.error(`Error fetching ${endpoint}:`, err);
    return null;
  }
}

async function loadStatus() {
  const data = await fetchJSON('/status');
  if (!data) return;

  // Real-time Clock
  const clockEl = document.getElementById('systemClock');
  if (clockEl) {
    const now = new Date();
    const ms = now.getMilliseconds().toString().padStart(3, '0');
    clockEl.innerText = now.toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' }) + ':' + ms;
  }

  // Numerical values with extra glow
  const updateVal = (id, val) => {
    const el = document.getElementById(id);
    if (el) el.innerHTML = `<span class="matrix-white">${val}</span>`;
  };

  updateVal('uptimeVal', data.uptime || '00:00:00');
  updateVal('versionVal', data.version || 'v1.0.0');
  
  const sidebarVer = document.getElementById('sidebarVersion');
  if (sidebarVer) sidebarVer.innerText = `ZION_OS // ${data.version || 'v2.0.0'}`;
  
  updateVal('modelVal', (data.agent?.model || 'CORE_BRAIN').split('/').pop());
  updateVal('agentsVal', data.active_agents || '0');
  updateVal('skillsVal', data.skills?.total || '0');

  const counter = document.getElementById('onlineCounter');
  if (counter) counter.innerText = data.active_agents || '0';
  
  if (data.system) {
    updateVal('memoryVal', data.system.memory_usage || '32MB');
    updateVal('goroutinesVal', data.system.goroutines || '12');
    updateVal('threadsVal', data.system.threads || '--');
    updateVal('heapVal', data.system.heap_objects ? (data.system.heap_objects / 1000).toFixed(1) + 'K' : '--');
    updateVal('loadVal', ((data.system.cpu_load || 0.4) * 100).toFixed(0) + '%');
  }

  if (data.workspace) {
    updateVal('filesVal', data.workspace.total_files || '0');
    updateVal('storageVal', data.workspace.size || '0 KB');
  }


  // Render Activity Feed
  const activityList = document.getElementById('activityList');
  if (data.recent_events && data.recent_events.length > 0) {
    activityList.innerHTML = '';
    data.recent_events.forEach(ev => {
      const item = document.createElement('div');
      const statusClass = `event-${(ev.type || 'info').toLowerCase()}`;
      item.className = `activity-item ${statusClass}`;
      
      const time = new Date(ev.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
      
      item.innerHTML = `
        <div class="activity-icon icon-${ev.source.toLowerCase()}">//</div>
        <div class="activity-content">
          <div class="activity-meta">
            <span class="activity-source">[ ${ev.source} ]</span>
            <span class="activity-time">${time}</span>
          </div>
          <div class="activity-msg">${ev.message.toUpperCase()}</div>
        </div>
      `;
      activityList.appendChild(item);
    });
  }
}


function renderLoadingBar(percent) {
  const blocks = Math.round(percent / 10);
  const empty = 10 - blocks;
  return '█'.repeat(blocks) + '░'.repeat(empty);
}

async function loadAgents() {
  const data = await fetchJSON('/agents');
  const tbody = document.getElementById('agentsTableBody');
  tbody.innerHTML = '';
  
  if (!data || !data.agents || data.agents.length === 0) {
    tbody.innerHTML = `<tr><td colspan="5" class="empty-state">No swarm agents are currently active.</td></tr>`;
    return;
  }

  data.agents.forEach(agent => {
    const tr = document.createElement('tr');
    const id = agent.id || agent.ID || 'unknown';
    const task = agent.task || agent.Task || 'Idle';
    const status = (agent.status || agent.Status || 'Running').toUpperCase();
    const created = agent.created || agent.Created || Date.now();
    
    // Matrix style ID
    const displayId = `[AG-${id.substring(0,4).toUpperCase()}]`;
    const isRunning = status === 'RUNNING' || status === 'ACTIVE';
    
    tr.innerHTML = `
      <td><span class="agent-id">${displayId}</span></td>
      <td><span class="${isRunning ? 'typing' : ''}">${task}</span></td>
      <td>
        <div style="font-size: 0.7srem; color: var(--matrix-dim); margin-bottom: 4px;">
          ${renderLoadingBar(isRunning ? 100 : 0)} ${status}
        </div>
      </td>
      <td>${Math.round((Date.now() - created)/60000)} MINS</td>
      <td><button class="danger" onclick="killAgent('${id}')">TERMINATE</button></td>
    `;
    tbody.appendChild(tr);
  });
}


async function killAgent(id) {
  if (!confirm(`Are you sure you want to terminate agent ${id}?`)) return;
  await fetchJSON(`/agents/${id}`, { method: 'DELETE' });
  loadAgents();
}

async function loadSkills() {
  const data = await fetchJSON('/skills');
  const tbody = document.getElementById('skillsTableBody');
  tbody.innerHTML = '';
  
  if (!data || !data.skills || data.skills.length === 0) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty-state">No builtin skills found.</td></tr>`;
    return;
  }

  data.skills.forEach(skill => {
    const tr = document.createElement('tr');
    // Handle both lowercase (from JSON tags) and uppercase (Go field names)
    const name = skill.name || skill.Name || 'undefined';
    const desc = skill.description || skill.Description || 'undefined';
    const source = skill.source || skill.Source || 'undefined';
    const caps = skill.capabilities || skill.Capabilities || 'None';

    tr.innerHTML = `
      <td><strong>${name}</strong><div class="skill-desc">${desc}</div></td>
      <td><span class="badge badge-info">${source}</span></td>
      <td>${caps}</td>
      <td><button onclick="executeSkill('${name}')">Run Test</button></td>
    `;
    tbody.appendChild(tr);
  });
}

async function executeSkill(skillName) {
  const input = prompt(`Enter test payload for ${skillName}:`, "Ping");
  if (!input) return;

  const loader = document.getElementById('chatLoader');
  if (loader) loader.classList.add('active');

  try {
    const res = await fetch(`${API_BASE}/skills/${skillName}/execute`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ input: input })
    });
    const data = await res.json();
    
    // Use correct casing from Go JSON tags
    const resultText = data.result || data.error || "Skill returned empty response";
    
    // Integrate with Chat Terminal for comprehensive visibility
    chatMessages.push({ role: 'user', content: `[Manual Skill Test: ${skillName}]\nInput: ${input}` });
    chatMessages.push({ role: 'assistant', content: resultText });
    renderChat();

    // Still show alert if user isn't looking at the chat
    const activeSection = document.querySelector('.section.active').id;
    if (activeSection !== 'chat') {
      alert(`Skill Execution: ${skillName}\n\n${resultText}`);
    }

  } catch (err) {
    console.error("Skill execution error:", err);
    alert("Skill execution failed. See console for details.");
  } finally {
    if (loader) loader.classList.remove('active');
    loadStatus(); // Instant refresh of activity feed
  }
}

async function loadFiles() {
  const data = await fetchJSON('/files');
  const fileList = document.getElementById('fileList');
  if (!data || !data.files) {
    fileList.innerHTML = '<li class="file-item">Failed to load files</li>';
    return;
  }

  // Preserve active selection if possible
  const activePath = document.querySelector('.file-item.active')?.dataset.path;
  
  fileList.innerHTML = '';
  
  // Build Nested Tree Structure
  const root = { name: 'root', type: 'folder', children: {}, open: true };
  
  data.files.forEach(file => {
    const parts = file.rel_path.split(/[\\\/]/);
    let current = root;
    
    parts.forEach((part, index) => {
      const isLast = index === parts.length - 1;
      if (isLast) {
        current.children[part] = { ...file, type: 'file', name: part };
      } else {
        if (!current.children[part]) {
          current.children[part] = { name: part, type: 'folder', children: {}, open: true };
        }
        current = current.children[part];
      }
    });
  });

  // Recursive Render Function
  function renderTree(node, container, level = 0) {
    const sortedKeys = Object.keys(node.children).sort((a, b) => {
      const nodeA = node.children[a];
      const nodeB = node.children[b];
      if (nodeA.type !== nodeB.type) return nodeA.type === 'folder' ? -1 : 1;
      return a.localeCompare(b);
    });

    sortedKeys.forEach(key => {
      const item = node.children[key];
      const itemEl = document.createElement('div');
      itemEl.className = `tree-item ${item.type}-item`;
      itemEl.style.paddingLeft = `${level * 15}px`;

      if (item.type === 'folder') {
        itemEl.innerHTML = `
          <span class="folder-toggle">${item.open ? '▼' : '▶'}</span>
          <span class="folder-icon">📁</span>
          <span class="folder-name">${item.name}</span>
        `;
        
        const childrenContainer = document.createElement('div');
        childrenContainer.className = 'folder-children';
        if (!item.open) childrenContainer.style.display = 'none';

        itemEl.onclick = (e) => {
          e.stopPropagation();
          item.open = !item.open;
          itemEl.querySelector('.folder-toggle').innerText = item.open ? '▼' : '▶';
          childrenContainer.style.display = item.open ? 'block' : 'none';
        };

        container.appendChild(itemEl);
        container.appendChild(childrenContainer);
        renderTree(item, childrenContainer, level + 1);
      } else {
        itemEl.innerHTML = `
          <span class="file-icon">📄</span>
          <span class="file-name">${item.name}</span>
        `;
        if (item.path === activePath) itemEl.classList.add('active');
        itemEl.dataset.path = item.path;
        itemEl.onclick = (e) => {
          e.stopPropagation();
          loadFileContent(item.path, itemEl);
        };
        container.appendChild(itemEl);
      }
    });
  }

  renderTree(root, fileList);

  if (data.files.length === 0) {
    fileList.innerHTML = '<div class="tree-item">No markdown files found.</div>';
  }
}

let currentlyEditingPath = '';

async function loadFileContent(path, el) {
  // Reset UI
  cancelEdit();
  
  document.querySelectorAll('.file-item').forEach(item => item.classList.remove('active'));
  if (el) el.classList.add('active');

  document.getElementById('fileContent').innerText = '// PULLING_DATA_FROM_GRID...';
  document.getElementById('docsControls').style.display = 'none';
  
  const data = await fetchJSON(`/files/content?path=${encodeURIComponent(path)}`);
  if (!data || data.content === undefined) {
    document.getElementById('fileContent').innerText = 'Failed to load file content.';
    return;
  }

  currentlyEditingPath = path;
  document.getElementById('currentFileName').innerText = data.name.toUpperCase();
  document.getElementById('fileContent').innerText = data.content;
  document.getElementById('fileEditor').value = data.content;
  document.getElementById('docsControls').style.display = 'flex';
  
  const viewer = document.querySelector('.viewer-content');
  viewer.scrollTop = 0;
}

function startEdit() {
  document.getElementById('editBtn').style.display = 'none';
  document.getElementById('saveControls').style.display = 'flex';
  document.getElementById('fileContent').style.display = 'none';
  document.getElementById('editArea').style.display = 'block';
}

function cancelEdit() {
  document.getElementById('editBtn').style.display = 'block';
  document.getElementById('saveControls').style.display = 'none';
  document.getElementById('fileContent').style.display = 'block';
  document.getElementById('editArea').style.display = 'none';
}

async function saveFile() {
  const content = document.getElementById('fileEditor').value;
  const originalSaveText = document.getElementById('saveBtn').innerText;
  
  document.getElementById('saveBtn').innerText = 'UPLOADING_DATA...';
  document.getElementById('saveBtn').disabled = true;

  try {
    const res = await fetch(`${API_BASE}/files/save`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        path: currentlyEditingPath,
        content: content
      })
    });
    const data = await res.json();
    
    if (data.success) {
      document.getElementById('fileContent').innerText = content;
      cancelEdit();
      // Record in activity feed via loadStatus
      loadStatus();
    } else {
      alert("FAIL: MISSION_CONTROL could not sync with VPS.");
    }
  } catch (err) {
    console.error("Save error:", err);
    alert("CONNECTION_LOST: Check SSH tunnel.");
  } finally {
    document.getElementById('saveBtn').innerText = originalSaveText;
    document.getElementById('saveBtn').disabled = false;
  }
}

// Attach Edit listeners
document.addEventListener('DOMContentLoaded', () => {
    const editBtn = document.getElementById('editBtn');
    if (editBtn) editBtn.onclick = startEdit;
    
    const cancelBtn = document.getElementById('cancelBtn');
    if (cancelBtn) cancelBtn.onclick = cancelEdit;
    
    const saveBtn = document.getElementById('saveBtn');
    if (saveBtn) saveBtn.onclick = saveFile;
});

// --- Chat functionality ---
let chatMessages = [
  { 
    role: 'assistant', 
    content: 'Hello, Commander. Embedded RDxClaw agent ready for input. How can I assist?',
    timestamp: new Date()
  }
];

function renderChat() {
  const container = document.getElementById('chatMessages');
  container.innerHTML = '';
  
  chatMessages.forEach((msg, idx) => {
    const div = document.createElement('div');
    div.className = `msg ${msg.role}`;
    
    const time = msg.timestamp ? new Date(msg.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }) : '';
    const isLast = idx === chatMessages.length - 1;
    const isAgent = msg.role === 'assistant' || msg.role === 'agent';
    
    div.innerHTML = `
      <div class="msg-meta">
        <span class="msg-role">${isAgent ? '[ AGENT_UPLINK ]' : '[ COMMANDER ]'}</span>
        <span class="msg-time">${time}</span>
      </div>
      <div class="msg-bubble ${isLast && isAgent ? 'typing' : ''}">${escapeHTML(msg.content)}</div>
    `;
    container.appendChild(div);
  });
  
  container.scrollTop = container.scrollHeight;
}


async function sendMessage() {
  const inputEl = document.getElementById('chatInput');
  const text = inputEl.value.trim();
  if (!text) return;

  // Add user msg
  chatMessages.push({ role: 'user', content: text, timestamp: new Date() });
  inputEl.value = '';
  renderChat();

  const loader = document.getElementById('chatLoader');
  loader.classList.add('active');

  try {
    const res = await fetch(`${API_BASE}/chat/completions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        messages: chatMessages,
        channel: 'mission-control',
        sessionKey: 'mc-' + Date.now()
      })
    });
    const data = await res.json();
    
    if (data.choices && data.choices.length > 0) {
      const assistantMsg = data.choices[0].message;
      assistantMsg.timestamp = new Date();
      chatMessages.push(assistantMsg);
    } else {
      chatMessages.push({ role: 'assistant', content: 'Error: Cannot communicate with brain.', timestamp: new Date() });
    }
  } catch (err) {
    chatMessages.push({ role: 'assistant', content: 'Connection failed.' });
  }

  loader.classList.remove('active');
  renderChat();
}

document.getElementById('chatInput').addEventListener('keypress', function (e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    sendMessage();
  }
});

function escapeHTML(str) {
  return str.replace(/[&<>'"]/g, 
    tag => ({
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      "'": '&#39;',
      '"': '&quot;'
    }[tag] || tag)
  );
}

// Initial render
renderChat();
