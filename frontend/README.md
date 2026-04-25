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

## 已接入接口

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/register`
- `GET /api/v1/auth/profile`
- `GET /api/v1/files`
- `POST /api/v1/files/upload`
- `GET /api/v1/files/{id}/download`
- `DELETE /api/v1/files/{id}`
- `PUT /api/v1/files/{id}/rename`
