package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DefaultMaxConns int32 = 10
	DefaultMinConns int32 = 2
)

var ErrInvalidPoolOptions = errors.New("invalid pool options")

type PoolOptions struct {
	MaxConns int32
	MinConns int32
}

func DefaultPoolOptions() PoolOptions {
	return PoolOptions{MaxConns: DefaultMaxConns, MinConns: DefaultMinConns}
}

func (o PoolOptions) validate() error {
	switch {
	case o.MaxConns < 1:
		return fmt.Errorf("%w: MaxConns must be >= 1", ErrInvalidPoolOptions)
	case o.MinConns < 0:
		return fmt.Errorf("%w: MinConns must be >= 0", ErrInvalidPoolOptions)
	case o.MinConns > o.MaxConns:
		return fmt.Errorf("%w: MinConns must be <= MaxConns", ErrInvalidPoolOptions)
	}
	return nil
}

func NewPool(ctx context.Context, databaseURL string, opts PoolOptions) (*pgxpool.Pool, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}
	cfg.MaxConns = opts.MaxConns
	cfg.MinConns = opts.MinConns

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}
