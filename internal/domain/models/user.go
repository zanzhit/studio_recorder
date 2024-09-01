package models

type User struct {
	Id       int    `db:"id"`
	Email    string `db:"email"`
	UserType string
	PassHash []byte `db:"password_hash"`
}
