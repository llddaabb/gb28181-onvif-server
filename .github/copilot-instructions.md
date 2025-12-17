# Copilot / AI agent instructions for this repository

短说明：该仓库是一个基于 Go 的 GB28181/ONVIF 媒体服务，集成 ZLMediaKit（可嵌入）和一个可选的 Python ONNX AI 检测服务。下面的说明帮助 AI 编码代理快速定位关键组件、遵循本项目约定并安全修改代码。

- **架构快照**: API 服务（Go）负责设备管理、通道和代理流；ZLM（ZLMediaKit）负责媒体转发与录制；AI 检测为可选外部/本地 HTTP 服务用于图像/录像触发。
  - API 启动入口: [cmd/server/main.go](cmd/server/main.go)
  - REST 实现与路由: [internal/api/server.go](internal/api/server.go)
  - AI 管理器: [internal/ai/manager.go](internal/ai/manager.go) 与检测/抓帧实现在 [internal/ai](internal/ai)
  - ZLM 管理: [internal/zlm](internal/zlm)（可嵌入或外部）
  - 配置文件: [configs/config.yaml](configs/config.yaml)

- **重要运行/构建命令**
  - 构建服务：`make build`（仅 Go 服务）或 `make build-all`（包含 ZLM 编译）
  - 运行（开发）: `make run` 或使用启动脚本 `./start.sh start`
  - 跳过嵌入 ZLM：`./server -config configs/config.yaml --no-zlm` 或 `make run-no-zlm`
  - 前端构建：`cd frontend && npm install && npm run build`
  - 测试：`make test`（等同 `go test -v ./...`）

- **项目约定 & 易错点（务必遵守）**
  - WriteTimeout 被显式设为 0（见 `internal/api/server.go`），以避免取消长连接的流式代理。当修改 HTTP server，切勿恢复为短的 WriteTimeout，除非验证对流媒体影响。
  - ZLM 嵌入检测由 `internal/zlm/embedded` 目录的存在决定（Makefile 会为其设置 `embed_zlm` build tag）。不要随意移动或重命名此目录。
  - 日志与调试：全局调试通过 `Debug` 配置控制，默认日志文件为 `logs/debug.log`（参见 `cmd/server/main.go` 和 `configs/config.yaml`）。新增日志使用 `internal/debug` 包的 `debug.Info/Warn/Error`。
  - API 路由模式：所有管理接口统一以 `/api/...` 前缀注册（见 `internal/api/server.go` 中的 `setupRoutes`），修改路由请在此文件中统一调整并保留现有兼容路径。

- **AI 服务与本地模型**
  - AI 在配置 `configs/config.yaml` 的 `AI.Enable: true` 时启用；服务由 `start_ai_detector.sh` 管理并生成 `ai_detector_service.py`（Flask + ONNXRuntime）。
  - 默认模型路径：`models/yolov8s.onnx`（或 `third-party/zlm/models/yolov8s.onnx`），修改模型路径请同时更新 `configs/config.yaml` 与 `start_ai_detector.sh` 的生成脚本环境变量。
  - 本地检测器以 HTTP API 形式暴露（默认端口 8001）；Go 端通过 `internal/ai/onnx_detector.go` 或 `HTTPDetector` 与之通信（`APIEndpoint` 配置项）。

- **修改/PR 指南（对 AI 代理特别重要）**
  - 优先修改最靠近问题的文件；例如路由或 HTTP 行为改动应修改 `internal/api/server.go`，AI 逻辑改动应在 `internal/ai/*` 或 `ai_detector_service.py` 中实现。
  - 不要更改 ZLM 启动/管理的语义（`internal/zlm/ProcessManager` 与 `cmd/server/main.go`），若确实需要变动，请同时更新 `Makefile` 的 build tag 检测逻辑。
  - 修改配置字段时，保持与 `configs/config.yaml` 的向后兼容；新增配置需在 `config/config.go` 中解析并在 `cmd/server/main.go` 中使用。
  - 增加日志时使用 `internal/debug`，避免直接 fmt.Println 除非是临时本地调试。

- **参考示例（常见任务）**
  - 启动服务并查看日志：`./start.sh start`，日志文件 `logs/server.log` 与 `logs/debug.log`。
  - 启动 AI 服务：`./start_ai_detector.sh start`（会生成 `ai_detector_service.py` 并运行 Flask 服务）。

遇到不明确的运行环境或第三方依赖（例如 ZLMediaKit 编译失败或 Python 包安装问题），请先在 PR 中添加明确的复现步骤与建议命令（例如 `make build-zlm` 或 `pip3 install onnxruntime`），并标注可能需要的外部资源。

如果你希望我把某一部分展开为更详尽的开发者指南（例如 ZLM 嵌入构建步骤或 AI 模型部署说明），请告诉我你想看到的部分。
