// Commands page
var cmdEditor = null;
var presetCommands = [];
var activeBatch = null;     // { cmdIds: {}, tabs: {deviceKey: {el, status, cmdId}} } — one batch at a time
var activeTabKey = null;
var sessionTotal = 0;       // expected device count for progress
var sessionCompleted = 0;   // completed device count
var watchingCmdId = null;
var watchPollTimer = null;
var allDevices = [];        // cached for re-rendering device cards

function loadCommands() {
    if (watchPollTimer) { clearInterval(watchPollTimer); watchPollTimer = null; }
    watchingCmdId = null;
    activeBatch = null;
    activeTabKey = null;
    sessionTotal = 0;
    sessionCompleted = 0;

    $.getJSON("/api/devices", function(devices) {
        allDevices = devices;
        renderCommandPage(devices);
    }).fail(function() {
        $("#content").html('<div class="empty-state">无法加载设备列表</div>');
    });
}

function parseDeviceIP(localIP) {
    if (!localIP) return '';
    try {
        var ips = JSON.parse(localIP);
        if (Array.isArray(ips) && ips.length > 0) {
            for (var i = 0; i < ips.length; i++) {
                if (ips[i].defaultRoute && ips[i].ipv4) return ips[i].ipv4;
            }
            for (var j = 0; j < ips.length; j++) {
                if (ips[j].ipv4) return ips[j].ipv4;
            }
        }
    } catch(e) { return localIP; }
    return '';
}

function renderDeviceCards() {
    var html = '<div class="device-selector">';
    html += '<div class="device-card device-card-all' + (selectedTargets.length === 0 ? ' selected' : '') + '" onclick="toggleSelectAll()">';
    html += '<div class="device-card-check"></div>';
    html += '<div class="device-card-name">全部设备</div>';
    html += '<div class="device-card-meta">广播模式</div>';
    html += '</div>';
    allDevices.forEach(function(d) {
        var sel = selectedTargets.indexOf(d.assigned_id) >= 0;
        var ip = parseDeviceIP(d.local_ip);
        html += '<div class="device-card' + (sel ? ' selected' : '') + (d.connected ? '' : ' offline') + '" onclick="toggleSelectDevice(' + d.assigned_id + ')">';
        html += '<div class="device-card-check">' + (sel ? '✓' : '') + '</div>';
        html += '<div class="device-card-name">#' + d.assigned_id + ' ' + escapeHtml(d.hostname || d.username) + '</div>';
        html += '<div class="device-card-meta">';
        html += '<span class="device-status-dot ' + (d.connected ? 'online' : '') + '"></span>';
        html += (d.connected ? '在线' : '离线');
        if (ip) html += ' | ' + escapeHtml(ip);
        html += '</div>';
        html += '</div>';
    });
    html += '</div>';
    $("#device-selector-container").html(html);
}

function toggleSelectAll() {
    selectedTargets = [];
    renderDeviceCards();
}

function toggleSelectDevice(id) {
    var idx = selectedTargets.indexOf(id);
    if (idx >= 0) {
        selectedTargets.splice(idx, 1);
    } else {
        selectedTargets.push(id);
    }
    renderDeviceCards();
}

function renderCommandPage(devices) {
    allDevices = devices;
    var html = '' +
    '<h2 class="section-title">命令执行</h2>' +
    '<div class="command-layout">' +
        '<div class="command-panel">' +
            '<div class="panel-title">目标设备</div>' +
            '<div id="device-selector-container"></div>' +
            '<div class="panel-title" style="margin-top:16px;">命令</div>' +
            '<div id="preset-buttons" style="margin-bottom:8px; display:flex; flex-wrap:wrap; gap:6px;">加载预设...</div>' +
            '<textarea id="command-editor">echo "Hello from ICPC!"</textarea>' +
            '<div style="margin-top:12px;">' +
                '<button class="btn btn-primary" onclick="executeCommand()">▶ 执行</button>' +
            '</div>' +
        '</div>' +
        '<div class="result-panel" style="display:flex; flex-direction:column;">' +
            '<div class="result-header">' +
                '<div class="panel-title">执行结果</div>' +
                '<span id="result-progress" style="font-size:12px; color:var(--text-secondary);"></span>' +
                '<div id="tab-bar" style="display:flex; gap:4px; flex-wrap:wrap;"></div>' +
            '</div>' +
            '<div id="result-area" style="flex:1; overflow:auto; min-height:300px;">' +
                '<div class="empty-state">选择目标设备并点击执行，或选择历史命令查看结果</div>' +
            '</div>' +
        '</div>' +
    '</div>' +

    '<div style="display:flex; justify-content:space-between; align-items:center; margin:24px 0 16px;">' +
        '<h2 class="section-title" style="margin:0;">命令历史</h2>' +
        '<button class="btn btn-danger btn-sm" onclick="clearCommandHistory()">清空历史</button>' +
    '</div>' +
    '<div class="table-container">' +
        '<table>' +
            '<thead><tr><th>ID</th><th>时间</th><th>目标</th><th>命令</th><th>状态</th><th>耗时</th></tr></thead>' +
            '<tbody id="history-tbody"><tr><td colspan="6" class="empty-state">加载中...</td></tr></tbody>' +
        '</table>' +
    '</div>';

    $("#content").html(html);
    renderDeviceCards();
    setTimeout(initCodeMirror, 100);
    loadPresets();
    loadCommandHistory();
}

