package zapm

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebServer struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
}

func StartMonitorServer() error {
	ws := &WebServer{
		clients: make(map[*websocket.Conn]bool),
	}

	// API 路由
	http.HandleFunc("/api/services", ws.getServicesJSON)
	http.HandleFunc("/api/service/", ws.handleServiceAction) // 统一处理服务操作
	http.HandleFunc("/api/logs", ws.getLogs)                 // 获取日志（非流式）
	http.HandleFunc("/api/stream-logs", ws.streamLogs)       // WebSocket流式日志
	http.HandleFunc("/ws/stats", ws.handleWebSocket)

	// 静态文件路由
	http.HandleFunc("/", serveStatic)

	// 启动状态广播
	go ws.broadcastStats()

	// 启动服务器
	return http.ListenAndServe(fmt.Sprintf("%s:%d", Conf.Server.Address, Conf.Server.Port), nil)
}

// handleServiceAction 统一处理服务操作请求
func (ws *WebServer) handleServiceAction(w http.ResponseWriter, r *http.Request) {
	// 记录请求详情
	log.Printf("处理服务操作请求: %s", r.URL.Path)

	// 解析路径格式: /api/service/<name>/<action>
	path := strings.TrimPrefix(r.URL.Path, "/api/service/")
	if path == r.URL.Path {
		log.Printf("无效路径格式: %s (缺少/api/service/前缀)", r.URL.Path)
		http.Error(w, "路径格式应为: /api/service/<服务名>/<操作>", http.StatusBadRequest)
		return
	}

	// 分割路径部分
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		log.Printf("路径部分不足: %v (需要服务名和操作)", parts)
		http.Error(w, "路径格式应为: /api/service/<服务名>/<操作>", http.StatusBadRequest)
		return
	}

	name := parts[0]
	action := parts[1]
	log.Printf("解析请求 - 服务名: %s, 操作: %s", name, action)

	// 验证操作类型
	validActions := map[string]bool{
		"start":   true,
		"stop":    true,
		"restart": true,
	}
	if !validActions[action] {
		log.Printf("无效操作类型: %s", action)
		http.Error(w, fmt.Sprintf("无效操作: %s (支持: start, stop, restart)", action), http.StatusBadRequest)
		return
	}

	log.Printf("处理服务操作请求 - 服务: %s, 操作: %s", name, action)

	// 查找服务
	var service *Service
	for i := range Conf.Services {
		if Conf.Services[i].Name == name {
			service = &Conf.Services[i]
			break
		}
	}

	if service == nil {
		msg := fmt.Sprintf("未找到服务: %s", name)
		log.Println(msg)
		http.Error(w, msg, http.StatusNotFound)
		return
	}

	// 执行操作
	var err error
	switch action {
	case "start":
		log.Printf("启动服务: %s", name)
		err = startService(service)
	case "stop":
		log.Printf("停止服务: %s", name)
		err = stopService(service)
	case "restart":
		log.Printf("重启服务: %s", name)
		err = restartService(service)
	default:
		msg := fmt.Sprintf("无效操作: %s (支持: start/stop/restart)", action)
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("服务操作失败 - 服务: %s, 操作: %s, 错误: %v", name, action, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("服务操作成功 - 服务: %s, 操作: %s", name, action)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("操作成功"))
}

