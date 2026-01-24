package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Address represents a single address entry for a user.
type Address struct {
	ID        string `bson:"id" json:"id"`
	Title     string `bson:"title" json:"title"`
	Detail    string `bson:"detail" json:"detail"`
	Note      string `bson:"note,omitempty" json:"note,omitempty"`
	IsDefault bool   `bson:"isDefault" json:"isDefault"`
}

// User represents the application user account.
type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email        string             `bson:"email" json:"email"`
	PasswordHash string             `bson:"passwordHash" json:"-"`
	Name         string             `bson:"name" json:"name"`
	Phone        string             `bson:"phone,omitempty" json:"phone,omitempty"`
	Addresses    []Address          `bson:"addresses" json:"addresses"`
	CreatedAt    time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt" json:"updatedAt"`
}
