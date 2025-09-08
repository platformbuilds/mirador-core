package vitess

import (
    "context"
    "database/sql"
    "fmt"
    "net/url"
    "strings"
    "time"

    _ "github.com/go-sql-driver/mysql"
    "github.com/platformbuilds/mirador-core/internal/config"
)

type Client struct {
    DB *sql.DB
}

func dsnFrom(cfg config.VitessConfig) string {
    user := cfg.User
    if user == "" { user = "root" }
    pass := cfg.Password
    host := cfg.Host
    if host == "" { host = "127.0.0.1" }
    port := cfg.Port
    if port == 0 { port = 15306 }
    dbName := cfg.Keyspace
    if dbName == "" { dbName = "mirador" }

    params := url.Values{}
    params.Set("parseTime", "true")
    if cfg.TLS {
        // Expect client to configure TLS profile in driver if needed; use preferred
        params.Set("tls", "preferred")
    }
    for k, v := range cfg.Params {
        params.Set(k, v)
    }
    auth := user
    if pass != "" { auth = fmt.Sprintf("%s:%s", user, pass) }
    return fmt.Sprintf("%s@tcp(%s:%d)/%s?%s", auth, host, port, dbName, params.Encode())
}

func Connect(cfg config.VitessConfig) (*Client, error) {
    dsn := dsnFrom(cfg)
    db, err := sql.Open("mysql", dsn)
    if err != nil { return nil, err }
    db.SetMaxOpenConns(20)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(30 * time.Minute)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := db.PingContext(ctx); err != nil {
        _ = db.Close()
        return nil, err
    }
    c := &Client{DB: db}
    if err := c.ensureSchema(); err != nil {
        _ = db.Close()
        return nil, err
    }
    return c, nil
}

func (c *Client) Close() error { return c.DB.Close() }

func (c *Client) ensureSchema() error {
    stmts := []string{
        `CREATE TABLE IF NOT EXISTS metric_def (
            tenant_id VARCHAR(128) NOT NULL,
            metric VARCHAR(255) NOT NULL,
            description TEXT,
            owner VARCHAR(128),
            tags JSON,
            current_version BIGINT DEFAULT 0,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            PRIMARY KEY (tenant_id, metric)
        )`,
        `CREATE TABLE IF NOT EXISTS metric_label_def (
            tenant_id VARCHAR(128) NOT NULL,
            metric VARCHAR(255) NOT NULL,
            label VARCHAR(255) NOT NULL,
            type VARCHAR(64),
            required TINYINT(1) DEFAULT 0,
            allowed_values JSON,
            description TEXT,
            PRIMARY KEY (tenant_id, metric, label)
        )`,
        `CREATE TABLE IF NOT EXISTS metric_def_versions (
            tenant_id VARCHAR(128) NOT NULL,
            metric VARCHAR(255) NOT NULL,
            version BIGINT NOT NULL,
            payload JSON,
            author VARCHAR(128),
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (tenant_id, metric, version)
        )`,
        `CREATE TABLE IF NOT EXISTS log_field_def (
            tenant_id VARCHAR(128) NOT NULL,
            field VARCHAR(255) NOT NULL,
            type VARCHAR(64),
            description TEXT,
            tags JSON,
            examples JSON,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            PRIMARY KEY (tenant_id, field)
        )`,
        `CREATE TABLE IF NOT EXISTS log_field_def_versions (
            tenant_id VARCHAR(128) NOT NULL,
            field VARCHAR(255) NOT NULL,
            version BIGINT NOT NULL,
            payload JSON,
            author VARCHAR(128),
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (tenant_id, field, version)
        )`,
        `CREATE TABLE IF NOT EXISTS traces_service_def (
            tenant_id VARCHAR(128) NOT NULL,
            service VARCHAR(255) NOT NULL,
            purpose TEXT,
            owner VARCHAR(128),
            tags JSON,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            PRIMARY KEY (tenant_id, service)
        )`,
        `CREATE TABLE IF NOT EXISTS traces_service_def_versions (
            tenant_id VARCHAR(128) NOT NULL,
            service VARCHAR(255) NOT NULL,
            version BIGINT NOT NULL,
            payload JSON,
            author VARCHAR(128),
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (tenant_id, service, version)
        )`,
        `CREATE TABLE IF NOT EXISTS traces_operation_def (
            tenant_id VARCHAR(128) NOT NULL,
            service VARCHAR(255) NOT NULL,
            operation VARCHAR(255) NOT NULL,
            purpose TEXT,
            owner VARCHAR(128),
            tags JSON,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            PRIMARY KEY (tenant_id, service, operation)
        )`,
        `CREATE TABLE IF NOT EXISTS traces_operation_def_versions (
            tenant_id VARCHAR(128) NOT NULL,
            service VARCHAR(255) NOT NULL,
            operation VARCHAR(255) NOT NULL,
            version BIGINT NOT NULL,
            payload JSON,
            author VARCHAR(128),
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (tenant_id, service, operation, version)
        )`,
    }
    for _, s := range stmts {
        if _, err := c.DB.Exec(s); err != nil {
            // vitess may return additional info; trim
            return fmt.Errorf("ensure schema failed: %s: %w", strings.SplitN(s, "(", 2)[0], err)
        }
    }
    return nil
}