// startService 启动单个服务
func startService(service *Service) error {
	cmdParts := strings.Fields(service.Run)
	if len(cmdParts) == 0 {
		return fmt.Errorf("invalid command")
	}

	command := exec.Command(cmdParts[0], cmdParts[1:]...)
	if service.WorkDir != "" {
		command.Dir = service.WorkDir
	}

	// 初始化服务的Logger
	if service.Logger == nil {
		// 使用全局日志目录
		logDir := LogsDir
		if logDir == "" {
			// 如果全局日志目录未设置，使用当前目录下的logs子目录
			var err error
			logDir, err = filepath.Abs("logs")
			if err != nil {
				return fmt.Errorf("获取日志目录绝对路径失败: %v", err)
			}
		}

		// 确保日志目录存在
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("创建日志目录失败: %v", err)
		}

		// 创建日志文件路径
		logPath := filepath.Join(logDir, fmt.Sprintf("%s.log", service.Name))

		// 记录日志路径
		log.Printf("服务 %s 的日志文件路径: %s", service.Name, logPath)

		// 初始化Logger
		logger, err := NewLogger(logPath, 10, 5, "INFO", true, service.Name)
		if err != nil {
			return fmt.Errorf("初始化日志系统失败: %v", err)
		}

		service.Logger = logger
	}

	// 将命令的标准输出和标准错误重定向到Logger
	command.Stdout = service.Logger
	command.Stderr = service.Logger

	if err := command.Start(); err != nil {
		return err
	}

	service.Status = "running"
	service.Pid = command.Process.Pid

	// 记录服务启动日志
	service.Logger.Info("服务已启动，PID: %d", service.Pid)

	// 在后台监控进程退出
	go func() {
		err := command.Wait()
		if err != nil {
			service.Logger.Error("服务异常退出: %v", err)
		} else {
			service.Logger.Info("服务正常退出")
		}
		service.Status = "stopped"
		service.Pid = 0
	}()

	return nil
}

// stopService 停止单个服务
func stopService(service *Service) error {
	if service.Pid == 0 {
		return nil
	}

	process, err := os.FindProcess(service.Pid)
	if err != nil {
		return err
	}

	// 在Windows平台上直接使用Kill()，在其他平台上尝试使用SIGTERM
	if runtime.GOOS == "windows" {
		if err := process.Kill(); err != nil {
			return err
		}
	} else {
		// 在非Windows平台上，先尝试SIGTERM，如果失败再使用Kill
		if err := process.Signal(syscall.SIGTERM); err != nil {
			// 如果发送SIGTERM失败，尝试强制终止
			if killErr := process.Kill(); killErr != nil {
				return fmt.Errorf("failed to terminate process: %v (kill attempt also failed: %v)", err, killErr)
			}
		}
	}

	service.Status = "stopped"
	service.Pid = 0
	return nil
}

// restartService 重启单个服务
func restartService(service *Service) error {
	if err := stopService(service); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	return startService(service)
}

// getServicesJSON 返回服务列表的JSON数据
func (ws *WebServer) getServicesJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 确保数据是最新的
	services := make([]Service, len(Conf.Services))
	copy(services, Conf.Services)

	json.NewEncoder(w).Encode(services)
}

// WebSocketLogSubscriber 实现LogSubscriber接口的WebSocket日志订阅者
type WebSocketLogSubscriber struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// Send 发送日志到WebSocket连接
func (ws *WebSocketLogSubscriber) Send(logLine string) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.conn.WriteMessage(websocket.TextMessage, []byte(logLine))
}

// Close 关闭WebSocket连接
func (ws *WebSocketLogSubscriber) Close() {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.conn.Close()
}

// getLogs 获取服务日志（非流式）
func (ws *WebServer) getLogs(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("service")
	if serviceName == "" {
		http.Error(w, "Missing service parameter", http.StatusBadRequest)
		return
	}

	// 查找服务
	var service *Service
	for i := range Conf.Services {
		if Conf.Services[i].Name == serviceName {
			service = &Conf.Services[i]
			break
		}
	}

	if service == nil {
		http.Error(w, fmt.Sprintf("Service not found: %s", serviceName), http.StatusNotFound)
		return
	}

	// 如果日志系统未初始化
	if service.Logger == nil {
		http.Error(w, fmt.Sprintf("Logger not initialized for service: %s", serviceName), http.StatusInternalServerError)
		return
	}

	// 获取最近的日志内容
	logs := service.Logger.GetRecentLogs()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(strings.Join(logs, "\n")))
}

