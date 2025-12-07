package zlm

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"gb28181-onvif-server/internal/zlm/embedded"
)

// ProcessManager ZLM 进程管理器
type ProcessManager struct {
	config        *ProcessConfig
	cmd           *exec.Cmd
	running       bool
	mutex         sync.RWMutex
	stopChan      chan struct{}
	exitChan      chan struct{} // 进程退出通知
	restartCount  int
	lastStart     time.Time
	pid           int
	embeddedZLM   *embedded.EmbeddedZLM // 嵌入式 ZLM 管理器
	configContent string                // 配置文件内容 (由 config.yaml 生成)
	apiClient     *ZLMAPIClient         // ZLM API 客户端
	secret        string                // API 密钥
}

// ProcessConfig ZLM 进程配置
type ProcessConfig struct {
	// ZLM 可执行文件路径
	BinPath string
	// ZLM 配置文件路径
	ConfigPath string
	// ZLM 工作目录
	WorkDir string
	// 日志目录
	LogDir string
	// 是否自动重启
	AutoRestart bool
	// 最大重启次数 (0=无限)
	MaxRestarts int
	// 重启间隔
	RestartDelay time.Duration
	// 健康检查间隔
	HealthCheckInterval time.Duration
	// HTTP API 端口 (用于健康检查)
	HTTPPort int
	// 启动超时
	StartTimeout time.Duration
	// 是否使用 sudo 运行 (用于绑定特权端口如 554, 80)
	UseSudo bool
	// 是否使用嵌入式 ZLM
	UseEmbedded bool
	// 嵌入式 ZLM 释放目录
	EmbeddedExtractDir string
}

// DefaultProcessConfig 默认进程配置
func DefaultProcessConfig() *ProcessConfig {
	return &ProcessConfig{
		BinPath:             "third-party/zlm/bin/MediaServer",
		ConfigPath:          "config.ini",
		WorkDir:             "third-party/zlm/bin",
		LogDir:              "third-party/zlm/log",
		AutoRestart:         false, // 默认不自动重启，避免错误时频繁重启
		MaxRestarts:         3,
		RestartDelay:        3 * time.Second,
		HealthCheckInterval: 10 * time.Second,
		HTTPPort:            8080, // ZLM HTTP 端口（非特权端口）
		StartTimeout:        15 * time.Second,
		UseSudo:             false, // 默认不使用 sudo（使用非特权端口）
		UseEmbedded:         true,  // 默认优先使用嵌入式 ZLM
		EmbeddedExtractDir:  "",    // 空则使用临时目录
	}
}

// NewProcessManager 创建进程管理器
func NewProcessManager(config *ProcessConfig) *ProcessManager {
	if config == nil {
		config = DefaultProcessConfig()
	}

	pm := &ProcessManager{
		config:   config,
		stopChan: make(chan struct{}),
		exitChan: make(chan struct{}),
	}

	// 如果启用嵌入式模式，创建嵌入式管理器
	if config.UseEmbedded {
		pm.embeddedZLM = embedded.NewEmbeddedZLM(config.EmbeddedExtractDir)
	}

	return pm
}

// SetConfigContent 设置 ZLM 配置文件内容 (由 config.yaml 生成的 INI 格式)
func (pm *ProcessManager) SetConfigContent(content string) {
	pm.configContent = content
}

// SetSecret 设置 API 密钥
func (pm *ProcessManager) SetSecret(secret string) {
	pm.secret = secret
}

// GetAPIClient 获取 ZLM API 客户端
func (pm *ProcessManager) GetAPIClient() *ZLMAPIClient {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	// 如果客户端不存在或进程刚启动，创建新客户端
	if pm.apiClient == nil && pm.running {
		baseURL := fmt.Sprintf("http://127.0.0.1:%d", pm.config.HTTPPort)
		pm.mutex.RUnlock()
		pm.mutex.Lock()
		pm.apiClient = NewZLMAPIClient(baseURL, WithSecret(pm.secret))
		pm.mutex.Unlock()
		pm.mutex.RLock()
	}

	return pm.apiClient
}

