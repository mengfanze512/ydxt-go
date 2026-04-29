# 阶段1: 编译阶段 (使用轻量级的 alpine 镜像构建)
FROM golang:alpine AS builder

# 设置 Go 环境变量
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=0

# 设置工作目录
WORKDIR /app

# 复制依赖描述文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码并进行编译
COPY . .
# 编译二进制文件，命名为 server
RUN GOOS=linux GOARCH=amd64 go build -o server ./cmd/server/main.go

# 阶段2: 运行阶段 (使用极其轻量的 alpine 镜像)
FROM alpine:latest

# 安装 tzdata (时区) 和 bash (云托管 initenv.sh 必须)
RUN apk add --no-cache tzdata bash
ENV TZ=Asia/Shanghai

WORKDIR /app

# 从 builder 阶段拷贝编译好的二进制文件和配置文件
COPY --from=builder /app/server .
COPY --from=builder /app/config/config.yaml ./config/config.yaml

# 暴露 80 端口 (微信云托管默认监听 80)
EXPOSE 80

# 启动服务
CMD ["./server"]
