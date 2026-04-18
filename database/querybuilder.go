// Package database provides SQL query building utilities.
//
// This package provides a lightweight query builder for PostgreSQL.
// It supports SELECT, INSERT, UPDATE, and DELETE operations with
// a fluent API for building queries programmatically.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// SelectBuilder provides a fluent interface for building SELECT queries
type SelectBuilder struct {
	db        *sql.DB
	selectCol []string
	fromTable string
	whereCl   []string
	whereArgs []interface{}
	orderBy   string
	orderAsc  bool
	limitN    int
	offsetN   int
}

// Select creates a new SelectBuilder for SELECT queries
func Select(db *sql.DB) *SelectBuilder {
	return &SelectBuilder{
		db:       db,
		limitN:   -1,
		offsetN:  -1,
		orderAsc: true,
	}
}

// Columns specifies the columns to select
func (qb *SelectBuilder) Columns(cols ...string) *SelectBuilder {
	if len(cols) == 0 {
		qb.selectCol = []string{"*"}
	} else {
		qb.selectCol = cols
	}
	return qb
}

// From specifies the table to query from
func (qb *SelectBuilder) From(table string) *SelectBuilder {
	qb.fromTable = table
	return qb
}

// Where adds a WHERE condition. Multiple calls create AND conditions.
// Use $1, $2, etc. for parameterized queries to prevent SQL injection.
func (qb *SelectBuilder) Where(cond string, args ...interface{}) *SelectBuilder {
	qb.whereCl = append(qb.whereCl, cond)
	qb.whereArgs = append(qb.whereArgs, args...)
	return qb
}

// OrderBy specifies the ORDER BY clause
func (qb *SelectBuilder) OrderBy(col string) *SelectBuilder {
	qb.orderBy = col
	return qb
}

// Asc sets the order to ascending (default)
func (qb *SelectBuilder) Asc() *SelectBuilder {
	qb.orderAsc = true
	return qb
}

// Desc sets the order to descending
func (qb *SelectBuilder) Desc() *SelectBuilder {
	qb.orderAsc = false
	return qb
}

// Limit sets the maximum number of rows to return
func (qb *SelectBuilder) Limit(n int) *SelectBuilder {
	qb.limitN = n
	return qb
}

// Offset sets the number of rows to skip
func (qb *SelectBuilder) Offset(n int) *SelectBuilder {
	qb.offsetN = n
	return qb
}

// Query returns the constructed query string
func (qb *SelectBuilder) Query() string {
	query, _ := qb.build()
	return query
}

// Args returns the query arguments
func (qb *SelectBuilder) Args() []interface{} {
	_, args := qb.build()
	return args
}

// Exec executes the query and returns rows
func (qb *SelectBuilder) Exec(ctx context.Context) (*sql.Rows, error) {
	query, args := qb.build()
	return qb.db.QueryContext(ctx, query, args...)
}

func (qb *SelectBuilder) build() (string, []interface{}) {
	var sb strings.Builder

	sb.WriteString("SELECT ")
	sb.WriteString(strings.Join(qb.selectCol, ", "))
	sb.WriteString(" FROM ")
	sb.WriteString(qb.fromTable)

	if len(qb.whereCl) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(qb.whereCl, " AND "))
	}

	if qb.orderBy != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(qb.orderBy)
		if !qb.orderAsc {
			sb.WriteString(" DESC")
		}
	}

	if qb.limitN >= 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", qb.limitN))
	}

	if qb.offsetN >= 0 {
		sb.WriteString(fmt.Sprintf(" OFFSET %d", qb.offsetN))
	}

	return sb.String(), qb.whereArgs
}

// InsertBuilder provides a fluent interface for building INSERT queries
type InsertBuilder struct {
	db             *sql.DB
	table          string
	columns        []string
	values         []interface{}
	onConflictDo   string
	onConflictCols []string
}

// Insert creates a new InsertBuilder
func Insert(db *sql.DB, table string) *InsertBuilder {
	return &InsertBuilder{
		db:     db,
		table:  table,
		values: []interface{}{},
	}
}

// Set adds a column value
func (ib *InsertBuilder) Set(column string, value interface{}) *InsertBuilder {
	ib.columns = append(ib.columns, column)
	ib.values = append(ib.values, value)
	return ib
}

