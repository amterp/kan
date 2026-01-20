// Package id provides unique ID generation for Kan entities.
//
// IMPORTANT: ID Format Is Not Part of the API
//
// IDs are opaque strings. The current format (prefix + flexid) is purely for
// human convenience—making it easy to distinguish a board ID from a card ID
// at a glance. Code MUST NOT parse, validate, or depend on ID structure.
//
// The format may change at any time. Existing IDs remain valid indefinitely;
// only the generation of new IDs may change. Always treat IDs as opaque strings
// for comparison and storage.
package id

import (
	"time"

	fid "github.com/amterp/flexid"
)

// Entity represents a type of entity that can have a generated ID.
// Used to assign human-friendly prefixes—see package doc for caveats.
type Entity int

const (
	Card    Entity = iota // a_
	Board                 // b_
	Comment               // c_
	Project               // p_
	// Future entities: d_, e_, ...
)

// prefixes maps entity types to their current prefix.
// These are purely cosmetic and may change—see package doc.
var prefixes = map[Entity]string{
	Card:    "a_",
	Board:   "b_",
	Comment: "c_",
	Project: "p_",
}

var generator *fid.Generator

func init() {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	config := fid.NewConfig().
		WithEpoch(epoch).
		WithTickSize(10 * time.Millisecond).
		WithNumRandomChars(3)

	generator = fid.MustNewGenerator(config)
}

// Generate returns a new unique ID for the given entity type.
func Generate(entity Entity) string {
	return prefixes[entity] + generator.MustGenerate()
}
