package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Li-Nepenthe/mini-redis/ai-backend/internal/domain"
	"github.com/go-sql-driver/mysql"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct {
	DB           *sql.DB
	DefaultQuota int64
}

func Open(ctx context.Context, dsn string, maxOpen, maxIdle int, lifetime time.Duration, quota int64) (*Store, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, errors.New("invalid DATABASE_DSN")
	}
	cfg.ParseTime, cfg.Loc, cfg.MultiStatements = true, time.UTC, false
	cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 5*time.Second, 30*time.Second, 30*time.Second
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	// 每API/Worker各最多20连接、10闲置、5分钟生命周期；不以连接池掩盖慢SQL。
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(lifetime)
	db.SetConnMaxIdleTime(time.Minute)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database unavailable: %w", err)
	}
	return &Store{DB: db, DefaultQuota: quota}, nil
}

func (s *Store) Migrate(ctx context.Context) error {
	// 命名锁绑定同一连接；API/Worker同时启动时不能交错运行迁移。
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var locked int
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK('mini-redis-ai-migrations', 30)").Scan(&locked); err != nil || locked != 1 {
		return errors.New("migration lock unavailable")
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(unlockCtx, "DO RELEASE_LOCK('mini-redis-ai-migrations')")
	}()
	if _, err := conn.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version VARCHAR(64) PRIMARY KEY, applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6))"); err != nil {
		return err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		var count int
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version=?", entry.Name()).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			continue
		}
		data, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		// MySQL DDL隐式提交：每条IF NOT EXISTS可在失败后重跑；不伪称迁移原子。
		for _, statement := range strings.Split(string(data), ";") {
			if strings.TrimSpace(statement) != "" {
				if _, err := conn.ExecContext(ctx, statement); err != nil {
					return fmt.Errorf("migration %s: %w", entry.Name(), err)
				}
			}
		}
		if _, err := conn.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

func translate(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return domain.ErrConflict
	}
	return err
}

func (s *Store) CreateUser(ctx context.Context, email, hash string, limit int64) (domain.User, error) {
	user := domain.User{ID: domain.NewID(), Email: email, PasswordHash: hash, Status: "active", CreatedAt: time.Now().UTC()}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return user, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "INSERT INTO users(id,email,password_hash,status,created_at) VALUES (?,?,?,?,?)", user.ID, email, hash, user.Status, user.CreatedAt); err != nil {
		return user, translate(err)
	}
	// 注册与初始额度同一事务：额度写入失败时账号也不能留下半完成状态。
	if _, err := tx.ExecContext(ctx, "INSERT INTO user_quotas(user_id,month,token_limit) VALUES (?,?,?)", user.ID, month(time.Now()), limit); err != nil {
		return user, err
	}
	return user, tx.Commit()
}

func month(now time.Time) string { return now.UTC().Format("2006-01") + "-01" }

func (s *Store) UserByEmail(ctx context.Context, email string) (domain.User, error) {
	var user domain.User
	err := s.DB.QueryRowContext(ctx, "SELECT id,email,password_hash,status,created_at FROM users WHERE email=?", email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Status, &user.CreatedAt)
	return user, translate(err)
}

func (s *Store) CreateConversation(ctx context.Context, owner, title string) (domain.Conversation, error) {
	now := time.Now().UTC()
	item := domain.Conversation{ID: domain.NewID(), UserID: owner, Title: title, CreatedAt: now, UpdatedAt: now}
	_, err := s.DB.ExecContext(ctx, "INSERT INTO conversations(id,user_id,title,created_at,updated_at) VALUES (?,?,?,?,?)", item.ID, owner, title, now, now)
	return item, err
}

func (s *Store) Conversation(ctx context.Context, id, owner string) (domain.Conversation, error) {
	var item domain.Conversation
	err := s.DB.QueryRowContext(ctx, "SELECT id,user_id,title,created_at,updated_at FROM conversations WHERE id=?", id).
		Scan(&item.ID, &item.UserID, &item.Title, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, translate(err)
	}
	if item.UserID != owner {
		return domain.Conversation{}, domain.ErrForbidden
	}
	return item, nil
}

func (s *Store) Conversations(ctx context.Context, owner string, page, size int) ([]domain.Conversation, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id,user_id,title,created_at,updated_at FROM conversations WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?", owner, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Conversation, 0)
	for rows.Next() {
		var item domain.Conversation
		if err := rows.Scan(&item.ID, &item.UserID, &item.Title, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) RenameConversation(ctx context.Context, id, owner, title string) (domain.Conversation, error) {
	if _, err := s.Conversation(ctx, id, owner); err != nil {
		return domain.Conversation{}, err
	}
	result, err := s.DB.ExecContext(ctx, "UPDATE conversations SET title=?,updated_at=CURRENT_TIMESTAMP(6) WHERE id=? AND user_id=?", title, id, owner)
	if err != nil {
		return domain.Conversation{}, err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return domain.Conversation{}, domain.ErrNotFound
	}
	return s.Conversation(ctx, id, owner)
}

func (s *Store) DeleteConversation(ctx context.Context, id, owner string) error {
	if _, err := s.Conversation(ctx, id, owner); err != nil {
		return err
	}
	result, err := s.DB.ExecContext(ctx, "DELETE FROM conversations WHERE id=? AND user_id=?", id, owner)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) Usage(ctx context.Context, owner string) (domain.Usage, error) {
	current := month(time.Now())
	if _, err := s.DB.ExecContext(ctx, "INSERT IGNORE INTO user_quotas(user_id,month,token_limit) VALUES (?,?,?)", owner, current, s.DefaultQuota); err != nil {
		return domain.Usage{}, err
	}
	var usage domain.Usage
	usage.Month = current[:7]
	err := s.DB.QueryRowContext(ctx, "SELECT token_used,token_limit FROM user_quotas WHERE user_id=? AND month=?", owner, current).Scan(&usage.TokenUsed, &usage.TokenLimit)
	return usage, translate(err)
}
