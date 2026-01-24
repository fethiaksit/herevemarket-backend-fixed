package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Customer struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FirstName    string             `bson:"firstName" json:"firstName"`
	LastName     string             `bson:"lastName" json:"lastName"`
	Email        string             `bson:"email" json:"email"`
	Phone        string             `bson:"phone,omitempty" json:"phone,omitempty"`
	PasswordHash string             `bson:"passwordHash" json:"-"`
	IsActive     bool               `bson:"isActive" json:"isActive"`
	Role         string             `bson:"role" json:"role"`
	CreatedAt    time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt" json:"updatedAt"`
}
