# ---------- 前端构建 ----------
FROM node:24-alpine AS webbuild
WORKDIR /build
COPY web/package.json web/package-lock.json* ./
RUN npm install --no-fund --no-audit
COPY web/ ./
RUN npm run build

# ---------- 后端构建 ----------
FROM golang:1.26-alpine AS gobuild
WORKDIR /src
# 依赖缓存层
COPY server/go.mod server/go.sum ./
RUN go mod download
# 拷贝后端源码与前端产物（embed 需要 web/dist 位于 server 下）
COPY server/ ./
COPY --from=webbuild /build/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/inkcollection .

# ---------- 运行时 ----------
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 1000 ink
WORKDIR /app
COPY --from=gobuild /app/inkcollection ./inkcollection
RUN mkdir -p /data && chown ink:ink /data
USER ink
ENV PORT=8080 DATA_DIR=/data
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s \
  CMD wget -qO- http://127.0.0.1:8080/api/categories > /dev/null || exit 1
CMD ["./inkcollection"]
