package host

import (
	"database/sql"
)

type Status int

const (
	_ Status = iota
	Created
	Starting
	Running
	Stopping
	Stopped
	Deleted
)

type Client interface {
	// FIXME(joey): Will this be needed? Maybe for other clients.
	// Setup() error

	Start() error

	WaitUntil(Status) error

	Stop() error

	Delete() error

	GetConnection() (*sql.DB, error)
}

// type FixtureLoader interface {
// 	CreateSchema() error
// 	AddData() error
// }
