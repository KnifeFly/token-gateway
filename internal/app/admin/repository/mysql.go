package repository

import (
	"database/sql"
)

// MySQLRepository stores Admin operators, sessions, and audit events in MySQL.
type MySQLRepository struct {
	db *sql.DB
}

// NewMySQLRepository returns a MySQL-backed Admin app repository.
func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}
