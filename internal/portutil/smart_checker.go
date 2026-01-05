package portutil

import (
	"fmt"
	"log"
	"os"
	"time"
)

// PortAllocation 端口分配结果
type PortAllocation struct {
	RequestedPort int    // 请求的原始端口
	AllocatedPort int    // 实际分配的端口
	IsAvailable   bool   // 原始端口是否可用
	Occupier      string // 占用端口的进程名 (如果被占用)
	OccupierPID   int    // 占用端口的进程ID (如果被占用)
	WasKilled     bool   // 是否杀死了占用进程
	KilledReason  string // 杀死原因
}

// SmartPortChecker 智能端口检查器
// 如果端口被占用，自动使用+1的端口，除非占用进程是自己的旧进程
type SmartPortChecker struct {
	currentPID        int
	appName           string                 // 应用名称，用于识别自己的旧进程
	maxRetries        int                    // 最大重试次数（防止无限循环）
	allocations       map[int]PortAllocation // 端口分配结果缓存
	previousProcesses map[string]struct{}    // 之前运行过的进程ID记录
	forceKill         bool                   // 是否强制杀死所有占用端口的进程（包括其他程序）
}

// NewSmartPortChecker 创建智能端口检查器
func NewSmartPortChecker(appName string) *SmartPortChecker {
	return &SmartPortChecker{
		currentPID:        os.Getpid(),
		appName:           appName,
		maxRetries:        10, // 最多尝试原端口+10的端口
		allocations:       make(map[int]PortAllocation),
		previousProcesses: make(map[string]struct{}),
		forceKill:         false,
	}
}

// SetForceKill 设置是否强制杀死所有占用端口的进程
func (spc *SmartPortChecker) SetForceKill(force bool) {
	spc.forceKill = force
}

// AllocatePort 分配一个可用端口
// 如果指定端口可用，返回该端口；否则返回port+1, port+2...直到找到可用端口
// 如果占用端口的是自己的旧进程，先清理掉再使用原端口
func (spc *SmartPortChecker) AllocatePort(port int) PortAllocation {
	allocation := PortAllocation{
		RequestedPort: port,
		AllocatedPort: port,
	}

	// 检查缓存
	if cached, exists := spc.allocations[port]; exists {
		return cached
	}

	// 尝试分配端口
	for i := 0; i < spc.maxRetries; i++ {
		currentPort := port + i
		infos := getProcessByPort(currentPort)

		if len(infos) == 0 {
			// 端口可用
			allocation.AllocatedPort = currentPort
			allocation.IsAvailable = true
			if i == 0 {
				log.Printf("[智能端口] 端口 %d 可用", currentPort)
			} else {
				log.Printf("[智能端口] 端口 %d 被占用，已分配端口 %d", port, currentPort)
			}
			spc.allocations[port] = allocation
			return allocation
		}

		// 端口被占用
		info := infos[0]
		allocation.OccupierPID = info.PID
		allocation.Occupier = info.Process

		if i == 0 {
			// 第一次检查：检查是否需要清理进程
			shouldKill := false
			killReason := ""

			if spc.isOwnProcess(info.Process, info.PID) {
				// 是自己的旧进程，必须清理
				shouldKill = true
				killReason = "自己的旧进程"
			} else if spc.forceKill {
				// 强制清理模式：清理所有占用进程
				shouldKill = true
				killReason = "强制清理占用端口的进程"
			}

			if shouldKill {
				allocation.WasKilled = true
				allocation.KilledReason = killReason
				log.Printf("[智能端口] 端口 %d 被进程占用 (PID=%d, 进程=%s)，正在清理 (%s)...",
					currentPort, info.PID, info.Process, killReason)

				if err := killProcess(info.PID); err != nil {
					log.Printf("[智能端口] ⚠ 杀死进程 %d 失败: %v, 将尝试其他端口", info.PID, err)
					// 继续尝试下一个端口
					continue
				}

				log.Printf("[智能端口] ✓ 已清理进程，继续尝试端口 %d", currentPort)
				// 等待端口释放
				time.Sleep(200 * time.Millisecond)

				// 再次检查该端口
				infos = getProcessByPort(currentPort)
				if len(infos) == 0 {
					allocation.AllocatedPort = currentPort
					allocation.IsAvailable = true
					spc.allocations[port] = allocation
					return allocation
				}
			}
		}

		// 端口被其他进程占用，不清理，尝试下一个端口
		log.Printf("[智能端口] 端口 %d 被其他进程占用: PID=%d, 进程=%s (不清理)，尝试端口 %d",
			currentPort, info.PID, info.Process, port+i+1)
	}

	// 无法找到可用端口
	allocation.IsAvailable = false
	allocation.AllocatedPort = -1
	allocation.KilledReason = fmt.Sprintf("无可用端口（已尝试%d个端口）", spc.maxRetries)
	log.Printf("[智能端口] ❌ 无法为端口 %d 分配可用端口，已尝试 %d 个端口", port, spc.maxRetries)

	spc.allocations[port] = allocation
	return allocation
}

