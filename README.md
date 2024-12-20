# API Proxy Server

一个高性能的 API 代理服务器，支持模型重定向和请求转发。主要用于代理 OpenAI API 请求，支持流式响应和自动模型映射。

## 功能特性

- ✨ API 请求代理转发
- 🔄 模型重定向映射
- 📡 流式响应支持
- 🔑 请求认证
- 📊 指标监控
- ⚡ 配置热重载
- 🔒 安全的密钥管理
- 🚀 高性能设计

## 快速开始

### 使用 Docker（推荐）

1. 克隆项目并准备配置：
```bash
# 克隆项目
git clone https://github.com/TerrySiu98/api-proxy.git
cd api-proxy

# 创建配置目录
mkdir -p config

# 复制示例配置文件
cp config.json.example config/config.json

# 编辑配置文件，填入您的 API 密钥和允许的客户端密钥
vim config/config.json
```

2. 使用 Docker 运行：
```bash
docker pull terrysiu/api-proxy:latest
docker run -p 5000:5000 -v $(pwd)/config:/app/config terrysiu/api-proxy:latest
```

或使用 docker-compose：
```bash
docker-compose up -d
```

### 手动构建

1. 克隆仓库：
```bash
git clone https://github.com/TerrySiu98/api-proxy.git
cd api-proxy
```

2. 准备配置：
```bash
cp config.json.example config/config.json
# 编辑 config/config.json 填入必要的配置
```

3. 构建和运行：
```bash
go build -o api-proxy
./api-proxy
```

## 配置说明

在 `config/config.json` 中配置：

```json
{
    "api": {
        "url": "https://api.openai.com/v1/chat/completions",
        "key": "your-api-key-here"
    },
    "allowed_keys": [
        "your-client-key-1",
        "your-client-key-2"
    ],
    "model_redirects": {
        "gpt-4": "gpt-4-turbo-preview",
        "gpt-3.5-turbo": "gpt-3.5-turbo-0125"
    }
}
```

- `api.url`: 目标 API 地址
- `api.key`: OpenAI API 密钥
- `allowed_keys`: 允许的客户端密钥列表
- `model_redirects`: 模型重定向映射配置

## API 端点

### 聊天完成接口
- 路径: `/v1/chat/completions`
- 方法: `POST`
- 请求头: 
  - `Authorization: Bearer your-client-key`
  - `Content-Type: application/json`
- 请求体示例:
```json
{
    "model": "gpt-4",
    "messages": [
        {
            "role": "user",
            "content": "Hello!"
        }
    ],
    "stream": false
}
```

### 健康检查
- 路径: `/health`
- 方法: `GET`
- 响应示例:
```json
{
    "status": "healthy",
    "time": "2024-03-19T10:30:00Z"
}
```

### 监控指标
- 路径: `/metrics`
- 方法: `GET`
- 响应示例:
```json
{
    "active_requests": 2,
    "total_requests": 1000,
    "error_count": 5,
    "uptime_seconds": 3600,
    "goroutines": 10,
    "cpu_cores": 4,
    "model_redirects": 2
}
```

## 环境变量

- `CONFIG_PATH`: 配置文件路径（默认: `config.json`）

## 性能优化

- 使用连接池管理 HTTP 连接
- 实现请求缓冲池
- 支持并发请求处理
- 自动内存管理
- 配置热重载
- 流式响应支持

## 监控指标

通过 `/metrics` 端点可以获取以下指标：
- 活跃请求数：当前正在处理的请求数量
- 总请求数：服务启动以来处理的总请求数
- 错误计数：请求处理过程中的错误��
- 运行时间：服务器运行时长（秒）
- Go 协程数：当前活跃的 goroutine 数量
- CPU 核心数：服务器 CPU 核心数
- 模型重定向数：配置的模型重定向规则数量

## 构建发布

项目使用 GitHub Actions 自动构建和发布 Docker 镜像：
- 每次推送到 main 分支时自动构建
- 自动发布到 Docker Hub (`terrysiu/api-proxy`)

## 安全说明

- API 密钥和客户端密钥存储在配置文件中
- 配置文件不会被提交到代码仓库
- 所有请求都需要认证
- 支持 HTTPS 代理
- 定期更新依赖以修复安全漏洞

## 许可证

[MIT License](LICENSE)

## 贡献指南

1. Fork 本仓库
2. 创建您的特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交您的更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开一个 Pull Request

## 作者

Terry Siu (@TerrySiu98)

## 支持

如果您在使用过程中遇到任何问题，请：
1. 查看 [issues](https://github.com/TerrySiu98/api-proxy/issues) 是否有类似问题
2. 创建新的 issue 描述您的问题