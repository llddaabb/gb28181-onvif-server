package portcheck

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

// PortInfo 端口占用信息
type PortInfo struct {
	Port      int
	Protocol  string // "tcp" 或 "udp"
	PID       int
	Process   string
	Available bool
}

// CheckPort 检查端口是否被占用
func CheckPort(port int, protocol string) *PortInfo {
	info := &PortInfo{
		Port:     port,
		Protocol: protocol,
	}

	// 尝试绑定端口
	var listener interface{}
	var err error

	switch protocol {
	case "tcp":
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			listener.(net.Listener).Close()
			info.Available = true
			return info
		}
	case "udp":
		var addr *net.UDPAddr
		addr, err = net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
		if err == nil {
			var conn *net.UDPConn
			conn, err = net.ListenUDP("udp", addr)
			if err == nil {
				conn.Close()
				info.Available = true
				return info
			}
		}
	}

	// 端口被占用，获取占用进程信息
	info.Available = false
	info.PID, info.Process = getProcessUsingPort(port, protocol)

	return info
}

// CheckPorts 批量检查端口
func CheckPorts(ports []PortConfig) []*PortInfo {
	var results []*PortInfo
	for _, p := range ports {
		results = append(results, CheckPort(p.Port, p.Protocol))
	}
	return results
}

// PortConfig 端口配置
type PortConfig struct {
	Name     string
	Port     int
	Protocol string
}

// getProcessUsingPort 获取占用端口的进程信息
func getProcessUsingPort(port int, protocol string) (int, string) {
	// 使用 ss 命令 (更现代，性能更好)
	pid, name := getProcessUsingSS(port, protocol)
	if pid > 0 {
		return pid, name
	}

	// 回退到 lsof
	pid, name = getProcessUsingLsof(port, protocol)
	if pid > 0 {
		return pid, name
	}

	// 回退到 /proc 扫描
	return getProcessFromProc(port, protocol)
}