// AllocatePorts 批量分配端口
func (spc *SmartPortChecker) AllocatePorts(ports []int) map[int]PortAllocation {
	result := make(map[int]PortAllocation)
	for _, port := range ports {
		result[port] = spc.AllocatePort(port)
	}
	return result
}

// isOwnProcess 判断是否是自己程序的旧进程
func (spc *SmartPortChecker) isOwnProcess(processName string, pid int) bool {
	// 检查进程名是否包含应用名
	// 例如: 应用名是 "server"，进程名可能是 "server" 或 "./bin/server"
	if processName == spc.appName ||
		processName == "./"+spc.appName ||
		processName == "bin/"+spc.appName ||
		processName == "./bin/"+spc.appName {
		return true
	}

	// 检查是否在之前记录的进程列表中
	pidStr := string(rune(pid))
	if _, exists := spc.previousProcesses[pidStr]; exists {
		return true
	}

	return false
}

// RecordPreviousProcess 记录之前运行的进程ID（用于下次启动时识别）
func (spc *SmartPortChecker) RecordPreviousProcess(pid int) {
	pidStr := fmt.Sprintf("%d", pid)
	spc.previousProcesses[pidStr] = struct{}{}
}

// GetAllocation 获取指定端口的分配结果
func (spc *SmartPortChecker) GetAllocation(requestedPort int) (PortAllocation, bool) {
	alloc, exists := spc.allocations[requestedPort]
	return alloc, exists
}

// GetAllocatedPort 获取实际分配的端口（简便函数）
func (spc *SmartPortChecker) GetAllocatedPort(requestedPort int) int {
	if alloc, exists := spc.allocations[requestedPort]; exists {
		return alloc.AllocatedPort
	}
	return -1
}

// WaitForPortsAvailable 等待所有分配的端口都可用
// timeout: 最大等待时间
// 返回是否在超时时间内所有端口都可用
func (spc *SmartPortChecker) WaitForPortsAvailable(timeout time.Duration) bool {
	startTime := time.Now()
	checkInterval := 100 * time.Millisecond // 检查间隔

	for time.Since(startTime) < timeout {
		allAvailable := true

		// 检查所有分配的端口
		for _, alloc := range spc.allocations {
			// 跳过未成功分配的端口
			if !alloc.IsAvailable || alloc.AllocatedPort <= 0 {
				continue
			}

			// 检查端口是否真的可用
			infos := getProcessByPort(alloc.AllocatedPort)
			if len(infos) > 0 {
				allAvailable = false
				break
			}
		}

		if allAvailable {
			log.Printf("[智能端口] ✓ 所有端口已释放，耗时: %v", time.Since(startTime))
			return true
		}

		// 等待一段时间后再次检查
		time.Sleep(checkInterval)
	}

	log.Printf("[智能端口] ⚠ 等待端口释放超时 (%v)", timeout)
	return false
}

// IsPortAvailable 检查原始端口是否可用
func (spc *SmartPortChecker) IsPortAvailable(port int) bool {
	if alloc, exists := spc.allocations[port]; exists {
		return alloc.IsAvailable
	}
	return false
}

// PrintAllocationSummary 打印分配摘要
func (spc *SmartPortChecker) PrintAllocationSummary() {
	log.Println("╔═══════════════════════════════════════════════════╗")
	log.Println("║          端口分配结果                            ║")
	log.Println("╚═══════════════════════════════════════════════════╝")

	for requested, alloc := range spc.allocations {
		if alloc.AllocatedPort == requested {
			if alloc.IsAvailable {
				log.Printf("[✓] 端口 %d 可用 → 使用端口 %d", requested, alloc.AllocatedPort)
			} else {
				log.Printf("[✗] 端口 %d 分配失败", requested)
			}
		} else {
			if alloc.WasKilled {
				log.Printf("[⚠] 端口 %d 被占用 (进程: %s, PID: %d) → 清理后使用端口 %d",
					requested, alloc.Occupier, alloc.OccupierPID, alloc.AllocatedPort)
			} else {
				log.Printf("[⚠] 端口 %d 被占用 (进程: %s, PID: %d) → 使用端口 %d (未清理)",
					requested, alloc.Occupier, alloc.OccupierPID, alloc.AllocatedPort)
			}
		}
	}
	log.Println("╚═══════════════════════════════════════════════════╝")
}

// getProcessByPort 获取占用指定端口的进程（内部辅助函数）
func getProcessByPort(port int) []PortInfo {
	pc := NewPortChecker([]int{port})
	return pc.findProcessByPort(port)
}

// killProcess 杀死进程（内部辅助函数）
func killProcess(pid int) error {
	pc := &PortChecker{}
	return pc.killProcess(pid)
}
