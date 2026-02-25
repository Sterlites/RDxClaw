const API_BASE = '/v1';

// Navigation State
document.querySelectorAll('.nav-item').forEach(el => {
  el.addEventListener('click', (e) => {
    document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
    el.classList.add('active');
    
    document.querySelectorAll('.section').forEach(s => s.classList.remove('active'));
    document.getElementById(el.dataset.target).classList.add('active');
    document.getElementById('pageTitle').innerText = el.querySelector('span').innerText;

    // Load data based on tab
    if (el.dataset.target === 'dashboard') loadStatus();
    if (el.dataset.target === 'swarm') loadAgents();
    if (el.dataset.target === 'skills') loadSkills();
  });
});

// Initialization
let refreshInterval;
document.addEventListener('DOMContentLoaded', () => {
  loadStatus();
  // Poll every 5 seconds
  refreshInterval = setInterval(() => {
    const activeSection = document.querySelector('.section.active');
    if (activeSection.id === 'dashboard') loadStatus();
    if (activeSection.id === 'swarm') loadAgents();
  }, 5000);
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

  document.getElementById('uptimeVal').innerText = data.uptime || 'N/A';
  document.getElementById('versionVal').innerText = data.version || 'v1.0.0';
  document.getElementById('sidebarVersion').innerText = `Framework ${data.version || 'v1.0.0'}`;
  document.getElementById('modelVal').innerText = data.agent?.model || 'N/A';
  document.getElementById('agentsVal').innerText = data.active_agents || '0';
  document.getElementById('skillsVal').innerText = data.skills?.total || '0';

  // Render Activity Feed
  const activityList = document.getElementById('activityList');
  if (data.recent_events && data.recent_events.length > 0) {
    activityList.innerHTML = '';
    data.recent_events.forEach(ev => {
      const item = document.createElement('div');
      item.className = 'activity-item';
      
      const time = new Date(ev.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
      const iconChar = ev.source.charAt(0).toUpperCase();
      
      item.innerHTML = `
        <div class="activity-icon icon-${ev.source.toLowerCase()}">${iconChar}</div>
        <div class="activity-content">
          <div class="activity-meta">
            <span class="activity-source">${ev.source}</span>
            <span class="activity-time">${time}</span>
          </div>
          <div class="activity-msg">${ev.message}</div>
        </div>
      `;
      activityList.appendChild(item);
    });
  }
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
    tr.innerHTML = `
      <td><span class="agent-id">${agent.id.substring(0,8)}...</span></td>
      <td>${agent.task || 'Idle'}</td>
      <td><span class="badge badge-success">Running</span></td>
      <td>${Math.round((Date.now() - new Date(agent.startedAt).getTime())/60000)} mins</td>
      <td><button class="danger" onclick="killAgent('${agent.id}')">Terminate</button></td>
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
    tr.innerHTML = `
      <td><strong>${skill.Name}</strong><div class="skill-desc">${skill.Description}</div></td>
      <td><span class="badge badge-info">${skill.Source}</span></td>
      <td>${(skill.Capabilities || []).join(', ') || 'None'}</td>
      <td><button onclick="executeSkill('${skill.Name}')">Run Test</button></td>
    `;
    tbody.appendChild(tr);
  });
}

async function executeSkill(skillName) {
  const input = prompt(`Enter test payload for ${skillName}:`, "Ping");
  if (!input) return;

  try {
    const res = await fetch(`${API_BASE}/skills/${skillName}/execute`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ input: input })
    });
    const result = await res.json();
    alert(`Execution Result:\n\n${result.Result || result.Error}`);
  } catch (err) {
    alert("Skill execution failed.");
  }
}

// --- Chat functionality ---
let chatMessages = [
  { role: 'assistant', content: 'Hello, Commander. Embedded RDxClaw agent ready for input. How can I assist?' }
];

function renderChat() {
  const container = document.getElementById('chatMessages');
  container.innerHTML = '';
  
  chatMessages.forEach(msg => {
    const div = document.createElement('div');
    div.className = `msg ${msg.role}`;
    div.innerHTML = `<div class="msg-bubble">${escapeHTML(msg.content)}</div>`;
    container.appendChild(div);
  });
  
  container.scrollTop = container.scrollHeight;
}

async function sendMessage() {
  const inputEl = document.getElementById('chatInput');
  const text = inputEl.value.trim();
  if (!text) return;

  // Add user msg
  chatMessages.push({ role: 'user', content: text });
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
      chatMessages.push(data.choices[0].message);
    } else {
      chatMessages.push({ role: 'assistant', content: 'Error: Cannot communicate with brain.' });
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
