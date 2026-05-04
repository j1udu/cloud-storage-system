# 前端架构详细说明

前端位于 `frontend` 目录，主要负责把个人云盘的功能呈现给用户。它不直接保存文件，也不直接连接 MySQL、Redis、MinIO，而是通过 HTTP 接口和后端通信。

当前前端是一个 Vue 3 单页应用。用户在浏览器中看到的登录页、文件列表、上传按钮、下载按钮，最终都会转化成对后端接口的调用。

---

## 整体分层

前端可以分成下面几层：

```
浏览器页面
  │
  │ 用户点击、输入、选择文件
  ▼
Vue 页面组件
  │
  │ 调用状态或接口方法
  ▼
Pinia 状态管理 / API 模块
  │
  │ 统一拼接请求、携带 token、处理响应
  ▼
request 请求封装
  │
  │ HTTP 请求
  ▼
Go 后端接口
```

**核心原则：页面负责交互，Store 负责状态，API 层负责通信。**

- 页面组件不直接拼接完整后端地址。
- 页面组件不直接读写复杂的登录状态。
- 认证 token 的携带逻辑集中在 `request.ts`。
- 路由是否允许进入，由路由守卫和登录状态共同决定。

---

## 目录结构

前端源码主要在 `frontend/src` 中：

```
src
├─ api        后端接口封装
├─ router     页面路由和访问控制
├─ stores     Pinia 状态管理
├─ types      前后端交互数据类型
├─ utils      通用工具函数
├─ views      页面组件
├─ App.vue    根组件
├─ main.ts    应用入口
└─ styles.css 全局样式
```

工程配置文件在 `frontend` 根目录：

| 文件 | 作用 |
|---|---|
| `package.json` | 依赖和 npm 脚本 |
| `vite.config.ts` | Vite 开发服务器、Vue 插件、路径别名、接口代理 |
| `tsconfig.json` | TypeScript 类型检查配置 |
| `index.html` | 浏览器加载的 HTML 入口 |

---

## 1. 应用入口

**文件**：`src/main.ts`

`main.ts` 是前端启动时第一个执行的 TypeScript 文件。它负责创建 Vue 应用，并注册项目需要的全局能力。

当前注册了三个主要插件：

| 插件 | 作用 |
|---|---|
| Pinia | 管理登录状态 |
| Vue Router | 控制页面跳转 |
| Element Plus | 提供按钮、表单、表格、弹窗等 UI 组件 |

代码核心是：

```ts
createApp(App).use(createPinia()).use(router).use(ElementPlus).mount('#app');
```

这句话可以按顺序理解：

1. 创建 Vue 应用。
2. 使用根组件 `App.vue`。
3. 注册 Pinia。
4. 注册路由。
5. 注册 Element Plus。
6. 把应用挂载到 `index.html` 中的 `#app` 节点。

---

## 2. 根组件

**文件**：`src/App.vue`

`App.vue` 当前只做一件事：提供路由出口。

```vue
<router-view />
```

`router-view` 会根据当前浏览器地址显示对应页面。

比如：

| 浏览器路径 | 渲染页面 |
|---|---|
| `/login` | `LoginView.vue` |
| `/files` | `FilesView.vue` |

也就是说，真正的业务界面不写在 `App.vue` 中，而是由路由决定显示哪个页面。

---

## 3. 路由层

**文件**：`src/router/index.ts`

路由层负责两件事：

1. 定义有哪些页面。
2. 判断用户是否有权限进入这些页面。

当前路由如下：

| 路径 | 页面 | 说明 |
|---|---|---|
| `/` | 重定向到 `/files` | 默认进入文件页 |
| `/login` | `LoginView.vue` | 登录和注册页面 |
| `/files` | `FilesView.vue` | 文件管理页面，需要登录 |
| 其他路径 | 重定向到 `/files` | 简单兜底 |

### 路由守卫

路由守卫是进入页面前执行的一段检查逻辑。

当前规则是：

- 如果访问 `/files` 时没有登录，就跳转到 `/login`。
- 如果已经登录，还访问 `/login`，就跳转到 `/files`。
- 如果从受保护页面被跳到登录页，会记录原本想去的地址。

比如用户直接打开：

```text
http://localhost:5173/files
```

如果本地没有 token，前端会跳到：

```text
http://localhost:5173/login?redirect=/files
```

登录成功后，再回到 `/files`。

---

## 4. 状态管理层

**文件**：`src/stores/auth.ts`

状态管理层使用 Pinia。当前只有一个认证 Store，名字是 `auth`。

它保存三类数据：

