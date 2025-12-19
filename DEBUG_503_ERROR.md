# HTTP 503 错误排查指南

## 问题描述
GetSystemDateAndTime 返回 HTTP 503，说明设备拒绝了请求。

## 可能原因

### 1. 请求体格式不匹配 ⚠️ **最可能**

**脚本方式** (onvif_test.sh):
```xml
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Header>
    <Security s:mustUnderstand="1" xmlns="...">
      <UsernameToken>
        ...
      </UsernameToken>
    </Security>
  </s:Header>
  <s:Body>
    <GetSystemDateAndTime xmlns="http://www.onvif.org/ver10/device/wsdl"/>
  </s:Body>
</s:Envelope>
```

**Go 方式** (soap_client.go):
```xml
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:tds="http://www.onvif.org/ver10/device/wsdl"
               ...>
  <soap:Header>
    <Security xmlns="...">
      <UsernameToken xmlns="...">
        ...
      </UsernameToken>
    </Security>
  </soap:Header>
  <soap:Body>
    <tds:GetSystemDateAndTime xmlns:tds="..."/>
  </soap:Body>
</soap:Envelope>
```

**关键差异**:
- 脚本: 前缀是 `s:`, 根元素是 `<s:Envelope>`
- Go: 前缀是 `soap:`, 根元素有多个 `xmlns:` 声明
- 脚本: Body 中直接用 `<GetSystemDateAndTime xmlns="..."/>`
- Go: Body 中用 `<tds:GetSystemDateAndTime xmlns:tds="..."/>`

### 2. Security 头的格式

**脚本**:
```xml
<Security s:mustUnderstand="1" xmlns="...">
  <UsernameToken>
```

**Go**:
```xml
<Security xmlns="...">
  <UsernameToken xmlns="...">
```

差异:
- 脚本有 `s:mustUnderstand="1"`
- Go 的 UsernameToken 有额外的 `xmlns:...` 声明

### 3. Nonce 和 Timestamp 格式

**脚本**:
```bash
nonce=$(openssl rand -base64 16)
timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")
digest=$(echo -n "$(echo "$nonce" | base64 -d)${timestamp}${PASSWORD}" | openssl sha1 -binary | base64)
```

**Go**:
```go
nonce, nonceBytes := generateNonce()  // 返回 base64 编码的
created := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
h := sha1.New()
h.Write(nonceBytes)     // 使用原始字节
h.Write([]byte(created))
h.Write([]byte(c.password))
```

这部分应该是对的。

---

## 🔍 调试步骤

### 步骤 1: 验证脚本方式确实有效

```bash
# 直接用脚本测试（使用你的设备IP）
./onvif_test.sh 192.168.1.250 admin a123456789
```

查看脚本是否能成功调用 GetSystemDateAndTime。

### 步骤 2: 对比请求体格式

```bash
# 运行对比脚本（新增）
./compare_wsse.sh 192.168.1.250 8888 admin a123456789
```

此脚本会:
1. 用脚本方式发送 SOAP 请求，显示成功或失败
2. 启动 Go 服务后再尝试 API 调用

### 步骤 3: 查看实际的 Go 请求体

1. 启动服务:
   ```bash
   ./server 2>&1 | grep -E "SOAP请求体|SOAP响应体"
   ```

2. 触发 GetProfiles 请求:
   ```bash
   curl http://localhost:8080/api/onvif/devices/192.168.1.250:8888/profiles
   ```

3. 观察日志输出的请求/响应预览

### 步骤 4: 用 tcpdump 抓包对比

```bash
# 终端1: 抓包
sudo tcpdump -i any -A 'tcp port 8888' -w /tmp/onvif.pcap

# 终端2: 用脚本方式测试
./onvif_test.sh 192.168.1.250 admin a123456789

# 终端3: 用 Go 方式测试
./server &
sleep 2
curl http://localhost:8080/api/onvif/devices/192.168.1.250:8888/profiles

# 分析
tcpdump -r /tmp/onvif.pcap -A | grep -A 10 "GetSystemDateAndTime"
```

---

## 💡 修复方案假设

基于对比，可能需要:

1. **简化 SOAP 信封** - 减少不必要的 `xmlns:` 声明
2. **添加 `mustUnderstand` 属性** 到 Security 元素
3. **移除 UsernameToken 的 `xmlns` 声明**
4. **检查 Body 中的方法名调用** - 是否需要使用命名空间前缀

---

## 📝 预期修改

如果发现 Go 方式的请求体格式与脚本不同，可能需要修改 [soap_client.go](internal/onvif/soap_client.go) 中的：

1. **SOAP 信封模板**:
   - 改用 `s:` 前缀而不是 `soap:`
   - 移除不必要的 `xmlns:tds`, `xmlns:trt` 等
   - 保持最小化的声明

2. **Security 头**:
   - 添加 `s:mustUnderstand="1"`
   - 简化 UsernameToken 的命名空间

3. **Body 内容**:
   - 根据需要调整方法调用的格式

---

## 🎯 立即行动

1. 运行 compare_wsse.sh 脚本
2. 启动服务，查看日志中的 "SOAP请求体预览" 和 "SOAP响应体预览"
3. 对比两种方式的差异
4. 根据差异提出具体的代码修改建议

---

## 日志查看命令

```bash
# 启动服务并过滤日志
./server 2>&1 | grep -E "SOAP|GetSystemDateAndTime|GetProfiles|503|Endpoint"

# 实时查看
./server 2>&1 | tee server.log | grep -E "❗|✅|📋"
```
