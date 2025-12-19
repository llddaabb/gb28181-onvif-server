# WSSE 认证修复总结

## 🔧 修复内容

### 1. **SOAP 信封格式** ✅
- **问题**: 使用了冗长的命名空间声明
- **修复**: 改用简化格式，匹配脚本样式
  ```xml
  <!-- 之前（复杂） -->
  <soap:Envelope xmlns:soap="..." xmlns:tds="..." xmlns:trt="..." xmlns:tptz="...">
  
  <!-- 之后（简化） -->
  <s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  ```

### 2. **WSSE Header 格式** ✅
- **问题**: Security 元素缺少 `s:mustUnderstand="1"` 属性
- **修复**: 添加必需的属性并改进命名空间声明
  ```xml
  <!-- 之前 -->
  <Security xmlns="...">
  
  <!-- 之后 -->
  <Security s:mustUnderstand="1" xmlns="..." xmlns:s="...">
  ```

### 3. **Nonce 处理** ✅
- **问题**: nonce 生成的字节序使用不当
- **修复**: 确保 nonce 原始字节用于摘要计算
  ```go
  // 之前（错误）
  hash := sha1.Sum([]byte(nonce + created + password))
  
  // 之后（正确）
  nonce, nonceBytes := generateNonce()
  h := sha1.New()
  h.Write(nonceBytes)      // 原始字节
  h.Write([]byte(created))
  h.Write([]byte(password))
  ```

### 4. **Timestamp 格式** ✅
- 确保格式为 `YYYY-MM-DDTHH:MM:SS.000Z`（带毫秒）

---

## 📋 修改文件

### [internal/onvif/soap_client.go](internal/onvif/soap_client.go)

**更改的函数**:

1. `generateNonce()` - 现在返回 `(string, []byte)` 元组
2. `generateWSSEHeader()` - 简化格式，添加 `s:mustUnderstand="1"`
3. `callSOAPOnEndpoint()` - 使用简化的 SOAP 信封

---

## 🧪 测试方法

### 方法 1: 快速对比测试
```bash
./quick_test.sh 192.168.1.250 8888 admin a123456789
```
对比脚本方式和 Go 服务方式的结果。

### 方法 2: 详细日志测试
```bash
# 终端1: 启动服务并查看详细日志
./server 2>&1 | grep -E "SOAP|GetSystemDateAndTime|GetProfiles|503"

# 终端2: 调用 API
curl http://localhost:8080/api/onvif/devices/192.168.1.250:8888/profiles
```

### 方法 3: 直接对比脚本请求
```bash
./compare_wsse.sh 192.168.1.250 8888 admin a123456789
```

---

## ✅ 预期结果

修复后：
- ✅ GetSystemDateAndTime 应返回 HTTP 200（而非 503）
- ✅ GetCapabilities 能正确解析 Media.XAddr 和 PTZ.XAddr
- ✅ GetProfiles 能从 Media 服务端点成功返回配置列表
- ✅ 日志显示 "✅ 成功获取 N 个媒体配置文件"

---

## 🔍 故障排查

如果仍然出现 503 错误，检查以下内容：

### 1. 请求体格式
运行 `./quick_test.sh`，对比脚本和 Go 的 HTTP 状态码。
- 脚本成功但 Go 失败？可能还有其他格式差异

### 2. 日志输出
```bash
./server 2>&1 | grep "SOAP请求体预览"
```
观察输出的 SOAP 请求体是否与脚本格式匹配。

### 3. 网络抓包
```bash
sudo tcpdump -i any -A 'tcp port 8888' | grep -A 5 "GetSystemDateAndTime"
```
对比脚本和 Go 发送的实际字节流。

---

## 📚 参考资源

- **脚本实现**: [onvif_test.sh](onvif_test.sh) - 行 184-214
- **WSSE 标准**: http://docs.oasis-open.org/wss/
- **ONVIF 标准**: https://www.onvif.org/

---

## 📝 下一步

1. **编译**:
   ```bash
   go build -o server ./cmd/server/
   ```

2. **测试**:
   ```bash
   ./quick_test.sh 192.168.1.250 8888 admin a123456789
   ```

3. **观察日志**:
   - 若 HTTP 200：修复成功 ✅
   - 若仍 HTTP 503：查看日志中的请求体预览，寻找其他差异

4. **验证完整流程**:
   ```bash
   curl http://localhost:8080/api/onvif/devices/192.168.1.250:8888/profiles
   ```
   应返回媒体配置文件列表。
