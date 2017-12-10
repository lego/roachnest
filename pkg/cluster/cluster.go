package cluster

import (
	"context"
	"database/sql"
)

type Settings struct {
	Size int

	SetupToxiproxy bool
}

type Cluster interface {
	Start(context.Context) error
	Cleanup(context.Context) error

	GetConnection(ctx context.Context, database string) (*sql.DB, error)
}

type Type string

const (
	Docker Type = "docker"
)

type Config interface {
	Type() Type
}
