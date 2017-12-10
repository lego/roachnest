package test

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/lego/roachnest/pkg/cluster"
	"github.com/lego/roachnest/pkg/testutils"
)

func TestCluster(t *testing.T) {
	ct := testutils.NewClusterTest(t, testutils.ClusterTestConfig{
		Settings: cluster.Settings{
			Size: 3,
		},
		Config: &cluster.DockerConfig{
			NetworkName: "roachnet",
			NamePrefix:  "roach",
			Image:       "cockroachdb/cockroach",
			Tag:         "latest",
		},
	})
	// defer ct.Cleanup()

	ct.LoadSchema(testutils.SchemaConfig{
		SchemaCreator: func(db *sql.DB, gen *testutils.NameGenerator) error {
			if _, err := db.Exec("CREATE TABLE basic (id INT PRIMARY KEY, name STRING)"); err != nil {
				return err
			}
			return nil
		},
	})

	ct.LoadData(testutils.DataConfig{
		Typ: testutils.Generator,
		RowGenerator: func(db *sql.DB) error {
			for i := 0; i < 1000; i++ {
				if _, err := db.Exec(
					`INSERT INTO basic (id, name) VALUES ($1, $2)`,
					i,
					fmt.Sprintf("huzza%d", i),
				); err != nil {
					return err
				}
			}
			return nil
		},
	})
}
