# 云盘系统面试题全覆盖

---

## 一、架构设计

### Q1. 三层架构的取舍

> 你在云盘系统中用了 Handler-Service-Repository 分层。如果业务继续增长，比如要加「分享」功能，分享给别人后对方也能下载，你会怎么安排这个逻辑？

分享涉及文件归属判断（Service层）、生成分享链接（新的ShareService）、权限校验（AuthMiddleware或新的ShareMiddleware）。核心逻辑写在 ShareService 里：生成随机提取码、记录分享关系（新表 share_records）、校验提取码、判断是否过期。Handler 负责解析分享请求和提取码参数，Repository 负责读写 share_records 表。权限判断不能放在 Handler 里，因为 Handler 只负责 HTTP 协议的翻译，不应该知道"分享是否过期"这种业务规则。

**追问：如果分享出去的文件被原作者删除了，怎么处理？**

ShareService 在校验分享链接时，除了检查分享记录本身（是否过期、提取码是否正确），还要去 matter 表查一下原始文件的 status。如果 status=2（回收站）或 status=3（已删除），返回"文件已被删除"。这意味着 ShareService 需要依赖 FileRepo 来查询文件状态，属于 Service 层之间的协作。

**追问：两个 Service 互相依赖会不会有问题？**

ShareService 依赖 FileRepo（而不是 FileService），这样是单向的，不会循环依赖。如果逻辑更复杂，比如分享时需要检查用户配额，可以让 ShareService 依赖 FileService 的接口（interface），而不是具体实现，方便测试和解耦。

---

### Q2. 依赖注入

> 你的 main.go 里手动组装依赖链。如果模块变多，这种方式有什么问题？

手动组装在 10 个模块时 main.go 会变得很长，而且依赖关系复杂时容易遗漏或写错顺序。可以引入依赖注入框架（比如 Google Wire），它能在编译期自动生成组装代码，或者用 Uber 的 fx 框架在运行时通过反射自动注入。不过项目规模不大时手动组装其实更清晰，每条链路一目了然，调试也方便，不需要理解框架的"魔法"。

**追问：你提到了 Wire，它和 fx 的区别是什么？**

Wire 是编译期依赖注入，你写 provider 函数，Wire 生成代码，没有反射开销，类型安全。fx 是运行时依赖注入，基于反射，启动时自动解析依赖图。Wire 更适合 Go 的风格（显式优于隐式），fx 更灵活但调试稍难。小项目两者差别不大。

**追问：为什么不用全局变量或者单例模式来获取 Service？**

全局变量的问题：隐式依赖，不知道谁改了它；测试时不好替换；并发时需要加锁。依赖注入把依赖关系显式写在构造函数参数里，谁依赖谁一目了然，测试时直接传 Mock 进去。

---

### Q3. 中间件设计

> 如果某个接口需要管理员权限，你会怎么扩展？

两种做法。一是写一个 AdminMiddleware，在 AuthMiddleware 之后挂载，从 Context 取出 user_id，查数据库判断是否是管理员，不是就 Abort 返回 403。二是给 AuthMiddleware 加一个可选参数（角色），但这样会让一个中间件承担太多职责，不符合单一职责原则。推荐第一种，两个中间件各管各的，用 `c.Set("role", "admin")` 在中间件之间传递信息。

**追问：中间件的执行顺序重要吗？**

非常重要。Gin 中间件按注册顺序执行，`c.Next()` 之前的代码按顺序，之后的按逆序。所以 AuthMiddleware 必须在 AdminMiddleware 之前注册，否则 AdminMiddleware 拿不到 user_id。同理，CORS 必须在所有中间件之前，否则预检请求（OPTIONS）就被 Auth 拦截了。

---

## 二、认证与安全

### Q4. JWT 无状态 vs Redis 有状态

> 你用 JWT 又在 Redis 存会话，不就变成有状态了吗？

确实如此。纯 JWT 是无状态的，服务端不存任何会话信息，只靠签名验证。但纯 JWT 有一个硬伤：无法主动让某个 token 失效。用户点"登出"后，token 在过期之前仍然合法。所以我用 Redis 存了一份当前有效 token，登出时删除，请求时校验 token 是否还在 Redis 里。这是一个折中方案：大部分验证走 JWT 本地校验（快），登出/踢人走 Redis（可控）。

**追问：那和纯 Session 方案比，JWT + Redis 还有什么优势？**

纯 Session 方案每次请求都要查 Redis/数据库，JWT 只在需要踢人的场景才查。绝大多数请求只做本地签名校验，不用访问 Redis。另外 JWT 天然支持跨服务验证，如果以后拆分成多个微服务，每个服务只需要 JWT 公钥就能验证，不需要共享 Session 存储。

**追问：如果 Redis 里存的是 token 的哈希而不是明文 token 呢？**

更安全。如果 Redis 被攻破，攻击者拿到的只是哈希值，无法还原出原始 token。实现上是在存 Redis 时对 token 做一次 SHA256，校验时对请求中的 token 也做 SHA256 再比对。不过对于当前项目规模，直接存 token 也可以接受，因为 Redis 本身就不应该对外暴露。

---

### Q5. Token 安全

> 如果 JWT 被盗，你的系统怎么让它失效？

