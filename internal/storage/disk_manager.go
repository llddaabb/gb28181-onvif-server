package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"gb28181-onvif-server/internal/debug"
)

// DiskStatus 磁盘状态
type DiskStatus string

const (
	DiskStatusOnline  DiskStatus = "online"  // 在线
	DiskStatusOffline DiskStatus = "offline" // 离线
	DiskStatusFull    DiskStatus = "full"    // 已满
	DiskStatusError   DiskStatus = "error"   // 错误
)

// RAIDMode RAID模式
type RAIDMode string

const (
	RAIDModeNone   RAIDMode = "none"   // 无RAID
	RAIDMode0      RAIDMode = "raid0"  // RAID0 (条带化，提高性能)
	RAIDMode1      RAIDMode = "raid1"  // RAID1 (镜像，提高可靠性)
	RAIDModeJBOD   RAIDMode = "jbod"   // JBOD (Just a Bunch Of Disks，逐个填满)
	RAIDModeCustom RAIDMode = "custom" // 自定义
)

// RecycleMode 循环录制模式
type RecycleMode string

const (
	RecycleModeNone    RecycleMode = "none"     // 不循环
	RecycleModeOldest  RecycleMode = "oldest"   // 删除最老的文件
	RecycleModeByTime  RecycleMode = "by_time"  // 按时间删除（保留N天）
	RecycleModeBySize  RecycleMode = "by_size"  // 按大小删除（保留N GB）
	RecycleModeByCount RecycleMode = "by_count" // 按数量删除（保留N个文件）
)

// Disk 磁盘信息
type Disk struct {
	ID          string     `json:"id"`          // 磁盘ID
	Name        string     `json:"name"`        // 磁盘名称
	MountPoint  string     `json:"mountPoint"`  // 挂载点
	TotalSize   uint64     `json:"totalSize"`   // 总容量(字节)
	UsedSize    uint64     `json:"usedSize"`    // 已用容量(字节)
	FreeSize    uint64     `json:"freeSize"`    // 可用容量(字节)
	Status      DiskStatus `json:"status"`      // 状态
	Priority    int        `json:"priority"`    // 优先级(数字越小优先级越高)
	Enabled     bool       `json:"enabled"`     // 是否启用
	LastCheck   time.Time  `json:"lastCheck"`   // 上次检查时间
	DevicePath  string     `json:"devicePath"`  // 设备路径 (如 /dev/sda1)
	FileSystem  string     `json:"fileSystem"`  // 文件系统类型
	Description string     `json:"description"` // 描述
}

// DiskGroup 磁盘组 (用于RAID)
type DiskGroup struct {
	ID          string   `json:"id"`          // 组ID
	Name        string   `json:"name"`        // 组名称
	Mode        RAIDMode `json:"mode"`        // RAID模式
	DiskIDs     []string `json:"diskIds"`     // 包含的磁盘ID列表
	TotalSize   uint64   `json:"totalSize"`   // 总容量
	UsedSize    uint64   `json:"usedSize"`    // 已用容量
	Enabled     bool     `json:"enabled"`     // 是否启用
	Description string   `json:"description"` // 描述
}

// RecyclePolicy 循环录制策略
type RecyclePolicy struct {
	Enabled             bool          `json:"enabled"`             // 是否启用
	Mode                RecycleMode   `json:"mode"`                // 循环模式
	KeepDays            int           `json:"keepDays"`            // 保留天数 (用于by_time模式)
	KeepSizeGB          int           `json:"keepSizeGB"`          // 保留容量GB (用于by_size模式)
	KeepCount           int           `json:"keepCount"`           // 保留文件数 (用于by_count模式)
	MinFreeSpacePercent int           `json:"minFreeSpacePercent"` // 最小剩余空间百分比(触发回收)
	CheckInterval       time.Duration `json:"checkInterval"`       // 检查间隔
}

// DiskManager 磁盘管理器
type DiskManager struct {
	disks         map[string]*Disk
	diskGroups    map[string]*DiskGroup
	recyclePolicy *RecyclePolicy
	recordRootDir string // 录像根目录
	configFile    string // 配置文件路径
	mutex         sync.RWMutex
	stopChan      chan struct{}
	running       bool
}

