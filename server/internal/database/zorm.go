// Package database provides database operations using IceWhaleTech/zorm.
package database

import (
	"context"
	"database/sql"
	"time"

	z "github.com/IceWhaleTech/zorm"
)

// ZormDB wraps zorm operations with additional utilities.
type ZormDB struct {
	db     *sql.DB
	config ZormConfig
}

// ZormConfig holds zorm database configuration.
type ZormConfig struct {
	// DSN is the data source name
	DSN string `yaml:"dsn"`
	// MaxOpenConns is the maximum number of open connections
	MaxOpenConns int `yaml:"max_open_conns"`
	// MaxIdleConns is the maximum number of idle connections
	MaxIdleConns int `yaml:"max_idle_conns"`
	// ConnMaxLifetime is the maximum lifetime of a connection
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	// ConnMaxIdleTime is the maximum idle time of a connection
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time"`
}

// DefaultZormConfig returns default zorm configuration.
func DefaultZormConfig() ZormConfig {
	return ZormConfig{
		DSN:             "./data.db",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 30 * time.Minute,
	}
}

// NewZormDB creates a new zorm database connection.
func NewZormDB(config ZormConfig) (*ZormDB, error) {
	db, err := sql.Open("sqlite3", config.DSN)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return &ZormDB{
		db:     db,
		config: config,
	}, nil
}

// DB returns the underlying sql.DB.
func (zdb *ZormDB) DB() *sql.DB {
	return zdb.db
}

// Close closes the database connection.
func (zdb *ZormDB) Close() error {
	return zdb.db.Close()
}

// Table returns a zorm table reference.
func (zdb *ZormDB) Table(name string) *z.ZormTable {
	return z.Table(zdb.db, name)
}

// TableContext returns a zorm table reference with context.
func (zdb *ZormDB) TableContext(ctx context.Context, name string) *z.ZormTable {
	return z.TableContext(ctx, zdb.db, name)
}

// Repository provides generic CRUD operations using zorm.
type Repository[T any] struct {
	db        *ZormDB
	tableName string
}

// NewRepository creates a new repository for the given table.
func NewRepository[T any](db *ZormDB, tableName string) *Repository[T] {
	return &Repository[T]{
		db:        db,
		tableName: tableName,
	}
}

// Insert inserts a new record.
func (r *Repository[T]) Insert(ctx context.Context, entity *T, opts ...z.ZormItem) (int64, error) {
	t := r.db.TableContext(ctx, r.tableName)
	n, err := t.Insert(entity, opts...)
	return int64(n), err
}

// InsertBatch inserts multiple records.
func (r *Repository[T]) InsertBatch(ctx context.Context, entities *[]T, opts ...z.ZormItem) (int64, error) {
	t := r.db.TableContext(ctx, r.tableName)
	n, err := t.Insert(entities, opts...)
	return int64(n), err
}

// InsertIgnore inserts a record, ignoring duplicates.
func (r *Repository[T]) InsertIgnore(ctx context.Context, entity *T, opts ...z.ZormItem) (int64, error) {
	t := r.db.TableContext(ctx, r.tableName)
	n, err := t.InsertIgnore(entity, opts...)
	return int64(n), err
}

// InsertMap inserts a record using a map.
func (r *Repository[T]) InsertMap(ctx context.Context, data map[string]interface{}, opts ...z.ZormItem) (int64, error) {
	t := r.db.TableContext(ctx, r.tableName)
	n, err := t.Insert(data, opts...)
	return int64(n), err
}

// FindByID finds a record by ID.
func (r *Repository[T]) FindByID(ctx context.Context, id interface{}, entity *T) error {
	t := r.db.TableContext(ctx, r.tableName)
	_, err := t.Select(entity, z.Where(z.Eq("id", id)), z.Limit(1))
	return err
}

// FindOne finds a single record matching the conditions.
func (r *Repository[T]) FindOne(ctx context.Context, entity *T, opts ...z.ZormItem) error {
	t := r.db.TableContext(ctx, r.tableName)
	opts = append(opts, z.Limit(1))
	_, err := t.Select(entity, opts...)
	return err
}

// FindAll finds all records matching the conditions.
func (r *Repository[T]) FindAll(ctx context.Context, entities *[]T, opts ...z.ZormItem) (int64, error) {
	t := r.db.TableContext(ctx, r.tableName)
	n, err := t.Select(entities, opts...)
	return int64(n), err
}

// FindWithPagination finds records with pagination.
func (r *Repository[T]) FindWithPagination(ctx context.Context, entities *[]T, page, pageSize int, opts ...z.ZormItem) (int64, error) {
	t := r.db.TableContext(ctx, r.tableName)
	offset := (page - 1) * pageSize
	opts = append(opts, z.Limit(pageSize, offset))
	n, err := t.Select(entities, opts...)
	return int64(n), err
}

// Count counts records matching the conditions.
func (r *Repository[T]) Count(ctx context.Context, opts ...z.ZormItem) (int64, error) {
	t := r.db.TableContext(ctx, r.tableName)
	var count int64
	opts = append([]z.ZormItem{z.Fields("count(1)")}, opts...)
	_, err := t.Select(&count, opts...)
	return count, err
}

// Update updates a record.
func (r *Repository[T]) Update(ctx context.Context, entity *T, opts ...z.ZormItem) (int64, error) {
	t := r.db.TableContext(ctx, r.tableName)
	n, err := t.Update(entity, opts...)
	return int64(n), err
}

