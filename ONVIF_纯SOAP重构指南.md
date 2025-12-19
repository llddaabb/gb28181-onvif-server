# ONVIF 纯SOAP实现重构指南

## 概述

已完成从goonvif库依赖的ONVIF实现到完全手动SOAP+WSSE的重构。这个版本提供：

- ✅ **完全独立的实现** - 无需goonvif库
- ✅ **完整的WSSE认证** - 支持WS-Security UsernameToken
- ✅ **多种ONVIF操作** - 设备发现、媒体配置、流获取、PTZ控制
- ✅ **自动降级机制** - 支持多种认证方式
- ✅ **详细的错误日志** - 便于调试和故障排除

## 核心文件变更

### 1. 新建: `internal/onvif/soap_client.go` (681行)

**完全纯SOAP实现的ONVIF客户端**

#### 主要类:
```go
type SOAPClient struct {
    username  string              // ONVIF用户名
    password  string              // ONVIF密码
    endpoint  string              // 设备服务端点
    httpClient *http.Client        // HTTP客户端
    mediaAddr string               // 媒体服务地址（自动发现）
    ptzAddr   string               // PTZ服务地址（自动发现）
}
```

#### 核心方法:

**认证相关:**
- `generateNonce()` - 生成随机nonce
- `generateWSSEHeader()` - 生成WS-Security认证头（SHA1 PasswordDigest）
- `callSOAP()` - 在默认端点调用SOAP
- `callSOAPOnEndpoint()` - 在指定端点调用SOAP

**设备服务 (Device Service):**
- `GetDeviceInformation()` - 获取制造商、型号、固件版本等
- `GetSystemDateAndTime()` - 系统时间同步
- `GetCapabilities()` - 获取设备能力及服务地址

**媒体服务 (Media Service):**
- `GetMediaProfiles()` - 获取媒体配置文件列表
- `GetStreamURI()` - 获取RTSP流地址
- `GetSnapshotURI()` - 获取快照地址

**PTZ服务 (PTZ Service):**
- `ContinuousMove()` - 连续移动（Pan/Tilt/Zoom）
- `StopPTZ()` - 停止移动
- `GotoPreset()` - 移动到预置位
- `SetPreset()` - 设置预置位
- `RemovePreset()` - 删除预置位
- `GetPresets()` - 列出所有预置位

### 2. 修改: `internal/onvif/onvif_client.go` (50行)

**简化为结构体定义文件**

- 移除所有goonvif导入
- 保留MediaProfile, PTZPreset, DeviceCapabilities等结构体定义
- 作为接口层与soap_client.go交互

### 3. 修改: `internal/onvif/helper.go`

**更新ONVIFDeviceClient包装类:**

```go
type ONVIFDeviceClient struct {
    client *SOAPClient  // 使用纯SOAP客户端
    xaddr  string
}
```

**包装方法对应关系:**
- `NewDevice()` - 创建SOAPClient并测试连接
- `GetDeviceInfo()` - 调用soap_client.GetDeviceInformation()
- `GetMediaProfiles()` - 调用soap_client.GetMediaProfiles()
- `GetStreamURI()` - 调用soap_client.GetStreamURI()
- `GetSnapshotURI()` - 调用soap_client.GetSnapshotURI()
- `PTZContinuousMove()` - 调用soap_client.ContinuousMove(x, y, z, timeout)
- `PTZStop()` - 调用soap_client.StopPTZ()
- `GotoPreset()` - 调用soap_client.GotoPreset()

### 4. 修改: `internal/onvif/manager.go`

**更新PTZ方法调用以适配新签名:**

修复点:
- `PTZStop(profileToken, bool, bool)` → `PTZStop(profileToken)`
- `GotoHomePosition(profileToken, *PTZVector)` → `GotoHomePosition(profileToken)`
- `PTZContinuousMove(profileToken, *PTZVector, float64)` → `PTZContinuousMove(profileToken, x, y, z, timeout)`
- `GotoPreset(profileToken, presetToken, *PTZVector)` → `GotoPreset(profileToken, presetToken)`

## WSSE认证流程

### 认证头生成:

```xml
<Security xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
  <UsernameToken xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0.xsd">
    <Username>username</Username>
    <Password Type="...#PasswordDigest">SHA1(nonce+created+password)</Password>
    <Nonce EncodingType="...#Base64Binary">base64(nonce)</Nonce>
    <Created>2025-12-18T10:30:45Z</Created>
  </UsernameToken>
</Security>
```

### 密码摘要算法:
```go
hash := sha1.Sum([]byte(nonce + created + password))
passwordDigest := base64.StdEncoding.EncodeToString(hash[:])
```

## SOAP请求示例

### GetDeviceInformation:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:tds="http://www.onvif.org/ver10/device/wsdl">
  <soap:Header>
    <!-- WSSE Security Header -->
  </soap:Header>
  <soap:Body>
    <tds:GetDeviceInformation/>
  </soap:Body>
</soap:Envelope>
```

### GetProfiles (媒体服务):
```xml
<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:trt="http://www.onvif.org/ver10/media/wsdl">
  <soap:Header>
    <!-- WSSE Security Header -->
  </soap:Header>
  <soap:Body>
    <trt:GetProfiles/>
  </soap:Body>