// getProcessUsingSS 使用 ss 命令获取进程信息
func getProcessUsingSS(port int, protocol string) (int, string) {
	// ss -tlnp 或 ss -ulnp
	flag := "-tlnp"
	if protocol == "udp" {
		flag = "-ulnp"
	}

	cmd := exec.Command("ss", flag)
	output, err := cmd.Output()
	if err != nil {
		return 0, ""
	}

	// 解析输出，查找匹配的端口
	// 格式: LISTEN 0 4096 *:5060 *:* users:(("gb28181-server",pid=12345,fd=3))
	lines := strings.Split(string(output), "\n")
	portStr := fmt.Sprintf(":%d", port)

	for _, line := range lines {
		if strings.Contains(line, portStr) {
			// 提取进程信息
			re := regexp.MustCompile(`users:\(\("([^"]+)",pid=(\d+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 3 {
				pid, _ := strconv.Atoi(matches[2])
				return pid, matches[1]
			}
		}
	}

	return 0, ""
}

// getProcessUsingLsof 使用 lsof 命令获取进程信息
func getProcessUsingLsof(port int, protocol string) (int, string) {
	protoFlag := "-iTCP"
	if protocol == "udp" {
		protoFlag = "-iUDP"
	}

	cmd := exec.Command("lsof", protoFlag+fmt.Sprintf(":%d", port), "-sTCP:LISTEN", "-P", "-n")
	if protocol == "udp" {
		cmd = exec.Command("lsof", protoFlag+fmt.Sprintf(":%d", port), "-P", "-n")
	}

	output, err := cmd.Output()
	if err != nil {
		return 0, ""
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return 0, ""
	}

	// 解析第二行（第一行是表头）
	fields := strings.Fields(lines[1])
	if len(fields) >= 2 {
		pid, _ := strconv.Atoi(fields[1])
		return pid, fields[0]
	}

	return 0, ""
}

// getProcessFromProc 从 /proc 目录获取进程信息
func getProcessFromProc(port int, protocol string) (int, string) {
	// 读取 /proc/net/tcp 或 /proc/net/udp
	procFile := "/proc/net/tcp"
	if protocol == "udp" {
		procFile = "/proc/net/udp"
	}

	file, err := os.Open(procFile)
	if err != nil {
		return 0, ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	hexPort := fmt.Sprintf("%04X", port)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		// local_address 格式: 00000000:1F90 (IP:Port 十六进制)
		localAddr := fields[1]
		if strings.HasSuffix(localAddr, ":"+hexPort) {
			// 找到匹配的端口，获取 inode
			inode := fields[9]
			return findProcessByInode(inode)
		}
	}

	return 0, ""
}

// findProcessByInode 根据 inode 查找进程
func findProcessByInode(inode string) (int, string) {
	// 遍历 /proc/*/fd/ 查找匹配的 socket
	procs, err := os.ReadDir("/proc")
	if err != nil {
		return 0, ""
	}

	socketLink := "socket:[" + inode + "]"

	for _, proc := range procs {
		if !proc.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(proc.Name())
		if err != nil {
			continue
		}

		fdDir := fmt.Sprintf("/proc/%d/fd", pid)
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}

		for _, fd := range fds {
			link, err := os.Readlink(fmt.Sprintf("%s/%s", fdDir, fd.Name()))
			if err != nil {
				continue
			}

			if link == socketLink {
				// 找到进程，获取进程名
				commPath := fmt.Sprintf("/proc/%d/comm", pid)
				comm, err := os.ReadFile(commPath)
				if err != nil {
					return pid, "unknown"
				}
				return pid, strings.TrimSpace(string(comm))
			}
		}
	}

	return 0, ""
}

// KillProcess 终止进程
func KillProcess(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("无效的 PID: %d", pid)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("找不到进程 %d: %w", pid, err)
	}

	// 先尝试 SIGTERM
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// 如果失败，尝试 SIGKILL
		return process.Signal(syscall.SIGKILL)
	}

	return nil
}

// PromptAction 交互式提示用户选择操作
// 返回: "kill" = 终止进程, "change" = 更改端口, "skip" = 跳过, "abort" = 中止启动
func PromptAction(info *PortInfo, serviceName string) (string, int) {
	fmt.Printf("\n")
	fmt.Printf("╔══════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  ⚠  端口冲突检测                                                  ║\n")
	fmt.Printf("╠══════════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("║  服务: %-58s ║\n", serviceName)
	fmt.Printf("║  端口: %-5d (%s)                                               ║\n", info.Port, strings.ToUpper(info.Protocol))
	if info.PID > 0 {
		fmt.Printf("║  占用进程: %-20s (PID: %-10d)             ║\n", info.Process, info.PID)
	} else {
		fmt.Printf("║  占用进程: 未知                                                   ║\n")
	}
	fmt.Printf("╠══════════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("║  请选择操作:                                                      ║\n")
	if info.PID > 0 {
		fmt.Printf("║    [1] 终止占用进程并继续启动                                     ║\n")
	}
	fmt.Printf("║    [2] 更改端口号                                                 ║\n")
	fmt.Printf("║    [3] 跳过此服务                                                 ║\n")
	fmt.Printf("║    [4] 中止启动                                                   ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════════╝\n")

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("请输入选项 (1-4): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			if info.PID > 0 {
				return "kill", 0
			}
			fmt.Println("无法终止未知进程，请选择其他选项")
		case "2":
			fmt.Printf("请输入新端口号 (1024-65535): ")
			portInput, _ := reader.ReadString('\n')
			portInput = strings.TrimSpace(portInput)
			newPort, err := strconv.Atoi(portInput)
			if err != nil || newPort < 1024 || newPort > 65535 {
				fmt.Println("无效的端口号，请重新输入")
				continue
			}
			// 检查新端口是否可用
			newInfo := CheckPort(newPort, info.Protocol)
			if !newInfo.Available {
				fmt.Printf("端口 %d 也被占用 (进程: %s, PID: %d)，请选择其他端口\n",
					newPort, newInfo.Process, newInfo.PID)
				continue
			}
			return "change", newPort
		case "3":
			return "skip", 0
		case "4":
			return "abort", 0
		default:
			fmt.Println("无效选项，请重新输入")
		}
	}
}

// CheckAndResolve 检查端口并解决冲突
// 返回: 实际使用的端口, 是否跳过该服务, 错误
func CheckAndResolve(serviceName string, port int, protocol string) (int, bool, error) {
	info := CheckPort(port, protocol)
	if info.Available {
		return port, false, nil
	}

	action, newPort := PromptAction(info, serviceName)

	switch action {
	case "kill":
		fmt.Printf("正在终止进程 %s (PID: %d)...\n", info.Process, info.PID)
		if err := KillProcess(info.PID); err != nil {
			return 0, false, fmt.Errorf("终止进程失败: %w", err)
		}
		fmt.Println("✓ 进程已终止")
		// 等待端口释放
		for i := 0; i < 10; i++ {
			newInfo := CheckPort(port, protocol)
			if newInfo.Available {
				return port, false, nil
			}
			fmt.Printf("等待端口释放... (%d/10)\n", i+1)
			// 使用简单的忙等待
			for j := 0; j < 100000000; j++ {
			}
		}
		return 0, false, fmt.Errorf("端口 %d 仍被占用", port)

	case "change":
		fmt.Printf("✓ 将使用新端口: %d\n", newPort)
		return newPort, false, nil

	case "skip":
		fmt.Printf("⚠ 跳过服务: %s\n", serviceName)
		return port, true, nil

	case "abort":
		return 0, false, fmt.Errorf("用户中止启动")
	}

	return port, false, nil
}

// NonInteractiveCheck 非交互式检查（用于后台运行）
func NonInteractiveCheck(serviceName string, port int, protocol string) error {
	info := CheckPort(port, protocol)
	if info.Available {
		return nil
	}

	if info.PID > 0 {
		return fmt.Errorf("%s 端口 %d (%s) 被进程 %s (PID: %d) 占用",
			serviceName, port, protocol, info.Process, info.PID)
	}
	return fmt.Errorf("%s 端口 %d (%s) 被占用", serviceName, port, protocol)
}

// IsInteractive 检查是否在交互式终端中运行
func IsInteractive() bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
