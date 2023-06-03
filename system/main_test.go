package main

import (
	"app.modules/core"
	"github.com/stretchr/testify/assert"
	"log"
	"math/rand"
	"testing"
	"time"
)

func TestSetProcessedUser(t *testing.T) {
	clientOption, ctx, err := Init()
	if err != nil {
		log.Println(err.Error())
		return
	}
	
	s, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer s.CloseFirestoreClient()
	// === ここまでおまじない ===
	
	// check initial values
	assert.Equal(t, s.ProcessedUserId, "")
	assert.Equal(t, s.ProcessedUserDisplayName, "")
	assert.Equal(t, s.ProcessedUserProfileImageUrl, "")
	assert.Equal(t, s.ProcessedUserIsModeratorOrOwner, false)
	assert.Equal(t, s.ProcessedUserIsMember, false)
	
	rand.Seed(time.Now().UnixNano())
	
	userId := "user1-id"
	userDisplayName := "user1-display-name"
	userProfileImageUrl := "https://example.com/user1-profile-image"
	isChatModerator := rand.Intn(2) == 0
	isChatOwner := rand.Intn(2) == 0
	isChatMember := rand.Intn(2) == 0
	s.SetProcessedUser(userId, userDisplayName, userProfileImageUrl, isChatModerator, isChatOwner, isChatMember)
	
	// 正しくセットされたか
	assert.Equal(t, s.ProcessedUserId, userId)
	assert.Equal(t, s.ProcessedUserDisplayName, userDisplayName)
	assert.Equal(t, s.ProcessedUserProfileImageUrl, userProfileImageUrl)
	assert.Equal(t, s.ProcessedUserIsModeratorOrOwner, isChatModerator || isChatOwner)
	assert.Equal(t, s.ProcessedUserIsMember, isChatMember)
}
