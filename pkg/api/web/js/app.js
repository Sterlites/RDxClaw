const API_BASE = '/v1';
let API_KEY = localStorage.getItem('RDXCLAW_API_KEY') || '';

function setApiKey(key) {
  API_KEY = key;
  localStorage.setItem('RDXCLAW_API_KEY', key);
  document.getElementById('authOverlay').classList.remove('active');
  loadStatus();
}

function showAuth(isManual = false) {
  const overlay = document.getElementById('authOverlay');
  const title = document.getElementById('authTitle');
  const desc = document.getElementById('authDesc');
  const closeBtn = document.getElementById('closeAuthBtn');
  const submitBtn = document.getElementById('saveKeyBtn');
  const input = document.getElementById('apiKeyInput');

  if (isManual) {
    title.innerText = 'UPLINK_CONFIG';
    title.dataset.text = 'UPLINK_CONFIG';
    desc.innerText = 'Modify API authentication settings for Mission Control.';
    closeBtn.style.display = 'block';
    submitBtn.innerText = 'SYNC_CREDENTIALS';
    input.value = API_KEY;
  } else {
    title.innerText = 'ACCESS_RESTRICTED';
    title.dataset.text = 'ACCESS_RESTRICTED';
    desc.innerText = 'Mission Control requires a valid API Key to establish uplink.';
    closeBtn.style.display = 'none';
    submitBtn.innerText = 'ESTABLISH_UPLINK';
  }

  overlay.classList.add('active');
}

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
    if (el.dataset.target === 'sessions') loadSessions();
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
let eventSource;
document.addEventListener('DOMContentLoaded', () => {
  startMatrix();
  triggerBoot();
  randomGlitch();
  
  loadStatus(); // Initial paint
  initEventSource(); // Real-time uplink
  startLocalClock(); // Local clock ticker independent of SSE
  
  // Auth Event Listeners
  document.getElementById('saveKeyBtn').onclick = () => {
    const val = document.getElementById('apiKeyInput').value.trim();
    if (val) setApiKey(val);
  };
  
  document.getElementById('authBtn').onclick = () => showAuth(true);
  document.getElementById('closeAuthBtn').onclick = () => {
    document.getElementById('authOverlay').classList.remove('active');
  };

  // Failsafe: Check for interrupted sessions on boot
  setTimeout(checkRecovery, 2000);

  // Drawer Listeners
  const drawer = document.getElementById('terminalDrawer');
  const drawerToggle = document.getElementById('drawerToggle');
  const closeDrawerBtn = document.getElementById('closeDrawerBtn');
  
  if (drawerToggle) {
    drawerToggle.onclick = () => {
        drawer.classList.add('active');
        drawerToggle.classList.add('hidden');
        document.getElementById('drawerInput').focus();
    };
  }

  if (closeDrawerBtn) {
    closeDrawerBtn.onclick = () => {
        drawer.classList.remove('active');
        drawerToggle.classList.remove('hidden');
    };
  }

  document.getElementById('drawerSendBtn').onclick = () => sendMessage('drawer');
  document.getElementById('drawerInput').onkeypress = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        sendMessage('drawer');
    }
  };
});

function initEventSource() {
  if (eventSource) eventSource.close();
  
  const url = new URL(`${window.location.origin}${API_BASE}/events`);
  if (API_KEY) url.searchParams.set('api_key', API_KEY); // Or handle via cookie/header if possible
  
  eventSource = new EventSource(url.toString());
  
  eventSource.addEventListener('status', (e) => {
    try {
      const payload = JSON.parse(e.data);
      const data = payload.data;
      lastSseTime = Date.now();
      
      if (data.telemetry && data.telemetry.last_response) {
          renderPulse(data.telemetry.last_response.total_ms);
      }
      
      renderStatus(data);
      
      // Auto-refresh active tabs that depend on status
      const activeSection = document.querySelector('.section.active');
      if (activeSection && activeSection.id === 'swarm') loadAgents();
      if (activeSection && activeSection.id === 'sessions') loadSessions();
    } catch (err) {
      console.error("Failed to parse status event:", err);
    }
  });
  
  eventSource.addEventListener('activity', (e) => {
    try {
      const payload = JSON.parse(e.data);
      addActivityItem(payload.data);
    } catch (err) {
      console.error("Failed to parse activity event:", err);
    }
  });

  eventSource.addEventListener('connected', (e) => {
    console.log("Mission Control Uplink Established:", JSON.parse(e.data));
    showToast('UPLINK_ESTABLISHED // SSE_CONNECTED', 'success');
  });

  eventSource.onerror = (err) => {
    console.warn("EventSource connection lost. Retrying...", err);
    showToast('UPLINK_LOST // RECONNECTING...', 'error');
  };
}

