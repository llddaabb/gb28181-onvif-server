package portutil

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// PortInfo 端口占用信息
type PortInfo struct {
	Port    int
	PID     int
	Process string
	Proto   string // tcp 或 udp
}

// PortChecker 端口检查器
type PortChecker struct {
	ports      []int
	autoKill   bool
	skipSelf   bool
	currentPID int
}

// NewPortChecker 创建端口检查器
func NewPortChecker(ports []int) *PortChecker {
	return &PortChecker{
		ports:      ports,
		autoKill:   true,
		skipSelf:   true,
		currentPID: os.Getpid(),
	}
}

// SetAutoKill 设置是否自动杀死占用端口的进程
func (pc *PortChecker) SetAutoKill(autoKill bool) {
	pc.autoKill = autoKill
}

// CheckAndClean 检查端口并清理占用的进程
// 返回被清理的端口信息列表
func (pc *PortChecker) CheckAndClean() []PortInfo {
	var cleaned []PortInfo

	for _, port := range pc.ports {
		infos := pc.findProcessByPort(port)
		for _, info := range infos {
			// 跳过当前进程
			if pc.skipSelf && info.PID == pc.currentPID {
				continue
			}

			log.Printf("[端口检查] 发现端口 %d 被进程占用: PID=%d, 进程名=%s, 协议=%s",
				port, info.PID, info.Process, info.Proto)

			if pc.autoKill {
				if err := pc.killProcess(info.PID); err != nil {
					log.Printf("[端口检查] ⚠ 杀死进程 %d 失败: %v", info.PID, err)
				} else {
					log.Printf("[端口检查] ✓ 已杀死进程 %d (%s)", info.PID, info.Process)
					cleaned = append(cleaned, info)
				}
			}
		}
	}

	return cleaned
}

// CheckPorts 仅检查端口占用情况，不进行清理
func (pc *PortChecker) CheckPorts() map[int][]PortInfo {
	result := make(map[int][]PortInfo)

	for _, port := range pc.ports {
		infos := pc.findProcessByPort(port)
		if len(infos) > 0 {
			result[port] = infos
		}
	}

	return result
}

// findProcessByPort 通过端口查找占用的进程
func (pc *PortChecker) findProcessByPort(port int) []PortInfo {
	var results []PortInfo

	// 方法1: 使用 ss 命令（更现代，更快）
	results = append(results, pc.findBySSCommand(port)...)

	// 如果 ss 没有找到结果，尝试使用 lsof
	if len(results) == 0 {
		results = append(results, pc.findByLsof(port)...)
	}

	// 去重
	return pc.deduplicateInfos(results)
}

// findBySSCommand 使用 ss 命令查找端口占用
func (pc *PortChecker) findBySSCommand(port int) []PortInfo {
	var results []PortInfo

	// 检查 TCP 和 UDP
	for _, proto := range []string{"tcp", "udp"} {
		var args []string
		if proto == "tcp" {
			args = []string{"-tlnp"}
		} else {
			args = []string{"-ulnp"}
		}

		cmd := exec.Command("ss", args...)
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			line := scanner.Text()
			// 查找包含指定端口的行
			portStr := fmt.Sprintf(":%d", port)
			if !strings.Contains(line, portStr) {
				continue
			}

			// 解析行，格式类似:
			// LISTEN 0      4096   0.0.0.0:9080   0.0.0.0:*    users:(("server",pid=12345,fd=8))
			fields := strings.Fields(line)
			if len(fields) < 5 {
				continue
			}

			// 检查本地地址是否包含目标端口
			localAddr := fields[3]
			if !strings.HasSuffix(localAddr, portStr) &&
				!strings.Contains(localAddr, portStr+" ") {
				// 进一步检查
				parts := strings.Split(localAddr, ":")
				if len(parts) > 0 {
					lastPart := parts[len(parts)-1]
					if p, _ := strconv.Atoi(lastPart); p != port {
						continue
					}
				} else {
					continue
				}
			}

			// 解析 PID
			pid := pc.extractPIDFromSSLine(line)
			if pid <= 0 {
				continue
			}

			processName := pc.getProcessName(pid)
			results = append(results, PortInfo{
				Port:    port,
				PID:     pid,
				Process: processName,
				Proto:   proto,
			})
		}
	}

	return results
}

