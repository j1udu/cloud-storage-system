CREATE TABLE IF NOT EXISTS shares (
    id           BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    user_id      BIGINT UNSIGNED  NOT NULL,
    matter_id    BIGINT UNSIGNED  NOT NULL,
    token        VARCHAR(64)      NOT NULL,
    access_code  VARCHAR(32)      NOT NULL DEFAULT '',
    expire_at    DATETIME(3)      NULL,
    status       TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '1=有效 2=取消',
    created_at   DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at   DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_token (token),
    INDEX idx_user_id (user_id),
    INDEX idx_matter_id (matter_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='分享表';
