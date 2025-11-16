package repository

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserRepository struct {
	collection *mongo.Collection
}

func NewUserRepository() *UserRepository {
	client, err := NewMongoClient()
	if err != nil {
		log.Fatalf("failed to initialize mongo client in user repository: %v", err)
	}
	db := GetDB(client)
	return &UserRepository{
		collection: db.Collection("users"),
	}
}

func (r *UserRepository) GetUserByYouTubeUserID(ctx context.Context, youtubeUserID string) (*UserDoc, error) {
	var user UserDoc
	filter := bson.M{"youtube_user_id": youtubeUserID}
	err := r.collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Not found is not an error
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) CreateUser(ctx context.Context, user *UserDoc) error {
	_, err := r.collection.InsertOne(ctx, user)
	return err
}

func (r *UserRepository) UpdateUser(ctx context.Context, user *UserDoc) error {
	filter := bson.M{"youtube_user_id": user.YouTubeUserID}
	update := bson.M{"$set": user}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}