// Start 启动 ZLM 进程
func (pm *ProcessManager) Start() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if pm.running {
		return fmt.Errorf("ZLM 进程已在运行")
	}

	// 检查可执行文件是否存在
	binPath, err := pm.findExecutable()
	if err != nil {
		return fmt.Errorf("ZLM 可执行文件不存在: %v\n请先运行 make build-zlm 编译 ZLM 或使用 --no-zlm 禁用", err)
	}

	// 确保日志目录存在
	if err := os.MkdirAll(pm.config.LogDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}

	// 构建命令
	var cmd *exec.Cmd
	if pm.config.UseSudo {
		// 使用 sudo 运行，保持环境变量
		cmd = exec.Command("sudo", "-E", binPath, "-c", pm.config.ConfigPath)
		log.Printf("[ZLM] 使用 sudo 权限启动 ZLM...")
	} else {
		cmd = exec.Command(binPath, "-c", pm.config.ConfigPath)
	}
	pm.cmd = cmd
	pm.cmd.Dir = pm.config.WorkDir

	// 设置环境变量
	pm.cmd.Env = append(os.Environ(),
		"LD_LIBRARY_PATH="+filepath.Dir(binPath)+":"+os.Getenv("LD_LIBRARY_PATH"),
	)

	// 捕获输出
	stdout, err := pm.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("获取stdout失败: %v", err)
	}
	stderr, err := pm.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("获取stderr失败: %v", err)
	}

	// 启动进程
	if err := pm.cmd.Start(); err != nil {
		return fmt.Errorf("启动 ZLM 进程失败: %v", err)
	}

	pm.pid = pm.cmd.Process.Pid
	pm.running = true
	pm.lastStart = time.Now()
	pm.restartCount++
	pm.exitChan = make(chan struct{}) // 重新创建退出通道

	log.Printf("[ZLM] ✓ ZLM 进程启动成功 (PID: %d)", pm.pid)

	// 异步读取日志
	go pm.readOutput("stdout", stdout)
	go pm.readOutput("stderr", stderr)

	// 等待进程退出
	go pm.waitProcess()

	// 启动健康检查
	go pm.healthCheck()

	// 异步等待服务就绪（不阻塞主程序启动）
	go func() {
		if err := pm.waitReady(); err != nil {
			log.Printf("[ZLM] ⚠ 服务可能未完全就绪: %v", err)
		} else {
			log.Printf("[ZLM] ✓ ZLM HTTP API 就绪 (端口: %d)", pm.config.HTTPPort)
		}
	}()

	return nil
}

// findExecutable 查找可执行文件
func (pm *ProcessManager) findExecutable() (string, error) {
	// 优先使用嵌入式 ZLM
	if pm.embeddedZLM != nil && pm.embeddedZLM.IsAvailable() {
		if err := pm.embeddedZLM.Extract(); err != nil {
			log.Printf("[ZLM] 释放嵌入式 ZLM 失败: %v，尝试使用外部文件", err)
		} else {
			binPath := pm.embeddedZLM.GetBinPath()
			if binPath != "" {
				// 如果有从 config.yaml 生成的配置内容，写入配置文件
				if pm.configContent != "" {
					configPath := filepath.Join(pm.embeddedZLM.GetWorkDir(), "conf", "config.ini")
					if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
						log.Printf("[ZLM] 创建配置目录失败: %v", err)
					} else if err := os.WriteFile(configPath, []byte(pm.configContent), 0644); err != nil {
						log.Printf("[ZLM] 写入配置文件失败: %v", err)
					} else {
						log.Printf("[ZLM] 配置文件已从 config.yaml 生成")
					}
				}
				// 更新配置使用嵌入式路径
				pm.config.BinPath = binPath
				pm.config.WorkDir = pm.embeddedZLM.GetWorkDir()
				pm.config.ConfigPath = "conf/config.ini"
				pm.config.LogDir = filepath.Join(pm.embeddedZLM.GetWorkDir(), "log")
				log.Printf("[ZLM] 使用嵌入式 ZLM: %s", binPath)
				return binPath, nil
			}
		}
	}

	// 检查配置的路径
	if pm.config.BinPath != "" {
		absPath, err := filepath.Abs(pm.config.BinPath)
		if err == nil {
			if _, err := os.Stat(absPath); err == nil {
				return absPath, nil
			}
		}
	}

	// 检查常见路径
	possiblePaths := []string{
		"third-party/zlm/bin/MediaServer",
		"third-party/zlm/bin/zlmediakit",
		"internal/zlm/embedded/MediaServer",
		"$THIRD_PARTY_DIR/zlm/bin/MediaServer",
		"/opt/zlm/bin/MediaServer",
		"/usr/local/bin/MediaServer",
	}

	for _, p := range possiblePaths {
		absPath, err := filepath.Abs(os.ExpandEnv(p))
		if err != nil {
			continue
		}
		if _, err := os.Stat(absPath); err == nil {
			return absPath, nil
		}
	}

	return "", fmt.Errorf("未找到 ZLM 可执行文件")
}

