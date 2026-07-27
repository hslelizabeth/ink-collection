# 文房藏珍

个人文房藏品记录网站：钢笔、墨水、砚台、墨条…… 藏器于身，静待知音。

- 后端：Go + Gin + SQLite（纯 Go 驱动，无 cgo），单二进制，内嵌前端
- 前端：Vue 3 + Vite，中国风设计，桌面 / 平板 / 手机自适应
- 部署：单 Docker 容器，数据全部落在一个 `/data` 卷里

## 功能

- 四类预设藏品：钢笔（笔尖类型）、墨水（颜色）、砚台（坑口）、墨条（年代、类型）
- 通用字段：名称、品牌、状态（收藏 / 已结缘）、多图、购入/结缘时间与价格、备注
- 钢笔 ↔ 墨水多对多关联（品类关联可在后台自定义）
- 品类后台配置化：新增品类、自定义专属字段、自定义品类间关联，均无需改代码
- 首页统计：当前收藏件数、收藏总值、分品类数量与金额占比
- 列表筛选：状态 / 品牌 / 各专属字段 + 名称搜索
- 数据备份：管理页一键下载数据库文件

## 目录结构

```
├── server/     # Go 后端（API + 静态服务 + embed 前端）
├── web/        # Vue3 前端源码
├── design/     # 设计稿原型（mockup.html，仅供参考）
├── Dockerfile
└── docker-compose.yml
```

## 本地开发

```bash
# 后端（Go 1.26+）
cd server && go run .
# 前端（Node 24+），dev server 已配置 /api 与 /uploads 代理到 8080
cd web && npm install && npm run dev
```

环境变量：`PORT`（默认 8080）、`DATA_DIR`（默认 ./data）、`WEB_DIR`（设置后从磁盘目录提供前端，不读 embed）。

本地构建完整二进制：

```bash
cd web && npm run build && cd ..
rm -rf server/web/dist && cp -R web/dist server/web/dist
cd server && CGO_ENABLED=0 go build -o inkcollection .
DATA_DIR=./data ./inkcollection
```

## Docker 部署（NAS）

### 方式一：GitHub Actions 构建（推荐，本机无需 Docker）

项目自带 `.github/workflows/docker.yml`：把代码推到 GitHub 后，Actions 会自动构建 **linux/amd64 + linux/arm64** 双架构镜像并推送到 GHCR（GitHub 容器镜像仓库）。

1. 推送代码到 GitHub（公开或私有仓库均可，无需配置任何 secret，`GITHUB_TOKEN` 自动注入）
2. 等 Actions 跑完，镜像地址为 `ghcr.io/<你的用户名>/<仓库名>:latest`
3. 在 NAS 上拉取并运行：

```bash
# 私有仓库需要先登录（Personal Access Token 勾选 read:packages）
echo "<PAT>" | docker login ghcr.io -u <你的用户名> --password-stdin

docker pull ghcr.io/<你的用户名>/<仓库名>:latest
docker run -d --name ink-collection \
  -p 8080:8080 \
  -v /volume1/docker/ink-collection/data:/data \
  -e TZ=Asia/Shanghai \
  --restart unless-stopped \
  ghcr.io/<你的用户名>/<仓库名>:latest
```

后续每次 `git push` 到 main 都会自动出新镜像，NAS 上 `docker pull` 后重启容器即完成升级。打 `v1.0.0` 之类的 tag 还会生成对应版本号镜像。

### 方式二：在 NAS 上直接构建

把项目目录（可剔除 `.toolchain/`、`design/`、`web/node_modules/`）拷贝到 NAS，SSH 执行：

```bash
cd ink-collection
docker build -t ink-collection:latest .
docker run -d --name ink-collection \
  -p 8080:8080 \
  -v /volume1/docker/ink-collection/data:/data \
  -e TZ=Asia/Shanghai \
  --restart unless-stopped \
  ink-collection:latest
```

或 `docker compose up -d`（compose 默认把数据放在项目目录下的 `./data`）。

### 方式三：在别的机器上构建后导入

```bash
# 在有 Docker 的机器上
docker build -t ink-collection:latest .
docker save ink-collection:latest | gzip > ink-collection.tar.gz
# 拷贝到 NAS 后
gunzip -c ink-collection.tar.gz | docker load
# 然后按方式一的 docker run 启动
```

群晖（Container Manager）：项目 → 新增 → 选择 docker-compose.yml 所在路径，或「映像 → 导入」上面的 tar.gz。

## 对外暴露域名

容器只监听 8080 HTTP，对外请用反向代理终结 HTTPS：

- 群晖：控制面板 → 登录门户 → 高级 → 反向代理服务器，来源 `https://你的域名:443`，目的地 `http://127.0.0.1:8080`
- 或 Nginx Proxy Manager / Caddy / 群晖外自建的 Nginx，反代到 `NAS_IP:8080`

Nginx 参考配置：

```nginx
server {
    listen 443 ssl;
    server_name ink.example.com;
    # ssl_certificate / ssl_certificate_key 略
    client_max_body_size 25m;   # 图片上传上限（单张 20MB）
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

> 注意：应用本身无登录认证。公网暴露前，建议在反代层加 Basic Auth 或访问控制。

## 数据与备份

- 所有数据都在挂载的 `/data` 卷：`collection.db`（SQLite）+ `uploads/`（图片与缩略图）
- 管理页「数据备份」可下载数据库文件；图片目录请随卷一起备份
- 完整备份 = 拷贝整个 data 目录；恢复 = 放回后重启容器

## API 概览

`/api/categories`、`/api/items`（支持 `category_id` `status` `brand` `q` `page` `f_<字段key>` 过滤）、`/api/items/:id/images`、`/api/filters`、`/api/stats`、`/api/backup`。状态值：`collecting`（收藏）/ `parted`（已结缘）。
