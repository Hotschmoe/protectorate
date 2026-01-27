let currentConnection = null;
let currentCloneJobId = null;
let clonePollingInterval = null;
let lastFetchAllTime = 0;
const FETCH_ALL_THROTTLE_MS = 60000;

const MSG_DATA = 0x30;
const MSG_RESIZE = 0x31;

function buildTerminalMessage(type, data) {
    const msg = new Uint8Array(data.length + 1);
    msg[0] = type;
    for (let i = 0; i < data.length; i++) {
        msg[i + 1] = data.charCodeAt(i);
    }
    return msg.buffer;
}

function switchTab(tabName) {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));

    const tabs = ['sleeves', 'needlecast', 'logs', 'config', 'workspaces', 'doctor'];
    const tabIndex = tabs.indexOf(tabName);
    if (tabIndex >= 0) {
        document.querySelectorAll('.tab')[tabIndex].classList.add('active');
        document.getElementById(`tab-${tabName}`).classList.add('active');
    }

    if (tabName === 'workspaces') {
        refreshWorkspacesTable();
        triggerFetchAllThrottled();
    } else if (tabName === 'doctor') {
        refreshDoctor();
    } else if (tabName === 'needlecast') {
        updateNeedlecastSleeveList();
    }
}

async function refreshDoctor() {
    const container = document.getElementById('doctor-checks');
    container.innerHTML = '<div class="empty-state"><span class="inline-spinner"></span> Running checks...</div>';

    try {
        const resp = await fetch('/api/doctor');
        const checks = await resp.json();

        if (checks.length === 0) {
            container.innerHTML = '<div class="empty-state">No checks available</div>';
            return;
        }

        container.innerHTML = checks.map(check => {
            const icon = check.status === 'pass' ? '[*]' : check.status === 'warning' ? '[!]' : '[x]';
            const suggestion = check.suggestion
                ? `<div class="doctor-check-suggestion">${escapeHtml(check.suggestion)}</div>`
                : '';
            return `
                <div class="doctor-check">
                    <div class="doctor-check-icon ${check.status}">${icon}</div>
                    <div class="doctor-check-content">
                        <div class="doctor-check-header">
                            <span class="doctor-check-name">${escapeHtml(check.name)}</span>
                            <span class="doctor-check-status ${check.status}">${check.status}</span>
                        </div>
                        <div class="doctor-check-message">${escapeHtml(check.message)}</div>
                        ${suggestion}
                    </div>
                </div>
            `;
        }).join('');
    } catch (e) {
        console.error('Failed to fetch doctor checks:', e);
        container.innerHTML = '<div class="empty-state">Failed to load checks</div>';
    }
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

async function triggerFetchAllThrottled() {
    const now = Date.now();
    if (now - lastFetchAllTime < FETCH_ALL_THROTTLE_MS) {
        return;
    }
    lastFetchAllTime = now;

    const indicator = document.getElementById('fetch-indicator');
    indicator.style.display = 'inline';
    const startTime = Date.now();
    const minDisplayMs = 1500;

    try {
        await fetch('/api/workspaces/branches?action=fetch-all', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
        });
        refreshWorkspacesTable();
    } catch (e) {
        console.error('Fetch all failed:', e);
    } finally {
        const elapsed = Date.now() - startTime;
        const remaining = minDisplayMs - elapsed;
        if (remaining > 0) {
            setTimeout(() => { indicator.style.display = 'none'; }, remaining);
        } else {
            indicator.style.display = 'none';
        }
    }
}

class TerminalConnection {
    constructor(container, wsPath, title, readOnly = false) {
        this.wsPath = wsPath;
        this.title = title;
        this.container = container;
        this.readOnly = readOnly;
        this.term = null;
        this.ws = null;
        this.fitAddon = null;
        this.webLinksAddon = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 10;
        this.baseDelay = 1000;
        this.maxDelay = 30000;
        this.shouldReconnect = true;
        this.resizeTimeout = null;
        this.resizeHandler = null;
    }

