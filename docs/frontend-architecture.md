# 前端架构图

## 整体架构

```mermaid
flowchart LR
    User[用户浏览器] --> Vite[Vite Dev Server / 静态资源]
    Vite --> VueApp[Vue 3 单页应用]

    subgraph Frontend[frontend]
        VueApp --> Router[Vue Router]
        VueApp --> Pinia[Pinia Auth Store]
        VueApp --> Views[LoginView / FilesView]
        Views --> ApiClient[API Client request]
        Pinia --> AuthStorage[localStorage 登录态]
    end

    ApiClient -->|HTTP /api/v1| Gin[Go Gin Backend]

    subgraph Backend[backend]
        Gin --> Middleware[CORS / JWT Middleware]
        Middleware --> Handler[Handler]
        Handler --> Service[Service]
        Service --> Repo[Repository]
        Service --> ObjectStorage[Object Storage Client]
    end

    Repo --> MySQL[(MySQL)]
    Service --> Redis[(Redis)]
    ObjectStorage --> MinIO[(MinIO Bucket)]
```

## 前端模块关系

```mermaid
flowchart TD
    Main[main.ts] --> App[App.vue]
    Main --> Router[index.ts]
    Main --> Pinia[Pinia]
    Main --> ElementPlus[Element Plus]

    Router --> LoginView[LoginView.vue]
    Router --> FilesView[FilesView.vue]
    Router --> Guard[路由守卫]

    Guard --> AuthStore[stores/auth.ts]
    LoginView --> AuthStore
    FilesView --> AuthStore

    AuthStore --> AuthApi[api/auth.ts]
    FilesView --> FilesApi[api/files.ts]

    AuthApi --> Request[api/request.ts]
    FilesApi --> Request
    Request --> AuthStorage[utils/authStorage.ts]
    FilesView --> Format[utils/format.ts]
    AuthApi --> ApiTypes[types/api.ts]
    FilesApi --> ApiTypes
```

## 登录态流转

```mermaid
sequenceDiagram
    participant U as 用户
    participant Login as LoginView
    participant Store as AuthStore
    participant API as request/auth API
    participant Backend as Go Backend
    participant LS as localStorage

    U->>Login: 输入用户名和密码
    Login->>Store: login(payload)
    Store->>API: POST /auth/login
    API->>Backend: 携带 JSON 请求体
    Backend-->>API: token / expires_at / user
    API-->>Store: 登录响应
    Store->>LS: 保存 token、过期时间、用户信息
    Store-->>Login: 登录成功
    Login->>U: 跳转 /files
```

## 文件操作流转

```mermaid
sequenceDiagram
    participant U as 用户
    participant Files as FilesView
    participant API as api/files.ts
    participant Request as request.ts
    participant Backend as Go Backend
    participant MinIO as MinIO
    participant DB as MySQL

    U->>Files: 进入文件页
    Files->>API: listFiles(folder_id, page, page_size)
    API->>Request: GET /files
    Request->>Backend: Authorization Bearer token
    Backend->>DB: 查询文件元数据
    Backend-->>Files: 文件列表

    U->>Files: 选择上传文件
    Files->>API: uploadFile(file, parent_id)
    API->>Request: POST /files/upload FormData
    Request->>Backend: Authorization Bearer token
    Backend->>MinIO: 保存对象
    Backend->>DB: 写入 matter 元数据
    Backend-->>Files: 上传结果

    U->>Files: 点击下载
    Files->>API: getDownloadUrl(id)
    API->>Request: GET /files/{id}/download
    Request->>Backend: Authorization Bearer token
    Backend->>MinIO: 生成预签名 URL
    Backend-->>Files: 下载 URL
    Files->>U: 新窗口打开下载链接
```

## 关键设计点

- 前端只负责交互、状态与 API 编排，不直接关心 MySQL、Redis、MinIO 等基础设施。
- `request.ts` 是前端访问后端的统一入口，集中处理 API 基础路径、JSON 头、token 注入和错误提示。
- `stores/auth.ts` 是登录态唯一来源，页面通过 Store 判断是否已认证。
- 路由守卫负责保护 `/files`，未登录用户会被引导到 `/login`。
- 文件列表、上传、下载、删除、重命名都通过后端鉴权接口完成，后端继续保证用户只能操作自己的文件。
