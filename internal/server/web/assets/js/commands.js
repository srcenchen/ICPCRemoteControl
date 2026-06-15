// Commands page
var cmdEditor = null;
var selectedTarget = null;
var presetCommands = [];
var activeSession = null; // {cmdId, tabs: {deviceKey: {el, status}}}
var activeTabKey = null;
var watchingCmdId = null; // command ID being watched from history click
var detailPollTimer = null;
var watchPollTimer = null;

function loadCommands() {
    if (detailPollTimer) { clearInterval(detailPollTimer); detailPollTimer = null; }
    if (watchPollTimer) { clearInterval(watchPollTimer); watchPollTimer = null; }
    watchingCmdId = null;
    activeSession = null;
    activeTabKey = null;

    $.getJSON("/api/devices", function(devices) {
        renderCommandPage(devices);
    }).fail(function() {
        $("#content").html('<div class="empty-state">无法加载设备列表</div>');
    });
}

function renderCommandPage(devices) {
    var devOptions = '<option value="broadcast">广播（全部设备）</option>';
    devices.forEach(function(d) {
        var icon = d.connected ? "🟢" : "⚫";
        devOptions += '<option value="' + d.assigned_id + '">' + icon + ' #' + d.assigned_id + ' — ' + escapeHtml(d.hostname || d.username) + '</option>';
    });

    var html = '' +
    '<h2 class="section-title">命令执行</h2>' +
    '<div class="command-layout">' +
        '<div class="command-panel">' +
            '<div class="panel-title">命令</div>' +
            '<div class="command-toolbar">' +
                '<select id="target-select" onchange="onTargetChange()">' + devOptions + '</select>' +
                '<button class="btn btn-primary" onclick="executeCommand()">▶ 执行</button>' +
            '</div>' +
            '<div id="preset-buttons" style="margin-bottom:8px; display:flex; flex-wrap:wrap; gap:6px;">加载预设...</div>' +
            '<textarea id="command-editor">echo "Hello from ICPC!"</textarea>' +
        '</div>' +
        '<div class="result-panel" style="display:flex; flex-direction:column;">' +
            '<div class="result-header">' +
                '<div class="panel-title">执行结果</div>' +
                '<div id="tab-bar" style="display:flex; gap:4px; flex-wrap:wrap;"></div>' +
            '</div>' +
            '<div id="result-area" style="flex:1; overflow:auto; min-height:300px;">' +
                '<div class="empty-state">点击执行或选择历史命令查看结果</div>' +
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
    setTimeout(initCodeMirror, 100);
    loadPresets();
    if (selectedTarget !== null) $("#target-select").val(selectedTarget);
    loadCommandHistory();
}

function initCodeMirror() {
    var ta = document.getElementById("command-editor");
    if (!ta) return;
    cmdEditor = CodeMirror.fromTextArea(ta, {
        mode: "shell", theme: "monokai", lineNumbers: true,
        lineWrapping: true, indentUnit: 2, tabSize: 2,
        matchBrackets: true, autoCloseBrackets: true
    });
    cmdEditor.setSize(null, "200px");
}

function onTargetChange() {
    var val = $("#target-select").val();
    selectedTarget = (val === "broadcast") ? null : parseInt(val);
}
function selectTarget(id) { selectedTarget = id; }

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

    var targetType = selectedTarget ? "single" : "broadcast";
    var body = { target_type: targetType, command: command.trim() };
    if (selectedTarget) body.target_id = selectedTarget;

    $.ajax({
        url: "/api/commands", method: "POST", contentType: "application/json",
        data: JSON.stringify(body),
        success: function(cmd) {
            // Start a new active session — only this session gets streaming updates.
            activeSession = { cmdId: cmd.id, tabs: {} };
            activeTabKey = null;
            $("#tab-bar").html('');
            $("#result-area").html('<div class="empty-state">已派发 #' + cmd.id + '，等待设备响应...</div>');
            loadCommandHistory();
        },
        error: function(xhr) {
            var err = "执行失败";
            try { err = JSON.parse(xhr.responseText).error || err; } catch(e) {}
            $("#result-area").html('<div class="command-output" style="color:var(--danger)">' + escapeHtml(err) + '</div>');
        }
    });
}

function getOrCreateTab(deviceId, cmdId) {
    var key = "dev_" + deviceId;
    var tabs = activeSession ? activeSession.tabs : {};
    if (!tabs[key]) {
        var el = $('<div class="command-output" style="flex:1; overflow:auto; font-size:13px; padding:8px;"></div>');
        tabs[key] = { el: el, cmdId: cmdId, status: "running" };

        var label = "#" + deviceId;
        var btn = $('<button class="btn btn-sm" style="border-radius:6px 6px 0 0; margin-bottom:-1px;">' + label + '</button>');
        btn.on("click", (function(k) { return function() { switchResultTab(k); }; })(key));
        $("#tab-bar").append(btn);
    }
    return tabs[key];
}

