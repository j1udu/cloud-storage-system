-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id           BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT COMMENT '主键',
    username     VARCHAR(64)      NOT NULL COMMENT '登录名',
    password     VARCHAR(255)     NOT NULL COMMENT 'bcrypt 哈希密码',
    nickname     VARCHAR(128)     NOT NULL DEFAULT '' COMMENT '昵称',
    status       TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '1=正常 2=禁用',
    created_at   DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at   DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';

-- 文件/文件夹表（dir=1表示文件夹，dir=0表示文件）
CREATE TABLE IF NOT EXISTS matter (
    id           BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT COMMENT '主键',
    user_id      BIGINT UNSIGNED  NOT NULL COMMENT '所属用户',
    parent_id    BIGINT UNSIGNED  NOT NULL DEFAULT 0 COMMENT '父目录ID，0=根目录',
    name         VARCHAR(255)     NOT NULL COMMENT '文件名或文件夹名',
    dir          TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '1=文件夹 0=文件',
    size         BIGINT UNSIGNED  NOT NULL DEFAULT 0 COMMENT '文件大小(字节)，文件夹为0',
    ext          VARCHAR(32)      NOT NULL DEFAULT '' COMMENT '扩展名',
    mime_type    VARCHAR(128)     NOT NULL DEFAULT '' COMMENT 'MIME类型',
    md5          CHAR(32)         NOT NULL DEFAULT '' COMMENT '文件MD5',
    storage_key  VARCHAR(512)     NOT NULL DEFAULT '' COMMENT 'MinIO对象键',
    path         VARCHAR(1024)    NOT NULL DEFAULT '' COMMENT '物化路径，如 /文档/项目/',
    status       TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '1=正常 2=回收站 3=已删除',
    created_at   DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at   DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    INDEX idx_user_parent (user_id, parent_id),
    INDEX idx_user_status (user_id, status),
    INDEX idx_md5 (md5)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文件/文件夹表';
