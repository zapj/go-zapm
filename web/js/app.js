// 全局变量
let cpuChart;
let memoryChart;
let socket;
let services = [];

// 初始化函数
function init() {
    setupWebSocket();
    setupCharts();
    loadServices();
    setupEventListeners();
}

// 设置WebSocket连接
function setupWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    socket = new WebSocket(`${protocol}//${host}/ws/stats`);

    socket.onmessage = function(event) {
        const stats = JSON.parse(event.data);
        updateServicesTable(stats);
        updateCharts(stats);
    };

    socket.onclose = function() {
        setTimeout(setupWebSocket, 1000);
    };
}

// 设置图表
function setupCharts() {
    const cpuCtx = document.getElementById('cpu-chart').getContext('2d');
    const memoryCtx = document.getElementById('memory-chart').getContext('2d');

    cpuChart = new Chart(cpuCtx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [{
                label: 'CPU使用率 (%)',
                data: [],
                borderColor: 'rgb(75, 192, 192)',
                tension: 0.1,
                fill: false
            }]
        },
        options: {
            responsive: true,
            scales: {
                y: {
                    beginAtZero: true,
                    max: 100
                }
            }
        }
    });

    memoryChart = new Chart(memoryCtx, {
        type: 'bar',
        data: {
            labels: [],
            datasets: [{
                label: '内存使用 (MB)',
                data: [],
                backgroundColor: 'rgba(54, 162, 235, 0.5)',
                borderColor: 'rgba(54, 162, 235, 1)',
                borderWidth: 1
            }]
        },
        options: {
            responsive: true,
            scales: {
                y: {
                    beginAtZero: true
                }
            }
        }
    });
}

// 加载服务列表
function loadServices() {
    fetch('/api/services')
        .then(response => response.json())
        .then(data => {
            services = data;
            updateServicesDropdown();
            updateServicesTable();
        });
}

// 更新服务下拉菜单
function updateServicesDropdown() {
    const select = document.getElementById('log-service-select');
    select.innerHTML = '<option value="">选择服务...</option>';
    
    services.forEach(service => {
        const option = document.createElement('option');
        option.value = service.name;
        option.textContent = service.name;
        select.appendChild(option);
    });
}

// 更新服务表格
function updateServicesTable(stats) {
    const tbody = document.getElementById('services-body');
    if (!tbody) {
        console.error('无法找到表格体元素');
        return;
    }

    // 清空现有内容
    while (tbody.firstChild) {
        tbody.removeChild(tbody.firstChild);
    }

    if (!services || services.length === 0) {
        const row = document.createElement('tr');
        const cell = document.createElement('td');
        cell.colSpan = 7;
        cell.textContent = '没有可用的服务数据';
        cell.className = 'text-center text-muted';
        row.appendChild(cell);
        tbody.appendChild(row);
        return;
    }

    services.forEach(service => {
        try {
            const stat = stats ? stats[service.name] : null;
            const row = document.createElement('tr');
            
            // 名称
            const nameCell = document.createElement('td');
            nameCell.textContent = service.name || '-';
            row.appendChild(nameCell);
            
            // 状态
            const statusCell = document.createElement('td');
            const status = stat ? stat.status : service.status;
            statusCell.textContent = status || 'unknown';
            statusCell.className = `status-${status}`;
            row.appendChild(statusCell);
            
            // PID
            const pidCell = document.createElement('td');
            pidCell.textContent = (stat && stat.pid) ? stat.pid : (service.pid || '-');
            row.appendChild(pidCell);
            
            // 运行时间
            const uptimeCell = document.createElement('td');
            uptimeCell.textContent = (stat && stat.uptime) ? formatUptime(stat.uptime) : '-';
            row.appendChild(uptimeCell);
            
            // CPU
            const cpuCell = document.createElement('td');
            cpuCell.textContent = (stat && stat.cpuUsage) ? `${stat.cpuUsage.toFixed(1)}%` : '-';
            row.appendChild(cpuCell);
            
            // 内存
            const memCell = document.createElement('td');
            memCell.textContent = (stat && stat.memoryUsage) ? formatMemory(stat.memoryUsage) : '-';
            row.appendChild(memCell);
            
            // 操作
            const actionCell = document.createElement('td');
            if (status === 'running') {
                actionCell.innerHTML = `
                    <button class="btn btn-danger btn-sm btn-action" onclick="stopService('${service.name}')">停止</button>
                    <button class="btn btn-warning btn-sm btn-action" onclick="restartService('${service.name}')">重启</button>
                `;
            } else {
                actionCell.innerHTML = `
                    <button class="btn btn-success btn-sm btn-action" onclick="startService('${service.name}')">启动</button>
                `;
            }
            
            row.appendChild(actionCell);
            tbody.appendChild(row);
        } catch (error) {
            console.error('渲染服务行时出错:', error);
        }
    });
}

// 更新图表
function updateCharts(stats) {
    const labels = [];
    const cpuData = [];
    const memoryData = [];
    
    for (const [name, stat] of Object.entries(stats)) {
        labels.push(name);
        cpuData.push(stat.cpuUsage);
        memoryData.push(stat.memoryUsage / (1024 * 1024)); // 转换为MB
    }
    
    cpuChart.data.labels = labels;
    cpuChart.data.datasets[0].data = cpuData;
    cpuChart.update();
    
    memoryChart.data.labels = labels;
    memoryChart.data.datasets[0].data = memoryData;
    memoryChart.update();
}

// 设置事件监听器
function setupEventListeners() {
    document.getElementById('log-service-select').addEventListener('change', function() {
        const serviceName = this.value;
        if (serviceName) {
            fetchLogs(serviceName);
        } else {
            document.getElementById('log-content').textContent = '';
        }
    });
}

// 获取日志
function fetchLogs(serviceName) {
    fetch(`/api/logs?service=${serviceName}`)
        .then(response => response.text())
        .then(logs => {
            document.getElementById('log-content').textContent = logs;
        });
}

// 格式化运行时间
function formatUptime(seconds) {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    const secs = Math.floor(seconds % 60);
    
    let result = '';
    if (days > 0) result += `${days}d `;
    if (hours > 0 || days > 0) result += `${hours}h `;
    if (mins > 0 || hours > 0 || days > 0) result += `${mins}m `;
    result += `${secs}s`;
    
    return result;
}

// 格式化内存
function formatMemory(bytes) {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

// 服务操作函数
function startService(name) {
    fetch(`/api/service/${name}/start`, { method: 'POST' })
        .then(response => {
            if (response.ok) {
                loadServices();
            }
        });
}

function stopService(name) {
    fetch(`/api/service/${name}/stop`, { method: 'POST' })
        .then(response => {
            if (response.ok) {
                loadServices();
            }
        });
}

function restartService(name) {
    fetch(`/api/service/${name}/restart`, { method: 'POST' })
        .then(response => {
            if (response.ok) {
                loadServices();
            }
        });
}

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', init);