// GetWorkDir 获取ZLM工作目录
func (pm *ProcessManager) GetWorkDir() string {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.config.WorkDir
}

// Stop 停止 ZLM 进程
func (pm *ProcessManager) Stop() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if !pm.running || pm.cmd == nil || pm.cmd.Process == nil {
		return nil
	}

	log.Printf("[ZLM] 正在停止 ZLM 进程 (PID: %d)...", pm.pid)

	// 关闭停止通道
	close(pm.stopChan)

	// 如果使用 sudo 启动，需要用 sudo kill
	if pm.config.UseSudo {
		// 使用 sudo pkill 杀死 MediaServer 进程
		exec.Command("sudo", "pkill", "-TERM", "MediaServer").Run()
	} else {
		// 先发送 SIGTERM
		if err := pm.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			log.Printf("[ZLM] 发送 SIGTERM 失败: %v", err)
		}
	}

	// 等待进程退出
	done := make(chan error, 1)
	go func() {
		done <- pm.cmd.Wait()
	}()

	select {
	case <-done:
		log.Printf("[ZLM] ✓ ZLM 进程已正常退出")
	case <-time.After(10 * time.Second):
		// 超时，强制杀死
		log.Printf("[ZLM] 进程未响应 SIGTERM，发送 SIGKILL")
		if pm.config.UseSudo {
			exec.Command("sudo", "pkill", "-KILL", "MediaServer").Run()
		} else {
			pm.cmd.Process.Kill()
		}
	}

	pm.running = false
	pm.stopChan = make(chan struct{})
	return nil
}

// Restart 重启 ZLM 进程
func (pm *ProcessManager) Restart() error {
	log.Printf("[ZLM] 正在重启 ZLM...")
	if err := pm.Stop(); err != nil {
		return fmt.Errorf("停止失败: %v", err)
	}
	time.Sleep(pm.config.RestartDelay)
	return pm.Start()
}

// IsRunning 检查进程是否运行中
func (pm *ProcessManager) IsRunning() bool {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.running
}

// GetPID 获取进程 PID
func (pm *ProcessManager) GetPID() int {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.pid
}

// GetStatus 获取进程状态
func (pm *ProcessManager) GetStatus() map[string]interface{} {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	status := map[string]interface{}{
		"running":      pm.running,
		"pid":          pm.pid,
		"restartCount": pm.restartCount,
		"lastStart":    pm.lastStart.Format(time.RFC3339),
		"uptime":       "",
		"httpPort":     pm.config.HTTPPort,
		"healthy":      false,
	}

	if pm.running && !pm.lastStart.IsZero() {
		status["uptime"] = time.Since(pm.lastStart).String()
		status["healthy"] = pm.checkHealth()
	}

	return status
}