// --- Local Clock & Uptime Ticker ---
let serverUptimeBase = null; // The uptime string last received from server
let serverUptimeReceivedAt = null; // When we received it

function startLocalClock() {
  setInterval(() => {
    // Tick the system clock every second
    const clockEl = document.getElementById('systemClock');
    if (clockEl) {
      const now = new Date();
      const ms = now.getMilliseconds().toString().padStart(3, '0');
      clockEl.innerText = now.toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' }) + ':' + ms;
    }

    // Interpolate uptime locally between SSE ticks
    if (serverUptimeBase && serverUptimeReceivedAt) {
      const elapsed = Math.floor((Date.now() - serverUptimeReceivedAt) / 1000);
      const baseSeconds = parseUptimeToSeconds(serverUptimeBase);
      const currentSeconds = baseSeconds + elapsed;
      const uptimeEl = document.getElementById('uptimeVal');
      if (uptimeEl) uptimeEl.innerHTML = `<span class="matrix-white">${formatSecondsToUptime(currentSeconds)}</span>`;
    }
  }, 200);
}

function parseUptimeToSeconds(str) {
  // Handles Go's time.Duration format like "1h2m3s", "5m30s", "45s"
  let total = 0;
  const h = str.match(/(\d+)h/);
  const m = str.match(/(\d+)m/);
  const s = str.match(/(\d+)s/);
  if (h) total += parseInt(h[1]) * 3600;
  if (m) total += parseInt(m[1]) * 60;
  if (s) total += parseInt(s[1]);
  return total;
}

function formatSecondsToUptime(sec) {
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = sec % 60;
  if (h > 0) return `${h}h${m}m${s}s`;
  if (m > 0) return `${m}m${s}s`;
  return `${s}s`;
}

// Visibility API
document.addEventListener('visibilitychange', () => {
  if (document.hidden) {
    clearInterval(matrixInterval);
  } else {
    matrixInterval = setInterval(drawMatrix, 50);
  }
});


const latencies = [];
const MAX_LATENCIES = 100;

function renderPulse(newLatency) {
    const canvas = document.getElementById('telemetryPulse');
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const dpr = window.devicePixelRatio || 1;
    
    // Resize if needed
    const rect = canvas.parentElement.getBoundingClientRect();
    if (rect.width > 0 && (canvas.width !== rect.width * dpr || canvas.height !== rect.height * dpr)) {
        canvas.width = rect.width * dpr;
        canvas.height = rect.height * dpr;
        canvas.style.width = rect.width + 'px';
        canvas.style.height = rect.height + 'px';
    }

    // If offline (no data), simulate a subtle heartbeat
    const val = newLatency ?? (20 + Math.random() * 5);
    latencies.push(val);
    if (latencies.length > MAX_LATENCIES) latencies.shift();

    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.scale(dpr, dpr);
    
    const baseLine = rect.height - 10;
    const step = rect.width / (MAX_LATENCIES - 1);
    const maxVal = Math.max(...latencies, 500);

    // Draw background grid
    ctx.strokeStyle = 'rgba(0, 255, 65, 0.05)';
    ctx.lineWidth = 0.5;
    for(let i=0; i<rect.width; i+=40) {
        ctx.beginPath(); ctx.moveTo(i, 0); ctx.lineTo(i, rect.height); ctx.stroke();
    }

    ctx.beginPath();
    // Get color from CSS var if possible
    ctx.strokeStyle = getComputedStyle(document.body).getPropertyValue('--matrix-green').trim() || '#00ff41';
    ctx.lineWidth = 2;
    ctx.lineJoin = 'round';
    
    latencies.forEach((v, i) => {
        const x = i * step;
        const h = (v / maxVal) * (rect.height - 20);
        const y = baseLine - h;
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
    });
    ctx.stroke();

    // Fill under curve
    if (latencies.length > 1) {
        ctx.lineTo((latencies.length - 1) * step, baseLine);
        ctx.lineTo(0, baseLine);
        ctx.fillStyle = 'rgba(0, 255, 65, 0.05)';
        ctx.fill();
    }

    if (!newLatency) {
        ctx.fillStyle = 'rgba(255, 0, 65, 0.6)';
        ctx.font = '9px Share Tech Mono';
        ctx.fillText('WARNING: LINK_OFFLINE // RUNNING_SCAN_LOOP', 10, 15);
    }
    
    ctx.setTransform(1,0,0,1,0,0);
}

