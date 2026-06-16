// 仪表盘页面
function loadDashboard() {
    $.getJSON("/api/stats", function(stats) {
        renderDashboard(stats);
    }).fail(function() {
        $("#content").html('<div class="empty-state">无法加载仪表盘数据</div>');
    });
}

function renderDashboard(stats) {
    var html = '' +
        '<div class="stats-grid">' +
            '<div class="stat-card">' +
                '<div class="stat-value">' + stats.total_devices + '</div>' +
                '<div class="stat-label">设备总数</div>' +
            '</div>' +
            '<div class="stat-card">' +
                '<div class="stat-value" style="color: var(--success)">' + stats.online_devices + '</div>' +
                '<div class="stat-label">在线设备</div>' +
            '</div>' +
            '<div class="stat-card">' +
                '<div class="stat-value" style="color: var(--danger)">' + stats.offline_devices + '</div>' +
                '<div class="stat-label">离线设备</div>' +
            '</div>' +
            '<div class="stat-card">' +
                '<div class="stat-value" style="color: var(--success)">' + (stats.checked_in || 0) + '</div>' +
                '<div class="stat-label">已签到</div>' +
            '</div>' +
            '<div class="stat-card">' +
                '<div class="stat-value">' + stats.total_commands + '</div>' +
                '<div class="stat-label">命令总数</div>' +
            '</div>' +
        '</div>' +

        '<h2 class="section-title">最近命令</h2>' +
        '<div class="table-container">' +
            '<table>' +
                '<thead>' +
                    '<tr>' +
                        '<th>ID</th><th>时间</th><th>目标</th><th>命令</th><th>状态</th><th>耗时</th>' +
                    '</tr>' +
                '</thead>' +
                '<tbody>' + renderCommandRows(stats.recent_commands) + '</tbody>' +
            '</table>' +
        '</div>';

    $("#content").html(html);
    updateStatusBar();
}

function renderCommandRows(commands) {
    if (!commands || commands.length === 0) {
        return '<tr><td colspan="6" class="empty-state">暂无命令记录</td></tr>';
    }
    return commands.map(function(cmd) {
        var target = cmd.target_type === "broadcast" ? "全部" : ("#" + cmd.target_id);
        return '<tr>' +
            '<td>#' + cmd.id + '</td>' +
            '<td>' + cmd.created_at + '</td>' +
            '<td>' + target + '</td>' +
            '<td><code>' + escapeHtml(cmd.command.substring(0, 60)) + (cmd.command.length > 60 ? '...' : '') + '</code></td>' +
            '<td><span class="badge badge-' + cmd.status + '">' + statusLabel(cmd.status) + '</span></td>' +
            '<td>' + (cmd.duration_ms > 0 ? (cmd.duration_ms + 'ms') : '-') + '</td>' +
        '</tr>';
    }).join("");
}
