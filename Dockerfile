# 第一阶段：构建应用
FROM golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./

# 复制后端源代码
COPY backend ./backend

# 构建二进制文件
RUN cd backend && \
    go env -w GOPROXY=https://goproxy.cn && \
    CGO_ENABLED=0 GOOS=linux go build -o /fus main.go

# 第二阶段：创建运行环境
FROM alpine:latest

# 设置工作目录
WORKDIR /

# 从构建阶段复制二进制文件
COPY --from=builder /fus /fus

# 复制前端文件
COPY frontend /frontend  

# 设置环境变量
ENV PORT=8080 \
    AUTH_USERS="admin:admin,admin123:admin123,zz:zz"

# 暴露端口
EXPOSE $PORT

# 启动应用
CMD ["/fus"]
