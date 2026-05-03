# 个人云盘前端

技术栈：Vue 3 + TypeScript + Vite + Pinia + Element Plus。

## 本地开发

```bash
npm install
npm run dev
```

开发服务默认运行在 `http://localhost:5173`，并通过 Vite proxy 将 `/api` 转发到后端 `http://localhost:8080`。

如需改后端地址，可以复制 `.env.example` 为 `.env.local` 并调整：

```bash
VITE_API_BASE_URL=/api/v1
```

## 源码结构说明

前端源码主要位于 `src` 目录中，整体职责是：提供登录/注册页面、维护登录状态、调用后端接口、展示文件列表，并支持上传、下载、重命名和删除文件。

### 应用入口

- `src/main.ts`：前端应用入口。创建 Vue 应用，并注册 Pinia、Vue Router 和 Element Plus，最后挂载到页面上的 `#app` 节点。
- `src/App.vue`：根组件。当前只包含 `<router-view />`，用于根据路由渲染登录页或文件页。
- `src/env.d.ts`：Vite/TypeScript 的环境类型声明文件，让 TypeScript 能识别 Vite 提供的类型。

### 路由

- `src/router/index.ts`：定义前端路由和访问控制。
  - `/login`：登录/注册页。
  - `/files`：文件管理页，需要登录后访问。
  - `/` 和未知路径会重定向到 `/files`。
  - 路由守卫会根据 Pinia 中的登录状态决定是否跳转到登录页。

### 状态管理

- `src/stores/auth.ts`：使用 Pinia 管理用户登录状态。
  - 保存 `token`、`expiresAt` 和 `user`。
  - 提供登录、注册、刷新用户信息、退出登录等 action。
  - 初始化时会从 `localStorage` 恢复登录信息。

### API 请求

- `src/api/request.ts`：统一的请求封装。
  - 设置默认请求头。
  - 自动携带 `Authorization: Bearer <token>`。
  - 统一解析后端返回的 `{ code, msg, data }` 格式。
  - 在 token 失效时清理本地登录状态并跳转到登录页。
- `src/api/auth.ts`：认证相关接口封装。
  - 登录：`POST /auth/login`
  - 注册：`POST /auth/register`
  - 获取当前用户：`GET /auth/profile`
- `src/api/files.ts`：文件相关接口封装。
  - 文件列表：`GET /files`
  - 上传文件：`POST /files/upload`
  - 获取下载链接：`GET /files/{id}/download`
  - 删除文件：`DELETE /files/{id}`
  - 重命名文件：`PUT /files/{id}/rename`

### 页面组件

- `src/views/LoginView.vue`：登录/注册页面。
  - 使用 Element Plus 表单组件。
  - 通过 `mode` 在登录和注册两种模式之间切换。
  - 登录成功后保存用户信息并跳转到 `/files`。
  - 注册成功后切回登录模式。
- `src/views/FilesView.vue`：文件管理页面。
  - 页面加载时刷新用户信息并拉取文件列表。
  - 展示文件名、类型、大小、更新时间和操作按钮。
  - 支持上传文件、下载文件、重命名、删除、刷新列表和分页。
  - 下载时先向后端请求预签名下载链接，再用 `window.open` 打开链接。

### 类型与工具函数

- `src/types/api.ts`：定义前后端交互使用的 TypeScript 类型。
  - `ApiResponse<T>`：统一响应结构。
  - `User`：用户信息。
  - `LoginRequest`、`LoginResponse`、`RegisterRequest`：认证相关类型。
  - `Matter`：文件或文件夹条目。
  - `FileListResponse`、`FileUploadResponse`、`DownloadResponse`：文件相关响应类型。
- `src/utils/authStorage.ts`：封装 `localStorage` 中登录信息的读写和清理。
- `src/utils/format.ts`：格式化工具。
  - `formatBytes`：将字节数格式化为 B、KB、MB、GB 等。
  - `formatDate`：将时间字符串格式化为中文日期时间。

### 样式

- `src/styles.css`：全局样式文件。
  - 定义页面字体、背景、登录页布局、文件页布局、表格、工具栏、分页等样式。
  - 包含移动端响应式布局规则。

### 构建与开发配置

- `vite.config.ts`：Vite 配置文件。
  - 启用 Vue 插件。
  - 配置 `@` 指向 `src` 目录，方便使用 `@/api/request` 这类路径。
  - 开发服务器端口为 `5173`。
  - 将 `/api` 代理到后端 `http://localhost:8080`。
- `package.json`：前端依赖和脚本。
  - `npm run dev`：启动 Vite 开发服务器。
  - `npm run build`：执行类型检查并构建生产产物。
  - `npm run preview`：预览构建后的生产产物。
  - `npm run typecheck`：只执行 TypeScript 类型检查。

## 已接入接口

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/register`
- `GET /api/v1/auth/profile`
- `GET /api/v1/files`
- `POST /api/v1/files/upload`
- `GET /api/v1/files/{id}/download`
- `DELETE /api/v1/files/{id}`
- `PUT /api/v1/files/{id}/rename`