| 状态 | 说明 |
|---|---|
| `token` | 后端返回的 JWT 令牌 |
| `expiresAt` | token 过期时间 |
| `user` | 当前登录用户信息 |

### Store 提供的方法

| 方法 | 作用 |
|---|---|
| `login` | 调用登录接口，保存 token 和用户信息 |
| `register` | 调用注册接口 |
| `refreshProfile` | 根据 token 刷新当前用户信息 |
| `logout` | 清空登录状态 |
| `persist` | 把登录状态同步到 `localStorage` |

### 为什么需要 Pinia

登录状态不只一个页面需要：

- 路由守卫要判断用户是否已登录。
- 文件页面要显示当前用户昵称。
- 请求层要携带 token。
- 退出登录时需要清理状态。

如果每个页面各自保存一份登录状态，就容易不一致。Pinia 的作用就是让登录状态只有一个统一来源。

---

## 5. 本地存储工具

**文件**：`src/utils/authStorage.ts`

浏览器刷新页面后，内存中的 Pinia 状态会消失。为了让用户刷新后仍保持登录，项目把登录信息保存到了 `localStorage`。

保存的 key 有三个：

| key | 内容 |
|---|---|
| `cloud-storage-token` | JWT token |
| `cloud-storage-expires-at` | 过期时间 |
| `cloud-storage-user` | 用户信息 JSON 字符串 |

这个文件封装了读取、写入和清理逻辑。其他模块不需要知道具体 key 的名字，只需要调用函数。

---

## 6. API 请求层

API 请求层位于 `src/api` 目录。

它的作用是把“业务动作”包装成函数。页面只需要调用函数，不需要关心请求路径、请求头、响应解析这些细节。

### 统一请求封装

**文件**：`src/api/request.ts`

这是前端访问后端的统一入口。它负责：

- 拼接 API 基础路径。
- 根据请求体类型设置 `Content-Type`。
- 从本地读取 token，并放入 `Authorization` 请求头。
- 解析后端统一响应格式。
- 在 token 失效时清理本地登录状态。
- 把错误转换成统一的 `ApiError`。

后端统一响应格式是：

```ts
interface ApiResponse<T> {
  code: number;
  msg: string;
  data: T;
}
```

前端判断成功失败时，主要看 `code` 是否为 `0`。

### 认证接口

**文件**：`src/api/auth.ts`

这个文件封装用户认证相关接口：

| 函数 | 后端接口 | 作用 |
|---|---|---|
| `login` | `POST /auth/login` | 登录 |
| `register` | `POST /auth/register` | 注册 |
| `getProfile` | `GET /auth/profile` | 获取当前用户信息 |

注意这里写的是 `/auth/login`，不是完整的 `/api/v1/auth/login`。因为 `request.ts` 会自动补上基础路径 `/api/v1`。

### 文件接口

**文件**：`src/api/files.ts`

这个文件封装文件管理相关接口：

| 函数 | 后端接口 | 作用 |
|---|---|---|
| `listFiles` | `GET /files` | 查询文件列表 |
| `uploadFile` | `POST /files/upload` | 上传文件 |
| `getDownloadUrl` | `GET /files/{id}/download` | 获取下载链接 |
| `deleteFile` | `DELETE /files/{id}` | 删除文件 |
| `renameFile` | `PUT /files/{id}/rename` | 重命名 |

上传文件时使用的是 `FormData`。这是浏览器上传文件最常见的方式，后端可以通过 multipart 表单读取文件内容。

---

## 7. 类型定义层

**文件**：`src/types/api.ts`

这个文件定义前端和后端交互时用到的数据结构。

常见类型如下：

| 类型 | 说明 |
|---|---|
| `ApiResponse<T>` | 后端统一响应格式 |
| `User` | 用户信息 |
| `LoginRequest` | 登录请求体 |
| `RegisterRequest` | 注册请求体 |
| `LoginResponse` | 登录响应 |
| `Matter` | 文件或文件夹条目 |
| `FileListResponse` | 文件列表响应 |
| `FileUploadResponse` | 上传成功响应 |
| `DownloadResponse` | 下载链接响应 |

这里的 `Matter` 是文件列表里最核心的类型。它既可以表示文件，也可以表示文件夹，通过 `dir` 字段区分。

```ts
interface Matter {
  id: number;
  name: string;
  dir: boolean;
  size: number;
  ext: string;
  updated_at: string;
}
```

当 `dir` 是 `true` 时，表示文件夹；当 `dir` 是 `false` 时，表示普通文件。

---

## 8. 登录注册页面

**文件**：`src/views/LoginView.vue`

