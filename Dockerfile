FROM golang:1.21-alpine

WORKDIR /app

# 安装基础工具
RUN apk add --no-cache git

# 复制 go.mod 和 go.sum（如果存在）
COPY go.* ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN go build -o api-proxy main.go

# 暴露端口
EXPOSE 5000

# 创建配置目录
RUN mkdir -p /app/config

# 运行应用
CMD ["./api-proxy"]