// NewDiskManager 创建磁盘管理器
func NewDiskManager(recordRootDir, configFile string) *DiskManager {
	return &DiskManager{
		disks:         make(map[string]*Disk),
		diskGroups:    make(map[string]*DiskGroup),
		recordRootDir: recordRootDir,
		configFile:    configFile,
		stopChan:      make(chan struct{}),
		recyclePolicy: &RecyclePolicy{
			Enabled:             true,
			Mode:                RecycleModeOldest,
			MinFreeSpacePercent: 10, // 默认保留10%空间
			CheckInterval:       5 * time.Minute,
		},
	}
}

// Start 启动磁盘管理器
func (dm *DiskManager) Start() error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if dm.running {
		return fmt.Errorf("disk manager already running")
	}

	// 确保录像根目录存在
	if err := os.MkdirAll(dm.recordRootDir, 0755); err != nil {
		return fmt.Errorf("创建录像根目录失败: %w", err)
	}

	// 加载配置
	if err := dm.loadConfig(); err != nil {
		debug.Warn("storage", "加载磁盘配置失败，使用默认配置: %v", err)
	}

	// 初始扫描所有磁盘
	if err := dm.scanDisks(); err != nil {
		return fmt.Errorf("scan disks failed: %w", err)
	}

	dm.running = true

	// 启动监控协程
	go dm.monitorLoop()

	debug.Info("storage", "磁盘管理器已启动，监控 %d 个磁盘", len(dm.disks))
	return nil
}

// Stop 停止磁盘管理器
func (dm *DiskManager) Stop() error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if !dm.running {
		return nil
	}

	close(dm.stopChan)
	dm.running = false

	// 保存配置
	if err := dm.saveConfig(); err != nil {
		debug.Error("storage", "保存磁盘配置失败: %v", err)
	}

	debug.Info("storage", "磁盘管理器已停止")
	return nil
}

// monitorLoop 监控循环
func (dm *DiskManager) monitorLoop() {
	ticker := time.NewTicker(dm.recyclePolicy.CheckInterval)
	defer ticker.Stop()

	debug.Info("storage", "监控循环已启动，检查间隔: %v", dm.recyclePolicy.CheckInterval)

	for {
		select {
		case <-dm.stopChan:
			return
		case <-ticker.C:
			debug.Info("storage", "执行定期检查...")
			// 更新磁盘状态
			dm.updateDiskStatus()

			// 执行循环录制检查
			if dm.recyclePolicy.Enabled {
				debug.Info("storage", "循环录制已启用，模式: %s", dm.recyclePolicy.Mode)
				dm.performRecycle()
			}
		}
	}
}

// scanDisks 扫描磁盘
func (dm *DiskManager) scanDisks() error {
	// 如果已有配置的磁盘，更新它们的状态
	if len(dm.disks) > 0 {
		return dm.updateDiskStatusNoLock()
	}

	// 否则，自动发现磁盘（从录像根目录）
	disk := &Disk{
		ID:         "disk_default",
		Name:       "默认磁盘",
		MountPoint: dm.recordRootDir,
		Priority:   0,
		Enabled:    true,
		FileSystem: "auto",
	}

	if err := dm.updateDiskInfo(disk); err != nil {
		return err
	}

	dm.disks[disk.ID] = disk
	return nil
}

// updateDiskStatus 更新所有磁盘状态（带锁）
func (dm *DiskManager) updateDiskStatus() error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	return dm.updateDiskStatusNoLock()
}

// updateDiskStatusNoLock 更新所有磁盘状态（不加锁，内部使用）
func (dm *DiskManager) updateDiskStatusNoLock() error {
	for _, disk := range dm.disks {
		if err := dm.updateDiskInfo(disk); err != nil {
			debug.Error("storage", "更新磁盘 %s 状态失败: %v", disk.ID, err)
			disk.Status = DiskStatusError
		}
	}
	return nil
}

// updateDiskInfo 更新单个磁盘信息
func (dm *DiskManager) updateDiskInfo(disk *Disk) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(disk.MountPoint, &stat); err != nil {
		return fmt.Errorf("statfs failed: %w", err)
	}

	// 计算容量
	disk.TotalSize = stat.Blocks * uint64(stat.Bsize)
	disk.FreeSize = stat.Bavail * uint64(stat.Bsize)
	disk.UsedSize = disk.TotalSize - disk.FreeSize
	disk.LastCheck = time.Now()

	// 更新状态
	freePercent := float64(disk.FreeSize) / float64(disk.TotalSize) * 100
	if freePercent < float64(dm.recyclePolicy.MinFreeSpacePercent) {
		disk.Status = DiskStatusFull
	} else {
		disk.Status = DiskStatusOnline
	}

	return nil
}

