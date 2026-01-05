//go:build linux
// +build linux

package embedded

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// 嵌入的 ZLMediaKit 文件
// 注意: 这些文件由 scripts/build_zlm.sh 脚本生成
// 变量声明在 embed.go 中 (通过 go:embed 指令)

// EmbeddedZLM 嵌入式 ZLM 管理器
type EmbeddedZLM struct {
	extractDir string
	binPath    string
	configPath string
	wwwPath    string
	extracted  bool
	mutex      sync.Mutex
}

// EmbedEnabled 是否启用嵌入式 ZLM (在 embed.go 的 init() 中设置为 true)
var EmbedEnabled = false

func init() {
	// 自动启用嵌入式 ZLM，只要可执行文件已通过 scripts/build_zlm.sh 嵌入
	if len(MediaServerBinary) > 0 {
		EmbedEnabled = true
	}
}

// NewEmbeddedZLM 创建嵌入式 ZLM 管理器
func NewEmbeddedZLM(extractDir string) *EmbeddedZLM {
	if extractDir == "" {
		extractDir = filepath.Join(os.TempDir(), "zlm-embedded")
	}
	// 转换为绝对路径
	absPath, err := filepath.Abs(extractDir)
	if err == nil {
		extractDir = absPath
	}
	return &EmbeddedZLM{
		extractDir: extractDir,
	}
}

// IsAvailable 检查嵌入式 ZLM 是否可用
func (e *EmbeddedZLM) IsAvailable() bool {
	return EmbedEnabled && len(MediaServerBinary) > 0
}

// Extract 释放嵌入的文件到指定目录
func (e *EmbeddedZLM) Extract() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.extracted {
		return nil
	}

	if !e.IsAvailable() {
		return fmt.Errorf("嵌入式 ZLM 不可用，请先运行 scripts/build_zlm.sh 编译")
	}

	// 创建目录结构
	dirs := []string{
		e.extractDir,
		filepath.Join(e.extractDir, "conf"),
		filepath.Join(e.extractDir, "log"),
		filepath.Join(e.extractDir, "www"),
		filepath.Join(e.extractDir, "recordings"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败 %s: %w", dir, err)
		}
	}

	// 释放可执行文件
	e.binPath = filepath.Join(e.extractDir, "MediaServer")
	if err := e.writeFile(e.binPath, MediaServerBinary, 0755); err != nil {
		return fmt.Errorf("释放 MediaServer 失败: %w", err)
	}

	// 释放配置文件
	e.configPath = filepath.Join(e.extractDir, "conf", "config.ini")
	if len(ConfigTemplate) > 0 {
		if err := e.writeFile(e.configPath, ConfigTemplate, 0644); err != nil {
			return fmt.Errorf("释放配置文件失败: %w", err)
		}
	}

	// 释放 www 目录
	e.wwwPath = filepath.Join(e.extractDir, "www")
	if err := e.extractEmbedFS(WWWFiles, "www", e.wwwPath); err != nil {
		// www 目录不是必需的，只记录警告
		fmt.Printf("警告: 释放 www 目录失败: %v\n", err)
	}

	e.extracted = true
	return nil
}

// writeFile 写入文件
func (e *EmbeddedZLM) writeFile(path string, data []byte, perm os.FileMode) error {
	// 检查文件是否已存在且内容相同
	if existing, err := os.ReadFile(path); err == nil {
		if len(existing) == len(data) {
			same := true
			for i := range data {
				if existing[i] != data[i] {
					same = false
					break
				}
			}
			if same {
				return nil // 文件已存在且内容相同
			}
		}
	}

	return os.WriteFile(path, data, perm)
}

// extractEmbedFS 从 embed.FS 释放文件
func (e *EmbeddedZLM) extractEmbedFS(efs embed.FS, srcDir, dstDir string) error {
	return fs.WalkDir(efs, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// 计算目标路径
		relPath, _ := filepath.Rel(srcDir, path)
		dstPath := filepath.Join(dstDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		// 读取源文件
		srcFile, err := efs.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		// 创建目标文件
		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

// GetBinPath 获取可执行文件路径
func (e *EmbeddedZLM) GetBinPath() string {
	return e.binPath
}

// GetConfigPath 获取配置文件路径
func (e *EmbeddedZLM) GetConfigPath() string {
	return e.configPath
}

// WriteConfig 写入配置文件内容
func (e *EmbeddedZLM) WriteConfig(content string) error {
	if e.configPath == "" {
		e.configPath = filepath.Join(e.extractDir, "conf", "config.ini")
	}
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(e.configPath), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	return os.WriteFile(e.configPath, []byte(content), 0644)
}

// GetWorkDir 获取工作目录
func (e *EmbeddedZLM) GetWorkDir() string {
	return e.extractDir
}

// GetWWWPath 获取 www 目录路径
func (e *EmbeddedZLM) GetWWWPath() string {
	return e.wwwPath
}

// Cleanup 清理释放的文件（保留录像目录）
func (e *EmbeddedZLM) Cleanup() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if !e.extracted {
		return nil
	}

	e.extracted = false

	// 只删除可执行文件和日志，保留录像和配置
	// 不再删除整个目录，因为录像需要持久化
	filesToClean := []string{
		filepath.Join(e.extractDir, "MediaServer"),
	}

	dirsToClean := []string{
		filepath.Join(e.extractDir, "log"),
	}

	for _, file := range filesToClean {
		os.Remove(file)
	}

	for _, dir := range dirsToClean {
		os.RemoveAll(dir)
	}

	return nil
}

// GetVersion 获取版本信息
func (e *EmbeddedZLM) GetVersion() string {
	return Version
}
