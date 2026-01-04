package id

import (
	"time"

	fid "github.com/amterp/flexid"
)

var generator *fid.Generator

func init() {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	config := fid.NewConfig().
		WithEpoch(epoch).
		WithTickSize(10 * time.Millisecond).
		WithNumRandomChars(3)

	generator = fid.MustNewGenerator(config)
}

// Generate returns a new unique ID.
func Generate() string {
	return generator.MustGenerate()
}
