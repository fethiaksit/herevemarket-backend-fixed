package database

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func EnsureProductIndexes(db *mongo.Database) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	indexes := db.Collection("products").Indexes()

	barcodeIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "barcode", Value: 1}},
		Options: options.Index().
			SetName("barcode_unique").
			SetUnique(true).
			SetPartialFilterExpression(bson.M{
				"barcode": bson.M{
					"$exists": true,
				},
			}),
	}

	log.Println("EnsureProductIndexes: creating barcode_unique index")
	_, err := indexes.CreateOne(ctx, barcodeIndex)
	if err != nil {
		log.Println("EnsureProductIndexes: barcode index error:", err)
		return err
	}
	log.Println("EnsureProductIndexes: barcode_unique index created")
	return nil
}

func EnsureUserIndexes(db *mongo.Database) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	indexes := db.Collection("users").Indexes()

	emailIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "email", Value: 1}},
		Options: options.Index().
			SetName("email_unique").
			SetUnique(true),
	}

	log.Println("EnsureUserIndexes: creating email_unique index")
	_, err := indexes.CreateOne(ctx, emailIndex)
	if err != nil {
		log.Println("EnsureUserIndexes: email index error:", err)
		return err
	}
	log.Println("EnsureUserIndexes: email_unique index created")
	return nil
}

func EnsureOrderIndexes(db *mongo.Database) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	indexes := db.Collection("orders").Indexes()

	userIDIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "userId", Value: 1}},
		Options: options.Index().SetName("userId_index"),
	}

	log.Println("EnsureOrderIndexes: creating userId_index index")
	_, err := indexes.CreateOne(ctx, userIDIndex)
	if err != nil {
		log.Println("EnsureOrderIndexes: userId index error:", err)
		return err
	}
	log.Println("EnsureOrderIndexes: userId_index index created")
	return nil
}
