# 后端架构详细说明

## 整体分层

```
客户端（浏览器/前端）
       │
       │ HTTP 请求
       ▼
┌─────────────────────────────────────────────────┐
│                    Gin 引擎                       │
│                                                   │
│  ┌──────────┐    接管请求                         │
│  │  Router   │ ← 根据URL找到对应的Handler          │
│  └────┬─────┘                                    │
│       │                                          │
│       ▼                                          │
│  ┌───────────┐                                   │
│  │ Middleware │ ← 中间件链，逐个执行               │
│  │ (CORS/    │   某个中间件可以拦截请求            │
│  │  Auth/    │   直接返回响应                      │
│  │  Logger)  │                                   │
│  └────┬──────┘                                   │
│       │ 通过所有中间件后                           │
│       ▼                                          │
│  ┌──────────┐                                    │
│  │ Handler   │ ← 解析请求参数                     │
│  │          │   调用Service                       │
│  │          │   返回响应                          │
│  └────┬─────┘                                    │
│       │                                          │
│       ▼                                          │
│  ┌──────────┐                                    │
│  │ Service   │ ← 业务逻辑核心                     │
│  │          │   协调多个数据源                     │
│  └──┬───┬───┘                                    │
│     │   │                                        │
│     ▼   ▼          ▼                             │
│  ┌────┐ ┌─────┐ ┌───────┐                       │
│  │Repo│ │Cache│ │Storage│                        │
│  │MySQL│ │Redis│ │MinIO  │                        │
│  └────┘ └─────┘ └───────┘                        │
└─────────────────────────────────────────────────┘
```

**核心原则：单向依赖，分层调用。**
- 上层可以调用下层，下层不能调用上层
- Handler 不直接操作数据库，Service 不直接处理 HTTP
- 每一层只关心自己的职责，通过接口和相邻层交互

---

## 各层详解

### 1. Router（路由层）

**文件**: `internal/router/router.go`

Router 的职责就是把 URL 映射到对应的 Handler 方法。当客户端发送 `POST /api/v1/auth/login` 时，Router 负责找到并调用 `UserHandler.Login`。

```go
// 当前路由注册代码
v1 := r.Group("/api/v1")           // 所有接口以 /api/v1 开头

auth := v1.Group("/auth")          // /api/v1/auth 下的路由
auth.POST("/register", userHandler.Register)  // POST → Register方法
auth.POST("/login", userHandler.Login)        // POST → Login方法

authRequired := v1.Group("/auth")
authRequired.Use(middleware.AuthMiddleware(jwtSecret))  // 需要JWT认证
authRequired.GET("/profile", userHandler.GetProfile)    // GET → GetProfile方法
```

**路由分组**:
- 无认证的接口（register、login）放在同一个 Group
- 需要认证的接口（profile）放在另一个 Group，用 `Use()` 挂载中间件
- 这样不需要在每个接口上重复写认证逻辑

**类比**: Router 就像餐厅的菜单——客人说要"1号套餐"，服务员就知道去哪个窗口取餐。

**当前路由注册**:
```go
// 无需认证
v1.POST("/auth/register", userHandler.Register)
v1.POST("/auth/login", userHandler.Login)

// 需要JWT认证
authRequired.GET("/auth/profile", userHandler.GetProfile)
authRequired.POST("/files/upload", fileHandler.Upload)
authRequired.GET("/files", fileHandler.List)
authRequired.GET("/files/:id/download", fileHandler.Download)
authRequired.DELETE("/files/:id", fileHandler.Delete)
authRequired.PUT("/files/:id/rename", fileHandler.Rename)
```

**路由分组**:
- 无认证的接口放在公共 Group
- 需要认证的接口放在另一个 Group，用 `Use()` 挂载 AuthMiddleware
- 文件管理接口统一放在 `/files` 路径下

---

### 2. Middleware（中间件层）

**文件**: `internal/middleware/auth.go`、`internal/middleware/cors.go`

**工作原理**:

```
请求进入
    │
    ▼
┌────────────┐   失败   返回401错误
│ Auth中间件  │ ──────→ 直接结束
│ 解析JWT    │
└────┬───────┘
     │ 成功（user_id注入上下文）
     ▼
┌────────────┐
│ CORS中间件  │ ← 后续添加
└────┬───────┘
     │
     ▼
  Handler 处理请求
     │
     ▼
  响应返回
     │
     ▼
┌────────────┐
│ Logger中间件│ ← 记录请求耗时（后续添加）
└────────────┘
```

