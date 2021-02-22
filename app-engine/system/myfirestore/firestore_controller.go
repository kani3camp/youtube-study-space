package myfirestore

import (
	"app.modules/system"
	"cloud.google.com/go/firestore"
	"context"
	"google.golang.org/api/option"
)

type FirestoreController struct {
	FirestoreClient *firestore.Client
}

func NewFirestoreController(ctx context.Context, projectId string, clientOption option.ClientOption) (*FirestoreController, error) {
	var client *firestore.Client
	var err error
	client, err = firestore.NewClient(ctx, system.ProjectId, clientOption)
	if err != nil {
		return nil, err
	}

	return &FirestoreController{
		FirestoreClient: client,
	}, nil
}

func (controller *FirestoreController) RetrieveYoutubeLiveInfo(ctx context.Context) (YoutubeLiveDoc, error) {
	doc, err := controller.FirestoreClient.Collection(CONFIG).Doc(YouTubeLiveDocName).Get(ctx)
	if err != nil {
		return YoutubeLiveDoc{}, err
	}
	var youtubeLiveData YoutubeLiveDoc
	err = doc.DataTo(&youtubeLiveData)
	if err != nil {
		return YoutubeLiveDoc{}, err
	}
	return youtubeLiveData, nil
}

func (controller *FirestoreController) RetrieveLiveChatId(ctx context.Context) (string, error) {
	youtubeLiveDoc, err := controller.RetrieveYoutubeLiveInfo(ctx)
	if err != nil {
		return "", err
	}
	return youtubeLiveDoc.LiveChatId, nil
}