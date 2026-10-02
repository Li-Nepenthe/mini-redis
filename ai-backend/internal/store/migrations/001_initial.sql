CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(32) PRIMARY KEY,
    email VARCHAR(254) NOT NULL,
    password_hash VARCHAR(100) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    UNIQUE KEY users_email (email)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS user_quotas (
    user_id VARCHAR(32) NOT NULL,
    month DATE NOT NULL,
    token_used BIGINT NOT NULL DEFAULT 0,
    token_limit BIGINT NOT NULL,
    PRIMARY KEY (user_id, month),
    CONSTRAINT quota_user FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT quota_positive CHECK (token_limit > 0 AND token_used >= 0)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS conversations (
    id VARCHAR(32) PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    title VARCHAR(200) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    KEY conversations_owner_created (user_id, created_at, id),
    CONSTRAINT conversation_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS messages (
    id VARCHAR(32) PRIMARY KEY,
    conversation_id VARCHAR(32) NOT NULL,
    role VARCHAR(16) NOT NULL,
    content MEDIUMTEXT NOT NULL,
    prompt_tokens BIGINT NOT NULL DEFAULT 0,
    completion_tokens BIGINT NOT NULL DEFAULT 0,
    citations JSON NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    KEY messages_conversation_created (conversation_id, created_at, id),
    CONSTRAINT message_conversation FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS documents (
    id VARCHAR(32) PRIMARY KEY,
    owner_id VARCHAR(32) NOT NULL,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    KEY documents_owner_created (owner_id, created_at, id),
    CONSTRAINT document_user FOREIGN KEY (owner_id) REFERENCES users(id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS chunks (
    id VARCHAR(32) PRIMARY KEY,
    document_id VARCHAR(32) NOT NULL,
    position INT NOT NULL,
    content TEXT NOT NULL,
    embedding BLOB NOT NULL,
    token_count INT NOT NULL,
    UNIQUE KEY chunks_document_position (document_id, position),
    CONSTRAINT chunk_document FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS task_events (
    event_id VARCHAR(32) PRIMARY KEY,
    task_id VARCHAR(32) NOT NULL,
    type VARCHAR(32) NOT NULL,
    payload JSON NOT NULL,
    processed_at DATETIME(6) NULL
) ENGINE=InnoDB;