中间件像一条流水线上的检查站，每个检查站都可以：
- **放行**: 调用 `c.Next()`，让请求继续到下一个检查站
- **拦截**: 调用 `c.Abort()`，直接返回错误，后面的检查站和 Handler 都不会执行

**当前实现——AuthMiddleware 详细流程**:

```go
func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 第1步：从请求头取 Authorization 字段
        authHeader := c.GetHeader("Authorization")
        // 期望格式: "Bearer eyJhbGciOiJIUzI1NiIs..."

        if authHeader == "" {
            // 没有令牌 → 拦截，返回"缺少认证令牌"
            Fail(c, errcode.ErrInvalidToken, "缺少认证令牌")
            c.Abort()
            return
        }

        // 第2步：去掉 "Bearer " 前缀，拿到纯令牌字符串
        tokenString := strings.TrimPrefix(authHeader, "Bearer ")

        // 第3步：调用 JWT 工具解析验证令牌
        claims, err := pkgjwt.ParseToken(tokenString, jwtSecret)
        if err != nil {
            // 令牌无效或过期 → 拦截，返回"令牌无效或已过期"
            Fail(c, errcode.ErrInvalidToken, "令牌无效或已过期")
            c.Abort()
            return
        }

        // 第4步：验证通过，把 user_id 存入上下文
        c.Set("user_id", claims.UserID)

        // 第5步：放行，继续执行后续中间件和 Handler
        c.Next()
    }
}
```

**关键概念——Gin 上下文（gin.Context）**:
- `c` 是贯穿整个请求生命周期的一个对象
- 中间件通过 `c.Set("user_id", ...)` 写入数据
- Handler 通过 `c.Get("user_id")` 读取数据
- 这就是中间件和 Handler 之间传递信息的方式

**后续会添加的中间件**:
- **RateLimit**: 限制请求频率，防止恶意刷接口
- **Logging**: 记录每个请求的方法、路径、耗时、状态码
- **Recovery**: 捕获 Handler 中的 panic，防止整个服务崩溃

---

### 3. Handler（接口层）

**文件**: `internal/handler/user_handler.go`、`internal/handler/file_handler.go`、`internal/handler/response.go`

Handler 是 HTTP 请求和 Go 代码之间的桥梁——把 JSON 解析成结构体，调用 Service，再把结果序列化回 JSON。

**Handler 做什么、不做什么**:

| 做的事                         | 不做的事                      |
|-------------------------------|------------------------------|
| 解析请求参数（JSON → 结构体）   | 不写业务逻辑                   |
| 调用 Service 方法              | 不直接操作数据库                |
| 把 Service 返回值包装成 HTTP 响应 | 不做密码加密、查重等业务判断     |
| 处理参数格式错误                |                               |

**三个核心方法的详细解读**:

#### Register（注册）

```go
func (h *UserHandler) Register(c *gin.Context) {
    // 1. 解析请求：把 JSON body 映射到 RegisterRequest 结构体
    var req model.RegisterRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        // JSON格式不对、缺少必填字段 → 400 错误
        Fail(c, errcode.ErrParamInvalid, "参数错误")
        return
    }

    // 2. 调用 Service 执行注册业务
    user, err := h.userService.Register(&req)
    if err != nil {
        // 业务错误（用户名已存在等）→ 返回对应错误码
        Fail(c, errcode.ErrUserExists, err.Error())
        return
    }

    // 3. 成功 → 返回用户信息
    Success(c, user)
}
```

#### Login（登录）

```go
func (h *UserHandler) Login(c *gin.Context) {
    var req model.LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        Fail(c, errcode.ErrParamInvalid, "参数错误")
        return
    }

    resp, err := h.userService.Login(&req)
    if err != nil {
        Fail(c, errcode.ErrPasswordWrong, err.Error())
        return
    }

    Success(c, resp)
    // 返回的是 LoginResponse{Token, ExpiresAt, User}
}
```

#### GetProfile（获取用户信息）