// Global pulse loop for when SSE is quiet
setInterval(() => {
    if (Date.now() - lastSseTime > 5000) renderPulse(null);
}, 200);

let lastSseTime = 0;

function showToast(message, type = 'info') {
    const container = document.getElementById('toastContainer');
    if (!container) return;

    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.innerHTML = `
        <div class="toast-content">
            <span class="toast-icon">></span>
            <span class="toast-msg">${message.toUpperCase()}</span>
        </div>
    `;

    container.appendChild(toast);
    setTimeout(() => {
        toast.classList.add('out');
        setTimeout(() => toast.remove(), 500);
    }, 4000);
}

// Override alert with toast
window.alert = (msg) => showToast(msg, 'error');

async function fetchJSON(endpoint, options = {}) {
  try {
    const headers = options.headers || {};
    if (API_KEY) {
      headers['Authorization'] = `Bearer ${API_KEY}`;
    }

    const res = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers: headers,
      cache: 'no-store'
    });

    if (res.status === 401) {
      showAuth();
      throw new Error("UNAUTHORIZED: API Key required.");
    }

    if (!res.ok) throw new Error(`HTTP error! status: ${res.status}`);
    return await res.json();
  } catch (err) {
    console.error(`Error fetching ${endpoint}:`, err);
    return null;
  }
}

async function loadStatus() {
  const data = await fetchJSON(`/status?session_key=${SESSION_ID}`);
  if (data) renderStatus(data);
}

