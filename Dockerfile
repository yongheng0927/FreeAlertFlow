# Fenghuo 多阶段构建：前端产物内嵌进 Go 单二进制

# 阶段 1：构建前端
FROM node:24-alpine AS web
WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# 阶段 2：构建后端（web/dist 来自阶段 1 的真实产物）
FROM golang:1.25-alpine AS server
# 构建期 Go 模块代理：默认空（走官方 proxy.golang.org），国内构建可传
# --build-arg GOPROXY=https://goproxy.cn,direct（compose 里从 .env 读取）
ARG GOPROXY
ENV GOPROXY=${GOPROXY}
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /build/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/fenghuo ./cmd/fenghuo

# 阶段 3：最小运行镜像（二进制为静态编译，无 libc 依赖）
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=server /out/fenghuo /usr/local/bin/fenghuo
EXPOSE 8080
ENTRYPOINT ["fenghuo"]
