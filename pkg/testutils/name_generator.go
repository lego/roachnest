package testutils

import (
	"fmt"
	"math/rand"
	"time"
)

type NameGenerator struct {
	r *rand.Rand
}

func NewNameGenerator() *NameGenerator {
	return &NameGenerator{
		// FIXME(joey): Allow re-usable seed.
		r: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (gen *NameGenerator) TableName() string {
	return fmt.Sprintf("table%d", gen.r.Int())
}

func (gen *NameGenerator) DatabaseName() string {
	return fmt.Sprintf("database%d", gen.r.Int())
}
