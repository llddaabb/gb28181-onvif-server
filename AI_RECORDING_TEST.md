# AI智能录像功能验证指南

## 快速验证步骤

### 1. 检查AI功能状态

```bash
# 查看AI配置
curl http://localhost:9080/api/ai/config

# 预期返回：
# {
#   "success": true,
#   "config": {
#     "Enable": false,  # 默认关闭
#     "APIEndpoint": "http://localhost:8000/detect",
#     "Confidence": 0.5,
#     ...
#   }
# }
```

### 2. 启用AI功能（在设置页面或通过API）

**方法一：前端界面**
1. 访问 `http://localhost:9080`
2. 进入"设置"页面
3. 找到"AI智能录像"section
4. 打开"启用AI录像"开关
5. 点击"保存配置"

**方法二：API直接启用**
```bash
curl -X PUT http://localhost:9080/api/ai/config \
  -H "Content-Type: application/json" \
  -d '{
    "Enable": true,
    "APIEndpoint": "http://localhost:8000/detect",
    "Confidence": 0.5,
    "DetectInterval": 2,
    "RecordDelay": 10,
    "MinRecordTime": 5
  }'
```

### 3. 部署AI检测服务（模拟服务）

由于还没有真实的AI检测服务，先创建一个模拟API：

```bash
# 创建简单的模拟AI服务
cat > /tmp/mock_ai_service.py << 'PYEOF'
from http.server import HTTPServer, BaseHTTPRequestHandler
import json
import random

class AIDetectHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path == '/detect':
            # 读取图像数据（忽略）
            content_length = int(self.headers['Content-Length'])
            self.rfile.read(content_length)
            
            # 模拟检测结果：50%概率检测到人
            has_person = random.random() > 0.5
            
            response = {
                "success": True,
                "has_person": has_person,
                "person_count": 1 if has_person else 0,
                "confidence": 0.85 if has_person else 0,
                "boxes": [
                    {
                        "x1": 100,
                        "y1": 150,
                        "x2": 300,
                        "y2": 450,
                        "confidence": 0.85,
                        "class": "person"
                    }
                ] if has_person else []
            }
            
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps(response).encode())
        else:
            self.send_response(404)
            self.end_headers()
    
    def log_message(self, format, *args):
        print(f"[AI检测] {args[0]} - {args[1]}")

if __name__ == '__main__':
    server = HTTPServer(('localhost', 8000), AIDetectHandler)
    print("🤖 AI检测模拟服务运行在 http://localhost:8000")
    print("发送POST请求到 /detect 即可获得模拟检测结果")
    server.serve_forever()
PYEOF

# 启动模拟服务
python3 /tmp/mock_ai_service.py &
MOCK_PID=$!
echo "模拟AI服务已启动，PID: $MOCK_PID"
```

### 4. 测试AI检测API

```bash
# 测试AI服务是否可用
echo "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==" | \
  base64 -d | \
  curl -X POST http://localhost:8000/detect \
    -H "Content-Type: image/jpeg" \
    --data-binary @-

# 预期返回：
# {"success": true, "has_person": true/false, ...}
```

### 5. 启动AI录像

**方法一：通过前端**
1. 进入"通道管理"页面
2. 找到任意通道
3. 点击"AI录像"按钮
4. 查看提示信息

**方法二：通过API**
```bash
# 启动AI录像
curl -X POST http://localhost:9080/api/ai/recording/start \
  -H "Content-Type: application/json" \
  -d '{
    "channel_id": "34020000001320000132",
    "stream_url": "rtsp://192.168.1.100:554/stream",
    "mode": "person"
  }'

# 预期返回：
# {
#   "success": true,
#   "channel_id": "34020000001320000132",
#   "mode": "person"
# }
```

### 6. 查看AI录像状态

```bash
# 查看单个通道状态
curl http://localhost:9080/api/ai/recording/status?channel_id=34020000001320000132

# 查看所有通道状态
curl http://localhost:9080/api/ai/recording/status/all

# 预期返回：
# {
#   "success": true,
#   "status": {
#     "channel_id": "...",
#     "mode": "person",
#     "is_recording": false,
#     "stats": {
#       "TotalDetections": 10,
#       "PersonDetections": 5,
#       "RecordingSessions": 2,
#       ...
#     }
#   }
# }
```

### 7. 停止AI录像

```bash
curl -X POST http://localhost:9080/api/ai/recording/stop \
  -H "Content-Type: application/json" \
  -d '{"channel_id": "34020000001320000132"}'
```

## 完整测试流程

### 准备工作
```bash
# 1. 确保服务运行
ps aux | grep "./server"

# 2. 检查配置
curl http://localhost:9080/api/ai/config

# 3. 启动模拟AI服务
python3 /tmp/mock_ai_service.py &
```