当前方案：用户点登出时，从 Redis 删除 `cloud:session:{user_id}` 这条记录。AuthMiddleware 在验证 JWT 签名之后，还会调用 `sessionCache.IsValid()` 检查这个 token 是否在 Redis 中。如果不在，说明用户已登出，返回 401。

**追问：要踢掉某个设备的登录呢？**

当前方案一个用户只存一条 session（最新 token），新登录会覆盖旧的，所以旧设备自动失效，相当于"单设备登录"。如果要支持多设备，需要把 Redis 的 key 改成 `cloud:session:{user_id}:{device_id}`，每个设备一条记录，踢某个设备就删对应的那条。

**追问：JWT 的过期时间你设了 24 小时，这个时间怎么定的？**

24 小时是用户体验和安全性的平衡。太短（比如 15 分钟）用户需要频繁重新登录，体验差；太长（比如 30 天）token 被盗的风险窗口太大。配合 Refresh Token 机制可以兼顾：Access Token 15 分钟过期，Refresh Token 7 天过期，前端自动用 Refresh Token 换新的 Access Token。

---

### Q6. 密码安全

> 为什么用 bcrypt 不用 MD5？

三个原因。第一，MD5 是哈希不是加密，设计目标是快速，攻击者每秒可以尝试几十亿次。bcrypt 故意设计得很慢（有 cost factor 控制计算次数），暴力破解成本高几个数量级。第二，MD5 不加盐，相同密码哈希值相同，查彩虹表就能破解。bcrypt 自动生成随机盐并存在哈希结果里，同样密码每次哈希值不同。第三，MD5 已经被密码学界认为不安全（存在碰撞攻击）。

**追问：bcrypt 的 cost 你设的多少？**

Go 的 `golang.org/x/crypto/bcrypt` 默认 cost 是 10，即 2^10 = 1024 轮迭代。我没改这个默认值。cost 每增加 1，计算时间翻倍。cost=14 在普通服务器上大概需要 1 秒，用户登录时等 1 秒是可以接受的，但 cost=20 可能要十几秒。选 cost 要看服务器 CPU 性能和用户容忍度。

**追问：用户忘记密码怎么处理？**

不能找回（密码是单向哈希，无法还原）。做法是让用户通过邮箱/手机验证身份后，设置一个新密码（直接覆盖旧的哈希值）。如果还没接入邮箱服务，可以提供管理员重置密码的功能。

---

### Q7. 文件上传安全

> 你做了哪些校验？有人上传 1GB 的 .exe 或伪造 MIME 类型怎么办？

当前做的校验：1. 通过 multipart 表单接收文件，Gin 框架层面有默认的内存限制（32MB 内存，超过写临时文件）；2. 文件大小通过 header.Size 记录到数据库。但还缺少几个关键校验：
- **文件大小限制**：应在 Handler 层检查 header.Size 是否超过配置的最大值（config.yaml 里已经有 `max_file_size_bytes` 字段但没用到），超过直接拒绝。
- **文件类型校验**：不能只信任 header.Get("Content-Type")，因为这是客户端传的，可以伪造。应该读取文件头部字节（magic number）判断真实类型，比如 PDF 文件头是 `%PDF-`。
- **扩展名过滤**：维护一个黑名单（.exe、.bat、.sh 等），拒绝可执行文件。

**追问：为什么要读 magic number 不直接看扩展名？**

因为扩展名可以随便改。一个 .exe 改成 .png，光看扩展名就通过了。Magic number 是文件内容的特征签名，不容易伪造。比如 JPEG 文件头固定是 `FF D8 FF`，PNG 是 `89 50 4E 47`。不过实际项目中用第三方库（如 `github.com/h2non/filetype`）来做这个判断更可靠。

---

## 三、缓存策略

### Q8. Cache-Aside 一致性

> 用户修改昵称后，缓存里的旧数据怎么处理？

Cache-Aside 模式的标准做法是"先更新数据库，再删除缓存"。当前项目中如果加了"修改个人信息"接口，应该在 Service 层更新数据库成功后，立即调用 `userCache.Delete(ctx, userID)` 删除缓存。下次 GetProfile 时缓存未命中，重新从数据库加载新值并写入缓存。

**追问：为什么是删缓存而不是更新缓存？**

删缓存比更新缓存更安全。更新缓存的问题是：如果并发修改，两个请求同时更新数据库和缓存，可能后更新的数据库值被先更新的缓存值覆盖（先更新DB后更新缓存）或者反过来（先更新缓存后更新DB）。删缓存是幂等操作，删几次效果一样，而且下次读时会从数据库拿最新值。

**追问："先更新数据库再删缓存"如果删缓存失败了怎么办？**

数据库更新成功但删缓存失败，会出现不一致。解决方案：
1. 重试：删缓存失败时用消息队列或定时任务重试。
2. 订阅数据库 binlog（Canal），监听到 users 表变更时自动删缓存，这是比较可靠的做法。
3. 依赖 TTL 兜底：当前缓存有 10 分钟 TTL，即使删除失败，最多 10 分钟后缓存自动过期，数据最终一致。对于昵称这种非关键数据，最终一致可以接受。

---

### Q9. 缓存异常

> Redis 挂了系统还能运行吗？