这个页面同时承担登录和注册两个功能。它通过 `mode` 状态判断当前处于哪种模式。

```ts
const mode = ref<'login' | 'register'>('login');
```

### 页面状态

| 状态 | 说明 |
|---|---|
| `form` | 用户名、密码、昵称 |
| `mode` | 当前是登录还是注册 |
| `loading` | 是否正在提交 |
| `formRef` | Element Plus 表单实例 |

### 表单校验

页面使用 Element Plus 的表单规则进行基础校验：

- 用户名不能为空。
- 用户名长度为 3 到 64 个字符。
- 密码不能为空。
- 密码长度为 6 到 128 个字符。
- 昵称不能超过 128 个字符。

这些校验主要是为了提升交互体验。真正的安全校验仍然需要后端保证。

### 登录流程

用户输入用户名和密码后，页面会：

1. 调用 Element Plus 表单校验。
2. 调用 `authStore.login`。
3. Store 调用 `api/auth.ts` 中的 `login`。
4. 请求层发送 `POST /api/v1/auth/login`。
5. 后端返回 token、过期时间、用户信息。
6. Store 保存登录状态。
7. 页面跳转到 `/files`。

### 注册流程

注册流程和登录共用同一个表单。注册成功后，当前设计不会自动登录，而是切回登录模式，让用户重新登录。

---

## 9. 文件管理页面

**文件**：`src/views/FilesView.vue`

这是当前前端最核心的业务页面。它负责展示文件列表，并提供上传、下载、删除、重命名等操作。

### 页面状态

| 状态 | 说明 |
|---|---|
| `files` | 当前目录下的文件列表 |
| `loading` | 文件列表是否正在加载 |
| `uploadLoading` | 上传是否正在进行 |
| `crumbs` | 面包屑路径 |
| `query` | 分页参数 |
| `renameVisible` | 重命名弹窗是否显示 |
| `currentFile` | 当前正在操作的文件 |
| `renameForm` | 重命名表单 |

### 页面初始化

文件页面加载时会做两件事：

1. 调用 `authStore.refreshProfile`，尝试刷新用户信息。
2. 调用 `fetchFiles`，加载当前目录文件列表。

### 文件列表

文件列表使用 Element Plus 的表格组件展示。表格列包括：

- 名称
- 类型
- 大小
- 更新时间
- 操作按钮

文件夹和普通文件使用同一个数据结构展示。区别是：

- 文件夹可以点击进入。
- 普通文件可以下载。
- 两者都可以重命名和删除。

### 面包屑目录

当前目录不是用一个单独变量保存，而是由 `crumbs` 的最后一个节点决定。

根目录默认是：

```ts
[{ id: 0, name: '全部文件' }]
```

进入文件夹时，会把新的文件夹追加到 `crumbs`。点击面包屑时，会截断后面的路径，然后重新拉取文件列表。

### 上传文件

上传按钮背后有一个隐藏的原生文件选择框：

```vue
<input ref="uploadInputRef" class="hidden-input" type="file" />
```

用户点击“上传文件”按钮时，代码会触发这个隐藏 input。用户选择文件后，页面把文件包装成 `FormData`，再调用上传接口。

上传成功后，页面会重新请求文件列表。

### 下载文件

下载不是前端直接从后端拿文件流，而是分两步：

1. 前端请求后端，获取下载链接。
2. 前端用新窗口打开这个链接。

后端返回的是 MinIO 的预签名 URL。这样真正的文件下载由对象存储完成，后端只负责鉴权和生成临时链接。

### 删除和重命名

删除前会弹出确认框，避免用户误删。

重命名使用 Element Plus 弹窗。打开弹窗时，页面会把当前文件保存到 `currentFile`，并把原文件名填入输入框。

操作成功后，都会重新调用 `fetchFiles` 刷新列表。

---

## 10. 工具函数

**文件**：`src/utils/format.ts`

这个文件放展示层常用的格式化逻辑。

| 函数 | 作用 |
|---|---|
| `formatBytes` | 把字节数转换成 B、KB、MB、GB 等格式 |
| `formatDate` | 把时间字符串格式化成中文日期时间 |

这些函数不涉及业务规则，只是为了让表格展示更适合用户阅读。

---

## 11. 全局样式

**文件**：`src/styles.css`

项目没有拆分多个样式文件，目前所有全局样式都集中在这里。

主要包含：

- 全局字体和背景。
- 登录页布局。
- 文件页顶栏。
- 文件表格容器。
- 工具栏和按钮排列。
- 文件名省略显示。
- 分页区域。
- 移动端响应式布局。

