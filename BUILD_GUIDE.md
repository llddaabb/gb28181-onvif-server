# 构建和部署指南

## 快速开始

### 1. 一键启动（推荐）

```bash
./start.sh start
```

这将自动启动：
- Go后端服务
- ZLMediaKit流媒体服务
- AI检测服务（如果启用）

### 2. 停止服务

```bash
./start.sh stop
```

### 3. 重启服务

```bash
./start.sh restart
```

### 4. 查看状态

```bash
./start.sh status
```

---

## 手动构建

### 后端构建

```bash
# 构建Go服务器
go build -o server cmd/server/main.go

# 运行
./server -config ./configs/config.yaml
```

### 前端构建

```bash
cd frontend

# 安装依赖
npm install

# 开发模式
npm run dev

# 生产构建
npm run build
```

### AI检测服务

```bash
# 安装依赖
./setup_ai_detector.sh

# 启动服务
./start_ai_detector.sh start

# 查看状态
./start_ai_detector.sh status

# 测试
./start_ai_detector.sh test
```

---

## 配置说明

### 主配置文件 `configs/config.yaml`

```yaml
Server:
  Port: 9080
  LogLevel: info

GB28181:
  SIPDomain: 3402000000
  ServerID: 34020000002000000001
  ServerIP: 192.168.18.222
  ServerPort: 5060

ZLM:
  HTTPPort: 8080
  RTSPPort: 8554
  RTMPPort: 1935

AI:
  Enable: true
  APIEndpoint: http://localhost:8001/detect
  Confidence: 0.5
```

### AI检测器配置

通过环境变量配置：

```bash
export AI_DETECTOR_PORT=8001
export AI_MODEL_PATH=models/yolov8s.onnx
export AI_CONFIDENCE=0.5
export AI_INPUT_SIZE=320
```

---

## Docker部署

### 构建镜像

```bash
docker build -t gb28181-server:v1.0.0 .
```

### 运行容器

```bash
docker-compose up -d
```

### 停止容器

```bash
docker-compose down
```

---

## 生产环境部署

### 1. 系统要求

- Ubuntu 20.04+ / CentOS 7+
- Go 1.20+
- Python 3.8+
- Node.js 16+
- 至少 2GB RAM
- 至少 10GB 磁盘空间

### 2. 安装依赖

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y golang-go python3-pip nodejs npm

# CentOS/RHEL
sudo yum install -y golang python3-pip nodejs npm
```

### 3. Python依赖

```bash
pip3 install flask opencv-python numpy onnxruntime ultralytics
```

### 4. 下载AI模型

```bash
# 方法1：使用脚本
./download_ai_model.sh

# 方法2：手动导出
python3 -c "from ultralytics import YOLO; YOLO('yolov8s.pt').export(format='onnx')"
mv yolov8s.onnx models/
```

### 5. 配置系统服务

创建 `/etc/systemd/system/gb28181-server.service`：

```ini
[Unit]
Description=GB28181/ONVIF Video Server
After=network.target

[Service]
Type=simple
User=your-user
WorkingDirectory=/path/to/zpip
ExecStart=/path/to/zpip/start.sh start
ExecStop=/path/to/zpip/start.sh stop
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable gb28181-server
sudo systemctl start gb28181-server
```

### 6. 配置防火墙

```bash
# 开放必要端口
sudo firewall-cmd --permanent --add-port=9080/tcp   # API
sudo firewall-cmd --permanent --add-port=8080/tcp   # ZLM HTTP
sudo firewall-cmd --permanent --add-port=8554/tcp   # RTSP
sudo firewall-cmd --permanent --add-port=1935/tcp   # RTMP
sudo firewall-cmd --permanent --add-port=5060/udp   # SIP
sudo firewall-cmd --permanent --add-port=5060/tcp   # SIP
sudo firewall-cmd --reload
```

### 7. 配置Nginx反向代理（可选）

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://localhost:9080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /ws {
        proxy_pass http://localhost:9080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

---

## 性能优化

### 1. ZLM配置优化

编辑 `third-party/zlm/conf/config.ini`：

```ini
[general]
maxConnections=1000
enableVhost=1

[http]
keepAliveSecond=30
```

### 2. AI检测优化

```bash
# 增加工作进程
export AI_WORKERS=4

# 降低输入尺寸提高速度
export AI_INPUT_SIZE=320

# 调整置信度
export AI_CONFIDENCE=0.6
```

### 3. 数据库优化（如果使用）

- 定期清理过期录像
- 建立索引优化查询
- 配置定期备份

---

## 故障排查

### 服务无法启动

```bash
# 查看日志
tail -f logs/server.log
tail -f logs/ai_detector.log
tail -f third-party/zlm/log/*.log

# 检查端口占用
lsof -i :9080
lsof -i :5060

# 检查进程
ps aux | grep server
ps aux | grep MediaServer
ps aux | grep ai_detector
```

### AI检测不工作

```bash
# 测试AI服务
curl http://localhost:8001/health

# 检查模型文件
ls -lh models/yolov8s.onnx

# 重启AI服务
./start_ai_detector.sh restart
```

### 录像失败

```bash
# 检查存储空间
df -h

# 检查ZLM状态
curl "http://localhost:8080/index/api/getMediaList?secret=YOUR_SECRET"

# 检查录像目录权限
ls -la third-party/zlm/www/record/
```

---

## 更新升级

```bash
# 1. 备份数据
cp -r logs logs.backup
cp -r third-party/zlm/www/record recordings.backup

# 2. 停止服务
./start.sh stop

# 3. 更新代码
git pull

# 4. 重新构建
go build -o server cmd/server/main.go
cd frontend && npm run build && cd ..

# 5. 启动服务
./start.sh start

# 6. 验证
./start.sh status
```

---

## 监控和维护

### 日志轮转

配置 `/etc/logrotate.d/gb28181-server`：

```
/path/to/zpip/logs/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0644 user user
}
```

### 磁盘清理

```bash
# 清理7天前的录像
find third-party/zlm/www/record/ -name "*.mp4" -mtime +7 -delete

# 清理旧日志
find logs/ -name "*.log.*" -mtime +30 -delete
```

### 健康检查脚本

```bash
#!/bin/bash
# health_check.sh

# 检查API
if ! curl -sf http://localhost:9080/api/system/health > /dev/null; then
    echo "API服务异常，正在重启..."
    ./start.sh restart
fi

# 检查AI服务
if ! curl -sf http://localhost:8001/health > /dev/null; then
    echo "AI服务异常，正在重启..."
    ./start_ai_detector.sh restart
fi
```

添加到crontab：

```bash
*/5 * * * * /path/to/zpip/health_check.sh
```

---

## 安全建议

1. **修改默认密钥**
   - 更改ZLM的secret
   - 更改GB28181的密码

2. **启用HTTPS**
   - 配置SSL证书
   - 使用Nginx反向代理

3. **访问控制**
   - 配置防火墙规则
   - 限制API访问IP

4. **定期更新**
   - 更新依赖包
   - 应用安全补丁

---

## 技术支持

- 查看日志：`./start.sh logs`
- 查看文档：`README.md`
- 问题反馈：GitHub Issues