// performRecycle 执行循环录制
func (dm *DiskManager) performRecycle() {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	for _, disk := range dm.disks {
		if !disk.Enabled {
			continue
		}

		// oldest模式只在磁盘满时触发
		if dm.recyclePolicy.Mode == RecycleModeOldest {
			if disk.Status != DiskStatusFull {
				continue
			}
			debug.Info("storage", "磁盘 %s 空间不足，开始执行循环录制", disk.Name)
			dm.deleteOldestFiles(disk)
		} else {
			// 其他模式总是执行（按时间/大小/数量）
			switch dm.recyclePolicy.Mode {
			case RecycleModeByTime:
				dm.deleteFilesByTime(disk, dm.recyclePolicy.KeepDays)
			case RecycleModeBySize:
				dm.deleteFilesBySize(disk, dm.recyclePolicy.KeepSizeGB)
			case RecycleModeByCount:
				dm.deleteFilesByCount(disk, dm.recyclePolicy.KeepCount)
			}
		}

		// 重新检查磁盘状态
		dm.updateDiskInfo(disk)
	}
}

// deleteOldestFiles 删除最老的文件直到释放足够空间
func (dm *DiskManager) deleteOldestFiles(disk *Disk) {
	targetFreePercent := float64(dm.recyclePolicy.MinFreeSpacePercent) + 5.0 // 额外释放5%

	files, err := dm.findRecordingFiles(disk.MountPoint)
	if err != nil {
		debug.Error("storage", "查找录像文件失败: %v", err)
		return
	}

	// 按修改时间排序（最老的在前）
	sort.Slice(files, func(i, j int) bool {
		return files[i].Info.ModTime().Before(files[j].Info.ModTime())
	})

	deletedCount := 0
	deletedSize := uint64(0)

	for _, file := range files {
		// 检查是否达到目标
		freePercent := float64(disk.FreeSize+deletedSize) / float64(disk.TotalSize) * 100
		if freePercent >= targetFreePercent {
			break
		}

		fileSize := uint64(file.Info.Size())

		if err := os.Remove(file.Path); err != nil {
			debug.Error("storage", "删除文件失败 %s: %v", file.Path, err)
			continue
		}

		deletedCount++
		deletedSize += fileSize
		debug.Info("storage", "已删除旧录像: %s (%.2f MB)", file.Info.Name(), float64(fileSize)/(1024*1024))
	}

	debug.Info("storage", "循环录制完成，删除 %d 个文件，释放 %.2f GB",
		deletedCount, float64(deletedSize)/(1024*1024*1024))
}

// deleteFilesByTime 按时间删除文件
func (dm *DiskManager) deleteFilesByTime(disk *Disk, keepDays int) {
	cutoffTime := time.Now().AddDate(0, 0, -keepDays)
	debug.Info("storage", "开始按时间删除文件，保留天数: %d，截止时间: %s", keepDays, cutoffTime.Format("2006-01-02"))

	files, err := dm.findRecordingFiles(disk.MountPoint)
	if err != nil {
		debug.Error("storage", "查找录像文件失败: %v", err)
		return
	}

	debug.Info("storage", "找到 %d 个录像文件", len(files))

	deletedCount := 0
	for _, file := range files {
		if file.Info.ModTime().Before(cutoffTime) {
			if err := os.Remove(file.Path); err != nil {
				debug.Error("storage", "删除文件失败 %s: %v", file.Path, err)
				continue
			}
			deletedCount++
			debug.Info("storage", "已删除过期录像: %s", filepath.Base(file.Path))
		}
	}

	debug.Info("storage", "按时间删除完成，删除 %d 个超过 %d 天的文件", deletedCount, keepDays)
}

// deleteFilesBySize 按大小删除文件
func (dm *DiskManager) deleteFilesBySize(disk *Disk, keepSizeGB int) {
	// 实现类似逻辑
	debug.Info("storage", "按大小删除文件（待实现）")
}

// deleteFilesByCount 按数量删除文件
func (dm *DiskManager) deleteFilesByCount(disk *Disk, keepCount int) {
	// 实现类似逻辑
	debug.Info("storage", "按数量删除文件（待实现）")
}

// findRecordingFiles 查找录像文件
// RecordingFile 录像文件信息
type RecordingFile struct {
	Path string      // 完整路径
	Info os.FileInfo // 文件信息
}