```go
func (h *UserHandler) GetProfile(c *gin.Context) {
    // 从上下文取 user_id（由 AuthMiddleware 注入）
    userID, exists := c.Get("user_id")
    if !exists {
        Fail(c, errcode.ErrInvalidToken, "无效的用户ID")
        return
    }

    // 类型断言：c.Get 返回 interface{}，需要转成 uint64
    user, err := h.userService.GetProfile(userID.(uint64))
    if err != nil {
        Fail(c, errcode.ErrUserNotFound, "用户不存在")
        return
    }

    Success(c, user)
}
```

**统一响应格式**（response.go）:

```go
// 成功响应
Success(c, data)  → {"code": 0, "msg": "success", "data": {...}}

// 失败响应
Fail(c, errcode, msg) → {"code": 10001, "msg": "用户名已存在", "data": null}
```

所有接口统一用这两种格式返回，前端只需要判断 `code == 0` 就是成功。

---

### 4. Service（业务逻辑层）

**文件**: `internal/service/user_service.go`、`internal/service/file_service.go`

所有业务规则和判断逻辑都集中在 Service 层。Handler 只负责翻译 HTTP，Repository 只负责执行 SQL，真正的决策都在这里。

```
Handler:  "Service，有人要注册，这是他的用户名和密码"
Service:  "好的，我先看看这个名字有没有人用过"
          → 调用 userRepo.GetByUsername("testuser")
          → 数据库返回空，没人用过
Service:  "没人用过，那我把密码加密一下"
          → 调用 hash.HashPassword("123456")
          → 得到 "$2a$10$xxxx..."
Service:  "加密好了，存入数据库"
          → 调用 userRepo.Create(&User{Username: "testuser", Password: "$2a$10$..."})
          → 数据库返回成功
Service:  "Handler，注册成功了，这是用户信息"
Handler:  → 返回给客户端
```

**三个方法的详细业务流程**:

#### Register

```
输入: RegisterRequest{Username: "testuser", Password: "123456"}

步骤:
1. userRepo.GetByUsername("testuser")
   → 查数据库，判断用户名是否已存在
   → 如果存在，返回错误 "用户名已存在"

2. hash.HashPassword("123456")
   → 用 bcrypt 算法加密密码
   → 得到 "$2a$10$xxxxx..."

3. userRepo.Create(&User{Username: "testuser", Password: "加密后的密码"})
   → INSERT INTO users (username, password, ...) VALUES (...)
   → 数据库返回带 ID 的用户信息

输出: User{ID: 1, Username: "testuser", ...}
```

#### Login

```
输入: LoginRequest{Username: "testuser", Password: "123456"}

步骤:
1. userRepo.GetByUsername("testuser")
   → 查数据库找这个用户
   → 如果不存在，返回错误 "用户不存在"

2. hash.CheckPassword("123456", user.Password)
   → 对比明文密码和数据库中的加密密码
   → 如果不匹配，返回错误 "密码错误"

3. jwt.GenerateToken(user.ID, secret, expireHours)
   → 生成 JWT 令牌，包含 user_id 和过期时间
   → 得到 token 字符串和过期时间戳

输出: LoginResponse{Token: "eyJ...", ExpiresAt: 1776617748, User: {...}}
```

#### GetProfile

```
输入: userID = 1（uint64）

步骤:
1. userRepo.GetByID(1)
   → SELECT * FROM users WHERE id = 1
   → 如果不存在，返回错误

输出: User{ID: 1, Username: "testuser", ...}
```

**为什么 Service 层很重要**:
- Handler 换成另一个框架（比如从 Gin 换成 Echo），Service 层代码不需要改
- Repository 换成另一个数据库，Service 层也不需要改
- 业务规则集中在 Service 中，改需求只改这一层

---

### 5. Repository（数据访问层）

**文件**: `internal/repository/user_repo.go`、`internal/repository/file_repo.go`

Repository 把 SQL 操作封装成简洁的 Go 方法，供 Service 调用。Service 不需要写 SQL，只需要调 `Create()`、`GetByUsername()` 这些方法。

**Repository 做什么、不做什么**:

| 做的事                    | 不做的事                    |
|--------------------------|----------------------------|
| 执行具体的 SQL 语句         | 不做业务判断                 |
| 把数据库行映射为 Go 结构体   | 不知道业务逻辑是什么          |
| 处理数据库连接和错误         | 不处理 HTTP 相关的东西        |

