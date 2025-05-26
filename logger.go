package zapm

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// LogLevel 定义日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// LogLevelNames 日志级别名称
var LogLevelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

// LogLevelFromString 从字符串解析日志级别
func LogLevelFromString(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return INFO
	}
}

// LogSubscriber 日志订阅者接口
type LogSubscriber interface {
	Send(logLine string) error
	Close()
}

// Logger 日志管理器
type Logger struct {
	file        *os.File
	filename    string
	maxSize     int64 // 最大大小，单位MB
	maxFiles    int   // 最大文件数
	size        int64 // 当前大小
	level       LogLevel
	mu          sync.Mutex
	compress    bool   // 是否压缩旧日志
	serviceTag  string // 服务标识
	subscribers map[LogSubscriber]struct{}
	subMu       sync.RWMutex
}

// NewLogger 创建新的日志管理器
func NewLogger(filename string, maxSize int, maxFiles int, level string, compress bool, serviceTag string) (*Logger, error) {
	if maxSize <= 0 {
		maxSize = 10 // 默认10MB
	}
	if maxFiles <= 0 {
		maxFiles = 5 // 默认保留5个文件
	}

	// 确保目录存在
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %v", err)
	}

	// 打开日志文件
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	// 获取当前文件大小
	info, err := f.Stat()
	var size int64
	if err == nil {
		size = info.Size()
	}

	return &Logger{
		file:        f,
		filename:    filename,
		maxSize:     int64(maxSize) * 1024 * 1024, // 转换为字节
		maxFiles:    maxFiles,
		size:        size,
		level:       LogLevelFromString(level),
		compress:    compress,
		serviceTag:  serviceTag,
		subscribers: make(map[LogSubscriber]struct{}),
	}, nil
}

// Write 实现io.Writer接口
func (l *Logger) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 检查是否需要轮转
	if l.size+int64(len(p)) >= l.maxSize {
		if err := l.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = l.file.Write(p)
	l.size += int64(n)
	return n, err
}

// Log 记录日志
func (l *Logger) Log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 格式化日志消息
	now := time.Now().Format("2006-01-02 15:04:05")
	levelName := LogLevelNames[level]
	msg := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] [%s] [%s] %s\n", now, levelName, l.serviceTag, msg)

	// 检查是否需要轮转
	if l.size+int64(len(logLine)) >= l.maxSize {
		if err := l.rotate(); err != nil {
			fmt.Fprintf(os.Stderr, "日志轮转失败: %v\n", err)
			return
		}
	}

	// 写入日志
	n, err := l.file.WriteString(logLine)
	if err != nil {
		fmt.Fprintf(os.Stderr, "写入日志失败: %v\n", err)
		return
	}
	l.size += int64(n)

	// 广播日志给所有订阅者
	l.Broadcast(logLine)
}

// Debug 记录调试级别日志
func (l *Logger) Debug(format string, args ...interface{}) {
	l.Log(DEBUG, format, args...)
}

// Info 记录信息级别日志
func (l *Logger) Info(format string, args ...interface{}) {
	l.Log(INFO, format, args...)
}

// Warn 记录警告级别日志
func (l *Logger) Warn(format string, args ...interface{}) {
	l.Log(WARN, format, args...)
}

// Error 记录错误级别日志
func (l *Logger) Error(format string, args ...interface{}) {
	l.Log(ERROR, format, args...)
}

// Fatal 记录致命级别日志
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.Log(FATAL, format, args...)
}

// rotate 轮转日志文件
func (l *Logger) rotate() error {
	// 关闭当前文件
	if err := l.file.Close(); err != nil {
		return err
	}

	// 生成轮转后的文件名
	timestamp := time.Now().Format("20060102-150405")
	rotatedName := fmt.Sprintf("%s.%s", l.filename, timestamp)

	// 重命名当前日志文件
	if err := os.Rename(l.filename, rotatedName); err != nil {
		return err
	}

	// 如果需要压缩
	if l.compress {
		go func(source string) {
			// 压缩文件
			if err := compressLogFile(source); err != nil {
				fmt.Fprintf(os.Stderr, "压缩日志文件失败: %v\n", err)
				return
			}
			// 删除原文件
			os.Remove(source)
		}(rotatedName)
	}

	// 清理旧日志文件
	if err := l.cleanup(); err != nil {
		return err
	}

	// 创建新的日志文件
	f, err := os.OpenFile(l.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	l.file = f
	l.size = 0

	return nil
}

// cleanup 清理旧日志文件
func (l *Logger) cleanup() error {
	// 获取日志文件所在目录
	dir := filepath.Dir(l.filename)
	base := filepath.Base(l.filename)

	// 列出目录中的所有文件
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// 筛选出轮转的日志文件
	var logFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), base+".") {
			logFiles = append(logFiles, filepath.Join(dir, file.Name()))
		}
	}

	// 按修改时间排序
	sort.Slice(logFiles, func(i, j int) bool {
		iInfo, _ := os.Stat(logFiles[i])
		jInfo, _ := os.Stat(logFiles[j])
		return iInfo.ModTime().After(jInfo.ModTime())
	})

	// 删除多余的文件
	if len(logFiles) > l.maxFiles {
		for _, file := range logFiles[l.maxFiles:] {
			os.Remove(file)
		}
	}

	return nil
}

