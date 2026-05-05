package database

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

// Connector manages a read-only MySQL connection for security scanning.
type Connector struct {
	db          *sql.DB
	tablePrefix string
}

// NewConnector creates a new MySQL connector with the given config.
// DSN format: user:password@tcp(host:port)/dbname?timeout=10s&readTimeout=30s
func NewConnector(host, port, username, password, dbname, tablePrefix string) (*Connector, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?timeout=10s&readTimeout=30s",
		username, password, host, port, dbname)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	db.SetMaxOpenConns(3)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Connector{
		db:          db,
		tablePrefix: tablePrefix,
	}, nil
}

// Close closes the database connection.
func (c *Connector) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// TableName returns the prefixed table name.
func (c *Connector) TableName(name string) string {
	return c.tablePrefix + name
}

// Ping verifies the connection is alive.
func (c *Connector) Ping() error {
	return c.db.Ping()
}
