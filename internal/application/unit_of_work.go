package application

import "context"

// UnitOfWork defines a transaction boundary for use cases.
// implementations live in infrastructure, application only sees this interface.
// this keeps database transaction details out of domain and application logic.
type UnitOfWork interface {
	// Begin starts a new transaction and returns a context scoped to it.
	// all repository operations using this context will participate in the transaction.
	Begin(ctx context.Context) (context.Context, error)

	// Commit commits the current transaction.
	Commit(ctx context.Context) error

	// Rollback aborts the current transaction.
	// safe to call multiple times or after commit (will be a no-op).
	Rollback(ctx context.Context) error
}

// TransactionalRepositories provides repository instances scoped to a transaction.
// use cases receive this to access repositories within a unit of work.
type TransactionalRepositories interface {
	// WithTx returns a new instance with repositories scoped to the given transaction context.
	// the returned instance should be used for all operations within the transaction.
	WithTx(ctx context.Context) TransactionalRepositories
}

// RunInTransaction is a helper that executes a function within a transaction.
// automatically commits on success, rolls back on error.
// this is the preferred way to handle transactions in use cases.
func RunInTransaction(ctx context.Context, uow UnitOfWork, fn func(ctx context.Context) error) error {
	txCtx, err := uow.Begin(ctx)
	if err != nil {
		return err
	}

	// always try to rollback on exit - it's a no-op if already committed
	defer uow.Rollback(txCtx)

	if err := fn(txCtx); err != nil {
		return err
	}

	return uow.Commit(txCtx)
}