// waitProcess 等待进程退出
func (pm *ProcessManager) waitProcess() {
	err := pm.cmd.Wait()

	pm.mutex.Lock()
	wasRunning := pm.running
	pm.running = false
	// 关闭退出通道，通知其他等待者
	close(pm.exitChan)
	pm.mutex.Unlock()

	if !wasRunning {
		return // 正常停止
	}

	if err != nil {
		log.Printf("[ZLM] ⚠ ZLM 进程异常退出: %v", err)
	} else {
		log.Printf("[ZLM] ZLM 进程已退出")
	}

	// 检查是否需要自动重启
	if pm.config.AutoRestart {
		select {
		case <-pm.stopChan:
			return // 主动停止，不重启
		default:
		}

		if pm.config.MaxRestarts > 0 && pm.restartCount >= pm.config.MaxRestarts {
			log.Printf("[ZLM] ✗ 达到最大重启次数 (%d)，不再重启", pm.config.MaxRestarts)
			return
		}

		log.Printf("[ZLM] %v 后自动重启...", pm.config.RestartDelay)
		time.Sleep(pm.config.RestartDelay)

		if err := pm.Start(); err != nil {
			log.Printf("[ZLM] ✗ 自动重启失败: %v", err)
		}
	}
}

// readOutput 读取进程输出
func (pm *ProcessManager) readOutput(name string, r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("[ZLM-%s] %s", name, line)
	}
}

// waitReady 等待服务就绪
func (pm *ProcessManager) waitReady() error {
	ctx, cancel := context.WithTimeout(context.Background(), pm.config.StartTimeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待服务就绪超时")
		case <-pm.stopChan:
			return fmt.Errorf("进程已停止")
		case <-pm.exitChan:
			return fmt.Errorf("进程已退出")
		case <-ticker.C:
			if pm.checkHealth() {
				return nil
			}
		}
	}
}

// healthCheck 健康检查协程
func (pm *ProcessManager) healthCheck() {
	ticker := time.NewTicker(pm.config.HealthCheckInterval)
	defer ticker.Stop()

	consecutiveFailures := 0
	maxFailures := 3

	for {
		select {
		case <-pm.stopChan:
			return
		case <-ticker.C:
			if !pm.IsRunning() {
				return
			}

			if pm.checkHealth() {
				consecutiveFailures = 0
			} else {
				consecutiveFailures++
				log.Printf("[ZLM] ⚠ 健康检查失败 (%d/%d)", consecutiveFailures, maxFailures)

				if consecutiveFailures >= maxFailures && pm.config.AutoRestart {
					log.Printf("[ZLM] 连续健康检查失败，触发重启")
					go pm.Restart()
					return
				}
			}
		}
	}
}

// checkHealth 执行健康检查
func (pm *ProcessManager) checkHealth() bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/index/api/getServerConfig", pm.config.HTTPPort)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// CheckAndDownload 检查并下载 ZLM (如果不存在)
func (pm *ProcessManager) CheckAndDownload() error {
	// 首先检查嵌入式 ZLM
	if pm.embeddedZLM != nil && pm.embeddedZLM.IsAvailable() {
		log.Printf("[ZLM] 使用嵌入式 ZLM (版本: %s)", pm.embeddedZLM.GetVersion())
		return nil
	}

	_, err := pm.findExecutable()
	if err == nil {
		return nil // 已存在
	}

	log.Printf("[ZLM] ZLM 可执行文件不存在")
	log.Printf("[ZLM] 请运行以下命令编译 ZLM:")
	log.Printf("[ZLM]   make build-zlm")
	log.Printf("[ZLM] 或使用 --no-zlm 参数禁用 ZLM")

	return fmt.Errorf("ZLM 不可用，请先编译")
}

// Cleanup 清理资源
func (pm *ProcessManager) Cleanup() error {
	if pm.embeddedZLM != nil {
		return pm.embeddedZLM.Cleanup()
	}
	return nil
}

// IsEmbedded 检查是否使用嵌入式模式
func (pm *ProcessManager) IsEmbedded() bool {
	return pm.embeddedZLM != nil && pm.embeddedZLM.IsAvailable()
}

// GetSystemInfo 获取系统信息
func GetSystemInfo() map[string]string {
	return map[string]string{
		"os":     runtime.GOOS,
		"arch":   runtime.GOARCH,
		"numCPU": fmt.Sprintf("%d", runtime.NumCPU()),
		"goRoot": runtime.GOROOT(),
	}
}
