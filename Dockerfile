# 构建阶段
FROM golang:1.21-alpine AS builder

# 安装必要的构建工具
RUN apk add --no-cache git gcc musl-dev

# 设置工作目录
WORKDIR /app

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o nofx .

# 运行阶段
FROM alpine:latest

# 安装 ca-certificates（HTTPS 请求需要）
RUN apk --no-cache add ca-certificates tzdata

# 设置时区为上海
ENV TZ=Asia/Shanghai

# 创建非 root 用户
RUN addgroup -g 1000 nofx && \
    adduser -D -u 1000 -G nofx nofx

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/nofx .

# 复制配置文件示例
COPY config.json.example ./config.json.example

# 创建必要的目录
RUN mkdir -p decision_logs coin_pool_cache && \
    chown -R nofx:nofx /app

# 切换到非 root 用户
USER nofx

# 暴露 API 端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 启动应用
CMD ["./nofx"]