    initTerminal() {
        this.container.innerHTML = '';
        this.term = new Terminal({
            cursorBlink: true,
            scrollback: 10000,
            altScrollPassthrough: true,
            fontFamily: '"JetBrains Mono", "Fira Code", "SF Mono", Menlo, Monaco, "Courier New", monospace',
            fontSize: 14,
            fontWeight: 400,
            fontWeightBold: 600,
            letterSpacing: 0,
            lineHeight: 1.2,
            theme: {
                background: '#0a0a0f',
                foreground: '#e0e0e0',
                cursor: '#00f5ff',
                cursorAccent: '#0a0a0f',
                selectionBackground: '#004d4f',
                selectionForeground: '#ffffff',
                selectionInactiveBackground: '#004d4f50',
                black: '#505060',
                red: '#ff0055',
                green: '#00f5ff',
                yellow: '#ffaa00',
                blue: '#00a5aa',
                magenta: '#ff0055',
                cyan: '#00f5ff',
                white: '#e0e0e0',
                brightBlack: '#808090',
                brightRed: '#ff3377',
                brightGreen: '#33ffff',
                brightYellow: '#ffcc33',
                brightBlue: '#33cccc',
                brightMagenta: '#ff3377',
                brightCyan: '#33ffff',
                brightWhite: '#ffffff'
            }
        });
        this.fitAddon = new FitAddon.FitAddon();
        this.term.loadAddon(this.fitAddon);
        this.webLinksAddon = new WebLinksAddon.WebLinksAddon();
        this.term.loadAddon(this.webLinksAddon);
        this.term.open(this.container);
        this.fitAddon.fit();

        this.term.element.addEventListener('wheel', (e) => {
            const buffer = this.term.buffer.active;
            if (buffer.type === 'alternate') {
                return;
            }
            const atTop = buffer.viewportY === 0;
            const atBottom = buffer.viewportY >= buffer.baseY;
            const scrollingUp = e.deltaY < 0;
            const scrollingDown = e.deltaY > 0;

            if ((scrollingDown && atBottom) || (scrollingUp && atTop)) {
                e.stopPropagation();
                e.preventDefault();
            }
        }, { capture: true, passive: false });

        if (!this.readOnly) {
            this.term.onData((data) => {
                if (this.ws && this.ws.readyState === WebSocket.OPEN) {
                    this.ws.send(buildTerminalMessage(MSG_DATA, data));
                }
            });
        }

        this.resizeHandler = () => {
            this.fitAddon.fit();
            this.sendResize();
        };
        window.addEventListener('resize', this.resizeHandler);
    }

    connect() {
        if (!this.term) {
            this.initTerminal();
        }

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        this.ws = new WebSocket(`${protocol}//${window.location.host}${this.wsPath}`);
        this.ws.binaryType = 'arraybuffer';

        this.updateStatus('connecting');

        this.ws.onopen = () => {
            this.reconnectAttempts = 0;
            this.updateStatus('connected');
            const initData = JSON.stringify({
                cols: this.term.cols,
                rows: this.term.rows
            });
            this.ws.send(initData);
        };

        this.ws.onmessage = (event) => {
            const data = new Uint8Array(event.data);
            if (data.length > 0 && data[0] === MSG_DATA) {
                this.term.write(data.slice(1));
            }
        };

        this.ws.onerror = () => {
            this.updateStatus('error');
        };

        this.ws.onclose = (event) => {
            this.updateStatus('disconnected');
            if (this.shouldReconnect && !event.wasClean) {
                this.scheduleReconnect();
            } else if (this.shouldReconnect) {
                this.term.write('\r\n[Connection closed]\r\n');
            }
        };
    }

    scheduleReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            this.term.write('\r\n[Max reconnection attempts reached]\r\n');
            this.updateReconnectInfo('Max attempts reached');
            return;
        }

        const delay = Math.min(
            this.baseDelay * Math.pow(2, this.reconnectAttempts),
            this.maxDelay
        );
        this.reconnectAttempts++;

        this.updateReconnectInfo(`Reconnecting in ${delay/1000}s (${this.reconnectAttempts}/${this.maxReconnectAttempts})`);

        setTimeout(() => {
            if (this.shouldReconnect) {
                this.updateReconnectInfo('');
                this.connect();
            }
        }, delay);
    }

    updateStatus(status) {
        const statusEl = document.getElementById('terminal-status');
        statusEl.className = 'terminal-status ' + status;
    }

    updateReconnectInfo(text) {
        const infoEl = document.getElementById('reconnect-info');
        infoEl.textContent = text;
    }

    sendResize() {
        clearTimeout(this.resizeTimeout);
        this.resizeTimeout = setTimeout(() => {
            if (this.ws && this.ws.readyState === WebSocket.OPEN) {
                const json = JSON.stringify({columns: this.term.cols, rows: this.term.rows});
                this.ws.send(buildTerminalMessage(MSG_RESIZE, json));
            }
        }, 100);
    }

    dispose() {
        this.shouldReconnect = false;
        clearTimeout(this.resizeTimeout);
        if (this.resizeHandler) {
            window.removeEventListener('resize', this.resizeHandler);
            this.resizeHandler = null;
        }
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        if (this.term) {
            this.term.dispose();
            this.term = null;
        }
    }
}