看代码逻辑。GetProfile 里 `s.userCache.Get(ctx, userID)` 返回错误时，代码不会崩溃，而是直接走数据库查询（`err == nil && cached != nil` 条件不满足就跳到查数据库）。这是 Cache-Aside 的一个好处：缓存是加速手段不是必需品，Redis 挂了不影响功能，只是慢一点。但有个前提：代码里不能对缓存错误做 panic 或 return err。当前代码用 `_ = s.userCache.Set(...)` 忽略了写入错误，`err` 不为 nil 时直接走数据库，逻辑是正确的。

**追问：如果 Redis 慢但没挂（比如网络抖动），会导致请求超时吗？**

会。Redis 调用有阻塞的风险。解决方案：1. 给 Redis 操作设置超时（`ctx` 配 `redis.Client` 的读写超时）；2. 用 circuit breaker（熔断器），连续失败几次后直接跳过缓存查数据库，等 Redis 恢复再放开。

---

### Q10. 缓存穿透/击穿/雪崩

> 大量请求同时查一个缓存刚过期的用户，会怎样？

这是缓存击穿。大量请求同时发现缓存过期，全部涌向数据库。解决方案：
1. **互斥锁**：只让一个请求去查数据库并回填缓存，其他请求等锁释放后读缓存。可以用 Redis 的 SETNX 实现。
2. **逻辑过期**：缓存里不设 TTL，而是在数据里存一个逻辑过期时间。发现逻辑过期时，异步更新缓存，当前请求仍返回旧数据。
3. **预加载**：在热点用户信息快过期时，后台任务提前刷新。

**追问：缓存穿透和雪崩呢？**

- **穿透**：查询一个数据库里根本不存在的数据（比如 user_id=-1），缓存查不到，数据库也查不到，每次都打到数据库。解决：1. 布隆过滤器，在查缓存前先判断数据是否可能存在；2. 缓存空值（查不到就缓存一个空结果，TTL 设短一点）。
- **雪崩**：大量缓存同一时刻过期，所有请求打到数据库。解决：给 TTL 加随机偏移（比如 10 分钟 ± 2 分钟），避免同时过期。

---

## 四、文件存储

### Q11. 预签名 URL

> 为什么不直接后端代理下载？

后端代理下载意味着后端要先从 MinIO 读完文件再转发给客户端。对于大文件，后端要占用内存/带宽和连接，100 个用户同时下载 1GB 文件就要占 100GB 内存带宽。预签名 URL 让客户端直接从 MinIO 下载，后端只负责生成 URL，不参与文件传输，压力转移到对象存储。

**追问：预签名 URL 有什么安全风险？**

URL 里包含了签名凭证，任何拿到 URL 的人都能在有效期内下载文件。风险：1. URL 泄露（被截获、日志里记录了）；2. 用户分享 URL 给别人。缓解措施：1. 缩短有效期（当前 1 小时可以改成 5 分钟）；2. 限制 IP（MinIO 支持在预签名 URL 中绑定 IP）；3. 一次性令牌（下载后失效，需要后端额外记录）。

**追问：过期了怎么办？**

前端检测到下载 URL 过期（比如 MinIO 返回 403），自动请求后端重新生成一个。用户体验上就是点下载 → 如果链接过期了前端自动刷新再下载。

---

### Q12. 文件去重（秒传）

> 你计算了 MD5，怎么实现秒传？

秒传的核心：上传前先检查文件的 MD5 是否已经存在于数据库中。流程：
1. 前端先在本地计算文件 MD5，调 `/upload/init` 接口传 `{file_hash, file_name, size}`。
2. 后端查 matter 表：`SELECT id FROM matter WHERE md5 = ? AND user_id = ? AND status = 1`。
3. 如果找到记录，说明这个用户之前传过相同文件，直接复制一条 matter 记录（新的 ID、新的 name、新的 parent_id，但 storage_key 相同），不实际上传文件。
4. 如果没找到，走正常上传流程。

**追问：如果不同用户传了相同文件（MD5 一样），能秒传吗？**

能，但需要改 storage_key 的设计。当前 key 是 `{user_id}/{md5}{ext}`，不同用户的 key 不同，无法共享。要支持跨用户秒传，key 应该只基于 MD5：`{md5}{ext}`。但这引入了 Q14 说的删除问题——需要引用计数，记录有多少 matter 记录指向同一个 storage_key。

---

### Q13. 大文件上传（分片）

> 当前方案上传 5GB 文件有什么问题？

问题：1. 内存：multipart 上传整个文件都在内存里（或者临时文件），Gin 默认内存上限 32MB，超过写临时文件但仍然很慢。2. 超时：HTTP 请求可能在上传过程中超时。3. 断点：网络断了整个文件要重传。

**分片上传设计**：
1. `POST /upload/init`：前端传文件名、大小、总 MD5。后端创建 upload_session 记录，返回 session_id 和 chunk_count（文件大小 / 5MB）。
2. `POST /upload/chunk`：前端把文件切成 5MB 的块，逐个上传，带 session_id 和 chunk_index。后端存到 MinIO 临时路径 `tmp/{session_id}/{chunk_index}`。
3. `POST /upload/merge/:id`：前端通知所有分片上传完成。后端按序读取所有分片，拼接后计算 MD5 与前端传的总 MD5 比对，一致则存为正式文件。

