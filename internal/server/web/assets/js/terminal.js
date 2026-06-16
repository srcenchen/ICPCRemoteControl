// Terminal page - opened from device detail
var termInstance = null;
var termSocket = null;
var termDeviceId = null;

function openTerminal(deviceId) {
    termDeviceId = deviceId;

    var html = '' +
    '<div class="modal-overlay" onclick="closeTerminal()">' +
        '<div class="modal" style="max-width:900px; width:95%; padding:16px;" onclick="event.stopPropagation()">' +
            '<div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:8px;">' +
                '<h3 style="color:var(--accent); margin:0;">终端 — 设备 #' + deviceId + '</h3>' +
                '<button class="btn btn-sm btn-danger" onclick="closeTerminal()">✕ 关闭</button>' +
            '</div>' +
            '<div id="terminal-container" style="height:500px; background:#f5f5f5; border:1px solid var(--border); border-radius:8px;"></div>' +
        '</div>' +
    '</div>';

    $("body").append(html);

    // Init xterm.js
    setTimeout(function() {
        termInstance = new Terminal({
            cursorBlink: true,
            fontSize: 14,
            fontFamily: '"Fira Code", "Consolas", monospace',
            theme: { background: "#fafafa", foreground: "#333", cursor: "#333" },
            rows: 28,
            cols: 100
        });

        var fitAddon = new FitAddon.FitAddon();
        termInstance.loadAddon(fitAddon);
        termInstance.open(document.getElementById("terminal-container"));
        fitAddon.fit();

        // Connect terminal WebSocket
        var proto = location.protocol === "https:" ? "wss:" : "ws:";
        termSocket = new WebSocket(proto + "//" + location.host + "/ws/terminal/" + deviceId + "?cols=" + termInstance.cols + "&rows=" + termInstance.rows);

        termSocket.binaryType = "arraybuffer";

        termSocket.onopen = function() {
            termInstance.write("\x1b[32m已连接到设备 #" + deviceId + "\x1b[0m\r\n");
        };

        termSocket.onmessage = function(evt) {
            if (termInstance && evt.data) {
                termInstance.write(new Uint8Array(evt.data));
            }
        };

        termSocket.onclose = function() {
            if (termInstance) termInstance.write("\r\n\x1b[31m连接已断开\x1b[0m\r\n");
        };

        termSocket.onerror = function() {
            if (termInstance) termInstance.write("\r\n\x1b[31m连接错误\x1b[0m\r\n");
        };

        // Send user input to server
        termInstance.onData(function(data) {
            if (termSocket && termSocket.readyState === WebSocket.OPEN) {
                termSocket.send(data);
            }
        });

        // Handle resize
        termInstance.onResize(function(size) {
            if (termSocket && termSocket.readyState === WebSocket.OPEN) {
                termSocket.send(JSON.stringify({ type: "resize", cols: size.cols, rows: size.rows }));
            }
        });
    }, 200);
}

function closeTerminal() {
    if (termSocket) { termSocket.close(); termSocket = null; }
    if (termInstance) { termInstance.dispose(); termInstance = null; }
    $(".modal-overlay").remove();
}