async function checkAuth() {
    try {
        const resp = await fetch('/api/auth/status');
        const data = await resp.json();
        const badge = document.getElementById('auth-badge');
        if (data.authenticated) {
            badge.classList.remove('unauthenticated');
            badge.classList.add('authenticated');
            badge.textContent = 'ADMIN';
        } else {
            badge.classList.remove('authenticated');
            badge.classList.add('unauthenticated');
            badge.textContent = 'AUTH';
        }
    } catch (e) {
        console.error('Failed to check auth:', e);
    }
}

let sleevesCache = [];

async function refreshSleeves() {
    try {
        const resp = await fetch('/api/sleeves');
        const sleeves = await resp.json();
        sleevesCache = sleeves;
        const grid = document.getElementById('sleeves-grid');
        const statActive = document.getElementById('stat-active');

        statActive.textContent = sleeves.length;

        if (sleeves.length === 0) {
            grid.innerHTML = '<div class="empty-state">No sleeves running. Click "+ SPAWN SLEEVE" to create one.</div>';
            return;
        }

        grid.innerHTML = sleeves.map(s => `
            <div class="sleeve-card healthy">
                <div class="sleeve-header">
                    <span class="sleeve-name">SLEEVE: ${escapeHtml(s.name)}</span>
                    <span class="sleeve-status active">ACTIVE</span>
                </div>
                <div class="sleeve-body">
                    <div class="sleeve-row">
                        <span class="sleeve-label">DHF</span>
                        <span class="sleeve-value">Claude Code</span>
                    </div>
                    <div class="sleeve-row">
                        <span class="sleeve-label">Workspace</span>
                        <span class="sleeve-value">${escapeHtml(s.workspace)}</span>
                    </div>
                    <div class="sleeve-row">
                        <span class="sleeve-label">Container</span>
                        <span class="sleeve-value" style="font-size: var(--text-sm); color: var(--text-secondary);">${escapeHtml(s.container_id)}</span>
                    </div>
                </div>
                <div class="sleeve-actions">
                    <button class="btn" onclick="openTerminal('${escapeHtml(s.name)}')">TERMINAL</button>
                    <button class="btn" onclick="openTerminalObserve('${escapeHtml(s.name)}')">OBSERVE</button>
                    <button class="btn btn-danger" onclick="killSleeve('${escapeHtml(s.name)}')">KILL</button>
                </div>
            </div>
        `).join('');

        updateNeedlecastSleeveList();
    } catch (e) {
        console.error('Failed to fetch sleeves:', e);
    }
}

function updateNeedlecastSleeveList() {
    const list = document.getElementById('needlecast-sleeve-list');
    if (!list) return;
    list.innerHTML = sleevesCache.map(s =>
        `<div class="needlecast-sleeve-item">${escapeHtml(s.name)}</div>`
    ).join('');
}

let workspacesCache = [];

async function refreshWorkspaces() {
    try {
        const resp = await fetch('/api/workspaces');
        workspacesCache = await resp.json();
        updateWorkspaceSelect();
    } catch (e) {
        console.error('Failed to fetch workspaces:', e);
    }
}

async function refreshWorkspacesTable() {
    try {
        const resp = await fetch('/api/workspaces');
        const workspaces = await resp.json();
        const tbody = document.querySelector('#workspaces-table tbody');
        const emptyState = document.getElementById('workspaces-empty');
        const table = document.getElementById('workspaces-table');

        if (workspaces.length === 0) {
            table.classList.add('hidden');
            emptyState.classList.remove('hidden');
            return;
        }

        table.classList.remove('hidden');
        emptyState.classList.add('hidden');

        tbody.innerHTML = workspaces.map(ws => {
            const branch = formatGitBranch(ws.git);
            const gitStatus = formatGitStatus(ws.git, ws.in_use);
            const cstackStatus = formatCstackStatus(ws.cstack, ws);
            const lastCommit = formatLastCommit(ws.git);
            const sleeve = ws.sleeve_name || '-';
            const actions = formatWorkspaceActions(ws);
            return `
            <tr>
                <td>${ws.name}</td>
                <td>${branch}</td>
                <td>${gitStatus}</td>
                <td>${cstackStatus}</td>
                <td>${lastCommit}</td>
                <td>${sleeve}</td>
                <td>${actions}</td>
            </tr>
            `;
        }).join('');
    } catch (e) {
        console.error('Failed to fetch workspaces:', e);
    }
}

function formatGitBranch(git) {
    if (!git) return '<span class="git-no-repo">-</span>';
    const branchClass = git.is_detached ? 'git-detached' : 'git-branch';
    const prefix = git.is_detached ? 'detached@' : '';
    return `<span class="${branchClass}">${prefix}${git.branch}</span>`;
}