**追问：断点续传怎么知道哪些分片传过了？**

前端中断后重新调 `/upload/init`（传相同文件 MD5），后端查 upload_chunks 表，返回已上传的 chunk_index 列表。前端跳过这些分片，只传缺失的。

---

### Q14. 存储路径与引用计数

> 两个用户上传同文件，其中一个删除怎么办？

当前 storage_key 包含 user_id，所以不同用户的相同文件在 MinIO 里是两份独立的副本，删除互不影响。缺点是浪费存储空间。

如果要节省空间（共享同一份），需要引用计数：matter 表里多条记录指向同一个 storage_key，删除时只删 matter 记录，不删 MinIO 文件。当引用计数归零（没有 matter 记录指向这个 key）时，才真正删除 MinIO 文件。查询引用计数：`SELECT COUNT(*) FROM matter WHERE storage_key = ? AND status = 1`。

---

## 五、数据库

### Q15. 表结构设计

> 一张 matter 表存文件和文件夹，为什么不分开？

优点：1. 查询简单，一条 SQL 拿到文件夹下所有内容（文件+文件夹混合排序）；2. 移动操作统一，不管是文件还是文件夹都是改 parent_id；3. 代码简洁，Repository 和 Service 不需要为文件和文件夹各写一套。缺点：1. 字段有冗余，文件夹的 size、ext、mime_type、md5、storage_key 都是空的；2. 如果以后文件和文件夹的属性差异越来越大（比如文件加版本号、文件夹加权限），一张表会越来越臃肿。

**追问：如果属性差异变大了怎么办？**

方案一：加一张 file_meta 表，通过 matter_id 关联，只文件有记录。方案二：拆成 files 和 folders 两张表，列表查询用 UNION ALL 合并。方案三：保持单表，空字段允许 NULL，占用空间有限（VARCHAR 不占定长空间）。当前阶段方案三最简单。

---

### Q16. 索引设计

> `WHERE user_id = ? AND parent_id = ? AND status = 1` 会走 idx_user_parent 索引吗？

会。联合索引 `idx_user_parent(user_id, parent_id)` 遵循最左前缀原则，查询条件 user_id 和 parent_id 正好匹配索引的前两列。status 不在索引里，所以在索引命中的记录上再做 status 过滤（回表查）。如果想进一步优化，可以建 `idx_user_parent_status(user_id, parent_id, status)` 三列联合索引，这样就是覆盖索引，不需要回表。

**追问：为什么列表查询要加 AND status = 1？**

因为软删除的文件 status=2，如果不加这个条件，回收站里的文件也会出现在列表里。加 status 过滤确保只显示正常文件。

---

### Q17. 软删除

> 软删除数据越来越多，查询性能会受影响吗？

会，因为 status=2 的记录也占空间，表越来越大，即使有索引，B+ 树也会变深。处理方式：1. 定期清理：写一个定时任务，把 status=2 超过 30 天的记录改为 status=3 并删除对应的 MinIO 文件。2. 分区表：按 status 分区，status=1 的数据在一个分区，查询时只扫这个分区。3. 归档：把 status=2 的数据移到另一张 matter_trash 表，主表保持干净。

---

### Q18. 面包屑路径查询优化

> 20 层嵌套要 20 次查询，怎么优化？

方案一：物化路径。matter 表已经有 path 字段（当前没用），创建文件夹时把完整路径写进去，比如 `/文档/项目/子目录/`。查询时读一次 path 字段，按 `/` 分割就能拿到完整路径链，只需要 1 次查询。缺点是移动文件夹时需要更新所有子文件夹的 path（递归更新）。方案二：把路径缓存到 Redis，key 为 folder_id，value 为路径 JSON，文件夹移动时更新缓存。

**追问：物化路径移动文件夹时怎么更新所有子文件夹？**

一条 SQL：`UPDATE matter SET path = REPLACE(path, '/旧路径/', '/新路径/') WHERE path LIKE '/旧路径/%'`。利用 LIKE 前缀匹配找到所有子文件夹，REPLACE 替换路径前缀。

---

## 六、工程实践

### Q19. 错误码设计

> 为什么不用 HTTP 状态码？

HTTP 状态码表达的是"传输层面"的状态（404 页面不存在、500 服务器错误），不够表达业务层面的具体原因。比如"文件不存在"和"无权访问此文件"都是业务失败，但原因不同，前端需要根据错误码展示不同的提示。用统一的 HTTP 200 + 业务错误码，前端只需要判断 `code == 0` 是否成功，`code != 0` 时根据具体错误码显示对应提示。

**追问：那 HTTP 状态码什么时候用？**

参数格式错误（JSON 解析失败）可以用 400，未认证用 401，服务端 panic 用 500。当前项目所有响应都返回 200 + 业务码，简化了前端处理逻辑。RESTful 风格的 API 会混用 HTTP 状态码和业务码，两种方式都可行，关键是保持一致。

---

### Q20. 配置管理

> JWT secret 泄露了怎么办？生产环境怎么管理？

secret 泄露意味着任何人都能伪造 token。应急措施：1. 立即更换 secret 并重启服务，所有旧 token 自动失效；2. 清空 Redis 中的所有会话。预防措施：1. config.yaml 加到 .gitignore 里，不提交到仓库；2. 生产环境通过环境变量覆盖配置（代码里 config.Load 已经支持环境变量）；3. 用密钥管理服务（如 Vault、K8s Secret）统一管理敏感配置。

