package main

const htmlContent = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>PhileasGo</title>
    <style>
        body { margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: #0f0f0f; color: #eee; height: 100vh; display: flex; flex-direction: column; overflow: hidden; }
        
        .tabs { display: flex; background: #1a1a1a; border-bottom: 1px solid #333; height: 40px; align-items: flex-end; padding-left: 8px; flex-shrink: 0; }
        .tab { 
            padding: 8px 16px; 
            cursor: pointer; 
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
            background: #0f0f0f; 
            color: #fff; 
            border-color: #333;
            border-bottom-color: #0f0f0f;
            margin-bottom: -1px;
            z-index: 10;
        }
        .tab:hover:not(.active) { background: #222; }
        .tab.disabled { pointer-events: none; opacity: 0.5; }

        .content { flex: 1; position: relative; overflow: hidden; min-height: 0; }
        .tab-content { display: none; position: absolute; top: 0; left: 0; right: 0; bottom: 0; }
        .tab-content.active { display: block; }

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
    </style>
</head>
<body>
    <div class="tabs">
        <div class="tab" id="tab-app" onclick="switchTab('app')">APP</div>
        <div class="tab active" id="tab-term" onclick="switchTab('term')">TERMINAL</div>
        <div class="tab" id="tab-config" onclick="switchTab('config')">CONFIG</div>
    </div>

    <div class="content">
        <!-- App Tab (Main Map) -->
        <div id="content-app" class="tab-content">
            <iframe id="frame-app"></iframe>
            <div id="error-app" class="error-overlay">
                <p>Failed to load. Check your connection and try again.</p>
                <button class="retry-btn" onclick="retryApp()">Retry</button>
            </div>
        </div>

        <!-- Terminal Tab -->
        <div id="content-term" class="tab-content active">
            <div id="terminal-output" class="terminal-container"></div>
        </div>

        <!-- Config Tab (Settings) -->
        <div id="content-config" class="tab-content">
            <iframe id="frame-config"></iframe>
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
            // We use the hash-aware search for settings and the top-level search for the main app
            const mainUrl = url + "/?gui=true";
            const settingsUrl = url + "/#/settings?gui=true";
            
            appUrl = mainUrl;
            frameApp.src = mainUrl;
            document.getElementById('frame-config').src = settingsUrl;
            
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