// findRecordingFiles 查找所有录像文件
func (dm *DiskManager) findRecordingFiles(dir string) ([]*RecordingFile, error) {
	var files []*RecordingFile

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误
		}
		if !info.IsDir() && filepath.Ext(path) == ".mp4" {
			files = append(files, &RecordingFile{
				Path: path,
				Info: info,
			})
		}
		return nil
	})

	return files, err
}

// AddDisk 添加磁盘
func (dm *DiskManager) AddDisk(disk *Disk) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if _, exists := dm.disks[disk.ID]; exists {
		return fmt.Errorf("disk %s already exists", disk.ID)
	}

	// 验证挂载点
	if _, err := os.Stat(disk.MountPoint); os.IsNotExist(err) {
		return fmt.Errorf("mount point not found: %s", disk.MountPoint)
	}

	// 更新磁盘信息
	if err := dm.updateDiskInfo(disk); err != nil {
		return err
	}

	dm.disks[disk.ID] = disk
	dm.saveConfig()

	debug.Info("storage", "已添加磁盘: %s (%s)", disk.Name, disk.MountPoint)
	return nil
}

// RemoveDisk 移除磁盘
func (dm *DiskManager) RemoveDisk(diskID string) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if _, exists := dm.disks[diskID]; !exists {
		return fmt.Errorf("disk not found: %s", diskID)
	}

	delete(dm.disks, diskID)
	dm.saveConfig()

	debug.Info("storage", "已移除磁盘: %s", diskID)
	return nil
}

// GetDisks 获取所有磁盘
func (dm *DiskManager) GetDisks() []*Disk {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	disks := make([]*Disk, 0, len(dm.disks))
	for _, disk := range dm.disks {
		disks = append(disks, disk)
	}

	// 按优先级排序
	sort.Slice(disks, func(i, j int) bool {
		return disks[i].Priority < disks[j].Priority
	})

	return disks
}

// GetAvailableDisk 获取可用磁盘（用于写入）
func (dm *DiskManager) GetAvailableDisk() (*Disk, error) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	disks := dm.GetDisks()
	for _, disk := range disks {
		if disk.Enabled && disk.Status == DiskStatusOnline {
			return disk, nil
		}
	}

	return nil, fmt.Errorf("no available disk")
}

// SetRecyclePolicy 设置循环录制策略
func (dm *DiskManager) SetRecyclePolicy(policy *RecyclePolicy) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	dm.recyclePolicy = policy
	dm.saveConfig()
}

// GetRecyclePolicy 获取循环录制策略
func (dm *DiskManager) GetRecyclePolicy() *RecyclePolicy {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	return dm.recyclePolicy
}

// loadConfig 加载配置
func (dm *DiskManager) loadConfig() error {
	if dm.configFile == "" {
		return fmt.Errorf("config file not specified")
	}

	data, err := os.ReadFile(dm.configFile)
	if err != nil {
		return err
	}

	var config struct {
		Disks         map[string]*Disk      `json:"disks"`
		DiskGroups    map[string]*DiskGroup `json:"diskGroups"`
		RecyclePolicy *RecyclePolicy        `json:"recyclePolicy"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	dm.disks = config.Disks
	dm.diskGroups = config.DiskGroups
	if config.RecyclePolicy != nil {
		dm.recyclePolicy = config.RecyclePolicy
	}

	return nil
}

// saveConfig 保存配置
func (dm *DiskManager) saveConfig() error {
	if dm.configFile == "" {
		return nil
	}

	config := map[string]interface{}{
		"disks":         dm.disks,
		"diskGroups":    dm.diskGroups,
		"recyclePolicy": dm.recyclePolicy,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(dm.configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(dm.configFile, data, 0644)
}

// GetDiskStats 获取磁盘统计信息
func (dm *DiskManager) GetDiskStats() map[string]interface{} {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	var totalSize, usedSize, freeSize uint64
	onlineCount := 0

	for _, disk := range dm.disks {
		if disk.Enabled {
			totalSize += disk.TotalSize
			usedSize += disk.UsedSize
			freeSize += disk.FreeSize
			if disk.Status == DiskStatusOnline {
				onlineCount++
			}
		}
	}

	return map[string]interface{}{
		"totalDisks":  len(dm.disks),
		"onlineDisks": onlineCount,
		"totalSize":   totalSize,
		"usedSize":    usedSize,
		"freeSize":    freeSize,
		"usedPercent": float64(usedSize) / float64(totalSize) * 100,
	}
}
