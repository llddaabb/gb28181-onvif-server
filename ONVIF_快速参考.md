# ONVIF 功能快速参考

## 设备发现与连接

### 自动发现
```bash
# 启动服务器后，会自动进行 WS-Discovery 多播发现
./server

# 日志中会显示发现的设备
# [ONVIF] 🔍 发现设备: ONVIF Camera (192.168.1.3:8888)
```

### API 获取已发现的设备
```bash
curl http://localhost:9080/api/onvif/devices
```

**响应示例:**
```json
{
  "devices": [
    {
      "deviceId": "192.168.1.3:8888",
      "name": "ONVIF Camera (192.168.1.3)",
      "ip": "192.168.1.3",
      "port": 8888,
      "status": "online",
      "manufacturer": "Manufacturer",
      "model": "IPC",
      "firmwareVersion": "v1.0",
      "onvifAddr": "http://192.168.1.3:8888/onvif/device_service"
    }
  ]
}
```

## 设备信息查询

### 获取单个设备详情
```bash
curl http://localhost:9080/api/onvif/devices/{deviceId}

# 例如
curl http://localhost:9080/api/onvif/devices/192.168.1.3:8888
```

### 获取设备能力
```bash
curl http://localhost:9080/api/onvif/devices/{deviceId}/capabilities
```

## 媒体配置（Profiles）

### 获取 Profiles
```bash
curl http://localhost:9080/api/onvif/devices/{deviceId}/profiles
```

**预期结果:**
- ✅ **标准认证设备** (Digest/Basic) - 返回完整的 profiles 列表
- ⚠️ **WS-Security 设备** - 返回空列表（需要额外实现）

### 当前限制
- WS-Security 认证的设备无法获取 Profiles
- 原因：需要在媒体服务端点调用，而该端点需要单独配置

## 认证测试

### 测试 WS-Security 认证（手动）
```bash
# 使用测试脚本验证 WS-Security 是否工作
/tmp/test_onvif_wsse.sh http://IP:PORT/onvif/device_service username password

# 例如
/tmp/test_onvif_wsse.sh http://192.168.1.3:8888/onvif/device_service test a123456789
```

### 测试设备连接
```bash
# 查看服务器日志，其中会记录所有连接尝试
# 例如：
# [ONVIF] ✅ WS-Security 认证通过: http://...
# [ONVIF] ⚠️ goonvif 初始化失败: ...
```

## 设备凭据

### 发现时的自动尝试顺序
1. `test / a123456789`
2. `admin / a123456789`
3. `admin / admin`
4. 匿名（无凭据）

### 添加自定义凭据
```bash
curl -X PUT http://localhost:9080/api/onvif/devices/{deviceId}/credentials \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "newpassword"
  }'
```

## 故障排查

### 问题：设备被发现但信息不完整
```
[ONVIF] ⚠️ 获取设备详细信息失败: ... | 但基本信息已添加
```
**解决:**
- 检查设备凭据是否正确
- 设备可能处于离线状态
- 网络连接可能有问题

### 问题：WS-Security 认证失败
```
[ONVIF] ⚠️ WS-Security 认证失败: ...
[ONVIF] 尝试标准认证...
```
**解决:**
- 设备可能不支持 WS-Security
- 用户名/密码可能错误
- 检查网络连接

### 问题：无法获取 Profiles
```
[ONVIF] ℹ️ WS-Security 纯模式：无法获取 Profiles
```
**解决:**
- 这是已知限制，当前版本不支持
- 可以实现媒体服务端点支持（见 ONVIF_实现总结.md）
- 或使用标准认证（Digest/Basic）的设备

### 问题：端口被占用
```
./start_server.sh: 端口 9080 被占用
```
**解决:**
```bash
# 使用自动关闭模式
./start_server.sh -a

# 或查看占用情况
./start_server.sh -l

# 或手动杀死进程
lsof -ti:9080 | xargs kill -9
```

## 日志说明

### 关键日志类型

**发现阶段:**
```
[ONVIF] 🔍 发现设备: ...
[ONVIF] ✅ 已将发现的设备添加到列表: ...
```

**认证阶段:**
```
[ONVIF] 🔐 尝试凭据: 用户名='...'
[ONVIF] ✅ 凭据验证成功: ...
[ONVIF] ❌ 凭据验证失败: ...
```

**获取信息阶段:**
```
[ONVIF] ✅ 已获取设备详细信息: ...
[ONVIF] ⚠️ 获取设备详细信息失败: ...
```

## 常用 API 端点

| 方法 | 端点 | 说明 |
|-----|------|------|
| GET | `/api/onvif/devices` | 获取所有设备列表 |
| GET | `/api/onvif/devices/{id}` | 获取单个设备信息 |
| GET | `/api/onvif/devices/{id}/capabilities` | 获取设备能力 |
| GET | `/api/onvif/devices/{id}/profiles` | 获取媒体配置文件 |
| POST | `/api/onvif/devices/{id}/refresh` | 刷新设备信息 |
| PUT | `/api/onvif/devices/{id}/credentials` | 更新凭据 |
| DELETE | `/api/onvif/devices/{id}` | 删除设备 |

## 配置调优

### config.yaml 相关配置
```yaml
ONVIF:
  DiscoveryInterval: 60        # WS-Discovery 扫描间隔（秒）
  MediaPortRange: "8000-9000"  # 媒体流端口范围
  MaxRetries: 3                # 连接重试次数
  Timeout: 30                  # 连接超时（秒）
```

### 增加发现间隔
```yaml
ONVIF:
  DiscoveryInterval: 120  # 改为 2 分钟发现一次
```

### 调整连接超时
```yaml
ONVIF:
  Timeout: 60  # 改为 60 秒（某些设备响应慢）
```