function renderStatus(data) {

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

  const formatDuration = (ms) => {
    if (ms === undefined || ms === null) return '--';
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
  };

  // Capture server uptime for local interpolation
  if (data.uptime) {
    serverUptimeBase = data.uptime;
    serverUptimeReceivedAt = Date.now();
  }
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
  if (data.recent_events && data.recent_events.length > 0) {
    const activityList = document.getElementById('activityList');
    if (activityList) {
      activityList.innerHTML = '';
      data.recent_events.forEach(ev => addActivityItem(ev, false));
    }
  }
  // Latency Telemetry
  if (data.telemetry) {
    const tel = data.telemetry;
    
    // Last Response
    if (tel.last_response) {
      updateVal('lastTotalLatency', formatDuration(tel.last_response.total_ms));
      updateVal('lastTurns', tel.last_response.iteration_count);
      updateVal('lastStartup', formatDuration(tel.last_response.startup_ms));
      updateVal('lastContext', formatDuration(tel.last_response.context_build_ms));
      updateVal('lastLLM', formatDuration(tel.last_response.llm_calls_ms));
      updateVal('lastTools', formatDuration(tel.last_response.tool_exec_ms));
      updateVal('lastPrep', formatDuration(tel.last_response.response_prepare_ms));
      
      renderLatencyVisualizer(tel.last_response);
      renderPulse(tel.last_response.total_ms);
    }
    
    // Session Averages
    if (tel.session_averages && tel.session_averages.count > 0) {
      updateVal('sessTotalLatency', formatDuration(Math.round(tel.session_averages.total_ms)));
      updateVal('sessTurns', tel.session_averages.average_iterations.toFixed(1));
      updateVal('sessStartup', formatDuration(Math.round(tel.session_averages.startup_ms)));
      updateVal('sessContext', formatDuration(Math.round(tel.session_averages.context_build_ms)));
      updateVal('sessLLM', formatDuration(Math.round(tel.session_averages.llm_calls_ms)));
      updateVal('sessTools', formatDuration(Math.round(tel.session_averages.tool_exec_ms)));
      updateVal('sessPrep', formatDuration(Math.round(tel.session_averages.response_prepare_ms)));
    }
    
    // Global Averages
    if (tel.overall_averages && tel.overall_averages.count > 0) {
      updateVal('globalTotalLatency', formatDuration(Math.round(tel.overall_averages.total_ms)));
      updateVal('globalTurns', tel.overall_averages.average_iterations.toFixed(1));
      updateVal('globalStartup', formatDuration(Math.round(tel.overall_averages.startup_ms)));
      updateVal('globalContext', formatDuration(Math.round(tel.overall_averages.context_build_ms)));
      updateVal('globalLLM', formatDuration(Math.round(tel.overall_averages.llm_calls_ms)));
      updateVal('globalTools', formatDuration(Math.round(tel.overall_averages.tool_exec_ms)));
      updateVal('globalPrep', formatDuration(Math.round(tel.overall_averages.response_prepare_ms)));
    }
  }
}

function addActivityItem(ev, prepend = true) {
  const activityList = document.getElementById('activityList');
  if (!activityList) return;

  // Remove empty state if present
  const empty = activityList.querySelector('.empty-state');
  if (empty) empty.remove();

  const item = document.createElement('div');
  const statusClass = `event-${(ev.type || 'info').toLowerCase()}`;
  item.className = `activity-item ${statusClass}`;
  
  const time = new Date(ev.timestamp || Date.now()).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  
  item.innerHTML = `
    <div class="activity-icon icon-${(ev.source || 'system').toLowerCase()}">//</div>
    <div class="activity-content">
      <div class="activity-meta">
        <span class="activity-source">[ ${ev.source || 'SYSTEM'} ]</span>
        <span class="activity-time">${time}</span>
      </div>
      <div class="activity-msg">${ev.message}</div>
    </div>
  `;

  if (prepend && activityList.firstChild) {
    activityList.insertBefore(item, activityList.firstChild);
    // Keep only last 50
    if (activityList.children.length > 50) {
      activityList.lastChild.remove();
    }
  } else {
    activityList.appendChild(item);
  }
}

