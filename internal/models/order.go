package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrderItem represents a single product entry within an order.
type OrderItem struct {
	ProductID primitive.ObjectID `bson:"productId" json:"productId"`
	Name      string             `bson:"name" json:"name"`
	Price     float64            `bson:"price" json:"price"`
	Quantity  int                `bson:"quantity" json:"quantity"`
}

// OrderCustomer captures lightweight customer contact details for an order.
type OrderCustomer struct {
	Title  string `bson:"title" json:"title"`
	Detail string `bson:"detail" json:"detail"`
	Note   string `bson:"note,omitempty" json:"note,omitempty"`
}

// Order defines the persisted order document.
type Order struct {
	ID            primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	UserID        *primitive.ObjectID `bson:"userId" json:"userId"`
	Items         []OrderItem         `bson:"items" json:"items"`
	TotalPrice    float64             `bson:"totalPrice" json:"totalPrice"`
	Customer      OrderCustomer       `bson:"customer" json:"customer"`
	PaymentMethod string              `bson:"paymentMethod" json:"paymentMethod"`
	Status        string              `bson:"status" json:"status"`
	CreatedAt     time.Time           `bson:"createdAt" json:"createdAt"`
}