**当前三个方法**:

```go
// UserRepo 持有数据库连接
type UserRepo struct {
    db *sql.DB
}

// Create - 插入一条用户记录
func (r *UserRepo) Create(user *model.User) error {
    // INSERT INTO users (username, password, nickname, status)
    // VALUES (?, ?, ?, ?)
    // 参数用 ? 占位，防止 SQL 注入
}

// GetByUsername - 按用户名查询
func (r *UserRepo) GetByUsername(username string) (*model.User, error) {
    // SELECT id, username, password, nickname, status, created_at, updated_at
    // FROM users WHERE username = ?
    // 用 rows.Scan() 把数据库列映射到 User 结构体的字段
}

// GetByID - 按ID查询
func (r *UserRepo) GetByID(id uint64) (*model.User, error) {
    // SELECT ... FROM users WHERE id = ?
}
```

**SQL 注入防护**:
Repository 使用参数化查询（`?` 占位符），而不是字符串拼接。这意味着：
```go
// ✅ 安全：参数化查询
rows, err := r.db.Query("SELECT * FROM users WHERE username = ?", username)

// ❌ 危险：字符串拼接（SQL注入漏洞）
rows, err := r.db.Query("SELECT * FROM users WHERE username = '" + username + "'")
```

---

### 6. Model（数据模型层）

**文件**: `internal/model/user.go`

Model 被所有层共用，但它不包含任何逻辑代码，只是数据容器。

Model 被所有层共用，但它不包含任何逻辑代码，只是数据容器。

**三种类型的模型**:

```
┌──────────────────────────────────────────────────────────┐
│                     Model 层的三个角色                      │
├──────────────────┬───────────────────┬───────────────────┤
│  数据库模型       │  请求模型          │  响应模型          │
│                  │                   │                   │
│  User            │  RegisterRequest  │  LoginResponse    │
│  (对应表结构)     │  LoginRequest     │  (返回给前端的数据) │
│                  │  (从前端接收的数据) │                   │
├──────────────────┼───────────────────┼───────────────────┤
│ 用在 Repository  │ 用在 Handler 入口  │ 用在 Handler 出口  │
│ 和 Service 中    │ 和 Service 中      │ 和 Service 中      │
└──────────────────┴───────────────────┴───────────────────┘
```

**为什么分请求模型和数据库模型**:

数据库模型 `User` 包含所有字段（包括密码），但注册请求 `RegisterRequest` 只需要用户名和密码。如果用同一个结构体，前端可能传入不该传的字段。分开后，每个模型只暴露需要的字段，更安全。

**json 标签的作用**:

```go
type User struct {
    ID        uint64    `json:"id"`         // 序列化时字段名为 "id"
    Username  string    `json:"username"`
    Password  string    `json:"-"`          // "-" 表示序列化时忽略这个字段（不返回给前端）
    Nickname  string    `json:"nickname"`
    Status    int       `json:"status"`
    CreatedAt time.Time `json:"created_at"` // Go 的驼峰命名 → JSON 的下划线命名
    UpdatedAt time.Time `json:"updated_at"`
}
```

当 `Success(c, user)` 把 User 序列化成 JSON 时：
- `json:"-"` 让 Password 字段不会出现在响应中
- `json:"created_at"` 把 Go 的 `CreatedAt` 转成前端习惯的 `created_at`

---

### 7. Config（配置层）

**文件**: `internal/config/config.go`

Config 在程序启动时加载一次，通过依赖注入传递给需要的组件。

**配置结构**:

```go
type Config struct {
    Server ServerConfig   // 端口号
    MySQL  MySQLConfig    // 数据库地址、端口、用户名、密码
    Redis  RedisConfig    // Redis 地址
    JWT    JWTConfig      // 密钥、过期时间
    MinIO  MinIOConfig    // MinIO 地址、密钥（后续使用）
}
```

**加载优先级**:
```
config.yaml 默认值  →  环境变量覆盖
```

YAML 文件提供默认值，敏感信息（密码、密钥）通过环境变量覆盖：
```bash
# 比如不想把密码写在 config.yaml 里
export CLOUD_MYSQL_PASSWORD="your_real_password"
```

**为什么不用配置中心**: 项目规模不大，YAML + 环境变量足够。如果后续部署到 Kubernetes，会改用 ConfigMap/Secret。