function formatGitStatus(git, inUse) {
    if (!git) return '-';
    const parts = [];

    if (git.is_dirty) {
        parts.push(`<span class="git-dirty">${git.uncommitted_count} uncommitted</span>`);
    } else {
        parts.push('<span class="git-clean">Clean</span>');
    }

    if (git.ahead_count > 0 || git.behind_count > 0) {
        const syncParts = [];
        if (git.ahead_count > 0) syncParts.push(`${git.ahead_count} ahead`);
        if (git.behind_count > 0) syncParts.push(`${git.behind_count} behind`);
        const syncClass = git.behind_count > 0 ? 'git-behind' : 'git-ahead';
        parts.push(`<span class="${syncClass}">${syncParts.join(', ')}</span>`);
    }

    return parts.join(' / ');
}

function formatLastCommit(git) {
    if (!git || !git.last_commit_hash) return '-';
    const msg = git.last_commit_msg.length > 30
        ? git.last_commit_msg.substring(0, 30) + '...'
        : git.last_commit_msg;
    return `<span class="git-commit-hash">${git.last_commit_hash}</span> ${msg} <span class="git-commit-time">(${git.last_commit_time})</span>`;
}

function formatWorkspaceActions(ws) {
    const isGitRepo = !!ws.git;
    const canSwitch = isGitRepo && !ws.in_use && !ws.git.is_dirty;
    const canPull = isGitRepo && !ws.in_use && !ws.git.is_dirty && ws.git.ahead_count === 0 && ws.git.behind_count > 0;
    const canCommit = isGitRepo && !ws.in_use && ws.git.is_dirty;
    const canPush = isGitRepo && !ws.in_use && ws.git.ahead_count > 0;

    const switchDisabled = !canSwitch ? 'disabled' : '';
    const fetchDisabled = !isGitRepo ? 'disabled' : '';
    const pullDisabled = !canPull ? 'disabled' : '';
    const commitDisabled = !canCommit ? 'disabled' : '';
    const pushDisabled = !canPush ? 'disabled' : '';

    let switchTitle = 'Switch branch';
    if (!isGitRepo) {
        switchTitle = 'Not a git repository';
    } else if (ws.in_use) {
        switchTitle = 'Workspace in use by ' + ws.sleeve_name;
    } else if (ws.git.is_dirty) {
        switchTitle = 'Has uncommitted changes';
    }

    let pullTitle = 'Pull from remote';
    if (!isGitRepo) {
        pullTitle = 'Not a git repository';
    } else if (ws.in_use) {
        pullTitle = 'Workspace in use by ' + ws.sleeve_name;
    } else if (ws.git.is_dirty) {
        pullTitle = 'Has uncommitted changes';
    } else if (ws.git.ahead_count > 0) {
        pullTitle = 'Has local commits - push first';
    } else if (ws.git.behind_count === 0) {
        pullTitle = 'Already up to date';
    }

    let commitTitle = 'Commit all changes';
    if (!isGitRepo) {
        commitTitle = 'Not a git repository';
    } else if (ws.in_use) {
        commitTitle = 'Workspace in use by ' + ws.sleeve_name;
    } else if (!ws.git.is_dirty) {
        commitTitle = 'No uncommitted changes';
    }

    let pushTitle = 'Push to remote';
    if (!isGitRepo) {
        pushTitle = 'Not a git repository';
    } else if (ws.in_use) {
        pushTitle = 'Workspace in use by ' + ws.sleeve_name;
    } else if (ws.git.ahead_count === 0) {
        pushTitle = 'No commits to push';
    }

    const escapedPath = ws.path.replace(/'/g, "\\'");
    const escapedName = ws.name.replace(/'/g, "\\'");

    return `<div class="action-buttons">
        <button class="btn btn-small" ${commitDisabled} title="${commitTitle}"
                onclick="commitChanges('${escapedPath}', this)">Commit</button>
        <button class="btn btn-small" ${pushDisabled} title="${pushTitle}"
                onclick="pushChanges('${escapedPath}', this)">Push</button>
        <button class="btn btn-small" ${switchDisabled} title="${switchTitle}"
                onclick="showSwitchModal('${escapedPath}', '${escapedName}')">Switch</button>
        <button class="btn btn-small" ${fetchDisabled} title="${isGitRepo ? 'Fetch from remote' : 'Not a git repository'}"
                onclick="fetchRemote('${escapedPath}', this)">Fetch</button>
        <button class="btn btn-small" ${pullDisabled} title="${pullTitle}"
                onclick="pullRemote('${escapedPath}', this)">Pull</button>
        <button class="btn btn-small" disabled title="Open in IDE (coming soon)"
                onclick="viewWorkspace('${escapedPath}')">View</button>
    </div>`;
}

let initializingCstackPaths = new Set();

function formatCstackStatus(cstack, ws) {
    if (initializingCstackPaths.has(ws.path)) {
        return `<span style="color:var(--text-secondary)"><span class="inline-spinner"></span> Initializing...</span>`;
    }

    if (!cstack || !cstack.exists) {
        const escapedPath = ws.path.replace(/'/g, "\\'");
        return `<button class="btn btn-small"
                onclick="showCstackInitModal('${escapedPath}', '${ws.name}')">
            Init Stack
        </button>`;
    }

    const parts = [];
    if (cstack.ready > 0) {
        parts.push(`<span style="color:var(--cyan-glow);font-weight:600">${cstack.ready} ready</span>`);
    }
    if (cstack.in_progress > 0) {
        parts.push(`<span style="color:var(--cyan-dim)">${cstack.in_progress} active</span>`);
    }
    if (cstack.blocked > 0) {
        parts.push(`<span style="color:var(--magenta-glow)">${cstack.blocked} blocked</span>`);
    }
    if (cstack.open > 0 && cstack.ready === 0 && cstack.in_progress === 0) {
        parts.push(`<span style="color:var(--amber-glow)">${cstack.open} open</span>`);
    }

    if (parts.length === 0) {
        return `<span style="color:var(--text-secondary)">No tasks</span>`;
    }

    return parts.join(' / ');
}

let currentSwitchWsPath = '';

async function showSwitchModal(wsPath, wsName) {
    currentSwitchWsPath = wsPath;
    document.getElementById('switch-ws-name').textContent = wsName;
    document.getElementById('switch-ws-path').value = wsPath;
    document.getElementById('switch-error').classList.add('hidden');
    document.getElementById('branch-select').innerHTML = '<option value="">Loading branches...</option>';
    document.getElementById('switch-branch-modal').classList.add('active');

    try {
        const resp = await fetch(`/api/workspaces/branches?workspace=${encodeURIComponent(wsPath)}`);
        if (!resp.ok) {
            const err = await resp.text();
            document.getElementById('switch-error').textContent = err;
            document.getElementById('switch-error').classList.remove('hidden');
            return;
        }

        const branches = await resp.json();
        const select = document.getElementById('branch-select');
        let options = '';

        if (branches.local && branches.local.length > 0) {
            options += '<optgroup label="Local Branches">';
            for (const b of branches.local) {
                const selected = b === branches.current ? 'selected' : '';
                const current = b === branches.current ? ' (current)' : '';
                options += `<option value="${b}" ${selected}>${b}${current}</option>`;
            }
            options += '</optgroup>';
        }

        if (branches.remote && branches.remote.length > 0) {
            options += '<optgroup label="Remote Branches">';
            for (const b of branches.remote) {
                options += `<option value="${b}">${b}</option>`;
            }
            options += '</optgroup>';
        }

        if (options === '') {
            options = '<option value="">No branches found</option>';
        }

        select.innerHTML = options;
    } catch (e) {
        document.getElementById('switch-error').textContent = e.message;
        document.getElementById('switch-error').classList.remove('hidden');
    }
}

function hideSwitchModal() {
    document.getElementById('switch-branch-modal').classList.remove('active');
    document.getElementById('switch-form').reset();
    document.getElementById('switch-error').classList.add('hidden');
    hideSwitchLoading();
    currentSwitchWsPath = '';
}

function showSwitchLoading(msg) {
    document.querySelector('.switch-loading-text').textContent = msg || 'Switching branch...';
    document.getElementById('switch-loading').classList.add('active');
    document.getElementById('switch-submit-btn').disabled = true;
}

function hideSwitchLoading() {
    document.getElementById('switch-loading').classList.remove('active');
    document.getElementById('switch-submit-btn').disabled = false;
}

async function switchBranch(e) {
    e.preventDefault();
    const wsPath = document.getElementById('switch-ws-path').value;
    const branch = document.getElementById('branch-select').value;

    if (!branch) {
        document.getElementById('switch-error').textContent = 'Please select a branch';
        document.getElementById('switch-error').classList.remove('hidden');
        return;
    }

    showSwitchLoading('Switching to ' + branch + '...');
    document.getElementById('switch-error').classList.add('hidden');

    try {
        const resp = await fetch(`/api/workspaces/branches?workspace=${encodeURIComponent(wsPath)}&action=switch`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ workspace: wsPath, branch: branch })
        });

        if (!resp.ok) {
            const err = await resp.text();
            document.getElementById('switch-error').textContent = err;
            document.getElementById('switch-error').classList.remove('hidden');
            hideSwitchLoading();
            return;
        }

        hideSwitchLoading();
        hideSwitchModal();
        refreshWorkspacesTable();
    } catch (e) {
        hideSwitchLoading();
        document.getElementById('switch-error').textContent = e.message;
        document.getElementById('switch-error').classList.remove('hidden');
    }
}