function initCodeMirror() {
    var ta = document.getElementById("command-editor");
    if (!ta) return;
    cmdEditor = CodeMirror.fromTextArea(ta, {
        mode: "shell", theme: "eclipse", lineNumbers: true,
        lineWrapping: true, indentUnit: 2, tabSize: 2,
        matchBrackets: true, autoCloseBrackets: true
    });
    cmdEditor.setSize(null, "200px");
}

// ---- Presets ----
function loadPresets() {
    $.getJSON("/api/presets", function(presets) {
        presetCommands = presets;
        var html = '';
        presets.forEach(function(p, i) {
            html += '<button class="btn btn-sm btn-' + (p.color || 'primary') + '" title="' + escapeHtml(p.desc) + '" onclick="applyPreset(' + i + ')">' + escapeHtml(p.name) + '</button>';
        });
        $("#preset-buttons").html(html);
    }).fail(function() { $("#preset-buttons").html(''); });
}
function applyPreset(idx) {
    var cmd = presetCommands[idx] ? presetCommands[idx].command : "";
    if (!cmd) return;
    if (cmdEditor) cmdEditor.setValue(cmd); else $("#command-editor").val(cmd);
}

// ---- Execute ----
function executeCommand() {
    var command = cmdEditor ? cmdEditor.getValue() : $("#command-editor").val();
    if (!command.trim()) { alert("请输入命令"); return; }
    if (selectedTargets.length === 0 && allDevices.length === 0) { alert("没有可用设备"); return; }

    // Start new batch — auto-registers any arriving command IDs
    activeBatch = { cmdIds: {}, tabs: {} };
    activeTabKey = null;
    sessionCompleted = 0;
    $("#tab-bar").html('');
    $("#result-progress").text('');
    $("#result-area").html('<div class="empty-state">正在派发...</div>');

    var commandText = command.trim();

    if (selectedTargets.length === 0) {
        // Broadcast to all devices
        var onlineCount = allDevices.filter(function(d) { return d.connected; }).length;
        sessionTotal = onlineCount > 0 ? onlineCount : allDevices.length;
        $.ajax({
            url: "/api/commands", method: "POST", contentType: "application/json",
            data: JSON.stringify({ target_type: "broadcast", command: commandText }),
            success: function(cmd) {
                activeBatch.cmdIds[cmd.id] = true;
                $("#result-area").html('<div class="empty-state">已派发 #' + cmd.id + '，等待设备响应...</div>');
                loadCommandHistory();
            },
            error: function(xhr) {
                showExecError(xhr);
            }
        });
    } else {
        // Send to each selected device individually
        sessionTotal = selectedTargets.length;
        for (var i = 0; i < selectedTargets.length; i++) {
            (function(deviceId) {
                $.ajax({
                    url: "/api/commands", method: "POST", contentType: "application/json",
                    data: JSON.stringify({ target_type: "single", target_id: deviceId, command: commandText }),
                    success: function(cmd) {
                        activeBatch.cmdIds[cmd.id] = true;
                        $("#result-area").html('<div class="empty-state">已派发 ' + selectedTargets.length + ' 个命令，等待响应...</div>');
                        loadCommandHistory();
                    },
                    error: function(xhr) {
                        showExecError(xhr);
                    }
                });
            })(selectedTargets[i]);
        }
    }
}

function showExecError(xhr) {
    var err = "执行失败";
    try { err = JSON.parse(xhr.responseText).error || err; } catch(e) {}
    $("#result-area").html('<div class="command-output" style="color:var(--danger)">' + escapeHtml(err) + '</div>');
}

// ---- Tab management ----
function getOrCreateTab(deviceId, cmdId) {
    var key = "dev_" + deviceId;
    if (!activeBatch.tabs[key]) {
        var el = $('<div class="command-output" style="flex:1; overflow:auto; font-size:13px; padding:8px;"></div>');
        activeBatch.tabs[key] = { el: el, cmdId: cmdId, status: "running" };

        var btn = $('<button class="btn btn-sm" style="border-radius:6px 6px 0 0; margin-bottom:-1px;">#' + deviceId + '</button>');
        btn.on("click", (function(k) { return function() { switchResultTab(k); }; })(key));
        $("#tab-bar").append(btn);
    }
    return activeBatch.tabs[key];
}