// extractPIDFromSSLine 从 ss 输出行中提取 PID
func (pc *PortChecker) extractPIDFromSSLine(line string) int {
	// 查找 pid=数字 模式
	idx := strings.Index(line, "pid=")
	if idx == -1 {
		return 0
	}

	pidStr := line[idx+4:]
	endIdx := strings.IndexAny(pidStr, ",)")
	if endIdx != -1 {
		pidStr = pidStr[:endIdx]
	}

	pid, _ := strconv.Atoi(pidStr)
	return pid
}

// findByLsof 使用 lsof 命令查找端口占用
func (pc *PortChecker) findByLsof(port int) []PortInfo {
	var results []PortInfo

	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-n", "-P")
	output, err := cmd.Output()
	if err != nil {
		return results
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	firstLine := true
	for scanner.Scan() {
		line := scanner.Text()
		// 跳过标题行
		if firstLine {
			firstLine = false
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		processName := fields[0]
		pid, _ := strconv.Atoi(fields[1])
		if pid <= 0 {
			continue
		}

		// 判断协议
		proto := "tcp"
		if strings.Contains(strings.ToLower(fields[7]), "udp") {
			proto = "udp"
		}

		results = append(results, PortInfo{
			Port:    port,
			PID:     pid,
			Process: processName,
			Proto:   proto,
		})
	}

	return results
}

// getProcessName 获取进程名称
func (pc *PortChecker) getProcessName(pid int) string {
	// 读取 /proc/[pid]/comm
	commPath := fmt.Sprintf("/proc/%d/comm", pid)
	data, err := os.ReadFile(commPath)
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}

// isProcessRunning 检查进程是否还在运行
func (pc *PortChecker) isProcessRunning(pid int) bool {
	// 检查 /proc/[pid] 目录是否存在
	procPath := fmt.Sprintf("/proc/%d", pid)
	_, err := os.Stat(procPath)
	return err == nil
}

// killProcess 杀死进程，先尝试 SIGTERM，等待后若仍存在则使用 SIGKILL
func (pc *PortChecker) killProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("找不到进程: %v", err)
	}

	// 先发送 SIGTERM 让进程优雅退出
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// 进程可能已经不存在
		if !pc.isProcessRunning(pid) {
			return nil
		}
		// 直接尝试 SIGKILL
		return pc.forceKill(process, pid)
	}

	// 等待进程退出 (最多等待 3 秒)
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if !pc.isProcessRunning(pid) {
			return nil // 进程已退出
		}
	}

	// 进程仍在运行，发送 SIGKILL
	log.Printf("[端口检查] 进程 %d 未响应 SIGTERM，发送 SIGKILL", pid)
	return pc.forceKill(process, pid)
}

// forceKill 强制杀死进程
func (pc *PortChecker) forceKill(process *os.Process, pid int) error {
	if err := process.Signal(syscall.SIGKILL); err != nil {
		if !pc.isProcessRunning(pid) {
			return nil // 进程已退出
		}
		return fmt.Errorf("SIGKILL 失败: %v", err)
	}

	// 等待进程退出
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		if !pc.isProcessRunning(pid) {
			return nil
		}
	}

	return fmt.Errorf("进程 %d 无法终止", pid)
}

// deduplicateInfos 去重端口信息
func (pc *PortChecker) deduplicateInfos(infos []PortInfo) []PortInfo {
	seen := make(map[string]bool)
	var result []PortInfo

	for _, info := range infos {
		key := fmt.Sprintf("%d-%d-%s", info.Port, info.PID, info.Proto)
		if !seen[key] {
			seen[key] = true
			result = append(result, info)
		}
	}

	return result
}

// CheckAndCleanPorts 便捷函数：检查并清理指定端口
func CheckAndCleanPorts(ports []int) []PortInfo {
	checker := NewPortChecker(ports)
	return checker.CheckAndClean()
}

// GetPortOccupancy 便捷函数：获取端口占用信息
func GetPortOccupancy(ports []int) map[int][]PortInfo {
	checker := NewPortChecker(ports)
	checker.SetAutoKill(false)
	return checker.CheckPorts()
}