async function fetchRemote(wsPath, btn) {
    const originalText = btn.textContent;
    btn.textContent = 'Fetching...';
    btn.disabled = true;

    try {
        const resp = await fetch(`/api/workspaces/branches?workspace=${encodeURIComponent(wsPath)}&action=fetch`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ workspace: wsPath })
        });

        if (!resp.ok) {
            const err = await resp.text();
            alert('Fetch failed: ' + err);
            btn.textContent = originalText;
            btn.disabled = false;
            return;
        }

        btn.textContent = 'Done!';
        setTimeout(() => {
            btn.textContent = originalText;
            btn.disabled = false;
            refreshWorkspacesTable();
        }, 1000);
    } catch (e) {
        alert('Fetch failed: ' + e.message);
        btn.textContent = originalText;
        btn.disabled = false;
    }
}

async function pullRemote(wsPath, btn) {
    const originalText = btn.textContent;
    btn.textContent = 'Pulling...';
    btn.disabled = true;

    try {
        const resp = await fetch(`/api/workspaces/branches?workspace=${encodeURIComponent(wsPath)}&action=pull`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ workspace: wsPath })
        });

        if (!resp.ok) {
            const err = await resp.text();
            alert('Pull failed: ' + err);
            btn.textContent = originalText;
            btn.disabled = false;
            return;
        }

        const result = await resp.json();
        if (!result.success) {
            alert('Pull failed: ' + (result.message || 'unknown error'));
            btn.textContent = originalText;
            btn.disabled = false;
            return;
        }

        btn.textContent = 'Done!';
        setTimeout(() => {
            btn.textContent = originalText;
            btn.disabled = false;
            refreshWorkspacesTable();
        }, 1000);
    } catch (e) {
        alert('Pull failed: ' + e.message);
        btn.textContent = originalText;
        btn.disabled = false;
    }
}

