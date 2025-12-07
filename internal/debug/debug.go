package debug

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

// DebugConfig 调试配置结构体
type DebugConfig struct {
	Enabled    bool     `yaml:"enabled"`
	LogLevel   string   `yaml:"log_level"` // debug, info, warn, error
	LogFile    string   `yaml:"log_file"`
	Services   []string `yaml:"services"` // 要调试的服务列表
	Timestamp  bool     `yaml:"timestamp"`
	CallerInfo bool     `yaml:"caller_info"`
}

// Debugger 调试器结构体
type Debugger struct {
	config *DebugConfig
	logger *log.Logger
	file   *os.File
}

// NewDebugger 创建调试器实例
func NewDebugger(config *DebugConfig) (*Debugger, error) {
	d := &Debugger{
		config: config,
	}

	// 设置日志输出
	var output = os.Stdout
	if config.LogFile != "" {
		file, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("打开日志文件失败: %w", err)
		}
		d.file = file
		output = file
	}

	d.logger = log.New(output, "", 0)
	return d, nil
}

// Close 关闭调试器
func (d *Debugger) Close() error {
	if d.file != nil {
		return d.file.Close()
	}
	return nil
}

// shouldLog 检查是否应该记录日志
func (d *Debugger) shouldLog(level, service string) bool {
	if !d.config.Enabled {
		return false
	}

	// 检查服务是否在调试列表中
	if len(d.config.Services) > 0 {
		serviceFound := false
		for _, s := range d.config.Services {
			if s == service || s == "*" {
				serviceFound = true
				break
			}
		}
		if !serviceFound {
			return false
		}
	}

	// 检查日志级别
	return d.isLevelEnabled(level)
}

// isLevelEnabled 检查日志级别是否启用
func (d *Debugger) isLevelEnabled(level string) bool {
	levels := map[string]int{
		"debug": 4,
		"info":  3,
		"warn":  2,
		"error": 1,
	}

	currentLevel, ok := levels[d.config.LogLevel]
	if !ok {
		currentLevel = 2 // 默认warn级别
	}

	logLevel, ok := levels[level]
	if !ok {
		logLevel = 3 // 默认info级别
	}

	return logLevel <= currentLevel
}

// formatMessage 格式化消息
func (d *Debugger) formatMessage(level, service, message string) string {
	var parts []string

	// 添加时间戳
	if d.config.Timestamp {
		parts = append(parts, time.Now().Format("2006-01-02 15:04:05"))
	}

	// 添加日志级别
	parts = append(parts, fmt.Sprintf("[%s]", strings.ToUpper(level)))

	// 添加服务名称
	parts = append(parts, fmt.Sprintf("[%s]", service))

	// 添加调用者信息
	if d.config.CallerInfo {
		if pc, file, line, ok := runtime.Caller(3); ok {
			funcName := runtime.FuncForPC(pc).Name()
			// 简化文件路径
			if idx := strings.LastIndex(file, "/"); idx >= 0 {
				file = file[idx+1:]
			}
			if idx := strings.LastIndex(funcName, "/"); idx >= 0 {
				funcName = funcName[idx+1:]
			}
			parts = append(parts, fmt.Sprintf("[%s:%d %s]", file, line, funcName))
		}
	}

	// 添加消息内容
	parts = append(parts, message)

	return strings.Join(parts, " ")
}

// Debug 调试级别日志
func (d *Debugger) Debug(service, format string, args ...interface{}) {
	if d.shouldLog("debug", service) {
		message := fmt.Sprintf(format, args...)
		d.logger.Println(d.formatMessage("debug", service, message))
	}
}

// Info 信息级别日志
func (d *Debugger) Info(service, format string, args ...interface{}) {
	if d.shouldLog("info", service) {
		message := fmt.Sprintf(format, args...)
		d.logger.Println(d.formatMessage("info", service, message))
	}
}

// Warn 警告级别日志
func (d *Debugger) Warn(service, format string, args ...interface{}) {
	if d.shouldLog("warn", service) {
		message := fmt.Sprintf(format, args...)
		d.logger.Println(d.formatMessage("warn", service, message))
	}
}

// Error 错误级别日志
func (d *Debugger) Error(service, format string, args ...interface{}) {
	if d.shouldLog("error", service) {
		message := fmt.Sprintf(format, args...)
		d.logger.Println(d.formatMessage("error", service, message))
	}
}

// JSON 输出JSON格式的调试信息
func (d *Debugger) JSON(service, level string, data interface{}) {
	if d.shouldLog(level, service) {
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			d.Error(service, "JSON序列化失败: %v", err)
			return
		}
		d.logger.Println(d.formatMessage(level, service, string(jsonData)))
	}
}

// GlobalDebugger 全局调试器实例
var GlobalDebugger *Debugger

// InitGlobalDebugger 初始化全局调试器
func InitGlobalDebugger(config *DebugConfig) error {
	debugger, err := NewDebugger(config)
	if err != nil {
		return err
	}
	GlobalDebugger = debugger
	return nil
}

// GetGlobalDebugger 获取全局调试器
func GetGlobalDebugger() *Debugger {
	return GlobalDebugger
}

// Debug 全局调试函数
func Debug(service, format string, args ...interface{}) {
	if GlobalDebugger != nil {
		GlobalDebugger.Debug(service, format, args...)
	}
}

// Info 全局信息函数
func Info(service, format string, args ...interface{}) {
	if GlobalDebugger != nil {
		GlobalDebugger.Info(service, format, args...)
	}
}

// Warn 全局警告函数
func Warn(service, format string, args ...interface{}) {
	if GlobalDebugger != nil {
		GlobalDebugger.Warn(service, format, args...)
	}
}

// Error 全局错误函数
func Error(service, format string, args ...interface{}) {
	if GlobalDebugger != nil {
		GlobalDebugger.Error(service, format, args...)
	}
}

// JSON 全局JSON函数
func JSON(service, level string, data interface{}) {
	if GlobalDebugger != nil {
		GlobalDebugger.JSON(service, level, data)
	}
}