**追问：你提到环境变量覆盖，具体怎么实现的？**

config.Load 里用 `os.Getenv("CLOUD_JWT_SECRET")` 读取环境变量，如果存在就覆盖 YAML 里的值。Docker 部署时在 docker-compose.yml 的 environment 里设置，K8s 部署时在 Deployment 的 env 或 Secret 里设置。

---

### Q21. 时区处理

> 为什么不在数据库里存北京时间？

数据库存 UTC 是工程最佳实践。原因：1. UTC 没有夏令时问题，不会出现"凌晨2点跳到凌晨3点"的情况；2. 全球用户统一基准，后端不需要知道用户在哪个时区；3. 数据库之间的数据迁移和同步不会因为时区不同出错。前端拿到 UTC 时间后，用 JavaScript 的 `new Date(utcString).toLocaleString()` 自动转成本地时间。

**追问：你之前用的是 loc=Local 后来改成 loc=UTC，踩了什么坑？**

`loc=Local` 让 Go 的 MySQL 驱动用服务器本地时区解释时间。服务器在中国就是 UTC+8，在美国就是 UTC-5。本地开发和部署环境时区不一致时，存入数据库的时间就不一样，排查问题很混乱。改成 UTC 后，所有环境存入的时间都是 UTC，可预测、可比较。

---

### Q22. 日志与监控

> 生产环境怎么设计日志系统？

不用 fmt.Println，用结构化日志库（如 zap 或 zerolog），输出 JSON 格式的日志，包含时间、级别、请求ID、方法、路径、耗时、状态码。配合 ELK（Elasticsearch + Logstash + Kibana）或 Loki + Grafana 做日志聚合和检索。线上排查文件上传失败时，用请求 ID 在日志系统里搜索，追踪一个请求从进入 Gin 到返回响应的完整链路。

**追问：请求 ID 怎么生成？**

在请求入口（中间件）用 UUID 或 Snowflake 生成一个唯一 ID，通过 `c.Set("request_id", id)` 存到 Context，日志库从 Context 读取。响应头也返回这个 ID（`X-Request-ID`），用户反馈问题时提供这个 ID 就能快速定位。

---

## 七、扩展性

### Q23. 水平扩展

> 10 万用户在线，单机扛不住怎么办？

核心思路：无状态的 HTTP 服务可以随便扩容。当前架构已经有基础：1. JWT 是无状态的，任何实例都能验证；2. Session 在 Redis 里，任何实例都能查；3. 文件在 MinIO 里，任何实例都能访问。只需要在前面加一个 Nginx 做负载均衡，后面跑多个 Go 实例。MySQL 可以用主从复制，写走主库，读走从库。

**追问：有什么是有状态的、不能随便扩的？**

WebSocket 连接（如果以后加实时通知）、本地文件缓存、本地内存中的限流计数器。这些需要额外处理：WebSocket 用 Redis Pub/Sub 同步消息，限流计数器放在 Redis 里而不是进程内存。

---

### Q24. 限流

> 怎么防恶意上传？

两种策略结合。IP 级别：用 Nginx 的 `limit_req` 或 Redis 令牌桶限制每个 IP 每秒请求数，防 DDoS。用户级别：在 AuthMiddleware 之后加限流中间件，从 Context 取 user_id，用 Redis 记录每个用户的操作频率（比如每分钟最多上传 10 次文件）。超过限制返回 HTTP 429 Too Many Requests。

**追问：令牌桶和漏桶的区别？**

令牌桶：以固定速率往桶里放令牌，请求消耗令牌，桶满了令牌溢出。允许突发流量（桶里有积累的令牌）。漏桶：请求进入漏桶，以固定速率流出处理。强制匀速，不允许突发。文件上传用令牌桶更合适（允许用户偶尔批量上传），登录接口用漏桶更合适（强制匀速防暴力破解）。

---

### Q25. Docker 部署

> 生产环境 docker-compose 要做什么改动？

1. 不用容器跑 MySQL 和 Redis，用云服务（RDS、ElastiCache），有自动备份、主从、监控。2. 后端服务加健康检查（`/ping`）和重启策略（`restart: always`）。3. 敏感配置不写在 yml 里，用环境变量或 Secret。4. 加 Nginx 容器做反向代理和 HTTPS 终止。5. 日志输出到 stdout/stderr，用日志收集器统一采集。6. MinIO 如果是自建，需要配置持久化存储和分布式模式（至少 4 个节点做纠删码）。

**追问：后端服务本身需要打成 Docker 镜像吗？**

需要。多阶段构建：第一阶段用 `golang:1.22` 镜像编译二进制，第二阶段用 `alpine` 或 `scratch` 镜像只拷贝二进制文件。最终镜像只有十几 MB，启动快、攻击面小。Dockerfile 大致：

```dockerfile
FROM golang:1.22 AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o server cmd/server/main.go

FROM alpine:3.19
COPY --from=builder /app/server /server
CMD ["/server"]
```

---

## 八、Go 语言特性

### Q26. Context 在你项目中是怎么用的？