async function commitChanges(wsPath, btn) {
    const originalText = btn.textContent;
    btn.textContent = 'Committing...';
    btn.disabled = true;

    try {
        const resp = await fetch(`/api/workspaces/branches?workspace=${encodeURIComponent(wsPath)}&action=commit`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
        });

        if (!resp.ok) {
            const err = await resp.text();
            alert('Commit failed: ' + err);
            btn.textContent = originalText;
            btn.disabled = false;
            return;
        }

        const result = await resp.json();
        if (!result.success) {
            alert('Commit failed: ' + (result.message || 'unknown error'));
            btn.textContent = originalText;
            btn.disabled = false;
            return;
        }

        btn.textContent = 'Done!';
        setTimeout(() => {
            btn.textContent = originalText;
            btn.disabled = false;
            refreshWorkspacesTable();
        }, 1000);
    } catch (e) {
        alert('Commit failed: ' + e.message);
        btn.textContent = originalText;
        btn.disabled = false;
    }
}

async function pushChanges(wsPath, btn) {
    const originalText = btn.textContent;
    btn.textContent = 'Pushing...';
    btn.disabled = true;

    try {
        const resp = await fetch(`/api/workspaces/branches?workspace=${encodeURIComponent(wsPath)}&action=push`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
        });

        if (!resp.ok) {
            const err = await resp.text();
            alert('Push failed: ' + err);
            btn.textContent = originalText;
            btn.disabled = false;
            return;
        }

        const result = await resp.json();
        if (!result.success) {
            alert('Push failed: ' + (result.message || 'unknown error'));
            btn.textContent = originalText;
            btn.disabled = false;
            return;
        }

        btn.textContent = 'Done!';
        setTimeout(() => {
            btn.textContent = originalText;
            btn.disabled = false;
            refreshWorkspacesTable();
        }, 1000);
    } catch (e) {
        alert('Push failed: ' + e.message);
        btn.textContent = originalText;
        btn.disabled = false;
    }
}

function viewWorkspace(wsPath) {
    alert('View workspace coming soon. Options: LazyGit, VS Code (code-server), or web file browser.');
}

