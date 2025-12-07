# Release v1.0.0 (2025-12-07)

## 🎉 首个正式版本发布

GB28181/ONVIF 智能视频监控平台正式发布！这是一个企业级的视频监控解决方案，集成了先进的AI人员检测和智能录像功能。

## ✨ 核心特性

### 📹 多协议视频监控
- **GB28181 国标协议**
  - 完整的SIP信令实现
  - 设备注册和心跳保活
  - 目录查询和通道管理
  - 实时视频预览
  - PTZ云台控制
  
- **ONVIF 协议支持**
  - 自动设备发现（WS-Discovery）
  - 设备信息查询
  - Profile管理
  - 实时流预览
  
- **多格式流媒体**
  - RTSP (端口: 8554)
  - RTMP (端口: 1935)
  - HLS (HTTP动态切片)
  - FLV (HTTP-FLV流)

### 🤖 AI智能录像

基于YOLOv8的先进AI检测引擎：

- **人员检测**
  - ONNX Runtime推理引擎
  - CPU/GPU灵活部署
  - 可调节置信度阈值（默认0.5）
  - 可配置检测频率（默认2秒）
  
- **智能触发录像**
  - 检测到人员自动开始录像
  - 无人时自动停止，节省存储
  - 支持多通道并发检测
  - 实时统计和状态监控
  
- **性能优化**
  - 轻量级YOLOv8s模型（43MB）
  - 320x320输入尺寸平衡速度和精度
  - 异步处理不影响视频流
  - 内存占用优化

### 🎯 录像管理

- **持久化录像**
  - 录像状态自动保存
  - 断流恢复后自动重启
  - 支持手动和AI两种录像模式
  
- **智能守护**
  - 10秒检测间隔
  - 自动检测录像中断
  - 流恢复后立即重启录像
  
- **录像查询和回放**
  - 按时间范围查询
  - 在线播放
  - 下载支持
  - 录像文件管理

### 🌐 Web管理界面

现代化的Vue 3单页应用：

- **技术栈**
  - Vue 3 Composition API
  - TypeScript
  - Vite构建工具
  - 响应式设计
  
- **功能模块**
  - 实时视频预览
  - GB28181设备管理
  - ONVIF设备管理
  - 通道管理
  - AI录像配置
  - 录像回放
  - 系统状态监控
  - ZLM管理

## 📦 部署和运维

### 一键启动
```bash
./start.sh start
```

### 独立服务管理
```bash
# 主服务
./start.sh start|stop|restart|status

# AI检测器
./start_ai_detector.sh start|stop|restart|status|test|logs
```

### 配置管理
- 统一配置文件 `configs/config.yaml`
- ZLM配置自动生成
- 环境变量支持

## 🔧 API接口

### RESTful API
- 完整的通道管理API
- AI录像控制API
- GB28181设备API
- ONVIF设备API
- 录像查询和下载API
- 系统状态API

### API文档
访问 http://localhost:8080/swagger 查看ZLM API文档

## 📊 系统要求

### 最低配置
- CPU: 2核
- 内存: 4GB
- 存储: 20GB（不含录像）
- 操作系统: Linux (Ubuntu 20.04+)

### 推荐配置
- CPU: 4核+
- 内存: 8GB+
- 存储: 100GB+（根据录像需求）
- 操作系统: Ubuntu 22.04 LTS

### 软件依赖
- Go 1.19+
- Python 3.10+
- Node.js 16+

## 🚀 快速开始

### 1. 克隆项目
```bash
git clone <repository-url>
cd zpip
```

### 2. 启动服务
```bash
./start.sh start
```

### 3. 访问界面
打开浏览器访问: http://localhost:5173

## 📝 配置示例

### 启用AI录像
```yaml
AI:
  Enable: true
  APIEndpoint: "http://localhost:8001/detect"
  Confidence: 0.5
  DetectInterval: 2
```

### 配置GB28181
```yaml
GB28181:
  Enable: true
  ServerID: "34020000002000000001"
  ServerDomain: "3402000000"
  ServerIP: "192.168.1.100"
  ServerPort: 5060
```

### 配置ONVIF
```yaml
ONVIF:
  Enable: true
  DiscoveryInterval: 60
```

## 🐛 已知问题

- 部分ONVIF设备可能需要手动配置认证信息
- AI检测在低端CPU上可能需要调整检测间隔
- 大量并发流时建议增加系统资源

## 🔜 后续计划

### v1.1.0 计划
- [ ] 增加更多AI检测类型（车辆、动物等）
- [ ] 支持多GPU并行检测
- [ ] 录像文件自动清理策略
- [ ] 移动端适配
- [ ] 告警通知功能

### v1.2.0 计划
- [ ] 级联服务器支持
- [ ] 云存储集成
- [ ] 用户权限管理
- [ ] API认证和安全加固

## 📄 文档

- [README.md](README.md) - 项目说明
- [AI_DETECTOR_README.md](AI_DETECTOR_README.md) - AI检测器文档
- [AI_RECORDING_README.md](AI_RECORDING_README.md) - AI录像文档
- [DOWNLOAD_MODEL_GUIDE.md](DOWNLOAD_MODEL_GUIDE.md) - 模型下载指南

## 🤝 贡献

欢迎提交Issue和Pull Request！

## 📮 反馈

如有问题或建议，请：
- 提交 Issue
- 发送邮件
- 加入讨论组

## 🙏 致谢

感谢以下开源项目：
- [ZLMediaKit](https://github.com/ZLMediaKit/ZLMediaKit)
- [Ultralytics YOLOv8](https://github.com/ultralytics/ultralytics)
- [ONNX Runtime](https://onnxruntime.ai/)
- [Vue.js](https://vuejs.org/)

---

⭐ **如果这个项目对你有帮助，请给个Star！**

📥 **下载**: [v1.0.0.tar.gz](https://github.com/yourusername/zpip/archive/refs/tags/v1.0.0.tar.gz)