---

### 8. Database（数据库连接层）

**文件**: `internal/database/mysql.go`、`internal/database/redis.go`

Database 层负责建立数据库连接并配置连接池参数。

**MySQL 连接池**:

```go
func InitMySQL(cfg MySQLConfig) (*sql.DB, error) {
    // DSN: "user:password@tcp(host:port)/dbname?parseTime=true"
    db, err := sql.Open("mysql", dsn)

    // 连接池参数
    db.SetMaxOpenConns(100)      // 最大打开连接数
    db.SetMaxIdleConns(10)       // 最大空闲连接数
    db.SetConnMaxLifetime(time.Hour) // 连接最大存活时间

    // Ping 验证连接是否真的通了
    err = db.Ping()

    return db, nil
}
```

**什么是连接池**:
每次 SQL 查询都新建/关闭连接很慢。连接池提前创建好一批连接，需要时直接取，用完归还。

```
┌─────────────────────────────────────┐
│          连接池（最多100个连接）        │
│  [conn1] [conn2] [conn3] ... [conn10] │ ← 空闲10个
└──────┬───────┬───────┬────────────────┘
       │       │       │
    请求1    请求2    请求3     ← 来了请求，从池中取连接
```

**Redis 连接**:
类似 MySQL，建立连接后返回 `*redis.Client`，供 Cache 层使用。

---

### 9. Pkg（工具包）

**文件**: `internal/pkg/jwt/jwt.go`、`internal/pkg/hash/password.go`、`internal/pkg/hash/md5.go`、`internal/pkg/errcode/errcode.go`

Pkg 里放的是不含业务逻辑的通用工具函数，任何层都可以调用。

| 包 | 文件 | 提供的方法 | 被谁调用 |
|---|---|---|---|
| jwt | jwt.go | GenerateToken, ParseToken | Service（生成）, Middleware（解析） |
| hash | password.go | HashPassword, CheckPassword | Service（注册时加密，登录时验证） |
| hash | md5.go | MD5FromReader | Service（上传时计算文件哈希） |
| errcode | errcode.go | 错误码常量, GetMsg | Handler（返回错误时使用） |

**为什么单独放在 pkg/ 下**:
这些函数跟具体业务无关。比如 `HashPassword` 不管是用户密码还是管理员密码，加密逻辑都一样。独立出来方便复用，也不会跟业务代码混在一起。

---

### 10. Cache（缓存层，待实现）

**目录**: `internal/cache/`

Cache 封装 Redis 操作，把热点数据缓存起来，减少数据库查询。

**后续典型使用场景**:

```
GetProfile(userID)
  │
  ├─ 先查 Cache: GET cloud:userinfo:{user_id}
  │  → 命中：直接返回，不查数据库
  │
  └─ 未命中：查数据库 → 结果写入 Cache（TTL 10分钟）→ 返回
```

---

### 11. Storage（对象存储层）

**文件**: `internal/storage/minio_client.go`、`internal/storage/object_storage.go`

Storage 封装 MinIO 操作，处理文件的实际上传、下载、删除。数据库只存元数据（文件名、大小、哈希等），真正的文件内容存在 MinIO 里。

**提供的操作**:

| 方法 | 说明 |
|---|---|
| PutObject | 上传文件到 MinIO |
| GetObject | 下载文件（返回 io.ReadCloser） |
| RemoveObject | 删除文件 |
| GetPresignedURL | 生成预签名下载 URL，带中文文件名编码 |

**文件存储路径**: `{user_id}/{md5}{ext}`，如 `1/abc123def456.pdf`

**上传流程**:
```
Handler 收到 multipart 文件
  → Service 计算 MD5 哈希（TeeReader 同时读内容算哈希）
  → Storage.PutObject 存到 MinIO
  → Repository.Create 写入 matter 表
  → 返回文件 ID、名称、大小
```

**下载流程**:
```
Handler 收到下载请求
  → Service 查数据库验证文件归属
  → Storage.GetPresignedURL 生成临时链接（1小时有效）
  → 返回 URL，前端在新窗口打开下载
```

预签名 URL 通过 `response-content-disposition` 头设置 `filename*=UTF-8''` 编码，解决中文文件名乱码。

---

## 完整请求流程——登录为例

从客户端发送请求到收到响应，完整经过每一层：

