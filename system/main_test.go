package main

import (
	"app.modules/core/constants"
	"app.modules/core/workspaceapp"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
	"time"
)

func TestSetProcessedUser(t *testing.T) {
	app := workspaceapp.WorkspaceApp{
		Configs:                         nil,
		Repository:                      nil,
		ProcessedUserId:                 "",
		ProcessedUserDisplayName:        "",
		ProcessedUserProfileImageUrl:    "",
		ProcessedUserIsModeratorOrOwner: false,
		ProcessedUserIsMember:           false,
	}

	// check initial values
	assert.Equal(t, app.ProcessedUserId, "")
	assert.Equal(t, app.ProcessedUserDisplayName, "")
	assert.Equal(t, app.ProcessedUserProfileImageUrl, "")
	assert.Equal(t, app.ProcessedUserIsModeratorOrOwner, false)
	assert.Equal(t, app.ProcessedUserIsMember, false)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	userId := "user1-id"
	userDisplayName := "user1-display-name"
	userProfileImageUrl := "https://example.com/user1-profile-image"
	isChatModerator := r.Intn(2) == 0
	isChatOwner := r.Intn(2) == 0
	isChatMember := r.Intn(2) == 0
	app.SetProcessedUser(userId, userDisplayName, userProfileImageUrl, isChatModerator, isChatOwner, isChatMember)

	// 正しくセットされたか
	assert.Equal(t, app.ProcessedUserId, userId)
	assert.Equal(t, app.ProcessedUserDisplayName, userDisplayName)
	assert.Equal(t, app.ProcessedUserProfileImageUrl, userProfileImageUrl)
	assert.Equal(t, app.ProcessedUserIsModeratorOrOwner, isChatModerator || isChatOwner)
	assert.Equal(t, app.ProcessedUserIsMember, isChatMember)
}

func TestCalculateRetryIntervalSec(t *testing.T) {
	type args struct {
		numContinuousFailed int
	}
	tests := []struct {
		args args
		want float64
	}{
		{
			args: args{numContinuousFailed: 0},
			want: 1,
		},
		{
			args: args{numContinuousFailed: 1},
			want: 1.2,
		},
		{
			args: args{numContinuousFailed: 2},
			want: 1.44,
		},
		{
			args: args{numContinuousFailed: 3},
			want: 1.728,
		},
		{
			args: args{numContinuousFailed: 4},
			want: 2.0736,
		},
		{
			args: args{numContinuousFailed: 5},
			want: 2.48832,
		},
		{
			args: args{numContinuousFailed: 10},
			want: 6.191736422,
		},
		{
			args: args{numContinuousFailed: 20},
			want: 38.337599924474700,
		},
		{ // 単純に計算すると300を超えるが、最大値は300
			args: args{numContinuousFailed: 50},
			want: 300,
		},
	}
	for _, tt := range tests {
		t.Run(pretty.Sprintf("%# v", tt), func(t *testing.T) {
			assert.InDeltaf(t, tt.want, CalculateRetryIntervalSec(constants.RetryIntervalCalculationBase, tt.args.numContinuousFailed), 0.1, "CalculateRetryIntervalSec(%v)", tt.args.numContinuousFailed)
		})
	}
}
