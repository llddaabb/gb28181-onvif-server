# WSSE 请求格式对比

## 脚本方式 (onvif_test.sh)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Header>
    <Security s:mustUnderstand="1" xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
      <UsernameToken>
        <Username>admin</Username>
        <Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">
          BASE64_DIGEST_HERE
        </Password>
        <Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">
          BASE64_NONCE_HERE
        </Nonce>
        <Created xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">
          2025-12-18T14:15:30.000Z
        </Created>
      </UsernameToken>
    </Security>
  </s:Header>
  <s:Body>
    <GetSystemDateAndTime xmlns="http://www.onvif.org/ver10/device/wsdl"/>
  </s:Body>
</s:Envelope>
```

**关键特征**:
- ✅ 根元素: `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">`
- ✅ Header: `<s:Header>` (带 s: 前缀)
- ✅ Body: `<s:Body>` (带 s: 前缀)
- ✅ Security: 有 `s:mustUnderstand="1"` 属性
- ✅ UsernameToken: 无额外的 xmlns 声明
- ✅ Timestamp: 格式 `YYYY-MM-DDTHH:MM:SS.000Z`

---

## Go 修复前 ❌

```xml
<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:tds="http://www.onvif.org/ver10/device/wsdl"
               xmlns:trt="http://www.onvif.org/ver10/media/wsdl"
               xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl"
               xmlns:tt="http://www.onvif.org/ver10/schema">
  <soap:Header>
    <Security xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
      <UsernameToken xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0.xsd" u:Id="UsernameToken-1">
        <Username>admin</Username>
        <Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">
          BASE64_DIGEST_HERE
        </Password>
        <Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">
          BASE64_NONCE_HERE
        </Nonce>
        <Created xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">
          2025-12-18T14:15:30Z
        </Created>
      </UsernameToken>
    </Security>
  </soap:Header>
  <soap:Body>
    <tds:GetSystemDateAndTime xmlns:tds="http://www.onvif.org/ver10/device/wsdl"/>
  </soap:Body>
</soap:Envelope>
```

**问题**:
- ❌ 根元素: 使用了 `soap:` 前缀（应该用 `s:`）
- ❌ 冗长的命名空间声明（`tds`, `trt`, `tptz`, `tt`）不必要
- ❌ Security 缺少 `s:mustUnderstand="1"` 属性
- ❌ UsernameToken 有多余的 `xmlns` 和 `u:Id` 属性
- ❌ Timestamp 缺少毫秒部分（`.000Z`）
- ❌ Body 中的方法调用使用了 `tds:` 前缀

---

## Go 修复后 ✅

```xml
<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Header>
    <Security s:mustUnderstand="1" xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd" xmlns:s="http://www.w3.org/2003/05/soap-envelope">
      <UsernameToken>
        <Username>admin</Username>
        <Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">
          BASE64_DIGEST_HERE
        </Password>
        <Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">
          BASE64_NONCE_HERE
        </Nonce>
        <Created xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">
          2025-12-18T14:15:30.000Z
        </Created>
      </UsernameToken>
    </Security>
  </s:Header>
  <s:Body>
    <GetSystemDateAndTime xmlns="http://www.onvif.org/ver10/device/wsdl"/>
  </s:Body>
</s:Envelope>
```

**修复**:
- ✅ 根元素: `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">`
- ✅ 移除了不必要的命名空间声明
- ✅ 添加了 `s:mustUnderstand="1"` 属性
- ✅ 简化了 UsernameToken（无额外属性）
- ✅ Timestamp 格式正确（含毫秒）
- ✅ Body 中的方法调用无前缀

---

## 代码对比

### generateWSSEHeader() 函数

**修复前** ❌:
```go
return fmt.Sprintf(`<Security xmlns="...">
  <UsernameToken xmlns="..." u:Id="UsernameToken-1">
    ...
  </UsernameToken>
</Security>`, ...)
```

**修复后** ✅:
```go
return fmt.Sprintf(`<Security s:mustUnderstand="1" xmlns="..." xmlns:s="http://www.w3.org/2003/05/soap-envelope">
      <UsernameToken>
        ...
      </UsernameToken>
    </Security>`, ...)
```

### callSOAPOnEndpoint() 函数

**修复前** ❌:
```go
soapEnvelope := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="..."
               xmlns:tds="..."
               xmlns:trt="..."
               xmlns:tptz="..."
               xmlns:tt="...">
  <soap:Header>%s</soap:Header>
  <soap:Body>%s</soap:Body>
</soap:Envelope>`, ...)
```

**修复后** ✅:
```go
soapEnvelope := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Header>%s</s:Header>
  <s:Body>%s</s:Body>
</s:Envelope>`, ...)
```

---

## 摘要计算对比

### 脚本方式:
```bash
nonce=$(openssl rand -base64 16)
timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")
# 关键: nonce 必须先被 base64 解码
digest=$(echo -n "$(echo "$nonce" | base64 -d)${timestamp}${password}" | openssl sha1 -binary | base64)
```

### Go 修复前 ❌:
```go
hash := sha1.Sum([]byte(nonce + created + c.password))  // 错误：直接拼接
```

### Go 修复后 ✅:
```go
nonce, nonceBytes := generateNonce()  // 获取 base64 和原始字节
h := sha1.New()
h.Write(nonceBytes)      // 使用原始字节
h.Write([]byte(created))
h.Write([]byte(c.password))
hash := h.Sum(nil)
```

---

## 验证清单

完成以下检查确保修复正确：

- [ ] Envelope 前缀改为 `s:`
- [ ] 移除 Header/Body 中的 `soap:` 前缀，改为 `s:`
- [ ] Security 添加 `s:mustUnderstand="1"`
- [ ] 移除 Security 中不必要的命名空间声明
- [ ] UsernameToken 中移除 `xmlns` 和 `u:Id` 属性
- [ ] Timestamp 格式包含毫秒 (`.000Z`)
- [ ] Body 中的方法调用无前缀
- [ ] Nonce 摘要计算使用原始字节

---

## 预期测试结果

修复后，运行:
```bash
./quick_test.sh 192.168.1.250 8888 admin a123456789
```

**脚本方式**: ✅ 成功 (HTTP 200)
**Go 方式**: ✅ 成功 (HTTP 200)

都应该返回 HTTP 200 和 GetSystemDateAndTimeResponse 响应。
