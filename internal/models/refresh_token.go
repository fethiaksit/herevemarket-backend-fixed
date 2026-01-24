package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RefreshToken struct {
	ID              primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	UserID          primitive.ObjectID  `bson:"userId" json:"userId"`
	TokenHash       string              `bson:"tokenHash" json:"tokenHash"`
	ExpiresAt       time.Time           `bson:"expiresAt" json:"expiresAt"`
	Revoked         bool                `bson:"revoked" json:"revoked"`
	CreatedAt       time.Time           `bson:"createdAt" json:"createdAt"`
	ReplacedByToken *primitive.ObjectID `bson:"replacedByToken,omitempty" json:"replacedByToken,omitempty"`
}
