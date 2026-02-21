package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Product struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Price       float64            `bson:"price" json:"price"`
	SaleEnabled bool               `bson:"saleEnabled" json:"saleEnabled"`
	SalePrice   float64            `bson:"salePrice" json:"salePrice"`
	IsOnSale    bool               `bson:"-" json:"isOnSale"`
	Category    StringList         `bson:"category" json:"category"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	Barcode     string             `bson:"barcode,omitempty" json:"barcode,omitempty"`
	Brand       string             `bson:"brand,omitempty" json:"brand,omitempty"`
	ImagePath   string             `bson:"imagePath,omitempty" json:"imagePath,omitempty"`
	Stock       int                `bson:"stock" json:"stock"`
	InStock     bool               `bson:"-" json:"inStock"`
	IsActive    bool               `bson:"isActive" json:"isActive"`
	IsCampaign  bool               `bson:"isCampaign" json:"isCampaign"`
	IsDeleted   bool               `bson:"isDeleted" json:"isDeleted,omitempty"`
	DeletedAt   *time.Time         `bson:"deletedAt,omitempty" json:"deletedAt,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
}
