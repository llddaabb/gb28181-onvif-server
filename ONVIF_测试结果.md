# ONVIF 测试结果报告

## 设备信息
- **IP地址**: 192.168.1.232
- **用户名**: test  
- **密码**: a123456789
- **开放端口**: 80, 443, 554

## 测试结果

### ✅ 成功发现的信息

1. **端点发现**: 成功找到3个ONVIF端点
   - `http://192.168.1.232:80/onvif/device_service` (需要认证)
   - `http://192.168.1.232:80/onvif/device` (需要认证)
   - `http://192.168.1.232:80/cgi-bin/onvif/device_service` (需要认证)

2. **网络连通性**: ✅ 设备可访问

### ❌ 失败原因

**主要问题**: `TransModeError - The ONVIF client may not support HTTPS mode`

设备返回的完整错误信息：
```
The ONVIF client may not support HTTPS mode. Please check the client 
settings or disable HTTPS in the Device configuration settings and try 
again. If the problem persists, contact technical support.
```

## 问题诊断

摄像头配置为 **强制使用HTTPS进行ONVIF通信**，但：
1. 我们使用HTTP (端口80) 进行访问
2. HTTPS端口 (443) 虽然开放但连接失败（可能需要特殊配置）

## 解决方案

### 方案 1: 在摄像头中禁用强制HTTPS（推荐）

1. 访问摄像头Web管理界面：http://192.168.1.232
2. 使用用户名 `test` 和密码 `a123456789` 登录
3. 找到 **ONVIF设置** 或 **网络设置**
4. 查找并**禁用**以下选项之一：
   - "强制HTTPS"
   - "HTTPS Only"  
   - "Require HTTPS for ONVIF"
   - "ONVIF安全模式"
5. 保存设置并重启摄像头（如需要）
6. 重新运行测试脚本

### 方案 2: 使用HTTPS连接（如果443端口配置正确）

修改连接地址为：
```
https://192.168.1.232:443/onvif/device_service
```

可能需要：
- 接受自签名证书
- 配置SSL/TLS设置

## 推荐的ONVIF端点

禁用强制HTTPS后，使用以下端点：
```
http://192.168.1.232:80/onvif/device_service
```

## 重新测试

配置修改后，运行以下命令重新测试：

```bash
# 完整测试
./onvif_test.sh 192.168.1.232 test a123456789

# 或直接测试端点
/tmp/test_onvif_wsse.sh "http://192.168.1.232:80/onvif/device_service" test a123456789
```

## 硬盘录像机为什么能连接？

硬盘录像机（NVR）能够成功连接是因为：
1. NVR通常支持HTTPS/TLS的ONVIF连接
2. NVR内部处理了证书验证
3. NVR使用标准的WS-Security (UsernameToken)认证

我们的测试脚本也需要类似的支持，或者你需要在摄像头中禁用强制HTTPS。

## 下一步

1. 登录摄像头管理界面
2. 禁用ONVIF的强制HTTPS选项
3. 重新运行测试
4. 如果问题仍存在，请提供摄像头品牌和型号以获得更具体的帮助