// UpdateByID updates a record by ID.
func (r *Repository[T]) UpdateByID(ctx context.Context, id interface{}, entity *T, opts ...z.ZormItem) (int64, error) {
	t := r.db.TableContext(ctx, r.tableName)
	opts = append(opts, z.Where(z.Eq("id", id)))
	n, err := t.Update(entity, opts...)
	return int64(n), err
}

// UpdateMap updates records using a map.
func (r *Repository[T]) UpdateMap(ctx context.Context, data map[string]interface{}, opts ...z.ZormItem) (int64, error) {
	t := r.db.TableContext(ctx, r.tableName)
	n, err := t.Update(z.V(data), opts...)
	return int64(n), err
}

// UpdateFields updates specific fields.
func (r *Repository[T]) UpdateFields(ctx context.Context, entity *T, fields []string, opts ...z.ZormItem) (int64, error) {
	t := r.db.TableContext(ctx, r.tableName)
	opts = append([]z.ZormItem{z.Fields(fields...)}, opts...)
	n, err := t.Update(entity, opts...)
	return int64(n), err
}

// Delete deletes records matching the conditions.
func (r *Repository[T]) Delete(ctx context.Context, opts ...z.ZormItem) (int64, error) {
	t := r.db.TableContext(ctx, r.tableName)
	n, err := t.Delete(opts...)
	return int64(n), err
}

// DeleteByID deletes a record by ID.
func (r *Repository[T]) DeleteByID(ctx context.Context, id interface{}) (int64, error) {
	t := r.db.TableContext(ctx, r.tableName)
	n, err := t.Delete(z.Where(z.Eq("id", id)))
	return int64(n), err
}

// Exec executes raw SQL.
func (r *Repository[T]) Exec(ctx context.Context, query string, args ...interface{}) (int64, error) {
	t := r.db.TableContext(ctx, r.tableName)
	n, err := t.Exec(query, args...)
	return int64(n), err
}

// Exists checks if a record exists.
func (r *Repository[T]) Exists(ctx context.Context, opts ...z.ZormItem) (bool, error) {
	count, err := r.Count(ctx, opts...)
	return count > 0, err
}

// QueryBuilder provides a fluent interface for building queries.
type QueryBuilder struct {
	table *z.ZormTable
	opts  []z.ZormItem
}

// NewQueryBuilder creates a new query builder.
func (r *Repository[T]) NewQueryBuilder(ctx context.Context) *QueryBuilder {
	return &QueryBuilder{
		table: r.db.TableContext(ctx, r.tableName),
		opts:  make([]z.ZormItem, 0),
	}
}

// Fields specifies the fields to select.
func (qb *QueryBuilder) Fields(fields ...string) *QueryBuilder {
	qb.opts = append(qb.opts, z.Fields(fields...))
	return qb
}

// Where adds a WHERE condition.
func (qb *QueryBuilder) Where(conds ...interface{}) *QueryBuilder {
	qb.opts = append(qb.opts, z.Where(conds...))
	return qb
}

// OrderBy adds ORDER BY clause.
func (qb *QueryBuilder) OrderBy(fields ...string) *QueryBuilder {
	qb.opts = append(qb.opts, z.OrderBy(fields...))
	return qb
}

// Limit adds LIMIT clause with optional offset.
func (qb *QueryBuilder) Limit(args ...interface{}) *QueryBuilder {
	qb.opts = append(qb.opts, z.Limit(args...))
	return qb
}

// GroupBy adds GROUP BY clause.
func (qb *QueryBuilder) GroupBy(fields ...string) *QueryBuilder {
	qb.opts = append(qb.opts, z.GroupBy(fields...))
	return qb
}

// Having adds HAVING clause.
func (qb *QueryBuilder) Having(conds ...interface{}) *QueryBuilder {
	qb.opts = append(qb.opts, z.Having(conds...))
	return qb
}

// InnerJoin adds INNER JOIN clause.
func (qb *QueryBuilder) InnerJoin(table string, on ...interface{}) *QueryBuilder {
	qb.opts = append(qb.opts, z.InnerJoin(table, on...))
	return qb
}

// LeftJoin adds LEFT JOIN clause.
func (qb *QueryBuilder) LeftJoin(table string, on ...interface{}) *QueryBuilder {
	qb.opts = append(qb.opts, z.LeftJoin(table, on...))
	return qb
}

// Select executes the query and returns results.
func (qb *QueryBuilder) Select(dest interface{}) (int64, error) {
	n, err := qb.table.Select(dest, qb.opts...)
	return int64(n), err
}

// Update executes an update query.
func (qb *QueryBuilder) Update(data interface{}) (int64, error) {
	n, err := qb.table.Update(data, qb.opts...)
	return int64(n), err
}

// Delete executes a delete query.
func (qb *QueryBuilder) Delete() (int64, error) {
	n, err := qb.table.Delete(qb.opts...)
	return int64(n), err
}

// Condition helpers re-exported from zorm for convenience.
var (
	Eq        = z.Eq
	Neq       = z.Neq
	Gt        = z.Gt
	Gte       = z.Gte
	Lt        = z.Lt
	Lte       = z.Lte
	In        = z.In
	Like      = z.Like
	Between   = z.Between
	IsNull    = z.IsNull
	IsNotNull = z.IsNotNull
	Cond      = z.Cond
	Fields    = z.Fields
	Where     = z.Where
	OrderBy   = z.OrderBy
	Limit     = z.Limit
	GroupBy   = z.GroupBy
	Having    = z.Having
	And       = z.And
	Or        = z.Or
)

// V is a type alias for map values.
type V = z.V

// U is a type alias for update expressions.
type U = z.U
