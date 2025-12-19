# ONVIF 实现总结

## 当前支持的功能

### ✅ 设备发现
- **WS-Discovery 多播发现** - 自动发现局域网内的 ONVIF 设备
- **端口自动回退** - 当 WS-Discovery 返回错误端口时，自动尝试常见端口 (80, 8888, 8080, 等)
- **XAddr 规范化** - 自动补全不完整的 ONVIF 端点地址

### ✅ 认证方式
1. **标准认证** (goonvif 库)
   - HTTP Digest 认证
   - Basic 认证
   
2. **WS-Security UsernameToken**
   - 基于 SHA1 密码摘要的认证
   - 支持 nonce 和 timestamp
   - 适用于严格要求安全的设备

### ✅ 设备管理
- **多凭据自动尝试** - 发现设备时尝试多组用户名/密码
  - test/a123456789
  - admin/a123456789
  - admin/admin
  - 匿名访问

- **设备详细信息获取**
  - 设备型号、生产商、固件版本、序列号
  - 设备能力（PTZ、媒体配置等）
  - 创建于发现时间自动保存

### ⚠️ 部分支持的功能

#### Profiles (媒体配置文件)
- **标准认证下** ✅ 正常获取
- **WS-Security 下** ⚠️ 返回空列表
  - 原因：GetProfiles 需要在媒体服务端点调用，但大多数设备的媒体服务端点不支持 WS-Security
  - 可能的改进：
    1. 通过 GetCapabilities 获取媒体服务 URI
    2. 为媒体服务实现单独的认证
    3. 或者使用 HTTP Digest 而非 WS-Security

#### PTZ 控制
- 通过 goonvif 库集成（需要 profiles 支持）

#### 快照和实时流
- 需要 profiles 支持

## 故障排查

### 设备无法连接
```
错误: "camera is not available at http://IP:PORT/onvif/device_service"
```
**原因**：goonvif 库无法初始化（可能不支持设备的认证方式）

**解决**：系统会自动回退到 WS-Security 纯模式，基本信息仍可保存

### 无法获取 Profiles
```
WS-Security 纯模式：返回空 profiles 列表
```
**原因**：媒体服务端点需要特殊的认证或实现

**临时解决**：使用标准认证（非 WS-Security）的设备可正常获取

## 测试脚本

### 手动测试 WS-Security 认证
```bash
/tmp/test_onvif_wsse.sh http://IP:PORT/onvif/device_service username password
```

### 测试完整 ONVIF 功能
```bash
./onvif_test.sh
```

## 配置文件

**主配置**: `configs/config.yaml`
```yaml
API:
  Port: 9080           # API 服务端口

ONVIF:
  DiscoveryInterval: 60  # WS-Discovery 间隔（秒）
  MediaPortRange: 8000-9000  # 媒体流端口范围
```

## 开发建议

### 改进 Profiles 获取（高优先级）

#### 当前问题
- WS-Security 认证的设备无法获取 Profiles
- 原因：GetProfiles 需要调用媒体服务端点，而媒体服务端点需要单独配置

#### 解决方案：实现完整的媒体服务客户端
按照 goonvif 的标准流程实现：

```
1. 使用 goonvif 调用 GetCapabilities (设备服务端点)
   ↓
2. 从响应中提取 Media.XAddr（媒体服务端点）
   ↓
3. 为媒体服务端点创建新的认证客户端
   （可以是 goonvif 实例，也可以是自定义 WS-Security SOAP 调用）
   ↓
4. 调用 Media.GetProfiles()
   ↓
5. 遍历 Profiles，获取 Token
   ↓
6. 对每个 Profile 调用 GetStreamUri、GetSnapshotUri 等
```

#### 实现步骤

**步骤 1：解析 GetCapabilities 响应**
```go
func (d *ONVIFDevice) GetMediaServiceURI() (string, error) {
    // 获取设备 Capabilities
    // 解析 XML，提取 Media.XAddr
    // 返回媒体服务端点 URL
}
```

**步骤 2：为媒体服务创建客户端**
```go
func (d *ONVIFDevice) GetMediaProfiles() ([]MediaProfile, error) {
    // 1. 获取媒体服务 URI
    mediaURI := d.GetMediaServiceURI()
    
    // 2. 创建媒体服务客户端
    mediaClient := NewONVIFDevice(d.Username, d.Password)
    mediaClient.Connect(mediaURI)  // 用 WS-Security 或标准认证
    
    // 3. 调用 GetProfiles
    return mediaClient.GetProfiles()
}
```

**步骤 3：实现媒体服务的 SOAP 调用**
```go
// 对于 WS-Security 模式
func (d *ONVIFDevice) getProfilesFromMediaService() ([]MediaProfile, error) {
    mediaURI := d.GetMediaServiceURI()
    
    // 使用 callSOAPWithWSSEOnEndpoint 在媒体服务端点调用 GetProfiles
    resp := d.callSOAPWithWSSEOnEndpoint(
        mediaURI,
        "GetProfiles",
        "http://www.onvif.org/ver10/media/wsdl",
        "",  // 空的请求体
    )
    
    // 解析响应，提取 Profiles
}
```

#### 关键代码参考（goonvif）
- `goonvif.Device.GetCapabilities()` - 获取能力
- `goonvif/sdk/media.Call_GetProfiles()` - 获取 profiles

### 改进 PTZ 和媒体流（中优先级）
1. 基于 Profiles 实现预置位管理
2. 支持 RTSP/RTMP 流获取
3. 实现抓图功能

### 增强 WS-Security（可选）
1. 支持 WS-Addressing
2. 支持加密的密码（而非仅 PasswordDigest）
3. 支持 Kerberos 认证


## 已知限制

1. **WS-Security 下无法获取 Profiles** - 影响 PTZ、媒体流等功能
2. **某些高级 ONVIF 功能未实现** - 如事件订阅、分析等
3. **不支持多个 IP 接口的设备** - WS-Discovery 可能发现多条记录

## 未来改进方向

- [ ] 完整的媒体服务端点支持
- [ ] 事件订阅和告警
- [ ] 分析功能集成
- [ ] 录像管理
- [ ] 更好的日志和诊断工具
