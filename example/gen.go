package example

//go:generate go run github.com/agrewal/norm example.norm.sql

type User struct {
	ID    int
	Email *string
}
