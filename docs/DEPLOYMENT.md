# 部署指南

本文档提供详细的部署步骤和最佳实践。

## 目录

- [生产环境部署](#生产环境部署)
- [Docker 部署](#docker-部署)
- [系统服务配置](#系统服务配置)
- [反向代理配置](#反向代理配置)
- [安全加固](#安全加固)

## 生产环境部署

### 1. 系统准备

#### Ubuntu/Debian

```bash
# 更新系统
sudo apt update && sudo apt upgrade -y

# 安装依赖
sudo apt install -y curl wget tar

# 创建服务用户
sudo useradd -r -s /bin/false gb28181

# 创建应用目录
sudo mkdir -p /opt/gb28181-server
sudo chown gb28181:gb28181 /opt/gb28181-server
```

#### CentOS/RHEL

```bash
# 更新系统
sudo yum update -y

# 安装依赖
sudo yum install -y curl wget tar

# 创建服务用户
sudo useradd -r -s /sbin/nologin gb28181

# 创建应用目录
sudo mkdir -p /opt/gb28181-server
sudo chown gb28181:gb28181 /opt/gb28181-server
```

### 2. 部署应用

```bash
# 下载发布包
cd /tmp
wget <发布包URL>/gb28181-server-linux-amd64.tar.gz

# 解压到应用目录
sudo tar -xzf gb28181-server-linux-amd64.tar.gz -C /opt/
sudo chown -R gb28181:gb28181 /opt/gb28181-server-linux-amd64
cd /opt/gb28181-server-linux-amd64

# 创建必要的目录
sudo mkdir -p logs recordings
sudo chown -R gb28181:gb28181 logs recordings
```

### 3. 配置应用

```bash
# 编辑配置文件
sudo vim configs/config.yaml
```

**关键配置项：**

```yaml
GB28181:
  SipIP: "0.0.0.0"
  SipPort: 5060
  LocalIP: "your-server-ip"      # 修改为服务器公网IP
  Realm: "3402000000"             # 修改为实际 Realm
  ServerID: "34020000002000000001"  # 修改为实际 ID

ZLM:
  UseEmbedded: true
  API:
    Secret: "your-strong-secret"  # 修改为强密码
    
Debug:
  Enabled: false                  # 生产环境关闭调试
  LogLevel: "info"
```

### 4. 配置防火墙

#### UFW (Ubuntu)

```bash
# 开放必要端口
sudo ufw allow 5060/udp comment 'GB28181 SIP'
sudo ufw allow 9080/tcp comment 'Web/API'
sudo ufw allow 10080/tcp comment 'ZLM API'
sudo ufw allow 554/tcp comment 'RTSP'
sudo ufw allow 1935/tcp comment 'RTMP'
sudo ufw allow 8000:9000/tcp comment 'RTP'
sudo ufw allow 30000:30500/udp comment 'RTP Receive'
```

#### FirewallD (CentOS)

```bash
# 开放必要端口
sudo firewall-cmd --permanent --add-port=5060/udp
sudo firewall-cmd --permanent --add-port=9080/tcp
sudo firewall-cmd --permanent --add-port=10080/tcp
sudo firewall-cmd --permanent --add-port=554/tcp
sudo firewall-cmd --permanent --add-port=1935/tcp
sudo firewall-cmd --permanent --add-port=8000-9000/tcp
sudo firewall-cmd --permanent --add-port=30000-30500/udp
sudo firewall-cmd --reload
```

## 系统服务配置

### Systemd 服务

创建服务文件：

```bash
sudo vim /etc/systemd/system/gb28181-server.service
```

```ini
[Unit]
Description=GB28181/ONVIF Video Surveillance Server
After=network.target

[Service]
Type=simple
User=gb28181
Group=gb28181
WorkingDirectory=/opt/gb28181-server-linux-amd64
ExecStart=/opt/gb28181-server-linux-amd64/gb28181-server
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

# 安全配置
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/gb28181-server-linux-amd64/logs /opt/gb28181-server-linux-amd64/recordings

# 资源限制
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
```

启用并启动服务：

```bash
# 重载 systemd
sudo systemctl daemon-reload

# 启用开机自启
sudo systemctl enable gb28181-server

# 启动服务
sudo systemctl start gb28181-server

# 查看状态
sudo systemctl status gb28181-server

# 查看日志
sudo journalctl -u gb28181-server -f
```

### 服务管理命令

```bash
# 启动服务
sudo systemctl start gb28181-server

# 停止服务
sudo systemctl stop gb28181-server

# 重启服务
sudo systemctl restart gb28181-server

# 查看状态
sudo systemctl status gb28181-server

# 查看日志
sudo journalctl -u gb28181-server --since today

# 实时日志
sudo journalctl -u gb28181-server -f
```

## Docker 部署

### Dockerfile

```dockerfile
FROM ubuntu:22.04

# 安装依赖
RUN apt-get update && \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# 创建应用目录
WORKDIR /app

# 复制应用文件
COPY gb28181-server /app/
COPY configs /app/configs
RUN mkdir -p /app/logs /app/recordings

# 开放端口
EXPOSE 5060/udp 9080 10080 554 1935 8000-9000 30000-30500/udp

# 运行应用
CMD ["/app/gb28181-server"]
```

### docker-compose.yml

```yaml
version: '3.8'

services:
  gb28181-server:
    build: .
    container_name: gb28181-server
    restart: unless-stopped
    network_mode: host
    volumes:
      - ./configs:/app/configs
      - ./logs:/app/logs
      - ./recordings:/app/recordings
    environment:
      - TZ=Asia/Shanghai
```

### 构建和运行

```bash
# 构建镜像
docker build -t gb28181-server .

# 运行容器
docker run -d \
  --name gb28181-server \
  --network host \
  -v $(pwd)/configs:/app/configs \
  -v $(pwd)/logs:/app/logs \
  -v $(pwd)/recordings:/app/recordings \
  -e TZ=Asia/Shanghai \
  gb28181-server

# 使用 docker-compose
docker-compose up -d

# 查看日志
docker logs -f gb28181-server
```

## 反向代理配置

### Nginx

```nginx
upstream gb28181_backend {
    server 127.0.0.1:9080;
}

server {
    listen 80;
    server_name your-domain.com;

    # 重定向到 HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name your-domain.com;

    # SSL 证书
    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # 安全头
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # 客户端最大上传大小
    client_max_body_size 100M;

    # 代理配置
    location / {
        proxy_pass http://gb28181_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # WebSocket 支持
    location /ws {
        proxy_pass http://gb28181_backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_read_timeout 86400;
    }

    # 静态文件缓存
    location ~* \.(jpg|jpeg|png|gif|ico|css|js)$ {
        proxy_pass http://gb28181_backend;
        expires 30d;
        add_header Cache-Control "public, immutable";
    }
}
```

## 安全加固

### 1. 修改默认端口

```yaml
# configs/config.yaml
API:
  Port: 8443  # 修改为非标准端口
  
ZLM:
  API:
    Port: 8880  # 修改为非标准端口
```

### 2. 配置访问控制

```yaml
API:
  AllowedIPs:
    - "192.168.1.0/24"
    - "10.0.0.0/8"
```

### 3. 启用 HTTPS

```yaml
API:
  EnableTLS: true
  CertFile: "/path/to/cert.pem"
  KeyFile: "/path/to/key.pem"
```

### 4. 强化 ZLM API 密钥

```yaml
ZLM:
  API:
    Secret: "$(openssl rand -base64 32)"
```

### 5. 限制文件权限

```bash
# 配置文件只读
sudo chmod 600 configs/config.yaml

# 日志目录只允许写入
sudo chmod 700 logs

# 录像目录限制访问
sudo chmod 700 recordings
```

### 6. 启用审计日志

```yaml
Debug:
  Enabled: true
  LogLevel: "info"
  AuditLog: "logs/audit.log"
```

## 监控和维护

### 1. 日志轮转

创建 `/etc/logrotate.d/gb28181-server`：

```
/opt/gb28181-server-linux-amd64/logs/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0640 gb28181 gb28181
    sharedscripts
    postrotate
        systemctl reload gb28181-server >/dev/null 2>&1 || true
    endscript
}
```

### 2. 健康检查

```bash
# 检查服务状态
curl -f http://localhost:9080/api/system/health || exit 1

# 检查 ZLM 状态
curl -f http://localhost:10080/index/api/getServerConfig || exit 1
```

### 3. 备份配置

```bash
#!/bin/bash
BACKUP_DIR="/backup/gb28181"
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p $BACKUP_DIR
tar -czf $BACKUP_DIR/config_$DATE.tar.gz \
    /opt/gb28181-server-linux-amd64/configs/

# 保留最近 30 天的备份
find $BACKUP_DIR -name "config_*.tar.gz" -mtime +30 -delete
```

### 4. 磁盘空间监控

```bash
# 监控录像目录
RECORDINGS_DIR="/opt/gb28181-server-linux-amd64/recordings"
USAGE=$(df -h $RECORDINGS_DIR | awk 'NR==2 {print $5}' | sed 's/%//')

if [ $USAGE -gt 80 ]; then
    echo "警告: 录像目录使用率 ${USAGE}%"
    # 发送告警通知
fi
```

## 性能调优

### 1. 系统参数

```bash
# /etc/sysctl.conf
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
net.ipv4.tcp_rmem = 4096 87380 67108864
net.ipv4.tcp_wmem = 4096 65536 67108864
net.core.netdev_max_backlog = 5000
net.ipv4.tcp_max_syn_backlog = 8192
fs.file-max = 1000000

# 应用配置
sudo sysctl -p
```

### 2. 资源限制

```bash
# /etc/security/limits.conf
gb28181 soft nofile 65536
gb28181 hard nofile 65536
gb28181 soft nproc 4096
gb28181 hard nproc 4096
```

### 3. ZLM 性能优化

参考 [配置优化文档](CONFIG_OPTIMIZATION.md)

## 故障排查

### 查看日志

```bash
# 应用日志
tail -f /opt/gb28181-server-linux-amd64/logs/debug.log

# 系统日志
sudo journalctl -u gb28181-server -f

# ZLM 日志
tail -f /opt/gb28181-server-linux-amd64/build/zlm-runtime/log/*.log
```

### 检查端口

```bash
# 查看监听端口
sudo ss -tulpn | grep gb28181

# 检查端口占用
sudo lsof -i :5060
sudo lsof -i :9080
```

### 网络诊断

```bash
# 测试 SIP 连接
sudo tcpdump -i any -n port 5060

# 测试 RTP 流
sudo tcpdump -i any -n 'udp port 30000-30500'
```

## 升级

### 1. 备份

```bash
# 停止服务
sudo systemctl stop gb28181-server

# 备份当前版本
sudo tar -czf /backup/gb28181-$(date +%Y%m%d).tar.gz \
    /opt/gb28181-server-linux-amd64
```

### 2. 升级

```bash
# 下载新版本
cd /tmp
wget <新版本URL>/gb28181-server-linux-amd64.tar.gz

# 解压
sudo tar -xzf gb28181-server-linux-amd64.tar.gz -C /opt/

# 恢复配置和数据
sudo cp /backup/gb28181-*/configs/* /opt/gb28181-server-linux-amd64/configs/
```

### 3. 验证

```bash
# 启动服务
sudo systemctl start gb28181-server

# 检查状态
sudo systemctl status gb28181-server

# 查看日志
sudo journalctl -u gb28181-server -f
```

## 卸载

```bash
# 停止服务
sudo systemctl stop gb28181-server
sudo systemctl disable gb28181-server

# 删除服务文件
sudo rm /etc/systemd/system/gb28181-server.service
sudo systemctl daemon-reload

# 删除应用
sudo rm -rf /opt/gb28181-server-linux-amd64

# 删除用户
sudo userdel gb28181
```
