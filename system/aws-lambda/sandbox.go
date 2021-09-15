package main

import (
	"app.modules/core"
	"app.modules/core/guardians"
	"context"
	"fmt"
	"google.golang.org/api/option"
)

type LiveStreamsListResponse struct {
	Kind string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}


func main() {
	fmt.Println("こんばんは")
	credentialFilePath := "C:/Dev/GCP credentials/youtube-study-space-95bb4187aace.json"
	
	ctx := context.Background()
	clientOption := option.WithCredentialsFile(credentialFilePath)
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		_ = _system.LineBot.SendMessageWithError("failed core.NewSystem()", err)
		return
	}
	
	checker := guardians.NewLiveStreamChecker(_system.FirestoreController, _system.LiveChatBot, _system.LineBot)
	
	err = checker.Check(ctx)
	if err != nil {
		fmt.Println(err.Error())
	}
	return
}

