# ONVIF GetProfiles 诊断指南

## 📋 问题诊断流程

当 GetProfiles 返回 HTTP 503 或其他错误时，按照以下步骤检查：

### 1️⃣ **设备发现阶段** (WS-Discovery)
```
日志应显示:
  [ONVIF] ✓ WS-Discovery 发现到设备
  [ONVIF] 📡 发现设备 XAddr: http://192.168.1.250:8888/onvif/device_service
```

**检查内容**:
- WS-Discovery 是否能发现设备？
- 返回的 XAddr 格式是否正确？
- XAddr 是否能被网络访问？

---

### 2️⃣ **设备连接阶段** (NewDevice)
```
日志应显示:
  [ONVIF] 🔍 连接设备: http://192.168.1.250:8888/onvif/device_service (用户: admin)
```

**检查内容**:
- 设备地址是否可解析？
- TCP 连接是否能建立？
- 是否收到凭据（用户名/密码）？

---

### 3️⃣ **GetCapabilities 阶段**（获取服务端点）
```
日志应显示:
  [ONVIF] 📋 GetCapabilities 请求: 端点=http://192.168.1.250:8888/onvif/device_service
  [ONVIF] 📍 进入 <Media> 部分
  [ONVIF] ✅ 发现媒体服务地址: http://192.168.1.250:8888/onvif/media_service
  [ONVIF] 📍 进入 <PTZ> 部分
  [ONVIF] ✅ 发现PTZ服务地址: http://192.168.1.250:8888/onvif/ptz_service
```

**检查内容**:
- GetCapabilities 是否成功返回？
- 是否正确解析出 Media.XAddr？
- Media.XAddr 的格式是否正确？
- 如果 GetCapabilities 失败，错误信息是什么？

---

### 4️⃣ **GetProfiles 阶段**（获取媒体配置）
```
日志应显示:
  [ONVIF] ✅ 使用 Media.XAddr: http://192.168.1.250:8888/onvif/media_service
  [ONVIF] 📡 GetProfiles 请求详情 | Endpoint=http://192.168.1.250:8888/onvif/media_service | Username=admin | Password=***
  [ONVIF] ✅ 成功获取 N 个媒体配置文件
```

**检查内容**:
- 是否使用了正确的 Media.XAddr？
- 是否使用了正确的凭据（用户名/密码）？
- GetProfiles 是否成功返回？

---

## 🔴 常见错误及解决方案

### ❌ "未获取到 Media.XAddr，使用设备端点"
**原因**: GetCapabilities 可能失败或响应格式不符合预期
**解决**:
- 检查设备是否支持 GetCapabilities
- 检查 XML 解析逻辑是否正确

### ❌ "HTTP 503"
**原因**: 请求被发送到了不支持该操作的端点
**解决**:
- 确认 GetCapabilities 返回了正确的 Media.XAddr
- 尝试直接访问 Media 端点: `curl -u admin:password http://192.168.1.250:8888/onvif/media_service`

### ❌ "HTTP 401/403"
**原因**: 凭据错误或不支持该服务的认证
**解决**:
- 验证用户名和密码是否正确
- 确认设备是否需要在不同的服务端点使用不同的凭据

### ❌ "端点不可达"
**原因**: 网络问题或端点 URL 格式错误
**解决**:
- 尝试 ping 设备 IP
- 检查防火墙设置
- 验证端口是否正确

---

## 📝 日志查看建议

### 启动服务时查看完整日志:
```bash
./server 2>&1 | grep -E "\[ONVIF\]"
```

### 实时监控 GetProfiles 请求:
```bash
./server 2>&1 | grep -E "GetProfiles|GetCapabilities|Media|PTZ"
```

### 查看所有错误信息:
```bash
./server 2>&1 | grep -E "❌|⚠️|ERROR|失败"
```

---

## 🧪 手动测试 (curl)

### 1. 测试设备连接
```bash
curl -u admin:password http://192.168.1.250:8888/onvif/device_service
```

### 2. 测试 GetCapabilities (手动SOAP)
```bash
curl -X POST -u admin:password \
  -H "Content-Type: application/soap+xml" \
  http://192.168.1.250:8888/onvif/device_service \
  -d '<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:tds="http://www.onvif.org/ver10/device/wsdl">
  <soap:Body>
    <tds:GetCapabilities>
      <tds:Category>All</tds:Category>
    </tds:GetCapabilities>
  </soap:Body>
</soap:Envelope>'
```

### 3. 测试 GetProfiles (手动SOAP)
```bash
curl -X POST -u admin:password \
  -H "Content-Type: application/soap+xml" \
  http://192.168.1.250:8888/onvif/media_service \
  -d '<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:trt="http://www.onvif.org/ver10/media/wsdl">
  <soap:Body>
    <trt:GetProfiles/>
  </soap:Body>
</soap:Envelope>'
```

---

## 📊 API 端点测试

### 获取设备列表
```bash
curl http://localhost:8080/api/onvif/devices
```

### 手动获取 Profiles
```bash
curl http://localhost:8080/api/onvif/devices/192.168.1.250:8888/profiles
```

---

## 📍 代码位置参考

- **SOAP 客户端**: `internal/onvif/soap_client.go` - 核心 SOAP 调用逻辑
- **设备包装器**: `internal/onvif/helper.go` - ONVIFDeviceClient 和 NewDevice
- **管理器**: `internal/onvif/manager.go` - 设备管理和发现
- **API 处理**: `internal/api/handlers_onvif.go` - REST API 端点

---

## 💡 调试提示

1. **启用详细日志**: 所有关键步骤都有 emoji 标记（✅/❌/📡/⚠️）
2. **检查顺序**: 从下往上检查（GetProfiles → GetCapabilities → 连接 → 发现）
3. **凭据验证**: 每个日志都包含使用的端点、用户名（密码已脱敏）
4. **端点追踪**: 跟踪实际使用的 XAddr，确认是否使用了 Media 端点
