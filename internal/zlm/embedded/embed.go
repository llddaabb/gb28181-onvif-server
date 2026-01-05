//go:build linux
// +build linux

package embedded

import (
	"embed"
)

// MediaServerBinary 嵌入的 MediaServer 可执行文件
//go:embed MediaServer
var MediaServerBinary []byte

// ConfigTemplate 嵌入的配置文件模板
//go:embed config.ini.template
var ConfigTemplate []byte

// WWWFiles 嵌入的 Web 控制台文件
//go:embed www
var WWWFiles embed.FS

// Version 版本信息
//go:embed VERSION
var Version string