Context 有两个作用。第一是传递请求级别的数据，比如 `c.Request.Context()` 携带了 HTTP 请求的生命周期，如果客户端断开连接，这个 context 会自动取消（Done channel 关闭）。第二是超时控制——如果 MinIO 上传耗时过长，ctx 超时后 `PutObject` 会返回 `context deadline exceeded` 错误，避免请求无限挂起。

当前代码里 Upload、Download 的 ctx 都来自 `c.Request.Context()`，意味着如果用户关闭了浏览器或取消了请求，后端的 MinIO 操作和数据库查询会被自动取消，不会白白占用资源。

**追问：如果你要给 MinIO 上传单独设一个超时，比如最多 30 秒，怎么做？**

用 `context.WithTimeout` 派生一个新的 ctx：

```go
uploadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
err := s.storage.PutObject(uploadCtx, key, reader, size, contentType)
```

注意 `defer cancel()` 必须调用，否则即使操作完成了，context 的定时器也不会被回收，会泄漏。

**追问：context.Background() 和请求传下来的 context 有什么区别？**

`context.Background()` 是一个空的、永远不会取消的 context，通常用在 main 函数初始化、测试、或者不关联任何请求的后台任务中。请求处理链路中必须用 `c.Request.Context()` 或从上游传下来的 ctx，这样请求取消时所有下游操作都能感知到。如果用 `context.Background()` 替代，请求被用户取消后，数据库查询和 MinIO 操作还会继续执行。

---

### Q27. 为什么用 `uint64` 做 ID 而不是 `int`？

BIGINT UNSIGNED 在数据库里是 8 字节无符号整数，范围 0 到 18446744073709551615（约 1800 亿亿）。Go 里对应 `uint64`。如果用 `int`（Go 里是 int64，有符号），正数范围只有一半，而且从数据库读出来的无符号大数会溢出成负数。另外 ID 不可能是负数，用无符号类型在语义上更准确。

**追问：雪花 ID（Snowflake）和自增 ID 的区别？适用场景？**

自增 ID 简单、有序、占空间小，但单点依赖数据库的自增序列，分库分表后不好保证唯一。雪花 ID 是分布式 ID 生成算法，由时间戳 + 机器 ID + 序列号组成，不依赖数据库，多个服务节点可以独立生成不重复的 ID。云盘系统当前单库够用，自增 ID 就行。如果后续要做分库分表，就需要换成雪花 ID。

---

### Q28. `defer file.Close()` 在上传接口里，如果 Create（写数据库）失败了，MinIO 里的文件会不会残留？

会残留。当前流程是先上传到 MinIO，再写数据库。如果写数据库失败，MinIO 里的文件不会被自动删除。这是一个需要修复的问题。

**追问：怎么解决？**

方案一：加补偿逻辑——Create 失败时调用 `s.storage.RemoveObject(ctx, storageKey)` 删除已上传的文件。方案二：反过来，先写数据库（状态设为"上传中"），成功后再上传 MinIO，最后更新状态为"正常"。如果上传失败，定时任务清理"上传中"状态的记录。方案三（最终做法）：先上传 MinIO，再写数据库，写数据库失败时删除 MinIO 文件。用事务或补偿机制保证最终一致。

---

## 九、并发与数据安全

### Q29. 两个用户同时在同一个文件夹下创建同名文件夹，会发生什么？

当前代码先 `ExistsByName` 检查重名，再 `Create` 插入。如果两个请求同时通过重名检查（都查到不存在），然后同时执行 INSERT，会因为数据库的唯一约束报错——但等等，matter 表没有 (user_id, parent_id, name) 的唯一约束（不像 users 表的 username），所以两个同名文件夹都能插入成功，导致同一目录下出现两个同名文件夹。

**追问：怎么修复？**

在数据库层面加唯一索引：`UNIQUE KEY uk_user_parent_name (user_id, parent_id, name)`。这样即使并发插入，数据库会拒绝第二个。代码层面在 Create 报 Duplicate entry 错误时，返回"同名已存在"的提示。

**追问：这是经典的 TOCTOU（Time of Check to Time of Use）问题，能解释一下吗？**

TOCTOU 指的是"检查时"和"使用时"之间的时间窗口里，状态可能被其他请求改变。这里"检查"是 `ExistsByName`，"使用"是 `Create`，中间的间隔里另一个请求可能已经插入了同名记录。解决方案就是不要依赖应用层的检查，而是让数据库的约束来兜底——应用层检查是优化用户体验（提前报错），数据库约束才是真正保证数据一致性的。

---

### Q30. 文件移动时的循环引用问题：把文件夹 A 移到自己的子文件夹 B 里，会发生什么？

当前 Move 只检查了 `fileID == targetID`（不能移到自己），但没有检查"不能移到自己的子文件夹里"。如果允许这种操作，会形成循环引用：A 的 parent 指向 B，B 的 parent 指向 A，GetPath 会死循环。

**追问：怎么修复？**

Move 时需要检查 targetID 是否是 fileID 的后代。从 targetID 开始沿 parent_id 往上追溯，如果途中遇到 fileID，说明目标是要移动的文件夹的子文件夹，拒绝操作：

```go
current := targetID
for current != 0 {
    if current == fileID {
        return fmt.Errorf("不能移动到自身的子文件夹")
    }
    m, err := s.repo.GetByID(current)
    if err != nil { break }
    current = m.ParentID
}
```