```
时间线  层          发生了什么
──────────────────────────────────────────────────────────────
T1     客户端      POST /api/v1/auth/login
                   Body: {"username":"testuser","password":"123456"}

T2     Router      匹配路由: POST + /api/v1/auth/login → UserHandler.Login
                   (这个路由没有挂载 AuthMiddleware，所以直接到 Handler)

T3     Handler     c.ShouldBindJSON(&req)
                   JSON → LoginRequest{Username:"testuser", Password:"123456"}

T4     Handler     调用 h.userService.Login(&req)

T5     Service     调用 userRepo.GetByUsername("testuser")

T6     Repository  执行 SQL: SELECT * FROM users WHERE username = 'testuser'
                   MySQL 返回一行数据

T7     Repository  rows.Scan() → User{ID:1, Username:"testuser", Password:"$2a$10$..."}

T8     Service     调用 hash.CheckPassword("123456", "$2a$10$...")
                   bcrypt 对比 → true（密码正确）

T9     Service     调用 jwt.GenerateToken(1, secret, 24)
                   生成 JWT: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
                   过期时间: 1776617748

T10    Service     组装 LoginResponse{Token, ExpiresAt, User}
                   返回给 Handler

T11    Handler     调用 Success(c, resp)
                   LoginResponse → JSON 响应

T12    客户端      收到响应:
                   {"code":0, "msg":"success",
                    "data":{"token":"eyJ...", "expires_at":1776617748,
                            "user":{"id":1, "username":"testuser", ...}}}
```

---

## 依赖注入——从 main.go 看所有层如何串联

`main.go` 是整个应用的组装工厂，所有组件在这里创建并连接：

```go
func main() {
    // 加载配置
    cfg, _ := config.Load("config.yaml")

    // 建立连接
    db, _ := database.InitMySQL(cfg.MySQL)
    rdb, _ := database.InitRedis(cfg.Redis)
    minioClient, _ := storage.InitMinIO(cfg.MinIO)
    objStorage := storage.NewObjectStorage(minioClient, cfg.MinIO.Bucket)

    // 用户模块：Repo → Service → Handler
    userRepo := repository.NewUserRepo(db)
    userService := service.NewUserService(userRepo, cfg.JWT.Secret, cfg.JWT.ExpireHour)
    userHandler := handler.NewUserHandler(userService)

    // 文件模块：Repo → Service → Handler
    fileRepo := repository.NewFileRepo(db)
    fileService := service.NewFileService(fileRepo, objStorage)
    fileHandler := handler.NewFileHandler(fileService)

    // 注册路由并启动
    r := gin.Default()
    router.Setup(r, userHandler, fileHandler, cfg.JWT.Secret)
    r.Run(":8080")
}
```

**组装顺序很重要**：必须从最底层（Repository/Storage）开始创建，因为上层依赖下层。

```
db/minio → userRepo/fileRepo → userService/fileService → userHandler/fileHandler → router
```

每个模块（用户、文件）都有独立的 Repo → Service → Handler 链路，互不干扰。新增模块只需加一条链路，注册到 Router 即可。

**为什么叫"依赖注入"**:
`NewUserService(userRepo, ...)` 把 userRepo "注入"到 userService 中。
userService 不需要自己去创建 userRepo，而是由 main.go 创建好传进来。

好处：
- userService 不知道 db 的存在，只知道 Repo 提供了哪些方法
- 测试时可以传入一个假的 Repo（Mock），不需要真的连数据库
- 组件之间松耦合，换掉某个组件不影响其他组件

---

## 数据流向总结

```
                        请求方向                          响应方向
                     ─────────────→                   ←─────────────

客户端                  HTTP JSON                        HTTP JSON
  │                        │                                │
  │  Router              URL匹配                           路由匹配
  │    │                    │                                │
  │  Middleware         认证/校验                           记录日志
  │    │                    │                                │
  │  Handler          JSON → Go结构体                   Go结构体 → JSON
  │    │                    │                                │
  │  Service           业务逻辑处理                       组装业务结果
  │    │                    │                                │
  │  Repository        执行SQL查询                       映射查询结果
  │    │                    │                                │
  │  MySQL             存储数据                           返回数据行
```

每一层只做自己该做的事，通过参数和返回值与相邻层交互。
