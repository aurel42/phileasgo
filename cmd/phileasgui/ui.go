package main

const htmlContent = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>PhileasGo</title>
    <style>
        :root {
            /* Palette */
            --bg-color: #0f0f0f;
            --text-color: #f4ecd8;
            --accent: #b8860b; /* Brass */
            --muted: #888;
            --panel-bg: #1a1a1a;
            
            /* Fonts */
            --font-main: 'Crimson Pro', serif;
            --font-display: 'IM Fell English SC', serif;
            --font-mono: 'Cutive Mono', monospace;
        }

        /* Role Classes */
        .role-title { font-family: var(--font-display); font-size: 24px; color: var(--accent); text-transform: uppercase; letter-spacing: 2px; }
        .role-header { font-family: var(--font-display); font-size: 14px; color: var(--accent); text-transform: uppercase; }
        .role-label { font-family: var(--font-main); font-style: italic; font-size: 15px; color: var(--muted); }
        .role-value { font-family: var(--font-mono); font-size: 13px; color: var(--accent); }
        .role-btn { font-family: var(--font-display); font-size: 14px; text-transform: uppercase; letter-spacing: 1px; }

        body { margin: 0; padding: 0; font-family: var(--font-main); background: var(--bg-color); color: var(--text-color); height: 100vh; display: flex; flex-direction: column; overflow: hidden; }
        
        .tabs { display: flex; background: #1a1a1a; border-bottom: 1px solid #333; height: 40px; align-items: flex-end; padding-left: 8px; flex-shrink: 0; position: relative; }
        .tab { 
            padding: 8px 16px; 
            cursor: pointer; 
            font-family: var(--font-display);
            font-size: 13px; 
            color: #888; 
            background: #1a1a1a;
            border-top-left-radius: 6px;
            border-top-right-radius: 6px;
            margin-right: 2px;
            border: 1px solid transparent;
            border-bottom: none;
            transition: all 0.2s;
            user-select: none;
        }
        .tab.active { 
            background: var(--bg-color); 
            color: #fff; 
            border-color: #333;
            border-bottom-color: var(--bg-color);
            margin-bottom: -1px;
            z-index: 10;
        }
        .tab:hover:not(.active) { background: #222; }
        .tab.disabled { pointer-events: none; opacity: 0.5; }

        .btn-settings {
            position: absolute;
            right: 8px;
            bottom: 6px;
            padding: 6px 12px;
            font-family: var(--font-display);
            font-size: 12px;
            color: var(--muted);
            cursor: pointer;
            border: 1px solid #333;
            border-radius: 4px;
            background: #222;
            transition: all 0.2s;
        }
        .btn-settings:hover { border-color: var(--accent); color: var(--accent); }

        .content { flex: 1; position: relative; overflow: hidden; min-height: 0; }
        .tab-content { display: none; position: absolute; top: 0; left: 0; right: 0; bottom: 0; }
        .tab-content.active { display: block; }

        .config-overlay {
            position: absolute;
            top: 0; left: 0; right: 0; bottom: 0;
            background: var(--bg-color);
            z-index: 100;
            display: none;
            flex-direction: column;
        }
        .config-overlay.visible { display: flex; }

        .terminal-container { 
            background: #060606; 
            color: #ccc; 
            font-family: 'Consolas', 'Monaco', 'Courier New', monospace; 
            font-size: 12px; 
            padding: 12px; 
            overflow-y: auto; 
            white-space: pre-wrap; 
            word-wrap: break-word;
            height: 100%;
            width: 100%;
            box-sizing: border-box;
        }
        
        iframe { width: 100%; height: 100%; border: none; background: #0f0f0f; }
        
        .error-overlay {
            display: none;
            position: absolute;
            top: 0; left: 0; right: 0; bottom: 0;
            background: #f5f5f5;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            gap: 16px;
            z-index: 100;
        }
        .error-overlay.visible { display: flex; }
        .error-overlay p { color: #666; font-size: 14px; margin: 0; }
        .retry-btn {
            padding: 10px 24px;
            font-size: 14px;
            background: #2196f3;
            color: white;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            transition: background 0.2s;
        }
        .retry-btn:hover { background: #1976d2; }

        #terminal-output span.info { color: #4caf50; }
        #terminal-output span.warn { color: #ff9800; }
        #terminal-output span.err { color: #f44336; }
        #terminal-output span.sys { color: #2196f3; font-weight: bold; }

        /* VICTORIAN CONFIG DIALOG STYLES */
        .config-container {
            display: flex;
            flex-direction: column;
            height: 100%;
            background: var(--bg-color);
            color: var(--text-color);
            padding: 0;
            user-select: none;
        }

        .config-header {
            padding: 20px 24px;
            border-bottom: 2px solid var(--accent);
            background: var(--panel-bg);
        }

        .config-header h1 {
            margin: 0;
        }

        .config-body {
            flex: 1;
            display: flex;
            overflow: hidden;
        }

        .config-sidebar {
            width: 180px;
            background: #151515;
            border-right: 1px solid #333;
            display: flex;
            flex-direction: column;
        }

        .config-nav-tab {
            padding: 12px 20px;
            cursor: pointer;
            font-family: var(--font-main);
            font-size: 16px;
            color: var(--muted);
            border-bottom: 1px solid #222;
            transition: all 0.2s;
        }

        .config-nav-tab:hover { background: #1a1a1a; color: #ccc; }
        .config-nav-tab.active {
            background: #222;
            color: var(--accent);
            border-left: 4px solid var(--accent);
            font-weight: bold;
        }

        .config-panel {
            flex: 1;
            padding: 24px;
            overflow-y: auto;
            display: none;
        }

        .config-panel.active { display: block; }

        .config-group {
            margin-bottom: 32px;
            border: 1px solid #333;
            padding: 20px;
            position: relative;
            background: #121212;
            box-shadow: inset 0 0 15px rgba(0,0,0,0.5);
        }

        .config-group-title {
            position: absolute;
            top: -10px;
            left: 15px;
            background: var(--bg-color);
            padding: 0 10px;
        }

        .config-field {
            margin-bottom: 24px;
            display: flex;
            flex-direction: column;
            gap: 12px;
            min-height: 60px; /* Prevent layout jump */
        }

        .config-label-row {
            display: flex;
            justify-content: space-between;
            align-items: baseline;
            margin-bottom: 4px;
        }

        .config-label-row .role-value {
            margin-left: 12px;
            min-width: 80px;
            text-align: right;
            font-weight: bold;
        }

        /* PRIMITIVES */
        .v-input, .v-select {
            background: #1a1a1a;
            border: 1px solid #444;
            color: var(--text-color);
            padding: 8px 12px;
            font-family: var(--font-main);
            font-size: 16px;
            outline: none;
            transition: border-color 0.2s;
        }

        .v-input:focus, .v-select:focus {
            border-color: var(--accent);
            box-shadow: 0 0 5px rgba(184, 134, 11, 0.3);
        }

        .v-slider {
            -webkit-appearance: none;
            width: 100%;
            height: 4px;
            background: #333;
            outline: none;
            margin: 10px 0;
        }

        .v-slider::-webkit-slider-thumb {
            -webkit-appearance: none;
            width: 16px;
            height: 16px;
            background: var(--accent);
            cursor: pointer;
            transform: rotate(45deg);
            border: 1px solid #1a1a1a;
        }

        .v-toggle {
            display: flex;
            align-items: center;
            gap: 12px;
            cursor: pointer;
        }

        .v-toggle input { display: none; }
        .v-toggle-track {
            width: 40px;
            height: 20px;
            background: #333;
            border-radius: 10px;
            position: relative;
            transition: background 0.3s;
        }

        .v-toggle input:checked + .v-toggle-track { background: var(--accent); }
        .v-toggle-thumb {
            width: 16px;
            height: 16px;
            background: var(--text-color);
            border-radius: 50%;
            position: absolute;
            top: 2px;
            left: 2px;
            transition: transform 0.3s;
        }
        .v-toggle input:checked + .v-toggle-track .v-toggle-thumb { transform: translateX(20px); }
        
        .role-value.status-enabled { color: var(--accent); }
        .role-value.status-disabled { color: var(--muted); }

        .config-footer {
            padding: 20px 24px;
            background: #1a1a1a;
            border-top: 1px solid #333;
            display: flex;
            justify-content: flex-end;
            gap: 12px;
        }

        .v-btn {
            padding: 10px 24px;
            font-family: 'IM Fell English SC', serif;
            font-size: 14px;
            text-transform: uppercase;
            letter-spacing: 1px;
            cursor: pointer;
            border: 1px solid #444;
            transition: all 0.2s;
        }

        .v-btn-primary {
            background: var(--accent);
            color: #0f0f0f;
            border-color: var(--accent);
        }

        .v-btn-primary:hover {
            background: #daa520;
        }

        .v-btn-secondary {
            background: transparent;
            color: var(--muted);
        }

        .v-btn-secondary:hover {
            border-color: var(--accent);
            color: var(--accent);
        }

        .config-dirty-hint {
            flex: 1;
            display: flex;
            align-items: center;
            font-family: 'Crimson Pro', serif;
            font-size: 14px;
            font-style: italic;
            color: #b8860b;
            opacity: 0;
            transition: opacity 0.3s;
        }

        .config-dirty-hint.visible { opacity: 1; }

        .config-loading {
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            display: flex;
            flex-direction: column;
            align-items: center;
            gap: 16px;
            color: #b8860b;
        }

        .config-loading-spinner {
            width: 48px;
            height: 48px;
            border: 4px solid #1a1a1a;
            border-top: 4px solid #b8860b;
            border-radius: 50%;
            animation: v-spin 2s linear infinite;
        }

        @keyframes v-spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }

        .config-loading-text {
            font-family: 'IM Fell English SC', serif;
            font-size: 18px;
            letter-spacing: 1px;
        }
    </style>
</head>
<body>
    <div class="tabs">
        <div class="tab" id="tab-app" onclick="switchTab('app')">APP</div>
        <div class="tab active" id="tab-term" onclick="switchTab('term')">TERMINAL</div>
        <div class="btn-settings" onclick="configMgr.toggle()">SETTINGS</div>
    </div>

    <div class="content">
        <!-- App Tab (Main Map) -->
        <div id="content-app" class="tab-content">
            <iframe id="frame-app"></iframe>
            <div id="error-app" class="error-overlay">
                <p>Failed to load. Check your connection and try again.</p>
                <button class="retry-btn" onclick="retryApp()">Retry</button>
            </div>

            <!-- Native Config Overlay -->
            <div id="config-overlay" class="config-overlay">
                <div class="config-container">
                    <div class="config-header">
                        <h1 class="role-title">Configuration</h1>
                    </div>
                    <div class="config-body">
                        <div class="config-sidebar" id="config-sidebar">
                            <!-- Tabs rendered here -->
                        </div>
                        <div class="config-panel-container" id="config-panels">
                            <!-- Panels rendered here -->
                        </div>
                    </div>
                    <div class="config-footer">
                        <div class="config-restart-legend" style="flex: 1; font-family: var(--font-main); font-size: 12px; font-style: italic; color: var(--muted); opacity: 0.8;">* needs a restart to apply</div>
                        <div class="config-dirty-hint" id="config-dirty-hint">◆ Pending changes detected...</div>
                        <button class="v-btn v-btn-secondary role-btn" onclick="configMgr.discard()">Discard</button>
                        <button class="v-btn v-btn-primary role-btn" onclick="configMgr.save()">Save</button>
                    </div>
                </div>
            </div>
        </div>

        <!-- Terminal Tab -->
        <div id="content-term" class="tab-content active">
            <div id="terminal-output" class="terminal-container"></div>
        </div>
    </div>

    <script>
        const output = document.getElementById('terminal-output');
        const tabTerm = document.getElementById('tab-term');
        let currentProcessName = "TERMINAL";
        let isSticky = true; // Default to auto-scrolling

        function switchTab(id) {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
            
            // Auto-hide config overlay if we switch away from app
            if (id !== 'app' && configMgr) {
                configMgr.close();
            }

            document.getElementById('tab-' + id).classList.add('active');
            document.getElementById('content-' + id).classList.add('active');

            // Restore scroll position if sticky
            if (id === 'term' && isSticky) {
                setTimeout(() => {
                    output.scrollTop = output.scrollHeight;
                }, 10);
            }
        }

        const MAX_LINES = 1000;

        // Update sticky state on user scroll
        output.addEventListener('scroll', () => {
            // Ignore scroll events when hidden (clientHeight == 0) to avoid state corruption
            if (output.clientHeight === 0) return;

            // Check distance from bottom
            const dist = output.scrollHeight - output.scrollTop - output.clientHeight;
            isSticky = (dist < 20);
        });

        function appendLog(text) {
            const line = document.createElement('div');
            // Basic highlighting
            if (text.includes('INFO')) line.innerHTML = '<span class="info">' + text + '</span>';
            else if (text.includes('WARN')) line.innerHTML = '<span class="warn">' + text + '</span>';
            else if (text.includes('ERROR') || text.includes('FAIL')) line.innerHTML = '<span class="err">' + text + '</span>';
            else if (text.startsWith('>')) line.innerHTML = '<span class="sys">' + text + '</span>';
            else line.innerText = text;

            output.appendChild(line);

            // Enforce Scrollback Buffer Limit
            while (output.childNodes.length > MAX_LINES) {
                output.removeChild(output.firstChild);
            }

            // Scroll to bottom if we are in sticky mode
            if (isSticky) {
                output.scrollTop = output.scrollHeight;
            }
        }

        window.setTerminalTitle = function(name) {
            currentProcessName = name;
            tabTerm.innerText = name.toUpperCase();
        };

        let appUrl = '';
        const frameApp = document.getElementById('frame-app');
        const errorApp = document.getElementById('error-app');

        function showAppError() {
            errorApp.classList.add('visible');
        }

        function hideAppError() {
            errorApp.classList.remove('visible');
        }

        window.retryApp = function() {
            if (appUrl) {
                hideAppError();
                frameApp.src = appUrl;
            }
        };

        // --- CONFIG MANAGER ---
        class ConfigManager {
            constructor() {
                this.state = {
                    filter_mode: 'fixed' // Default to show at least one slider
                };
                this.original = {};
                this.schema = [
                    {
                        id: 'sim',
                        label: 'Simulator',
                        groups: [
                            {
                                label: 'Source & Connection',
                                fields: [
                                    { key: 'sim_source', label: 'Simulator Provider', type: 'select', restart: true, options: [{v:'simconnect', t:'Microsoft Flight Simulator'}, {v:'mock', t:'Mock Simulator (Internal)'}] }
                                ]
                            }
                        ]
                    },
                    {
                        id: 'narrator',
                        label: 'Narrator',
                        groups: [
                            {
                                label: 'Voice & Style',
                                fields: [
                                    { key: 'text_length', label: 'Narration Verbosity', type: 'slider', min: 1, max: 5, step: 1, labels: {1:'shortest', 2:'shorter', 3:'normal', 4:'longer', 5:'longest'} },
                                    { key: 'narration_frequency', label: 'Narration Frequency', type: 'slider', min: 1, max: 4, step: 1, labels: {1:'Rarely', 2:'Normal', 3:'Active', 4:'Hyperactive'} }
                                ]
                            },
                             {
                                label: 'Filtering',
                                fields: [
                                    { key: 'filter_mode', label: 'Filtering Mode', type: 'select', options: [{v:'adaptive', t:'Adaptive Score'}, {v:'fixed', t:'Fixed Score'}] },
                                    { key: 'min_poi_score', label: 'Minimum POI Score', type: 'slider', min: -10.0, max: 10.0, step: 0.1, condition: (s) => s.filter_mode === 'fixed' },
                                    { key: 'target_poi_count', label: 'Target POI Quota', type: 'slider', min: 5, max: 50, step: 5, condition: (s) => s.filter_mode === 'adaptive' }
                                ]
                            }
                        ]
                    },
                    {
                        id: 'ui',
                        label: 'Interface',
                        groups: [
                            {
                                label: 'Map',
                                fields: [
                                    { key: 'units', label: 'Range Ring Units', type: 'select', options: [{v:'km', t:'Metric (km)'}, {v:'nm', t:'Nautical (nm)'}] },
                                    { key: 'show_cache_layer', label: 'Show Research Coverage', type: 'toggle' },
                                    { key: 'show_visibility_layer', label: 'Show Line-of-Sight Heatmap', type: 'toggle' }
                                ]
                            }
                        ]
                    }
                ];
                this.activeTab = 'sim';
                this.apiUrl = '';
                this.loading = false;
                this.visible = false;
            }

            toggle() {
                this.visible = !this.visible;
                document.getElementById('config-overlay').classList.toggle('visible', this.visible);
                if (this.visible) {
                    switchTab('app');
                    this.fetch();
                }
            }

            init(baseUrl) {
                this.apiUrl = baseUrl + '/api/config';
                this.fetch();
            }

            async fetch() {
                this.loading = true;
                this.render();
                try {
                    const resp = await fetch(this.apiUrl);
                    const data = await resp.json();
                    this.state = JSON.parse(JSON.stringify(data));
                    this.original = JSON.parse(JSON.stringify(data));
                    this.loading = false;
                    this.render();
                    this.checkDirty();
                } catch (e) {
                    console.error("Failed to fetch config", e);
                    this.loading = false;
                    this.render();
                }
            }

            render() {
                const sidebar = document.getElementById('config-sidebar');
                const panels = document.getElementById('config-panels');
                
                sidebar.innerHTML = '';
                panels.innerHTML = '';

                if (this.loading && Object.keys(this.state).length === 0) {
                    panels.innerHTML = '<div class="config-loading"><div class="config-loading-spinner"></div><div class="config-loading-text role-header">Consulting the Archives...</div></div>';
                }

                this.schema.forEach(tab => {
                    // Render Tab
                    const tabEl = document.createElement('div');
                    tabEl.className = 'config-nav-tab' + (this.activeTab === tab.id ? ' active' : '');
                    tabEl.innerText = tab.label;
                    tabEl.onclick = () => this.switchTab(tab.id);
                    sidebar.appendChild(tabEl);

                    // Render Panel (only if not loading, unless we have state)
                    if (this.loading && Object.keys(this.state).length === 0) return;

                    const panelEl = document.createElement('div');
                    panelEl.className = 'config-panel' + (this.activeTab === tab.id ? ' active' : '');
                    
                    tab.groups.forEach(group => {
                        const groupEl = document.createElement('div');
                        groupEl.className = 'config-group';
                        groupEl.innerHTML = '<div class="config-group-title role-header">' + group.label + '</div>';
                        
                        group.fields.forEach(field => {
                            const val = this.state[field.key];
                            if (field.condition && !field.condition(this.state)) return;

                            const fieldEl = document.createElement('div');
                            fieldEl.className = 'config-field';
                            
                            const labelRow = document.createElement('div');
                            labelRow.className = 'config-label-row';
                            
                            const labelEl = document.createElement('div');
                            labelEl.className = 'role-label';
                            labelEl.innerText = field.label + (field.restart ? ' *' : '');
                            
                            const valId = 'val-' + field.key;
                            const valueEl = document.createElement('div');
                            valueEl.className = 'role-value';
                            valueEl.id = valId;

                            labelRow.appendChild(labelEl);
                            labelRow.appendChild(valueEl);
                            
                            let input;
                            if (field.type === 'select') {
                                input = document.createElement('select');
                                input.className = 'v-select';
                                field.options.forEach(opt => {
                                    const o = document.createElement('option');
                                    o.value = opt.v;
                                    o.text = opt.t;
                                    o.selected = (val === opt.v);
                                    input.appendChild(o);
                                });
                                input.onchange = (e) => this.update(field.key, e.target.value);
                            } else if (field.type === 'slider') {
                                valueEl.innerText = (val !== undefined) ? (field.labels ? (field.labels[val] || val) : val) : '...';
                                
                                input = document.createElement('input');
                                input.type = 'range';
                                input.className = 'v-slider';
                                input.min = field.min;
                                input.max = field.max;
                                input.step = field.step;
                                input.value = val !== undefined ? val : field.min;
                                
                                input.oninput = (e) => {
                                    const v = parseFloat(e.target.value);
                                    this.state[field.key] = v;
                                    const display = field.labels ? (field.labels[v] || v) : v;
                                    document.getElementById(valId).innerText = display;
                                    this.checkDirty();
                                };
                                input.onchange = () => this.render();
                            } else if (field.type === 'toggle') {
                                const checked = val === true;
                                valueEl.innerText = (val !== undefined) ? (checked ? 'ENABLED' : 'DISABLED') : '...';
                                valueEl.className = 'role-value ' + (checked ? 'status-enabled' : 'status-disabled');
                                
                                input = document.createElement('label');
                                input.className = 'v-toggle';
                                input.innerHTML = 
                                    '<input type="checkbox" ' + (checked ? 'checked' : '') + '>' +
                                    '<div class="v-toggle-track"><div class="v-toggle-thumb"></div></div>';
                                input.querySelector('input').onchange = (e) => this.update(field.key, e.target.checked);
                            }

                            fieldEl.appendChild(labelRow);
                            if (input) fieldEl.appendChild(input);
                            groupEl.appendChild(fieldEl);
                        });
                        panelEl.appendChild(groupEl);
                    });
                    panels.appendChild(panelEl);
                });
            }

            update(key, val) {
                // Convert numeric strings if necessary
                if (!isNaN(val) && typeof val === 'string' && val.trim() !== '') {
                    val = parseFloat(val);
                }
                
                this.state[key] = val;
                this.render();
                this.checkDirty();
            }

            checkDirty() {
                const hint = document.getElementById('config-dirty-hint');
                let dirty = false;
                for (let k in this.state) {
                    if (this.state[k] !== this.original[k]) {
                        dirty = true;
                        break;
                    }
                }
                hint.classList.toggle('visible', dirty);
            }

            switchTab(id) {
                this.activeTab = id;
                this.render();
            }

            discard() {
                this.state = JSON.parse(JSON.stringify(this.original));
                this.render();
                this.checkDirty();
                this.hide();
            }

            close() { this.discard(); }

            hide() {
                this.visible = false;
                const overlay = document.getElementById('config-overlay');
                if (overlay) overlay.classList.remove('visible');
            }

            async save() {
                const diff = {};
                let hasChanges = false;
                for (let k in this.state) {
                    if (this.state[k] !== this.original[k]) {
                        diff[k] = this.state[k];
                        hasChanges = true;
                    }
                }

                if (!hasChanges) {
                    this.hide();
                    return;
                }

                const hint = document.getElementById('config-dirty-hint');
                if (hint) {
                    hint.innerText = "◆ Saving changes...";
                    hint.classList.add('visible');
                }

                try {
                    // Use POST as requested for backend compatibility
                    const resp = await fetch(this.apiUrl, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(diff)
                    });
                    if (resp.ok) {
                        const data = await resp.json();
                        this.original = JSON.parse(JSON.stringify(data));
                        this.state = JSON.parse(JSON.stringify(data));
                        this.checkDirty();
                        this.hide();
                    } else {
                        if (hint) hint.innerText = "◆ Save failed. Status: " + resp.status;
                    }
                } catch (e) {
                    console.error("Save failed", e);
                    if (hint) hint.innerText = "◆ Connection error.";
                }
            }
        }

        const configMgr = new ConfigManager();

        // Detect navigation errors in iframe
        frameApp.addEventListener('load', function() {
            try {
                // Try to access the iframe's content - if it loaded an error page,
                // the title often contains error-related text
                const iframeDoc = frameApp.contentDocument || frameApp.contentWindow.document;
                const title = iframeDoc.title.toLowerCase();
                const body = iframeDoc.body ? iframeDoc.body.innerText.toLowerCase() : '';
                
                if (title.includes('error') || body.includes('network change') || 
                    body.includes('cannot be reached') || body.includes('err_')) {
                    showAppError();
                } else {
                    hideAppError();
                }
            } catch (e) {
                // Cross-origin or other access error - page probably loaded fine
                hideAppError();
            }
        });

        window.enableApp = function(url) {
            // Append ?gui=true to trigger environment-aware UI logic
            const mainUrl = url + "/?gui=true";
            
            appUrl = mainUrl;
            frameApp.src = mainUrl;
            
            // Initialize native config manager
            configMgr.init(url);
            
            // Auto switch if currently viewing startup logs
            switchTab('app');
        };

        window.addLogLine = function(line) {
            appendLog(line);
        };

        // Disable Context Menu and Refresh Shortcuts
        document.addEventListener('contextmenu', event => event.preventDefault());
        document.addEventListener('keydown', function(event) {
            if (event.key === 'F5' || (event.ctrlKey && event.key === 'r')) {
                event.preventDefault();
            }
        });
    </script>
</body>
</html>
`