function showCstackInitModal(wsPath, wsName) {
    document.getElementById('cstack-init-ws-name').textContent = wsName;
    document.getElementById('cstack-init-ws-path').value = wsPath;
    document.getElementById('cstack-init-error').classList.add('hidden');
    document.getElementById('cstack-init-modal').classList.add('active');
}

function hideCstackInitModal() {
    document.getElementById('cstack-init-modal').classList.remove('active');
    document.getElementById('cstack-init-form').reset();
    document.getElementById('cstack-init-error').classList.add('hidden');
}

async function initCstack(e) {
    e.preventDefault();
    const wsPath = document.getElementById('cstack-init-ws-path').value;
    const mode = document.querySelector('input[name="cstack-mode"]:checked').value;

    hideCstackInitModal();
    initializingCstackPaths.add(wsPath);
    await refreshWorkspacesTable();

    try {
        const resp = await fetch(
            `/api/workspaces/cstack?workspace=${encodeURIComponent(wsPath)}&action=init`,
            {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ mode: mode })
            }
        );

        const result = await resp.json();
        initializingCstackPaths.delete(wsPath);

        if (!result.success && result.error && !result.error.includes('already')) {
            alert('Cstack init failed: ' + result.error);
        }

        await refreshWorkspacesTable();

    } catch (err) {
        initializingCstackPaths.delete(wsPath);
        alert('Cstack init failed: ' + err.message);
        await refreshWorkspacesTable();
    }
}

function updateWorkspaceSelect() {
    const select = document.getElementById('existing-workspace-select');
    const available = workspacesCache.filter(ws => !ws.in_use);
    if (available.length === 0) {
        select.innerHTML = '<option value="">No available workspaces</option>';
        return;
    }
    select.innerHTML = available.map(ws =>
        `<option value="${ws.path}">${ws.name}</option>`
    ).join('');
}

function toggleWorkspaceMode() {
    const mode = document.querySelector('input[name="ws-mode"]:checked').value;
    const newGroup = document.getElementById('new-ws-group');
    const existingGroup = document.getElementById('existing-ws-group');
    const newInput = document.getElementById('new-workspace-input');
    const existingSelect = document.getElementById('existing-workspace-select');

    newGroup.classList.add('hidden');
    existingGroup.classList.add('hidden');
    newInput.required = false;
    existingSelect.required = false;

    if (mode === 'new') {
        newGroup.classList.remove('hidden');
        newInput.required = true;
    } else if (mode === 'existing') {
        existingGroup.classList.remove('hidden');
        existingSelect.required = true;
    }
}

async function showSpawnModal() {
    await refreshWorkspaces();
    toggleWorkspaceMode();
    document.getElementById('spawn-modal').classList.add('active');
}

function hideSpawnModal() {
    document.getElementById('spawn-modal').classList.remove('active');
    document.getElementById('spawn-form').reset();
    hideSpawnLoading();
    toggleWorkspaceMode();
}

function showSpawnLoading(msg) {
    document.querySelector('.spawn-loading-text').textContent = msg || 'Spawning sleeve...';
    document.getElementById('spawn-loading').classList.add('active');
}

function hideSpawnLoading() {
    document.getElementById('spawn-loading').classList.remove('active');
}

function showCloneModal() {
    document.getElementById('clone-modal').classList.add('active');
    document.getElementById('clone-error').classList.add('hidden');
}

function hideCloneModal() {
    document.getElementById('clone-modal').classList.remove('active');
    document.getElementById('clone-form').reset();
    document.getElementById('clone-error').classList.add('hidden');
    hideCloneLoading();
    stopClonePolling();
}

function showCloneLoading(msg) {
    document.querySelector('.clone-loading-text').textContent = msg || 'Cloning repository...';
    document.getElementById('clone-loading').classList.add('active');
    document.getElementById('clone-submit-btn').disabled = true;
}

function hideCloneLoading() {
    document.getElementById('clone-loading').classList.remove('active');
    document.getElementById('clone-submit-btn').disabled = false;
}

function stopClonePolling() {
    if (clonePollingInterval) {
        clearInterval(clonePollingInterval);
        clonePollingInterval = null;
    }
    currentCloneJobId = null;
}