function switchResultTab(key) {
    activeTabKey = key;
    $("#tab-bar button").css({background:"",color:""});
    var allKeys = Object.keys(activeBatch.tabs);
    var idx = allKeys.indexOf(key);
    if (idx >= 0) $("#tab-bar button").eq(idx).css({background:"var(--accent)",color:"#fff"});

    var tab = activeBatch.tabs[key];
    if (!tab) return;
    var html = tab.el.prop("outerHTML");
    if (tab.status === "running") {
        html = '<div style="margin-bottom:8px;"><button class="btn btn-danger btn-sm" onclick="cancelActiveCommand(\'' + key + '\')">⏹ 终止此设备</button></div>' + html;
    }
    $("#result-area").html(html);
}

function updateProgress() {
    if (sessionTotal <= 0) { $("#result-progress").text(''); return; }
    var completed = 0;
    for (var k in activeBatch.tabs) {
        if (activeBatch.tabs[k].status !== "running") completed++;
    }
    var allDone = completed >= sessionTotal;
    $("#result-progress")
        .text(completed + '/' + sessionTotal + (allDone ? ' 已完成' : ''))
        .css('color', allDone ? 'var(--success)' : 'var(--text-secondary)')
        .css('font-weight', allDone ? '600' : 'normal');
}

function cancelActiveCommand(key) {
    if (activeBatch && activeBatch.tabs[key]) {
        $.ajax({ url: "/api/commands/" + activeBatch.tabs[key].cmdId + "/cancel", method: "POST" });
    }
}

// ---- Real-time streaming (called from app.js WebSocket) ----

// isActiveEvent returns true if the event belongs to the current active batch.
function isActiveEvent(evt) {
    if (!activeBatch) return false;
    // Already registered command ID
    if (activeBatch.cmdIds[evt.command_id]) return true;
    // For broadcast, child command IDs arrive in events but aren't registered yet.
    // Auto-register if there's an active batch and the event has a device_id.
    if (evt.command_id && evt.device_id) {
        activeBatch.cmdIds[evt.command_id] = true;
        return true;
    }
    return false;
}

function handleCommandOutput(evt) {
    if (isActiveEvent(evt)) {
        var key = "dev_" + evt.device_id;
        var isNew = !activeBatch.tabs[key];
        var tab = getOrCreateTab(evt.device_id, evt.command_id);
        var color = evt.stream === "stderr" ? "var(--danger)" : "inherit";
        tab.el.append('<span style="color:' + color + '">' + escapeHtml(evt.line) + '\n</span>');
        tab.el.scrollTop(tab.el[0].scrollHeight);
        // Auto-focus on first responding device, or refresh currently visible tab
        if (isNew || activeTabKey === key) {
            switchResultTab(key);
        }
        return;
    }
    // Watching from history click
    if (watchingCmdId && (evt.command_id === watchingCmdId || evt.device_id)) {
        var el = $("#result-area .command-output");
        if (el.length > 0) {
            var color = evt.stream === "stderr" ? "var(--danger)" : "var(--success)";
            el.append('<span style="color:' + color + '">' + escapeHtml(evt.line) + '\n</span>');
            el.scrollTop(el[0].scrollHeight);
        }
    }
}

function handleCommandResult(evt) {
    if (isActiveEvent(evt)) {
        var key = "dev_" + evt.device_id;
        var isNew = !activeBatch.tabs[key];
        var tab = getOrCreateTab(evt.device_id, evt.command_id);
        tab.status = evt.status || "completed";
        // Show (无输出) if nothing was streamed before completion
        if (isNew && !evt.error_output) {
            tab.el.append('<span style="color:var(--text-secondary)">(无输出)</span>\n');
        }
        if (evt.error_output) tab.el.append('<span style="color:var(--danger)">' + escapeHtml(evt.error_output) + '\n</span>');
        tab.el.append('\n<span style="color:var(--accent)">--- ' + statusLabel(evt.status) + ' (' + (evt.duration_ms || 0) + 'ms) ---</span>');
        sessionCompleted++;
        updateProgress();
        // Auto-switch if new tab (device had no output) or currently viewing
        if (isNew || activeTabKey === key) {
            switchResultTab(key);
        }
        loadCommandHistory();
        return;
    }
    // Watching from history click
    if (watchingCmdId && evt.command_id) {
        var completedWatched = (evt.command_id === watchingCmdId);
        if (completedWatched) {
            watchingCmdId = null;
            if (watchPollTimer) { clearInterval(watchPollTimer); watchPollTimer = null; }
        }
        $.getJSON("/api/commands/" + evt.command_id, function(cmd) {
            renderStaticOutput(cmd);
            loadCommandHistory();
        });
    }
}

