package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// txKey is the context key for storing transaction.
type txKey struct{}

// UnitOfWork implements application.UnitOfWork using pgx transactions.
// provides transaction boundaries without leaking pgx types to application layer.
type UnitOfWork struct {
	pool *pgxpool.Pool
}

// NewUnitOfWork creates a new UnitOfWork.
func NewUnitOfWork(pool *pgxpool.Pool) *UnitOfWork {
	return &UnitOfWork{pool: pool}
}

// Begin starts a new transaction and stores it in context.
func (u *UnitOfWork) Begin(ctx context.Context) (context.Context, error) {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	return context.WithValue(ctx, txKey{}, tx), nil
}

// Commit commits the transaction stored in context.
func (u *UnitOfWork) Commit(ctx context.Context) error {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	if !ok {
		return errors.New("no transaction in context")
	}
	return tx.Commit(ctx)
}

// Rollback rolls back the transaction stored in context.
// safe to call after commit (will return nil).
func (u *UnitOfWork) Rollback(ctx context.Context) error {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	if !ok {
		return nil // no transaction, nothing to rollback
	}
	// pgx returns nil if already committed/rolled back
	return tx.Rollback(ctx)
}

// Querier is an interface that both pgxpool.Pool and pgx.Tx satisfy.
// allows repositories to work with either direct pool or transaction.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// GetQuerier returns the appropriate querier for the context.
// if a transaction exists in context, returns the transaction.
// otherwise returns the pool for direct queries.
func GetQuerier(ctx context.Context, pool *pgxpool.Pool) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}