async function cloneWorkspace(e) {
    e.preventDefault();
    const repoUrl = document.getElementById('clone-url-input').value;
    const name = document.getElementById('clone-name-input').value;

    if (!repoUrl) {
        alert('Please enter a repository URL');
        return;
    }

    showCloneLoading('Starting clone...');
    document.getElementById('clone-error').classList.add('hidden');

    try {
        const resp = await fetch('/api/workspaces/clone', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ repo_url: repoUrl, name: name || undefined })
        });

        if (!resp.ok) {
            const err = await resp.text();
            document.getElementById('clone-error').textContent = err;
            document.getElementById('clone-error').classList.remove('hidden');
            hideCloneLoading();
            return;
        }

        const job = await resp.json();
        currentCloneJobId = job.id;
        showCloneLoading('Cloning repository...');

        clonePollingInterval = setInterval(async () => {
            try {
                const statusResp = await fetch(`/api/workspaces/clone?id=${currentCloneJobId}`);
                const jobStatus = await statusResp.json();

                if (jobStatus.status === 'completed') {
                    stopClonePolling();
                    hideCloneLoading();
                    hideCloneModal();
                    refreshWorkspacesTable();
                    refreshWorkspaces();
                } else if (jobStatus.status === 'failed') {
                    stopClonePolling();
                    hideCloneLoading();
                    document.getElementById('clone-error').textContent = jobStatus.error || 'Clone failed';
                    document.getElementById('clone-error').classList.remove('hidden');
                }
            } catch (pollErr) {
                console.error('Failed to poll clone status:', pollErr);
            }
        }, 1000);

    } catch (e) {
        hideCloneLoading();
        document.getElementById('clone-error').textContent = e.message;
        document.getElementById('clone-error').classList.remove('hidden');
    }
}

async function spawnSleeve(e) {
    e.preventDefault();
    const mode = document.querySelector('input[name="ws-mode"]:checked').value;
    const name = document.getElementById('name-input').value;
    let workspace;

    try {
        if (mode === 'new') {
            const wsName = document.getElementById('new-workspace-input').value;
            if (!wsName) {
                alert('Please enter a workspace name');
                return;
            }
            showSpawnLoading('Creating workspace...');
            const createResp = await fetch('/api/workspaces', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name: wsName })
            });
            if (!createResp.ok) {
                hideSpawnLoading();
                const err = await createResp.text();
                alert('Failed to create workspace: ' + err);
                return;
            }
            const wsData = await createResp.json();
            workspace = wsData.path;
        } else if (mode === 'existing') {
            workspace = document.getElementById('existing-workspace-select').value;
            if (!workspace) {
                alert('Please select a workspace');
                return;
            }
        }

        showSpawnLoading('Spawning sleeve...');
        const body = { workspace, name: name || undefined };
        const resp = await fetch('/api/sleeves', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        });

        if (!resp.ok) {
            hideSpawnLoading();
            const err = await resp.text();
            alert('Failed to spawn sleeve: ' + err);
            return;
        }

        hideSpawnLoading();
        hideSpawnModal();
        refreshSleeves();
    } catch (e) {
        hideSpawnLoading();
        console.error('Failed to spawn sleeve:', e);
        alert('Failed to spawn sleeve: ' + e.message);
    }
}

async function killSleeve(name) {
    if (!confirm(`Are you sure you want to kill sleeve "${name}"?`)) {
        return;
    }

    try {
        const resp = await fetch(`/api/sleeves/${name}`, { method: 'DELETE' });
        if (!resp.ok) {
            const err = await resp.text();
            alert('Failed to kill sleeve: ' + err);
            return;
        }
        refreshSleeves();
    } catch (e) {
        console.error('Failed to kill sleeve:', e);
    }
}

function openTerminalWithPath(title, wsPath, readOnly = false) {
    document.getElementById('terminal-title').textContent = title;
    document.getElementById('terminal-modal').classList.add('active');
    document.getElementById('reconnect-info').textContent = '';

    const readOnlyBadge = document.getElementById('terminal-readonly-badge');
    if (readOnly) {
        readOnlyBadge.classList.add('active');
    } else {
        readOnlyBadge.classList.remove('active');
    }

    const container = document.getElementById('terminal-container');

    if (currentConnection) {
        currentConnection.dispose();
    }

    currentConnection = new TerminalConnection(container, wsPath, title, readOnly);
    currentConnection.connect();
}

function openTerminal(name) {
    openTerminalWithPath(`Terminal - ${name}`, `/sleeves/${name}/terminal`);
}

function openTerminalObserve(name) {
    openTerminalWithPath(`Terminal - ${name} (Observe)`, `/sleeves/${name}/terminal?mode=observe`, true);
}

function openEnvoyTerminal() {
    openTerminalWithPath('Terminal - Envoy (Poe)', '/envoy/terminal');
}

function hideTerminalModal() {
    document.getElementById('terminal-modal').classList.remove('active');
    if (currentConnection) {
        currentConnection.dispose();
        currentConnection = null;
    }
}

// Initialize on page load
checkAuth();
refreshSleeves();

setInterval(() => {
    refreshSleeves();
}, 5000);