// OnConflictAdd adds ON CONFLICT (column) clause
func (ib *InsertBuilder) OnConflictAdd(columns ...string) *InsertBuilder {
	ib.onConflictCols = columns
	return ib
}

// DoNothing adds DO NOTHING to ON CONFLICT clause
func (ib *InsertBuilder) DoNothing() *InsertBuilder {
	ib.onConflictDo = "NOTHING"
	return ib
}

// Exec executes the INSERT query
func (ib *InsertBuilder) Exec(ctx context.Context) (sql.Result, error) {
	if len(ib.columns) == 0 || len(ib.values) == 0 {
		return nil, fmt.Errorf("no columns or values to insert")
	}

	var sb strings.Builder
	sb.WriteString("INSERT INTO ")
	sb.WriteString(ib.table)
	sb.WriteString(" (")
	sb.WriteString(strings.Join(ib.columns, ", "))
	sb.WriteString(") VALUES (")

	placeholders := make([]string, len(ib.columns))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	sb.WriteString(strings.Join(placeholders, ", "))
	sb.WriteString(")")

	if len(ib.onConflictCols) > 0 && ib.onConflictDo != "" {
		sb.WriteString(" ON CONFLICT (")
		sb.WriteString(strings.Join(ib.onConflictCols, ", "))
		sb.WriteString(") DO ")
		sb.WriteString(ib.onConflictDo)
	}

	query := sb.String()
	return ib.db.ExecContext(ctx, query, ib.values...)
}

// UpdateBuilder provides a fluent interface for building UPDATE queries
type UpdateBuilder struct {
	db        *sql.DB
	table     string
	sets      []string
	setArgs   []interface{}
	whereCl   []string
	whereArgs []interface{}
}

// Update creates a new UpdateBuilder
func Update(db *sql.DB, table string) *UpdateBuilder {
	return &UpdateBuilder{
		db:    db,
		table: table,
	}
}

// Set adds a SET clause (column = $n)
func (ub *UpdateBuilder) Set(column string, value interface{}) *UpdateBuilder {
	ub.sets = append(ub.sets, fmt.Sprintf("%s = $%d", column, len(ub.setArgs)+1))
	ub.setArgs = append(ub.setArgs, value)
	return ub
}

// Where adds a WHERE condition
func (ub *UpdateBuilder) Where(cond string, args ...interface{}) *UpdateBuilder {
	ub.whereCl = append(ub.whereCl, cond)
	ub.whereArgs = append(ub.whereArgs, args...)
	return ub
}

// Exec executes the UPDATE query
func (ub *UpdateBuilder) Exec(ctx context.Context) (sql.Result, error) {
	if len(ub.sets) == 0 {
		return nil, fmt.Errorf("no columns to update")
	}

	var sb strings.Builder
	sb.WriteString("UPDATE ")
	sb.WriteString(ub.table)
	sb.WriteString(" SET ")
	sb.WriteString(strings.Join(ub.sets, ", "))

	if len(ub.whereCl) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(ub.whereCl, " AND "))
	}

	query := sb.String()
	args := append(ub.setArgs, ub.whereArgs...)

	return ub.db.ExecContext(ctx, query, args...)
}

// DeleteBuilder provides a fluent interface for building DELETE queries
type DeleteBuilder struct {
	db        *sql.DB
	table     string
	whereCl   []string
	whereArgs []interface{}
}

// Delete creates a new DeleteBuilder
func Delete(db *sql.DB, table string) *DeleteBuilder {
	return &DeleteBuilder{
		db:    db,
		table: table,
	}
}

// Where adds a WHERE condition
func (db *DeleteBuilder) Where(cond string, args ...interface{}) *DeleteBuilder {
	db.whereCl = append(db.whereCl, cond)
	db.whereArgs = append(db.whereArgs, args...)
	return db
}

// Exec executes the DELETE query
func (db *DeleteBuilder) Exec(ctx context.Context) (sql.Result, error) {
	var sb strings.Builder
	sb.WriteString("DELETE FROM ")
	sb.WriteString(db.table)

	if len(db.whereCl) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(db.whereCl, " AND "))
	}

	query := sb.String()
	return db.db.ExecContext(ctx, query, db.whereArgs...)
}
