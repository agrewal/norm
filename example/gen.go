package example

//go:generate norm example.norm.sql

type User struct {
	ID    int
	Email string
}
