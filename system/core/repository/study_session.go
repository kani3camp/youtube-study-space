package repository

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type StudySessionRepository struct {
	collection *mongo.Collection
}

func NewStudySessionRepository() *StudySessionRepository {
	client, err := NewMongoClient()
	if err != nil {
		log.Fatalf("failed to initialize mongo client in study session repository: %v", err)
	}
	db := GetDB(client)
	return &StudySessionRepository{
		collection: db.Collection("study_sessions"),
	}
}

func (r *StudySessionRepository) CreateStudySession(ctx context.Context, session *StudySession) error {
	_, err := r.collection.InsertOne(ctx, session)
	return err
}

func (r *StudySessionRepository) GetStudySessionByID(ctx context.Context, id string) (*StudySession, error) {
	var session StudySession
	filter := bson.M{"id": id}
	err := r.collection.FindOne(ctx, filter).Decode(&session)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Not found is not an error
		}
		return nil, err
	}
	return &session, nil
}

func (r *StudySessionRepository) UpdateStudySession(ctx context.Context, session *StudySession) error {
	filter := bson.M{"id": session.ID}
	update := bson.M{"$set": session}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}