// streamLogs 流式传输日志
func (ws *WebServer) streamLogs(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("service")
	if serviceName == "" {
		http.Error(w, "Missing service parameter", http.StatusBadRequest)
		return
	}

	// 升级HTTP连接为WebSocket连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}

	// 创建WebSocket日志订阅者
	subscriber := &WebSocketLogSubscriber{
		conn: conn,
	}

	// 查找服务
	var service *Service
	for i := range Conf.Services {
		if Conf.Services[i].Name == serviceName {
			service = &Conf.Services[i]
			break
		}
	}

	if service == nil {
		subscriber.Send(fmt.Sprintf("错误：未找到服务 %s", serviceName))
		subscriber.Close()
		return
	}

	// 订阅日志
	if service.Logger != nil {
		service.Logger.Subscribe(subscriber)

		// 等待连接关闭
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				if !websocket.IsCloseError(err, websocket.CloseGoingAway) {
					log.Printf("WebSocket读取错误: %v", err)
				}
				break
			}
		}

		// 取消订阅并清理资源
		service.Logger.Unsubscribe(subscriber)
		subscriber.Close()
	} else {
		subscriber.Send(fmt.Sprintf("错误：服务 %s 的日志系统未初始化", serviceName))
		subscriber.Close()
	}
}

// handleWebSocket 处理WebSocket连接
func (ws *WebServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}

	// 注册客户端
	ws.mu.Lock()
	ws.clients[conn] = true
	ws.mu.Unlock()

	// 发送初始数据
	if err := conn.WriteJSON(ws.getCurrentStats()); err != nil {
		log.Printf("发送初始数据失败: %v", err)
		conn.Close()
		return
	}

	// 处理连接
	go func() {
		defer func() {
			ws.mu.Lock()
			delete(ws.clients, conn)
			ws.mu.Unlock()
			conn.Close()
		}()

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				if !websocket.IsCloseError(err, websocket.CloseGoingAway) {
					log.Printf("WebSocket读取错误: %v", err)
				}
				break
			}
		}
	}()
}

// getCurrentStats 获取当前服务状态
func (ws *WebServer) getCurrentStats() map[string]interface{} {
	stats := make(map[string]interface{})
	for _, service := range Conf.Services {
		stats[service.Name] = map[string]interface{}{
			"cpuUsage":    service.CpuUsage,
			"memoryUsage": service.MemoryUsage,
			"uptime":      service.Uptime,
			"status":      service.Status,
			"pid":         service.Pid,
			"lastUpdate":  time.Now().Format(time.RFC3339),
		}
	}
	return stats
}

// broadcastStats 广播服务状态
func (ws *WebServer) broadcastStats() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := make(map[string]interface{})
		for _, service := range Conf.Services {
			// 确保数据有效
			cpu := service.CpuUsage
			if cpu < 0 {
				cpu = 0
			}
			memory := service.MemoryUsage
			if memory < 0 {
				memory = 0
			}

			stats[service.Name] = map[string]interface{}{
				"cpuUsage":    cpu,
				"memoryUsage": memory,
				"uptime":      service.Uptime,
				"status":      service.Status,
				"pid":         service.Pid,
				"lastUpdate":  time.Now().Format(time.RFC3339), // 添加最后更新时间
			}
		}

		// 记录广播的数据
		// log.Printf("广播服务状态数据: %+v", stats)

		ws.mu.Lock()
		for client := range ws.clients {
			if err := client.WriteJSON(stats); err != nil {
				log.Printf("发送WebSocket数据失败: %v", err)
				delete(ws.clients, client)
				client.Close()
			}
		}
		ws.mu.Unlock()
	}
}

// serveStatic 提供静态文件
func serveStatic(w http.ResponseWriter, r *http.Request) {
	// 处理根路径请求
	if r.URL.Path == "/" {
		// 直接从嵌入式文件系统读取index.html
		content, err := webContent.ReadFile("web/index.html")
		if err != nil {
			http.Error(w, "无法读取index.html", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(content)
		return
	}

	// 其他静态文件使用文件服务器
	fileServer := http.FileServer(getEmbeddedFileSystem())
	fileServer.ServeHTTP(w, r)
}
