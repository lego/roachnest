package testutils

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/lego/roachnest/pkg/cluster"
	"github.com/moby/moby/client"
)

// TODO(joey): Plumb a context through the connections.

type ClusterTest struct {
	ctx context.Context
	t   *testing.T
	c   cluster.Cluster
	gen *NameGenerator

	database string
}

type ClusterTestConfig struct {
	Settings cluster.Settings
	Config   cluster.Config
}

type RowGeneratorFunc func(*sql.DB) error

type DataSourceType int

const (
	_ DataSourceType = iota
	Generator
	File
)

type DataConfig struct {
	Typ DataSourceType

	RowGenerator RowGeneratorFunc

	Source string

	Cacheable bool
}

type SchemaCreatorFunc func(*sql.DB, *NameGenerator) error

type SchemaConfig struct {
	Database      string
	SchemaCreator SchemaCreatorFunc
}

func NewClusterTest(t *testing.T, config ClusterTestConfig) *ClusterTest {
	ctx := context.Background()
	var c cluster.Cluster
	switch v := config.Config.(type) {
	case *cluster.DockerConfig:
		client, err := client.NewEnvClient()
		if err != nil {
			t.Fatal(err)
		}
		c, err = cluster.NewDockerCluster(ctx, client, config.Settings, *v)
		if err != nil {
			// FIXME(joey): Clean is called inside the function. It should
			// maybe be moved out.
			//
			// if err := c.Cleanup(ctx); err != nil {
			// 	t.Error(err)
			// }
			t.Fatal(err)
		}
		if err := c.Start(ctx); err != nil {
			if err := c.Cleanup(ctx); err != nil {
				t.Error(err)
			}
			t.Fatal(err)
		}
	}

	gen := NewNameGenerator()
	return &ClusterTest{t: t, c: c, ctx: ctx, gen: gen}
}

func (ct *ClusterTest) Cleanup() error {
	return ct.c.Cleanup(ct.ctx)
}

func (ct *ClusterTest) LoadData(config DataConfig) error {
	if config.Typ != Generator && config.RowGenerator != nil {
		ct.t.Fatal("bad DataConfig.  RowGenerator was set but type is not Generator. got type: %s", config.Typ)
	} else if config.Typ != File && config.Source != "" {
		ct.t.Fatal("bad DataConfig. Source was set but not type is not File. got type: %s", config.Typ)
	}

	// FIXME(joey): We should require LoadSchema by here, or initialize
	// this elsewhere.
	if ct.database == "" {
		ct.database = ct.gen.DatabaseName()
	}

	switch config.Typ {
	case Generator:
		conn, err := ct.c.GetConnection(ct.ctx, ct.database)
		if err != nil {
			ct.t.Fatal(err)
		}
		if err := config.RowGenerator(conn); err != nil {
			ct.t.Fatal(err)
		}
	}
	return nil
}

func (ct *ClusterTest) LoadSchema(config SchemaConfig) error {
	if config.SchemaCreator == nil {
		ct.t.Fatal("no SchemaCreator provided")
	}

	ct.database = config.Database
	if ct.database == "" {
		ct.database = ct.gen.DatabaseName()
	}

	conn, err := ct.c.GetConnection(ct.ctx, ct.database)
	if err != nil {
		ct.t.Fatal(err)
	}

	if _, err := conn.ExecContext(ct.ctx,
		fmt.Sprintf("CREATE DATABASE %q", ct.database),
	); err != nil {
		ct.t.Fatal(err)
	}
	// if _, err := conn.ExecContext(ct.ctx,
	// 	fmt.Sprintf("USE %q", ct.database),
	// ); err != nil {
	// 	ct.t.Fatal(err)
	// }

	if err := config.SchemaCreator(conn, ct.gen); err != nil {
		ct.t.Fatal(err)
	}
	return nil
}

func (ct *ClusterTest) Error(args ...interface{}) {
	ct.t.Error(args...)
}

func (ct *ClusterTest) Errorf(format string, args ...interface{}) {
	ct.t.Errorf(format, args...)
}

func (ct *ClusterTest) Fail() {
	ct.t.Fail()
}

func (ct *ClusterTest) FailNow() {
	ct.t.FailNow()
}

func (ct *ClusterTest) Failed() bool {
	return ct.t.Failed()
}

// func (ct *ClusterTest) Fatal(args ...interface{}) {
// 	ct.t.Fatal(args...)
// }

func (ct *ClusterTest) Fatalf(format string, args ...interface{}) {
	ct.t.Fatalf(format, args...)
}

func (ct *ClusterTest) Log(args ...interface{}) {
	ct.t.Log(args...)
}

func (ct *ClusterTest) Logf(format string, args ...interface{}) {
	ct.t.Logf(format, args...)
}

func (ct *ClusterTest) Name() string {
	return ct.t.Name()
}

func (ct *ClusterTest) Skip(args ...interface{}) {
	ct.t.Skip(args...)
}

func (ct *ClusterTest) SkipNow() {
	ct.t.SkipNow()
}

func (ct *ClusterTest) Skipf(format string, args ...interface{}) {
	ct.t.Skipf(format, args...)
}

func (ct *ClusterTest) Skipped() bool {
	return ct.t.Skipped()
}

func (ct *ClusterTest) Helper() {
	ct.t.Helper()
}
