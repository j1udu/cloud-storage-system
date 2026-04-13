-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id           BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT COMMENT '主键',
    username     VARCHAR(64)      NOT NULL COMMENT '登录名',
    password     VARCHAR(255)     NOT NULL COMMENT 'bcrypt 哈希密码',
    nickname     VARCHAR(128)     NOT NULL DEFAULT '' COMMENT '昵称',
    email        VARCHAR(255)     NOT NULL DEFAULT '' COMMENT '邮箱',
    avatar_url   VARCHAR(512)     NOT NULL DEFAULT '' COMMENT '头像',
    storage_used BIGINT UNSIGNED  NOT NULL DEFAULT 0 COMMENT '已用空间(字节)',
    storage_limit BIGINT UNSIGNED NOT NULL DEFAULT 107374182400 COMMENT '空间配额(默认100GB)',
    status       TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '1=正常 2=禁用',
    created_at   DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at   DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_username (username),
    UNIQUE KEY uk_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';

-- 文件夹表
CREATE TABLE IF NOT EXISTS folders (
    id         BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT COMMENT '主键',
    user_id    BIGINT UNSIGNED  NOT NULL COMMENT '所属用户',
    parent_id  BIGINT UNSIGNED  NOT NULL DEFAULT 0 COMMENT '父文件夹ID，0=根目录',
    name       VARCHAR(255)     NOT NULL COMMENT '文件夹名',
    path       VARCHAR(1024)    NOT NULL DEFAULT '' COMMENT '物化路径，如 /文档/项目/',
    created_at DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    INDEX idx_user_parent (user_id, parent_id),
    INDEX idx_path (user_id, path(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文件夹表';

-- 文件表
CREATE TABLE IF NOT EXISTS files (
    id           BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT COMMENT '主键',
    user_id      BIGINT UNSIGNED  NOT NULL COMMENT '所属用户',
    folder_id    BIGINT UNSIGNED  NOT NULL DEFAULT 0 COMMENT '所在文件夹，0=根目录',
    name         VARCHAR(255)     NOT NULL COMMENT '文件名',
    size         BIGINT UNSIGNED  NOT NULL DEFAULT 0 COMMENT '文件大小(字节)',
    ext          VARCHAR(32)      NOT NULL DEFAULT '' COMMENT '扩展名',
    mime_type    VARCHAR(128)     NOT NULL DEFAULT '' COMMENT 'MIME类型',
    file_hash    CHAR(32)         NOT NULL DEFAULT '' COMMENT '完整文件MD5',
    storage_key  VARCHAR(512)     NOT NULL DEFAULT '' COMMENT 'MinIO对象键',
    is_chunked   TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '是否分片上传',
    chunk_count  INT UNSIGNED     NOT NULL DEFAULT 0 COMMENT '分片数',
    status       TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '1=正常 2=回收站 3=已删除',
    created_at   DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at   DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    INDEX idx_user_folder (user_id, folder_id),
    INDEX idx_file_hash (file_hash),
    INDEX idx_user_status (user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文件表';

-- 上传会话表
CREATE TABLE IF NOT EXISTS upload_sessions (
    id              BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT COMMENT '主键',
    user_id         BIGINT UNSIGNED  NOT NULL COMMENT '上传用户',
    file_name       VARCHAR(255)     NOT NULL COMMENT '文件名',
    file_size       BIGINT UNSIGNED  NOT NULL DEFAULT 0 COMMENT '文件大小(字节)',
    file_hash       CHAR(32)         NOT NULL DEFAULT '' COMMENT '完整文件MD5',
    chunk_size      INT UNSIGNED     NOT NULL DEFAULT 5242880 COMMENT '分片大小(默认5MB)',
    chunk_count     INT UNSIGNED     NOT NULL DEFAULT 0 COMMENT '总分片数',
    folder_id       BIGINT UNSIGNED  NOT NULL DEFAULT 0 COMMENT '目标文件夹',
    uploaded_chunks INT UNSIGNED     NOT NULL DEFAULT 0 COMMENT '已上传分片数',
    status          TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '1=活跃 2=合并中 3=完成 4=过期',
    expires_at      DATETIME(3)      NOT NULL COMMENT '过期时间',
    created_at      DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at      DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    INDEX idx_user_status (user_id, status),
    INDEX idx_expires (status, expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='上传会话表';

-- 上传分片表
CREATE TABLE IF NOT EXISTS upload_chunks (
    id          BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT COMMENT '主键',
    session_id  BIGINT UNSIGNED  NOT NULL COMMENT '所属会话',
    chunk_index INT UNSIGNED     NOT NULL COMMENT '分片序号',
    chunk_hash  CHAR(32)         NOT NULL DEFAULT '' COMMENT '分片MD5',
    chunk_size  INT UNSIGNED     NOT NULL DEFAULT 0 COMMENT '分片大小(字节)',
    storage_key VARCHAR(512)     NOT NULL DEFAULT '' COMMENT 'MinIO临时键',
    created_at  DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_session_chunk (session_id, chunk_index),
    INDEX idx_session (session_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='上传分片表';
