package model

// Store is an abstraction.
type Store interface {
	Get(id string) Record
}

// Record is concrete.
type Record struct {
	ID string
}

type hidden struct{}

// Alias is not a declaration of its own.
type Alias = Record