### 端到端测试
```bash
#!/bin/bash
echo "=== AI录像功能端到端测试 ==="

CHANNEL_ID="34020000001320000132"
STREAM_URL="rtsp://test.stream/live"

# Step 1: 启用AI功能
echo -e "\n[1] 启用AI功能..."
curl -s -X PUT http://localhost:9080/api/ai/config \
  -H "Content-Type: application/json" \
  -d '{"Enable":true,"Confidence":0.5,"DetectInterval":2}' | python3 -m json.tool

# Step 2: 启动AI录像
echo -e "\n[2] 启动AI录像..."
curl -s -X POST http://localhost:9080/api/ai/recording/start \
  -H "Content-Type: application/json" \
  -d "{\"channel_id\":\"$CHANNEL_ID\",\"stream_url\":\"$STREAM_URL\",\"mode\":\"person\"}" | python3 -m json.tool

# Step 3: 等待一段时间
echo -e "\n[3] 等待检测运行 (10秒)..."
sleep 10

# Step 4: 查看状态
echo -e "\n[4] 查看AI录像状态..."
curl -s "http://localhost:9080/api/ai/recording/status?channel_id=$CHANNEL_ID" | python3 -m json.tool

# Step 5: 停止AI录像
echo -e "\n[5] 停止AI录像..."
curl -s -X POST http://localhost:9080/api/ai/recording/stop \
  -H "Content-Type: application/json" \
  -d "{\"channel_id\":\"$CHANNEL_ID\"}" | python3 -m json.tool

# Step 6: 查看最终状态
echo -e "\n[6] 查看最终状态..."
curl -s http://localhost:9080/api/ai/recording/status/all | python3 -m json.tool

echo -e "\n=== 测试完成 ==="
```

## 验证检查点

### ✅ 系统就绪检查
- [ ] 服务正常运行（`ps aux | grep server`）
- [ ] AI配置API可访问（`/api/ai/config`）
- [ ] AI管理器已初始化（查看日志：`grep AI server.log`）

### ✅ 配置验证
- [ ] AI功能可启用/禁用
- [ ] 配置参数可修改
- [ ] 配置保存到config.yaml

### ✅ API功能验证
- [ ] `/api/ai/recording/start` - 启动成功
- [ ] `/api/ai/recording/stop` - 停止成功
- [ ] `/api/ai/recording/status` - 状态查询
- [ ] `/api/ai/recording/status/all` - 全局状态
- [ ] `/api/ai/config` - 配置获取/更新

### ✅ 前端界面验证
- [ ] 设置页面显示AI配置section
- [ ] AI配置表单可编辑
- [ ] 通道管理页面显示"AI录像"按钮
- [ ] 点击按钮触发正确的API调用
- [ ] 错误提示信息正确显示

### ✅ 集成验证（需要真实AI服务）
- [ ] AI服务可访问
- [ ] 帧捕获功能正常
- [ ] 人形检测返回正确
- [ ] 录像自动启停
- [ ] 统计数据准确

## 常见问题排查

### 问题1: 503 Service Unavailable
**原因**: AI功能未启用或AI管理器未初始化

**解决方案**:
```bash
# 检查日志
grep "AI管理器" server.log

# 如果看到"AI功能未启用"，则：
curl -X PUT http://localhost:9080/api/ai/config \
  -H "Content-Type: application/json" \
  -d '{"Enable":true}'

# 重启服务
pkill -f ./server && sleep 2 && ./server &
```

### 问题2: JSON parse error
**原因**: API返回非JSON格式（如错误文本）

**解决方案**:
- 已在前端添加错误处理
- 检查响应状态码
- 显示友好错误信息

### 问题3: AI检测失败
**原因**: AI服务不可访问或返回错误

**检查**:
```bash
# 测试AI服务
curl http://localhost:8000/detect

# 检查AIEndpoint配置
curl http://localhost:9080/api/ai/config | grep APIEndpoint
```

### 问题4: 帧捕获失败
**原因**: FFmpeg未安装或流地址无效

**解决方案**:
```bash
# 检查FFmpeg
which ffmpeg

# 如未安装
sudo apt install ffmpeg  # Ubuntu/Debian
sudo yum install ffmpeg  # CentOS/RHEL
```

## 实际使用场景

### 场景1: 门口监控（省空间）
```yaml
AI:
  Enable: true
  Confidence: 0.6      # 较高置信度，减少误报
  DetectInterval: 3    # 3秒检测一次
  RecordDelay: 15      # 人离开后继续录15秒
  MinRecordTime: 10    # 最少录10秒
```

### 场景2: 停车场（快速响应）
```yaml
AI:
  Enable: true
  Confidence: 0.5      # 中等置信度
  DetectInterval: 1    # 1秒检测一次
  RecordDelay: 5       # 快速停止
  MinRecordTime: 3     # 短片段即可
```

### 场景3: 仓库（低频检测）
```yaml
AI:
  Enable: true
  Confidence: 0.7      # 高置信度
  DetectInterval: 10   # 10秒检测一次
  RecordDelay: 30      # 长延迟
  MinRecordTime: 30    # 完整片段
```

## 监控和日志

### 查看运行日志
```bash
# AI相关日志
tail -f server.log | grep -i "ai\|detect\|recording"

# 实时监控
watch -n 2 'curl -s http://localhost:9080/api/ai/recording/status/all | python3 -m json.tool'
```

### 性能指标
- 检测频率: 根据DetectInterval
- CPU占用: 主要来自FFmpeg帧捕获
- 内存占用: 每个通道 ~50-100MB
- 网络流量: 取决于AI服务位置

## 下一步

### 生产环境部署
1. **部署真实AI服务**
   - Python + YOLOv8
   - TensorFlow Serving
   - 云AI服务（阿里云、腾讯云）

2. **优化配置**
   - 根据场景调整参数
   - 监控资源使用
   - 设置告警阈值

3. **扩展功能**
   - 多类别检测（车辆、动物）
   - 行为分析（跌倒、打架）
   - 人脸识别集成