// ---- History ----
function loadCommandHistory() {
    $.getJSON("/api/commands?limit=30", function(cmds) {
        var rows = cmds.length === 0
            ? '<tr><td colspan="6" class="empty-state">暂无命令记录</td></tr>'
            : cmds.map(function(c) {
                var target = c.target_type === "broadcast" ? "全部" : ("#" + (c.target_id || '?'));
                return '<tr class="clickable-row" onclick="showCommandDetail(' + c.id + ')">' +
                    '<td>#' + c.id + '</td><td>' + c.created_at + '</td><td>' + target + '</td>' +
                    '<td><code>' + escapeHtml(c.command.substring(0, 50)) + (c.command.length > 50 ? '...' : '') + '</code></td>' +
                    '<td><span class="badge badge-' + c.status + '">' + statusLabel(c.status) + '</span></td>' +
                    '<td>' + (c.duration_ms > 0 ? (c.duration_ms + 'ms') : '-') + '</td>' +
                '</tr>';
            }).join("");
        $("#history-tbody").html(rows);
    });
}

function showCommandDetail(cmdID) {
    watchingCmdId = null;
    if (watchPollTimer) { clearInterval(watchPollTimer); watchPollTimer = null; }
    activeBatch = null;
    $("#tab-bar").html('');
    activeTabKey = null;
    sessionTotal = 0;
    sessionCompleted = 0;
    $("#result-progress").text('');

    $.getJSON("/api/commands/" + cmdID, function(cmd) {
        if (cmd.command) {
            if (cmdEditor) cmdEditor.setValue(cmd.command);
            else $("#command-editor").val(cmd.command);
        }

        renderStaticOutput(cmd);

        var running = (cmd.status === "dispatched" || cmd.status === "running" || cmd.status === "pending");
        if (running) {
            watchingCmdId = cmdID;
        }
    });
}

function renderStaticOutput(cmd) {
    var out = cmd.output || cmd.error_output || '';
    var running = (cmd.status === "dispatched" || cmd.status === "running" || cmd.status === "pending");
    var html = '';

    if (running) {
        html += '<div style="margin-bottom:8px;">';
        html += '<button class="btn btn-danger btn-sm" onclick="cancelWatchedCommand(' + cmd.id + ')">⏹ 终止</button>';
        html += ' <span class="badge badge-' + cmd.status + '">' + statusLabel(cmd.status) + '</span>';
        html += '</div>';
    } else {
        html += '<div style="margin-bottom:8px;">';
        html += '<span class="badge badge-' + cmd.status + '">' + statusLabel(cmd.status) + '</span>';
        if (cmd.duration_ms) html += ' <small style="color:var(--text-secondary)">' + cmd.duration_ms + 'ms</small>';
        html += '</div>';
    }

    if (cmd.children && cmd.children.length > 0) {
        cmd.children.forEach(function(child) {
            html += '<div style="padding:8px; margin:4px 0; background:#f8f9fa; border:1px solid var(--border); border-radius:6px;">';
            html += '<div style="margin-bottom:4px;">';
            html += '<strong style="color:var(--accent)">设备 #' + (child.target_id || '?') + '</strong> ';
            html += '<span class="badge badge-' + child.status + '">' + statusLabel(child.status) + '</span>';
            if (child.duration_ms) html += ' <small style="color:var(--text-secondary)">' + child.duration_ms + 'ms</small>';
            html += '</div>';
            var cOut = child.output || child.error_output || '';
            html += '<div class="command-output" style="max-height:300px; overflow:auto;">' + (cOut ? escapeHtml(cOut) : '<span style="color:var(--text-secondary)">(无输出)</span>') + '</div>';
            html += '</div>';
        });
    } else {
        html += '<div class="command-output" style="max-height:500px; overflow:auto;">';
        html += out ? escapeHtml(out) : '<span style="color:var(--text-secondary)">(无输出)</span>';
        html += '</div>';
    }

    $("#result-area").html(html);
}

function cancelWatchedCommand(cmdId) {
    $.ajax({ url: "/api/commands/" + cmdId + "/cancel", method: "POST" });
}

function clearCommandHistory() {
    if (!confirm("确定要清空所有命令历史？")) return;
    $.ajax({
        url: "/api/commands/clear", method: "POST",
        success: function() {
            activeBatch = null;
            $("#tab-bar").html('');
            $("#result-area").html('<div class="empty-state">选择目标设备并点击执行，或选择历史命令查看结果</div>');
            $("#result-progress").text('');
            loadCommandHistory();
        }
    });
}