---

### Q31. Redis 的 Set 和 Delete 操作是原子的吗？

单个 Redis 命令（SET、GET、DEL、HSET）是原子的，因为 Redis 是单线程执行命令的。所以 `sessionCache.Set` 和 `sessionCache.Delete` 各自是安全的。但 `GetProfile` 里的"先查缓存，未命中查数据库，再写缓存"这三步不是原子的——查缓存和写缓存之间，其他请求可能已经更新了数据库。这就是 Cache-Aside 模式可能出现的短暂不一致，依赖 TTL 兜底。

**追问：如果要求强一致性呢？**

用 Redis 分布式锁（`SET key value NX EX timeout`），GetProfile 时先获取锁，再查缓存/数据库/写缓存，最后释放锁。但这样性能会下降（每个请求都要争锁），对于用户信息这种读多写少的数据，强一致性的性价比不高。

---

## 十、接口设计细节

### Q32. 你的错误响应统一用 `errcode.ErrParamInvalid`（10005），不管是"文件不存在"还是"无权操作"都返回同一个错误码。怎么改进？

当前 Service 层通过 `fmt.Errorf("文件不存在")` 返回不同消息字符串，但 Handler 统一用 `errcode.ErrParamInvalid` 作为 code。改进方案：

1. Service 返回自定义错误类型，携带错误码：
```go
type BizError struct {
    Code int
    Msg  string
}
func NewBizError(code int, msg string) *BizError {
    return &BizError{Code: code, Msg: msg}
}
```

2. Handler 统一判断错误类型，提取对应的 code 和 msg：
```go
var bizErr *BizError
if errors.As(err, &bizErr) {
    Fail(c, bizErr.Code, bizErr.Msg)
} else {
    Fail(c, errcode.ErrDBError, "服务器内部错误")
}
```

这样前端能根据 code 精确判断错误类型，展示对应提示。

**追问：为什么不在 Service 层直接返回错误码数字？**

返回数字（`return 10007, err`）不够类型安全，容易传错。用自定义错误类型，`errors.As` 能在错误链中精确匹配，而且错误类型可以携带额外信息（比如原始 error）。这和 Go 标准库的设计一致——`os.PathError`、`net.DNSError` 都是自定义错误类型。

---

### Q33. 文件列表返回了 `user_id` 字段，但列表只能查自己的文件，这个字段有必要吗？

没必要。`user_id` 在列表接口中是冗余的，因为所有返回的文件都属于当前登录用户。可以定义一个列表专用的响应结构体，省掉 `user_id`。但保留也有理由：如果将来加了"别人分享给我的文件"功能，列表里会出现别人的文件，user_id 就有意义了。当前阶段去掉更干净，等需要时再加。

**追问：Matter 结构体同时用于数据库映射和 JSON 响应，这有什么问题？**

耦合了内部数据模型和外部 API 契约。如果数据库加了字段（比如内部备注 `internal_note`），不想暴露给前端，每次都要记得加 `json:"-"`。更好的做法是定义专门的响应 DTO，Repository 返回 Model，Service 转成 DTO 再返回给 Handler。但这会增加代码量，小项目可以接受当前的耦合。

---

## 十一、项目决策与取舍

### Q34. 为什么选 Gin 不选 Echo 或标准库？

Gin 是 Go 最流行的 Web 框架，生态好、文档多、中间件丰富。Echo 性能接近但社区小一些。标准库 `net/http` 在 Go 1.22 之后路由能力提升了不少（支持方法+路径模式匹配），但对于项目需要的参数绑定（`ShouldBindJSON`）、路由分组、中间件链等特性，Gin 开箱即用更方便。选 Gin 主要是学习成本低，遇到问题容易查到解决方案。

**追问：Gin 和标准库 `net/http` 的核心区别是什么？**

路由和上下文。Gin 用 Radix 树实现路由匹配，支持路径参数（`:id`）和分组，比标准库的 `HandleFunc` 灵活。Gin 的 `gin.Context` 封装了请求/响应的读写、参数绑定、JSON 序列化、中间件控制（`Next`/`Abort`），而标准库需要自己组合这些能力。Go 1.22 的 `http.ServeMux` 已经支持方法路由和路径参数了，如果不需要 Gin 的中间件链和参数绑定，标准库足够。

---

### Q35. 为什么 MinIO 不直接用本地文件系统存文件？

三个原因。第一，本地文件系统不支持预签名 URL——要让客户端直接下载，需要后端自己读文件再返回，大文件占用后端带宽和内存。MinIO 生成预签名 URL 后客户端直接从 MinIO 下载，后端不参与传输。第二，本地文件系统不支持水平扩展——多个后端实例不能共享同一块磁盘。MinIO 作为独立服务，所有实例都能通过网络访问。第三，MinIO 提供了纠删码、多副本、自动修复等数据可靠性保障，自己用文件系统实现这些成本很高。

**追问：那为什么不用 AWS S3？**

S3 功能更全更稳定，但需要 AWS 账号和付费，本地开发调试不方便。MinIO 是 S3 兼容的对象存储，API 和 S3 基本一致，可以本地 Docker 启动。开发和测试用 MinIO，生产环境可以直接切换到 S3，只需要改 endpoint 和 credentials 配置。