function switchResultTab(key) {
    activeTabKey = key;
    $("#tab-bar button").css({background:"",color:""});
    var keys = Object.keys(activeSession ? activeSession.tabs : {});
    var idx = keys.indexOf(key);
    if (idx >= 0) $("#tab-bar button").eq(idx).css({background:"var(--accent)",color:"#000"});
    var tab = activeSession ? activeSession.tabs[key] : null;
    if (tab) {
        var html = tab.el.prop("outerHTML");
        if (tab.status === "running") {
            html = '<div style="margin-bottom:8px;"><button class="btn btn-danger btn-sm" onclick="cancelActiveCommand(\'' + key + '\')">⏹ 终止此设备</button></div>' + html;
        }
        $("#result-area").html(html);
    }
}

function cancelActiveCommand(key) {
    var tab = activeSession ? activeSession.tabs[key] : null;
    if (!tab) return;
    $.ajax({ url: "/api/commands/" + tab.cmdId + "/cancel", method: "POST" });
}

// ---- Real-time streaming (called from app.js WebSocket) ----
// Only updates if there's an active session and the command matches.

function handleCommandOutput(evt) {
    // Active session (from Execute button) — tabbed output.
    if (activeSession) {
        var tab = getOrCreateTab(evt.device_id, evt.command_id);
        if (tab) {
            var color = evt.stream === "stderr" ? "var(--danger)" : "inherit";
            tab.el.append('<span style="color:' + color + '">' + escapeHtml(evt.line) + '\n</span>');
            tab.el.scrollTop(tab.el[0].scrollHeight);
        }
        return;
    }
    // Watching from history click — append to static view.
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
    // Active session.
    if (activeSession) {
        var tab = getOrCreateTab(evt.device_id, evt.command_id);
        if (tab) {
            tab.status = "completed";
            if (evt.error_output) tab.el.append('<span style="color:var(--danger)">' + escapeHtml(evt.error_output) + '\n</span>');
            tab.el.append('\n<span style="color:var(--accent)">--- ' + statusLabel(evt.status) + ' (' + (evt.duration_ms || 0) + 'ms) ---</span>');
        }
        loadCommandHistory();
        return;
    }
    // Watching from history click — refresh to get final output from DB.
    if (watchingCmdId) {
        watchingCmdId = null;
        if (watchPollTimer) { clearInterval(watchPollTimer); watchPollTimer = null; }
        loadCommandHistory();
        // Reload the final output.
        if (evt.command_id) {
            $.getJSON("/api/commands/" + evt.command_id, function(cmd) {
                renderStaticOutput(cmd);
                loadCommandHistory();
            });
        }
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
    // Clear any previous watch/poll state.
    watchingCmdId = null;
    if (watchPollTimer) { clearInterval(watchPollTimer); watchPollTimer = null; }
    if (detailPollTimer) { clearInterval(detailPollTimer); detailPollTimer = null; }
    // Don't clear activeSession — Execute-button tabs stay alive independently.

    $.getJSON("/api/commands/" + cmdID, function(cmd) {
        // Fill editor.
        if (cmd.command) {
            if (cmdEditor) cmdEditor.setValue(cmd.command);
            else $("#command-editor").val(cmd.command);
            if (cmd.target_type === "single" && cmd.target_id) {
                $("#target-select").val(cmd.target_id);
                selectedTarget = cmd.target_id;
            }
        }

        renderStaticOutput(cmd);

        var running = (cmd.status === "dispatched" || cmd.status === "running" || cmd.status === "pending");
        if (running) {
            // Subscribe to WebSocket streaming for this command.
            watchingCmdId = cmdID;
        }
    });
}

function renderStaticOutput(cmd) {
    var out = cmd.output || cmd.error_output || '';
    var running = (cmd.status === "dispatched" || cmd.status === "running" || cmd.status === "pending");
    var html = '';

    // Cancel button for running commands.
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
            html += '<div style="padding:8px; margin:4px 0; background:rgba(0,0,0,0.2); border-radius:6px;">';
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
            if (detailPollTimer) { clearInterval(detailPollTimer); detailPollTimer = null; }
            activeSession = null;
            $("#tab-bar").html('');
            $("#result-area").html('<div class="empty-state">点击执行或选择历史命令查看结果</div>');
            loadCommandHistory();
        }
    });
}
