package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelectBuilder_BuildSimpleQuery(t *testing.T) {
	sb := &SelectBuilder{
		selectCol: []string{"id", "name", "email"},
		fromTable: "users",
		limitN:    10,
		offsetN:   0,
		orderAsc:  true,
	}

	query, args := sb.build()

	assert.Equal(t, "SELECT id, name, email FROM users LIMIT 10 OFFSET 0", query)
	assert.Empty(t, args)
}

func TestSelectBuilder_BuildWithWhere(t *testing.T) {
	sb := &SelectBuilder{
		selectCol: []string{"*"},
		fromTable: "users",
		whereCl:   []string{"active = $1", "created_at > $2"},
		whereArgs: []interface{}{true, "2024-01-01"},
		limitN:    20,
	}

	query, args := sb.build()

	assert.Contains(t, query, "WHERE")
	assert.Contains(t, query, "active = $1")
	assert.Contains(t, query, "created_at > $2")
	assert.Contains(t, query, "AND")
	assert.Equal(t, 2, len(args))
}

func TestSelectBuilder_BuildWithOrderBy(t *testing.T) {
	sb := &SelectBuilder{
		selectCol: []string{"*"},
		fromTable: "users",
		orderBy:   "created_at",
		orderAsc:  false,
	}

	query, _ := sb.build()

	assert.Contains(t, query, "ORDER BY created_at DESC")
}

func TestSelectBuilder_BuildWithAllOptions(t *testing.T) {
	sb := Select(nil).Columns("id", "name").From("users").Where("status = $1", "active").OrderBy("name").Limit(5).Offset(10)

	query := sb.Query()

	assert.Equal(t, "SELECT id, name FROM users WHERE status = $1 ORDER BY name LIMIT 5 OFFSET 10", query)
}

func TestSelectBuilder_DefaultColumns(t *testing.T) {
	sb := &SelectBuilder{}
	sb.Columns() // Empty - should default to *

	assert.Equal(t, []string{"*"}, sb.selectCol)
}

func TestSelectBuilder_SpecificColumns(t *testing.T) {
	sb := &SelectBuilder{}
	sb.Columns("id", "name", "email")

	assert.Equal(t, []string{"id", "name", "email"}, sb.selectCol)
}

func TestSelectBuilder_OrderByAscending(t *testing.T) {
	sb := &SelectBuilder{orderAsc: true, orderBy: "name"}

	query, _ := sb.build()
	assert.Contains(t, query, "ORDER BY name")
	assert.NotContains(t, query, "DESC")
}

func TestSelectBuilder_OrderByDescending(t *testing.T) {
	sb := &SelectBuilder{orderAsc: false, orderBy: "name"}

	query, _ := sb.build()
	assert.Contains(t, query, "ORDER BY name DESC")
}

func TestInsertBuilder_Set(t *testing.T) {
	ib := Insert(nil, "users").Set("name", "John").Set("email", "john@example.com")

	assert.Equal(t, 2, len(ib.columns))
	assert.Equal(t, 2, len(ib.values))
	assert.Equal(t, "name", ib.columns[0])
	assert.Equal(t, "John", ib.values[0])
}

func TestUpdateBuilder_Set(t *testing.T) {
	ub := Update(nil, "users").Set("name", "Jane").Set("status", "active")

	assert.Equal(t, 2, len(ub.sets))
	assert.Equal(t, 2, len(ub.setArgs))
}

func TestDeleteBuilder_Where(t *testing.T) {
	db := Delete(nil, "users").Where("id = $1", 123)

	assert.Equal(t, 1, len(db.whereCl))
	assert.Equal(t, 1, len(db.whereArgs))
}

func TestInsertBuilder_OnConflict(t *testing.T) {
	ib := Insert(nil, "users").Set("email", "test@example.com").OnConflictAdd("email").DoNothing()

	assert.Equal(t, []string{"email"}, ib.onConflictCols)
	assert.Equal(t, "NOTHING", ib.onConflictDo)
}