当前视觉结构比较简单：

- 登录页是左侧品牌区、右侧表单区。
- 文件页是顶部栏加中间文件表格。
- 小屏幕下横向布局会变成纵向布局。

---

## 12. Vite 配置

**文件**：`vite.config.ts`

Vite 配置主要做三件事：

### 启用 Vue 插件

```ts
plugins: [vue()]
```

没有这个插件，Vite 不知道如何处理 `.vue` 单文件组件。

### 配置路径别名

```ts
'@': fileURLToPath(new URL('./src', import.meta.url))
```

配置后，可以这样导入：

```ts
import { request } from '@/api/request';
```

不用写成：

```ts
import { request } from '../../api/request';
```

### 代理后端接口

开发服务器运行在 `5173` 端口，后端运行在 `8080` 端口。

浏览器访问前端时，请求路径是：

```text
/api/v1/files
```

Vite 会把它代理到后端：

```text
http://localhost:8080/api/v1/files
```

这样开发时前端不用直接跨域访问后端。

---

## 13. TypeScript 配置

**文件**：`tsconfig.json`、`src/env.d.ts`

`tsconfig.json` 负责告诉 TypeScript 如何检查项目代码。

当前重要配置包括：

| 配置 | 说明 |
|---|---|
| `strict` | 开启严格类型检查 |
| `moduleResolution` | 使用适合前端打包器的模块解析方式 |
| `paths` | 配置 `@/*` 路径别名 |
| `noUnusedLocals` | 不允许未使用的局部变量 |
| `noUnusedParameters` | 不允许未使用的函数参数 |

`src/env.d.ts` 负责补充 Vite 和 Vue 文件的类型声明。

其中 `declare module '*.vue'` 的作用是让 TypeScript 识别下面这种导入：

```ts
import App from './App.vue';
```

---

## 完整流程：从打开页面到看到文件

用户打开：

```text
http://localhost:5173
```

前端会按下面的顺序工作：

1. 浏览器加载 `index.html`。
2. `index.html` 加载 `src/main.ts`。
3. `main.ts` 创建 Vue 应用，并注册路由、Pinia、Element Plus。
4. 路由把 `/` 重定向到 `/files`。
5. 路由守卫检查登录状态。
6. 如果没有 token，跳转到 `/login`。
7. 用户登录成功后，Store 保存 token 和用户信息。
8. 页面跳转到 `/files`。
9. `FilesView.vue` 请求文件列表。
10. `request.ts` 自动携带 token。
11. Vite 把 `/api/v1/files` 代理到后端。
12. 后端返回文件列表。
13. 前端把文件列表渲染到表格中。

---

## 完整流程：上传文件

用户点击“上传文件”后：

1. 页面触发隐藏的文件选择框。
2. 用户选择本地文件。
3. `FilesView.vue` 读取选择的 `File` 对象。
4. `api/files.ts` 把文件放入 `FormData`。
5. `request.ts` 自动携带 token。
6. 前端发送 `POST /api/v1/files/upload`。
7. 后端验证用户身份。
8. 后端把文件保存到 MinIO。
9. 后端把文件元数据写入 MySQL。
10. 前端收到上传成功响应。
11. 页面重新拉取文件列表。

---

## 完整流程：下载文件

用户点击“下载”后：

1. `FilesView.vue` 调用 `getDownloadUrl`。
2. 前端发送 `GET /api/v1/files/{id}/download`。
3. 后端检查文件是否属于当前用户。
4. 后端向 MinIO 生成预签名下载链接。
5. 前端收到下载链接。
6. 前端用 `window.open` 打开链接。
7. 浏览器开始下载文件。

这里的重点是：前端没有直接接触 MinIO，也没有自己拼接文件地址。下载地址由后端生成，前端只负责打开。

---

## 数据流向总结

从前端角度看，数据流向可以总结为：

```
用户操作
  │
页面组件
  │
Store 或 API 函数
  │
request 统一请求封装
  │
后端接口
  │
返回 JSON 数据
  │
更新页面状态
  │
重新渲染界面
```

每一层都有明确职责：

| 层 | 负责什么 |
|---|---|
| 页面组件 | 展示界面，响应用户操作 |
| Store | 保存跨页面共享状态 |
| API 模块 | 描述具体后端接口 |
| request | 统一处理 HTTP 细节 |
| utils | 提供无业务副作用的工具函数 |
| types | 约束前后端数据结构 |

这种结构让代码更容易维护。新增页面时，通常新增一个 `views` 文件；新增接口时，通常新增或扩展 `api` 文件；新增共享状态时，再考虑新增 Store。
