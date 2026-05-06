# 个人云盘系统

分布式云盘系统，支持文件上传下载、文件夹管理、用户认证等功能。

## 技术栈

- **后端**: Go + Gin + MySQL + Redis + MinIO
- **前端**: Vue 3 + TypeScript + Vite + Element Plus

## 架构

```
前端 (Vue) --> 后端 (Go/Gin) --> MySQL
                               --> Redis
                               --> MinIO (对象存储)
```

## 功能

- 用户注册、登录、JWT 认证
- 文件上传 / 下载（预签名 URL）
- 文件夹创建 / 重命名 / 移动
- 文件重命名 / 移动 / 软删除
- 面包屑导航
- 跨用户权限隔离
- 分页查询

## 快速启动

### 环境要求

- [Docker Desktop](https://www.docker.com/products/docker-desktop/)
- [Go 1.24+](https://go.dev/dl/)
- [Node.js 22+](https://nodejs.org/)

<details>
<summary><b>一键启动</b></summary>

Windows:

```bash
.\dev.bat
```

Linux / Mac:

```bash
bash dev.sh
```

</details>

<details>
<summary><b>手动启动</b></summary>

**1. 启动基础设施**

```bash
docker compose up -d
```

启动 MySQL (3306)、Redis (6379)、MinIO (9000/9001)。

**2. 启动后端**

```bash
cd backend
go mod download
go run cmd/server/main.go
```

运行在 http://localhost:8080

**3. 启动前端**

```bash
cd frontend
npm install
npm run dev
```

运行在 http://localhost:5173

**4. 访问**

浏览器打开 http://localhost:5173

</details>

## 项目结构

```
├── backend/                # Go 后端
│   ├── cmd/server/         # 入口
│   ├── internal/
│   │   ├── config/         # 配置
│   │   ├── handler/        # HTTP 处理器
│   │   ├── service/        # 业务逻辑
│   │   ├── repository/     # 数据库访问
│   │   ├── middleware/      # 中间件（认证、CORS、日志）
│   │   ├── model/          # 数据模型
│   │   ├── cache/          # Redis 缓存
│   │   ├── storage/        # MinIO 操作
│   │   ├── router/         # 路由注册
│   │   └── pkg/            # 工具包（JWT、哈希、错误码）
│   ├── migrations/         # 数据库初始化 SQL
│   └── test.sh             # API 集成测试
├── frontend/               # Vue 前端
│   └── src/
│       ├── api/            # API 请求封装
│       ├── views/          # 页面组件
│       ├── stores/         # Pinia 状态管理
│       ├── router/         # 路由
│       └── utils/          # 工具函数
├── docs/                   # API 文档 (OpenAPI)
└── docker-compose.yml      # MySQL、Redis、MinIO
```

## API 文档

完整接口规范见 [docs/api/openapi.yaml](docs/api/openapi.yaml)

## License

MIT