---

### Q36. 为什么不用 ORM（GORM）？

`database/sql` 写原生 SQL，对数据库操作有完全控制，性能开销几乎为零，但代码量大——每个查询都要手写 SQL、手写 Scan。GORM 自动映射结构体到表，提供链式查询，代码更简洁，但会生成你可能不预期的 SQL，排查性能问题时要看实际执行的 SQL。

当前项目选择原生 SQL 是因为：1. 表结构简单，SQL 不复杂；2. 学习阶段，理解 SQL 本身比学 ORM 的抽象更重要；3. 面试时更容易解释底层原理。

**追问：如果用 GORM，Repository 层的代码会变成什么样？**

```go
// 原生 SQL
row := r.db.QueryRow("SELECT id, username FROM users WHERE id = ?", id)

// GORM
var user User
err := r.db.First(&user, id).Error
```

GORM 省掉了手写 SQL 和 Scan 的代码，但需要定义 GORM tag（`gorm:"column:username"`）。对于复杂查询（多表 JOIN、子查询），GORM 的链式写法有时反而不如原生 SQL 直观。

---

## 十二、数据库深入

### Q37. `LAST_INSERT_ID()` 在并发插入时安全吗？

安全。MySQL 文档明确说明 `LAST_INSERT_ID()` 返回的是当前连接（session）最后插入的自增 ID，不受其他连接的插入影响。Go 的 `database/sql` 连接池在执行 `Exec` + `LastInsertId` 时使用的是同一个连接，所以不会出现并发问题。

**追问：那 `RowsAffected` 呢？**

也是连接级别的。`result.RowsAffected()` 返回的是当前 Exec 语句影响的行数，不受其他连接的事务影响。

---

### Q38. 如果查询返回 0 行，`QueryRow` 和 `Query` 行为有什么区别？

`QueryRow` 返回的 `*Row` 本身不会是 nil。调用 `Scan` 时，如果查询结果为空，`Scan` 返回 `sql.ErrNoRows`。这就是为什么 `GetByID` 在找不到记录时返回 `sql.ErrNoRows`，Service 层用 `errors.Is(err, sql.ErrNoRows)` 判断是"不存在"还是"数据库出错了"。

`Query`（返回多行）如果没有匹配行，`*Rows` 也不是 nil，但 `rows.Next()` 第一次调用就返回 false，循环不执行，items 保持 nil。这就是为什么 List 里要 `if items == nil { items = []model.Matter{} }`——把 nil 转成空切片，前端收到 `[]` 而不是 `null`。

---

### Q39. 如果数据库连接池满了，新的请求会怎样？

`database/sql` 的 `db.Query` 和 `db.Exec` 会阻塞等待，直到有连接释放。如果没有设置 context 超时，请求会一直挂起。所以需要：1. 合理设置 `MaxOpenConns`；2. 给 context 设超时（`context.WithTimeout`）；3. 监控连接池使用率（`db.Stats()` 返回连接池状态）。

**追问：MaxOpenConns 设多少合适？**

取决于数据库服务器能扛多少并发连接和后端服务的并发量。MySQL 默认 `max_connections` 是 151。如果只有一个后端实例，`MaxOpenConns` 设 100 留有余量。如果有 5 个后端实例，每个设 20。公式：实例数 × MaxOpenConns < MySQL max_connections。

---

## 十三、场景题

### Q40. 用户上传同一个文件但改了名字，数据库里会怎样？

会新增一条 matter 记录（不同的 ID、不同的 name），但 storage_key 相同（因为 MD5 一样）。MinIO 里只有一份文件，数据库里有两条记录指向同一个 key。当前 Delete 只是软删除（改 status），不删 MinIO 文件，所以暂时没问题。但如果以后加"永久删除"功能，必须先检查有没有其他记录指向同一个 storage_key，有则不删文件。

---

### Q41. 你的 CORS 中间件允许所有来源（`*`），生产环境能用吗？

不能。`*` 表示任何网站都能调用你的 API。生产环境应该改成具体的域名白名单：

```go
origin := c.GetHeader("Origin")
allowed := map[string]bool{
    "https://your-domain.com": true,
    "http://localhost:5173":   true,
}
if allowed[origin] {
    c.Header("Access-Control-Allow-Origin", origin)
}
```

注意不能用 `*` 同时配合 `Access-Control-Allow-Credentials: true`，浏览器会拒绝。

---

### Q42. 面试官问：这个项目有什么不足？如果给你更多时间你会改进什么？

**安全**：文件类型校验（读 magic number）、上传大小限制、CORS 白名单、JWT Refresh Token 机制。

**可靠性**：数据库唯一索引防并发重复、循环引用检查、上传失败补偿（删除 MinIO 残留文件）。

**性能**：面包屑路径优化（物化路径或缓存）、文件列表缓存到 Redis、数据库读写分离。

**功能**：分片上传（大文件断点续传+秒传）、回收站管理（恢复+定期清理）、文件分享。

**工程化**：结构化日志（zap）、请求 ID 追踪、Prometheus 监控指标、API 集成测试、CI/CD 流水线。

选 3-4 个说具体怎么改，展示改进意识而不是停留在"知道有问题"。
