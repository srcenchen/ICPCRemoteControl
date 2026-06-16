// ICPC Remote Control - App Core
"use strict";

var currentPage = "dashboard";
var adminWS = null;
var selectedTargets = []; // shared with commands.js — empty = broadcast

$(function() {
    $(".nav-link").on("click", function(e) {
        e.preventDefault();
        var page = $(this).data("page");
        navigateTo(page);
    });

    connectAdminWS();
    navigateTo("dashboard");
});

function navigateTo(page) {
    currentPage = page;
    $(".nav-link").removeClass("active");
    $('.nav-link[data-page="' + page + '"]').addClass("active");

    switch (page) {
        case "dashboard": loadDashboard(); break;
        case "devices":   loadDevices(); break;
        case "commands":  loadCommands(); break;
        case "network":   loadNetwork(); break;
        case "checkin":   renderCheckinPage(); break;
        case "settings":  loadSettings(); break;
    }
}

function connectAdminWS() {
    var protocol = location.protocol === "https:" ? "wss:" : "ws:";
    var wsURL = protocol + "//" + location.host + "/ws/admin";
    adminWS = new WebSocket(wsURL);

    adminWS.onopen = function() {
        console.log("[admin-ws] 已连接");
        refreshCurrentPage();
    };

    adminWS.onmessage = function(event) {
        try {
            var msg = JSON.parse(event.data);
            handleAdminEvent(msg);
        } catch(e) {
            console.error("[admin-ws] 解析错误:", e);
        }
    };

    adminWS.onclose = function() {
        console.log("[admin-ws] 断开，3秒后重连");
        setTimeout(connectAdminWS, 3000);
    };

    adminWS.onerror = function(err) {
        console.error("[admin-ws] 错误:", err);
    };
}

function handleAdminEvent(msg) {
    switch (msg.event) {
        case "device_connected":
        case "device_disconnected":
        case "device_updated":
            updateStatusBar();
            if (currentPage === "dashboard") loadDashboard();
            if (currentPage === "devices") loadDevices();
            break;
        case "command_status":
            if (currentPage === "commands" && typeof updateCommandResult === "function") updateCommandResult(msg.data);
            if (currentPage === "dashboard") loadDashboard();
            break;
        case "command_output":
            if (typeof handleCommandOutput === "function") handleCommandOutput(msg.data);
            break;
        case "command_result":
            if (typeof handleCommandResult === "function") handleCommandResult(msg.data);
            break;
    }
}

function refreshCurrentPage() {
    updateStatusBar();
    navigateTo(currentPage);
}

function updateStatusBar() {
    $.getJSON("/api/stats", function(stats) {
        $("#online-count").text("在线: " + stats.online_devices);
        $("#total-count").text("总计: " + stats.total_devices);
    }).fail(function() {
        console.error("获取统计失败");
    });
}

function formatBytes(bytes) {
    if (bytes === 0) return "0 B";
    var units = ["B", "KB", "MB", "GB", "TB"];
    var i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(1) + " " + units[i];
}

function escapeHtml(str) {
    var div = document.createElement("div");
    div.textContent = str;
    return div.innerHTML;
}

function statusLabel(status) {
    var map = {
        pending: "等待中",
        dispatched: "已派发",
        running: "运行中",
        completed: "已完成",
        failed: "失败",
        timeout: "超时"
    };
    return map[status] || status;
}
