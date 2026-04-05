(function() {
    const palette = document.getElementById('commandPalette');
    const input = document.getElementById('paletteInput');
    const results = document.getElementById('paletteResults');
    let selectedIndex = -1;
    let filteredItems = [];

    const ACTIONS = [
        { id: 'nav-dashboard', text: 'GO TO DASHBOARD', type: 'NAV', action: () => document.querySelector('[data-target="dashboard"]').click() },
        { id: 'nav-swarm', text: 'GO TO SWARM MGMT', type: 'NAV', action: () => document.querySelector('[data-target="swarm"]').click() },
        { id: 'nav-skills', text: 'GO TO SKILLS LIB', type: 'NAV', action: () => document.querySelector('[data-target="skills"]').click() },
        { id: 'nav-terminal', text: 'GO TO TERMINAL', type: 'NAV', action: () => document.querySelector('[data-target="chat"]').click() },
        { id: 'nav-docs', text: 'GO TO MEMORY DOCS', type: 'NAV', action: () => document.querySelector('[data-target="docs"]').click() },
        { id: 'ui-drawer', text: 'TOGGLE GLOBAL DRAWER', type: 'UI', action: () => document.getElementById('drawerToggle').click() },
        { id: 'theme-matrix', text: 'SET THEME: MATRIX', type: 'THEME', action: () => setTheme('matrix') },
        { id: 'theme-zion', text: 'SET THEME: ZION', type: 'THEME', action: () => setTheme('zion') },
        { id: 'theme-neuromancer', text: 'SET THEME: NEUROMANCER', type: 'THEME', action: () => setTheme('neuromancer') },
        { id: 'cmd-terminate', text: 'TERMINATE ALL AGENTS', type: 'CMD', action: async () => {
            if(confirm('TERMINATE ALL ACTIVE AGENTS?')) {
                const data = await fetchJSON('/agents');
                if (data && data.agents) {
                    for (const a of data.agents) {
                       if (!a.is_primary) await fetchJSON(`/agents/${a.id}`, { method: 'DELETE' });
                    }
                    showToast('ALL SUB-AGENTS TERMINATED', 'success');
                    loadAgents();
                }
            }
        }},
        { id: 'cmd-reload', text: 'RELOAD MISSION CONTROL', type: 'CMD', action: () => window.location.reload() }
    ];

    window.addEventListener('keydown', (e) => {
        if ((e.ctrlKey || e.metaKey) && e.key === ' ') {
            e.preventDefault();
            togglePalette();
        }
        if (e.key === 'Escape' && palette.classList.contains('active')) {
            togglePalette();
        }
    });

    function togglePalette() {
        palette.classList.toggle('active');
        if (palette.classList.contains('active')) {
            input.value = '';
            input.focus();
            renderResults('');
        }
    }

    input.addEventListener('input', (e) => {
        renderResults(e.target.value);
    });

    input.addEventListener('keydown', (e) => {
        if (e.key === 'ArrowDown') {
            e.preventDefault();
            selectedIndex = Math.min(selectedIndex + 1, filteredItems.length - 1);
            updateSelection();
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            selectedIndex = Math.max(selectedIndex - 1, 0);
            updateSelection();
        } else if (e.key === 'Enter') {
            e.preventDefault();
            if (selectedIndex >= 0) executeItem(filteredItems[selectedIndex]);
        }
    });

    async function renderResults(query) {
        const q = query.toLowerCase();
        
        // Combine static actions with dynamic results if needed
        filteredItems = ACTIONS.filter(item => item.text.toLowerCase().includes(q));

        // Add dynamic file search if in docs tab or as global
        if (q.length > 1) {
            try {
                const fileData = await fetchJSON('/files');
                if (fileData && fileData.files) {
                    const matches = fileData.files
                        .filter(f => f.name.toLowerCase().includes(q))
                        .map(f => ({
                            id: `file-${f.path}`,
                            text: `OPEN: ${f.name}`,
                            type: 'FILE',
                            action: () => {
                                document.querySelector('[data-target="docs"]').click();
                                loadFileContent(f.path);
                            }
                        }));
                    filteredItems = [...filteredItems, ...matches];
                }
            } catch (err) { /* ignore */ }
        }

        results.innerHTML = '';
        filteredItems.forEach((item, index) => {
            const el = document.createElement('div');
            el.className = 'palette-item';
            el.innerHTML = `
                <span class="palette-text">${item.text}</span>
                <span class="palette-type">${item.type}</span>
            `;
            el.onclick = () => executeItem(item);
            results.appendChild(el);
        });

        selectedIndex = filteredItems.length > 0 ? 0 : -1;
        updateSelection();
    }

    function updateSelection() {
        const items = results.querySelectorAll('.palette-item');
        items.forEach((item, i) => {
            item.classList.toggle('selected', i === selectedIndex);
            if (i === selectedIndex) item.scrollIntoView({ block: 'nearest' });
        });
    }

    function executeItem(item) {
        if (item && item.action) {
            item.action();
            togglePalette();
        }
    }

    function setTheme(theme) {
        document.body.className = 'matrix-theme';
        if (theme !== 'matrix') document.body.classList.add(`${theme}-theme`);
        localStorage.setItem('rdx_theme', theme);
        showToast(`THEME_UPDATED: ${theme.toUpperCase()}`, 'success');
    }

    // Load persisted theme
    const savedTheme = localStorage.getItem('rdx_theme');
    if (savedTheme) {
        // Delay slightly to ensure DOM is ready if needed, 
        // but since this script is at the end of body, it's fine.
        setTheme(savedTheme);
    }
})();