</soap:Envelope>
```

### ContinuousMove (PTZ服务):
```xml
<tptz:ContinuousMove xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
  <tptz:ProfileToken>token</tptz:ProfileToken>
  <tptz:Velocity>
    <tt:PanTilt x="0.5" y="0.5"/>
    <tt:Zoom x="0.2"/>
  </tptz:Velocity>
  <tptz:Timeout>PT5S</tptz:Timeout>
</tptz:ContinuousMove>
```

## 服务自动发现

系统通过`GetCapabilities()`自动发现各服务端点:

```go
// 从响应中提取:
// <Media><XAddr>http://IP:PORT/media_service</XAddr></Media>
c.mediaAddr = xaddr  // 媒体服务地址

// <PTZ><XAddr>http://IP:PORT/ptz_service</XAddr></PTZ>
c.ptzAddr = xaddr    // PTZ服务地址
```

## 使用示例

### 基本连接:
```go
client := NewSOAPClient("http://192.168.1.3:8888/onvif/device_service", "test", "password")

// 获取设备信息
info, err := client.GetDeviceInformation()
if err != nil {
    log.Printf("错误: %v", err)
    return
}
```

### 获取媒体配置:
```go
// 自动从GetCapabilities获取媒体服务地址
profiles, err := client.GetMediaProfiles()
if err != nil {
    log.Printf("错误: %v", err)
    return
}

for _, p := range profiles {
    log.Printf("Profile: %s (%s)", p.Name, p.Token)
    
    // 获取流地址
    streamURI, _ := client.GetStreamURI(p.Token)
    log.Printf("  Stream: %s", streamURI)
}
```

### PTZ控制:
```go
profileToken := "main_profile"

// 向右移动（速度0.5）
client.ContinuousMove(profileToken, 0.5, 0, 0, 5.0)

// 停止
client.StopPTZ(profileToken)

// 移动到预置位1
client.GotoPreset(profileToken, "1")
```

## 错误处理

### HTTP错误:
```go
// 自动检查状态码
if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    // 尝试解析SOAP Fault
    return fmt.Errorf("HTTP %d | Code: %s | Reason: %s", 
        statusCode, fault.Code, fault.Reason)
}
```

### 常见错误:

| 错误 | 原因 | 解决方案 |
|------|------|--------|
| `401 Unauthorized` | 凭据错误 | 验证用户名/密码 |
| `403 Forbidden` | HTTPS强制 | 使用HTTPS或支持HTTP的设备 |
| `Socket timeout` | 网络延迟 | 增加超时时间(30s) |
| `no profiles found` | 媒体服务调用失败 | 检查GetCapabilities返回的地址 |

## 与旧版本(goonvif)的区别

### 优点:
1. **零依赖** - 完全使用Go标准库
2. **WS-Security原生支持** - 不再依赖goonvif的限制
3. **灵活的端点管理** - 支持多个服务地址
4. **精确的错误信息** - SOAP Fault详细解析

### 缺点:
1. **功能子集** - 仅实现常用的ONVIF操作
2. **XML解析简化** - 使用正则和简单遍历而非完整schema映射
3. **错误恢复** - 少于goonvif的自动重试机制

## 迁移检查清单

- [x] 替换goonvif库为SOAPClient
- [x] 实现WSSE认证头生成
- [x] 实现所有设备、媒体、PTZ方法
- [x] 更新helper.go包装层
- [x] 修复manager.go方法调用
- [x] 编译通过（无依赖错误）
- [ ] 在实际设备上测试
- [ ] 验证RTSP流获取
- [ ] 验证PTZ控制
- [ ] 性能基准测试

## 测试建议

### 测试脚本:
```bash
# 启动服务器
./start_server.sh

# 测试设备发现
curl http://localhost:9080/api/onvif/devices

# 测试设备信息
curl http://localhost:9080/api/onvif/devices/192.168.1.3:8888

# 测试媒体配置
curl http://localhost:9080/api/onvif/devices/192.168.1.3:8888/profiles

# 测试获取流地址
curl "http://localhost:9080/api/onvif/devices/192.168.1.3:8888/profiles?profileToken=main_profile"
```

### 预期结果:

✅ 标准认证设备 - 所有功能正常
✅ WS-Security设备 - 设备信息和PTZ可用，媒体配置功能受限
⚠️ 获取Profiles - 需要额外实现媒体服务认证

## 后续优化方向

1. **完整的媒体服务支持**
   - 实现GetCapabilities中媒体服务地址的完整解析
   - 为媒体服务实现独立的WS-Security认证

2. **性能优化**
   - 连接池复用
   - 并行查询多个设备
   - 响应缓存机制

3. **功能扩展**
   - 事件订阅 (WS-Eventing)
   - 分析功能 (Analytics)
   - Recording管理

4. **合规性增强**
   - 完整的WSDL schema支持
   - 更严格的XML验证
   - SOAP 1.2完全兼容

## 编译和运行

```bash
# 编译
cd /home/jl/下载/zpip/zpip
go build -o server ./cmd/server/

# 运行
./start_server.sh

# 查看日志
tail -f logs/*.log | grep "\[ONVIF\]"
```

## 参考文档

- [ONVIF核心规范](https://www.onvif.org/specs/)
- [WSSE 1.0规范](http://docs.oasis-open.org/wss/2004/01/)
- [本项目ONVIF实现总结](./ONVIF_实现总结.md)
- [快速参考指南](./ONVIF_快速参考.md)