// Close 关闭日志管理器
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 关闭所有订阅者
	l.subMu.Lock()
	for subscriber := range l.subscribers {
		subscriber.Close()
	}
	l.subscribers = make(map[LogSubscriber]struct{})
	l.subMu.Unlock()

	// 关闭日志文件
	return l.file.Close()
}

// compressLogFile 压缩日志文件
func compressLogFile(source string) error {
	// 创建压缩文件
	dest := source + ".gz"
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	// 创建gzip写入器
	gw := gzip.NewWriter(out)
	defer gw.Close()

	// 打开源文件
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	// 复制内容
	_, err = io.Copy(gw, in)
	if err != nil {
		return err
	}

	return nil
}

// Subscribe 添加日志订阅者
func (l *Logger) Subscribe(subscriber LogSubscriber) {
	l.subMu.Lock()
	defer l.subMu.Unlock()
	l.subscribers[subscriber] = struct{}{}
}

// Unsubscribe 移除日志订阅者
func (l *Logger) Unsubscribe(subscriber LogSubscriber) {
	l.subMu.Lock()
	defer l.subMu.Unlock()
	delete(l.subscribers, subscriber)
}

// Broadcast 广播日志给所有订阅者
func (l *Logger) Broadcast(logLine string) {
	l.subMu.RLock()
	defer l.subMu.RUnlock()

	for subscriber := range l.subscribers {
		// 使用goroutine避免阻塞
		go func(s LogSubscriber, line string) {
			if err := s.Send(line); err != nil {
				// 如果发送失败，移除订阅者
				l.Unsubscribe(s)
				s.Close()
			}
		}(subscriber, logLine)
	}
}

// GetRecentLogs 获取最近的日志内容
func (l *Logger) GetRecentLogs() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 默认读取最后100行日志
	const maxLines = 100

	// 如果文件不存在，返回空数组
	if l.file == nil {
		return []string{}
	}

	// 获取文件信息
	info, err := l.file.Stat()
	if err != nil {
		return []string{"Error reading log file: " + err.Error()}
	}

	// 打开文件进行读取
	file, err := os.Open(l.filename)
	if err != nil {
		return []string{"Error opening log file: " + err.Error()}
	}
	defer file.Close()

	// 如果文件很小，直接读取全部内容
	if info.Size() < 1024*1024 { // 小于1MB
		content, err := io.ReadAll(file)
		if err != nil {
			return []string{"Error reading log content: " + err.Error()}
		}
		return strings.Split(strings.TrimSpace(string(content)), "\n")
	}

	// 对于大文件，从末尾读取
	var lines []string
	var pos int64 = 0
	var buffer = make([]byte, 4096)
	var lineCount = 0

	// 从文件末尾开始读取
	for pos < info.Size() && lineCount < maxLines {
		// 计算读取位置
		readPos := info.Size() - pos - int64(len(buffer))
		if readPos < 0 {
			buffer = make([]byte, info.Size()-pos)
			readPos = 0
		}

		// 设置读取位置
		_, err := file.Seek(readPos, io.SeekStart)
		if err != nil {
			return append([]string{"Error seeking in log file: " + err.Error()}, lines...)
		}

		// 读取数据
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return append([]string{"Error reading log file: " + err.Error()}, lines...)
		}

		// 处理读取的数据
		content := string(buffer[:n])
		newLines := strings.Split(content, "\n")

		// 添加新行到结果中
		for i := len(newLines) - 1; i >= 0 && lineCount < maxLines; i-- {
			if newLines[i] != "" {
				lines = append([]string{newLines[i]}, lines...)
				lineCount++
			}
		}

		// 更新位置
		pos += int64(n)
	}

	return lines
}