function renderLatencyVisualizer(stats) {
    const container = document.getElementById('latencyVisualizer');
    if (!container) return;
    container.innerHTML = '';
    
    const total = Math.max(stats.total_ms, 1);
    const pStartup = (stats.startup_ms / total) * 100;
    const pContext = (stats.context_build_ms / total) * 100;
    const pLLM = (stats.llm_calls_ms / total) * 100;
    const pTools = (stats.tool_exec_ms / total) * 100;
    const pPrep = (stats.response_prepare_ms / total) * 100;
    
    const addSegment = (p, label, colorClass) => {
        if (p <= 0.1) return; // Ignore tiny segments
        const bar = document.createElement('div');
        bar.className = `bar-segment ${colorClass}`;
        bar.style.width = `${p}%`;
        bar.dataset.label = label;
        container.appendChild(bar);
    };

    addSegment(pStartup, `START: ${stats.startup_ms}ms`, 'bar-startup');
    addSegment(pContext, `CTX: ${stats.context_build_ms}ms`, 'bar-context');
    addSegment(pLLM, `LLM: ${stats.llm_calls_ms}ms`, 'bar-llm');
    addSegment(pTools, `TOOLS: ${stats.tool_exec_ms}ms`, 'bar-tools');
    addSegment(pPrep, `PREP: ${stats.response_prepare_ms}ms`, 'bar-prep');
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
    
    const isPrimary = agent.is_primary || agent.IsPrimary || false;
    const actionBtn = isPrimary ? `<button class="primary disabled" disabled>CORE</button>` : `<button class="danger" onclick="killAgent('${id}')">TERMINATE</button>`;
    
    tr.innerHTML = `
      <td><span class="agent-id">${displayId}</span></td>
      <td><span class="${isRunning ? 'typing' : ''}">${task}</span></td>
      <td>
        <div style="font-size: 0.7srem; color: var(--matrix-dim); margin-bottom: 4px;">
          ${renderLoadingBar(isRunning ? 100 : 0)} ${status}
        </div>
      </td>
      <td>${Math.round((Date.now() - created)/60000)} MINS</td>
      <td>${actionBtn}</td>
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
    const headers = { 'Content-Type': 'application/json' };
    if (API_KEY) headers['Authorization'] = `Bearer ${API_KEY}`;

    const res = await fetch(`${API_BASE}/skills/${skillName}/execute`, {
      method: 'POST',
      headers: headers,
      body: JSON.stringify({ 
        input: input,
        session_key: SESSION_ID
      })
    });

    if (res.status === 401) {
      showAuth();
      return;
    }
    const data = await res.json();
    
    // Use correct casing from Go JSON tags
    const resultText = data.result || data.error || "Skill returned empty response";
    
    // Integrate with Chat Terminal for comprehensive visibility
    chatMessages.push({ role: 'user', content: `[Manual Skill Test: ${skillName}]\nInput: ${input}`, timestamp: new Date() });
    chatMessages.push({ role: 'assistant', content: resultText, timestamp: new Date(), telemetry: data.duration });
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

const treeFolderStates = {};

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
    let currentPath = '';
    
    parts.forEach((part, index) => {
      currentPath = currentPath ? currentPath + '/' + part : part;
      const isLast = index === parts.length - 1;
      if (isLast) {
        current.children[part] = { ...file, type: 'file', name: part };
      } else {
        if (!current.children[part]) {
          let initialOpen = treeFolderStates[currentPath] !== undefined ? treeFolderStates[currentPath] : false;
          current.children[part] = { name: part, type: 'folder', children: {}, open: initialOpen, path: currentPath };
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
          if (item.path) treeFolderStates[item.path] = item.open;
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
  document.getElementById('currentFileName').innerText = data.name;
  
  // Syntax Highlight if it's Go or MD
  let content = data.content;
  if (data.name.endsWith('.go') || data.name.endsWith('.md')) {
      document.getElementById('fileContent').innerHTML = highlightCode(content, data.name.split('.').pop());
  } else {
      document.getElementById('fileContent').innerText = content;
  }
  
  document.getElementById('fileEditor').value = data.content;
  document.getElementById('docsControls').style.display = 'flex';
  
  const viewer = document.querySelector('.viewer-content');
  viewer.scrollTop = 0;
}

function highlightCode(code, lang) {
    // Ultra-light regex highlighter
    let h = code
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;");

    if (lang === 'go') {
        const keywords = /\b(package|import|func|type|struct|interface|return|var|const|if|else|for|range|chan|go|select|case|default|map|chan|defer)\b/g;
        const types = /\b(string|int|int64|float64|bool|error|any|interface|Map|Server|AgentLoop|MessageBus)\b/g;
        const comments = /(\/\/.*$|\/\*[\s\S]*?\*\/)/gm;
        const strings = /("[^"]*"|'[^']*')/g;

        h = h.replace(comments, '<span class="c-comment">$1</span>');
        h = h.replace(strings, '<span class="c-string">$1</span>');
        h = h.replace(keywords, '<span class="c-keyword">$1</span>');
        h = h.replace(types, '<span class="c-type">$1</span>');
    } else if (lang === 'md') {
        h = h.replace(/^#+ (.*)$/gm, '<span class="md-h">$0</span>');
        h = h.replace(/`([^`]+)`/g, '<span class="md-code">$1</span>');
        h = h.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<span class="md-link">$1</span>');
        h = h.replace(/^\- (.*)$/gm, '<span class="md-list">$0</span>');
    }
    return h;
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
    const headers = { 'Content-Type': 'application/json' };
    if (API_KEY) headers['Authorization'] = `Bearer ${API_KEY}`;

    const res = await fetch(`${API_BASE}/files/save`, {
      method: 'POST',
      headers: headers,
      body: JSON.stringify({
        path: currentlyEditingPath,
        content: content
      })
    });

    if (res.status === 401) {
      showAuth();
      return;
    }
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
  const containers = [
    document.getElementById('chatMessages'),
    document.getElementById('drawerMessages')
  ];

  containers.forEach(container => {
    if (!container) return;
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
            <span class="msg-time">${time}${msg.telemetry ? ` // LATENCY: ${msg.telemetry}ms` : ''}</span>
          </div>
          <div class="msg-bubble ${isLast && isAgent ? 'typing' : ''}">${msg.content}</div>
        `;
        container.appendChild(div);
    });
    container.scrollTop = container.scrollHeight;
  });
}


const SESSION_ID = 'mc-' + Date.now();

async function sendMessage(source = 'main') {
  const inputId = source === 'drawer' ? 'drawerInput' : 'chatInput';
  const inputEl = document.getElementById(inputId);
  const text = inputEl.value.trim();
  if (!text) return;

  // Add user msg
  chatMessages.push({ role: 'user', content: text, timestamp: new Date() });
  inputEl.value = '';
  renderChat();

  const loader = document.getElementById('chatLoader');
  loader.classList.add('active');
  const startTime = Date.now();

  try {
    const headers = { 'Content-Type': 'application/json' };
    if (API_KEY) headers['Authorization'] = `Bearer ${API_KEY}`;

    const res = await fetch(`${API_BASE}/chat/completions`, {
      method: 'POST',
      headers: headers,
      body: JSON.stringify({
        messages: chatMessages.slice(-20), // Send last 20 messages for context
        channel: 'mission-control',
        session_key: SESSION_ID,
        stream: true
      })
    });

    if (res.status === 401) {
      showAuth();
      throw new Error("UNAUTHORIZED");
    }
    
    if (!res.ok) {
      const errorText = await res.text();
      throw new Error(`HTTP error ${res.status}: ${errorText}`);
    }

    const reader = res.body.getReader();
    const decoder = new TextDecoder('utf-8');
    
    const assistantMsg = { role: 'assistant', content: '', timestamp: new Date() };
    chatMessages.push(assistantMsg);
    renderChat();

    let done = false;
    let buffer = '';
    while (!done) {
      const { value, done: streamDone } = await reader.read();
      done = streamDone;
      if (value) {
        buffer += decoder.decode(value, { stream: true });
        let newlineIdx;
        while ((newlineIdx = buffer.indexOf('\n')) >= 0) {
          const line = buffer.slice(0, newlineIdx).trim();
          buffer = buffer.slice(newlineIdx + 1);
          
          if (line.startsWith('data: ')) {
            const dataStr = line.slice(6);
            if (dataStr === '[DONE]') {
              done = true; // stop parsing
              break;
            }
            try {
              const data = JSON.parse(dataStr);
              if (data.choices && data.choices.length > 0) {
                const choice = data.choices[0];
                const msg = choice.message;
                
                if (msg && msg.content) {
                  // If it's the final stop message or an intermediate piece
                  if (choice.finish_reason === 'stop' || choice.finish_reason === 'error') {
                    // Final response
                    if (assistantMsg.content) {
                      assistantMsg.content += '\n\n---\n\n' + msg.content;
                    } else {
                      assistantMsg.content = msg.content;
                    }
                  } else {
                    // Intermediate thought
                    // Only append if it's different from what we already have, or just overwrite it if it's sending the cumulative thought
                    // Wait, the API sends the full thought block from the agent loop intermediate step.
                    // If multiple tools are called, it builds up. Our SSE chunk sends `response.Content` which is the FULL thought so far!
                    // Wait, no. If iteration 1 has thought A, then iteration 2 has thought B.
                    // Let's just append with a separator.
                    if (assistantMsg.content && !assistantMsg.content.endsWith(msg.content)) {
                      assistantMsg.content += '\n\n> ' + msg.content.replace(/\n/g, '\n> ');
                    } else if (!assistantMsg.content) {
                      assistantMsg.content = '> ' + msg.content.replace(/\n/g, '\n> ');
                    }
                  }
                  renderChat();
                }
              }
            } catch (e) {
              console.error("Stream parse error:", e, dataStr);
            }
          }
        }
      }
    }

    assistantMsg.telemetry = Date.now() - startTime;
    assistantMsg.timestamp = new Date();

  } catch (err) {
    chatMessages.push({ role: 'assistant', content: `Connection failed: ${err.message}` });
  }

  loader.classList.remove('active');
  renderChat();
}

async function loadSessions() {
  const data = await fetchJSON('/sessions');
  const tbody = document.getElementById('sessionsTableBody');
  if (!tbody) return;
  tbody.innerHTML = '';
  
  if (!data || !data.sessions || data.sessions.length === 0) {
    tbody.innerHTML = `<tr><td colspan="5" class="empty-state">No recorded missions found in workspace/sessions.</td></tr>`;
    return;
  }

  // Sort by last update descending
  const sorted = data.sessions.sort((a, b) => new Date(b.last_update) - new Date(a.last_update));

  sorted.forEach(sess => {
    const tr = document.createElement('tr');
    const statusClass = sess.status === 'interrupted' ? 'interrupted' : (sess.status === 'active' ? 'active' : 'completed');
    
    tr.innerHTML = `
      <td><span class="agent-id">[ ${sess.key} ]</span></td>
      <td>${sess.turn_count} TURNS</td>
      <td>${new Date(sess.last_update).toLocaleString()}</td>
      <td><span class="status-tag ${statusClass}">${sess.status.toUpperCase()}</span></td>
      <td>
        ${sess.status === 'interrupted' ? `<button class="btn-matrix btn-small" onclick="resumeSession('${sess.key}')">RESUME</button>` : ''}
        <button class="btn-matrix btn-small" onclick="viewSessionLogs('${sess.key}')">LOGS</button>
      </td>
    `;
    tbody.appendChild(tr);
  });
}

async function resumeSession(key) {
  const res = await fetchJSON('/sessions/resume', {
    method: 'POST',
    body: JSON.stringify({ session_key: key })
  });
  
  if (res && res.success) {
    document.getElementById('recoveryOverlay').classList.remove('active');
    // Switch to chat tab to see the resumed output
    document.querySelector('[data-target="chat"]').click();
  } else {
    alert("FAILED_TO_RESUME: Context synchronization error.");
  }
}

async function checkRecovery() {
  const data = await fetchJSON('/sessions');
  if (!data || !data.sessions) return;
  
  const interrupted = data.sessions.find(s => s.status === 'interrupted');
  if (interrupted) {
    const overlay = document.getElementById('recoveryOverlay');
    const info = document.getElementById('recoveryInfo');
    
    info.innerHTML = `
      <div class="recovery-meta"><span>MISSION_KEY:</span><span>${interrupted.key}</span></div>
      <div class="recovery-meta"><span>LAST_TURN:</span><span>${interrupted.turn_count}</span></div>
      <div class="recovery-meta"><span>TIME_STAMP:</span><span>${new Date(interrupted.last_update).toLocaleTimeString()}</span></div>
    `;
    
    document.getElementById('resumeSessionBtn').onclick = () => resumeSession(interrupted.key);
    document.getElementById('discardRecoveryBtn').onclick = () => overlay.classList.remove('active');
    
    overlay.classList.add('active');
  }
}

function viewSessionLogs(key) {
  // Mock: in real use, this would load the session history into the terminal view
  alert(`Loading context logs for ${key}...`);
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
