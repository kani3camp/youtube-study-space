package workspaceapp

import (
	"context"
	"testing"

	"app.modules/core/repository"
	mock_repository "app.modules/core/repository/mocks"
	"app.modules/core/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRoomEntryOrchestrator_ValidatePreConditions(t *testing.T) {
	tests := []struct {
		name           string
		isMember       bool
		targetMemberSeat bool
		expectedSuccess bool
	}{
		{
			name:           "member accessing member seat",
			isMember:       true,
			targetMemberSeat: true,
			expectedSuccess: true,
		},
		{
			name:           "member accessing general seat",
			isMember:       true,
			targetMemberSeat: false,
			expectedSuccess: true,
		},
		{
			name:           "non-member accessing general seat",
			isMember:       false,
			targetMemberSeat: false,
			expectedSuccess: true,
		},
		{
			name:           "non-member accessing member seat",
			isMember:       false,
			targetMemberSeat: true,
			expectedSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			
			mockRepo := mock_repository.NewMockRepository(ctrl)
			
			// Set up mock expectations for IsUserInRoom method calls
			// IsUserInRoom calls ReadSeatWithUserId twice - for member seat and general seat
			// Some test cases may not call IsUserInRoom if validation fails early
			notFoundErr := status.Error(codes.NotFound, "seat not found")
			mockRepo.EXPECT().ReadSeatWithUserId(gomock.Any(), "test-user", true).Return(repository.SeatDoc{}, notFoundErr).AnyTimes()
			mockRepo.EXPECT().ReadSeatWithUserId(gomock.Any(), "test-user", false).Return(repository.SeatDoc{}, notFoundErr).AnyTimes()
			app := &WorkspaceApp{
				ProcessedUserId:          "test-user",
				ProcessedUserDisplayName: "Test User",
				ProcessedUserIsMember:    tt.isMember,
				Repository:               mockRepo,
				Configs: &Configs{
					Constants: repository.ConstantsConfigDoc{
						YoutubeMembershipEnabled: true,
						MinWorkTimeMin:          10,
						MaxWorkTimeMin:          480,
						DefaultWorkTimeMin:      60,
					},
				},
			}

			orchestrator := NewRoomEntryOrchestrator(app)

			request := &RoomEntryRequest{
				UserId: "test-user",
				InOption: &utils.InOption{
					IsMemberSeat: tt.targetMemberSeat,
				},
				IsMemberSeat: tt.targetMemberSeat,
			}

			result := orchestrator.ValidatePreConditions(request)

			assert.Equal(t, tt.expectedSuccess, result.Success)
			if !tt.expectedSuccess {
				assert.True(t, result.ShouldReturn)
				assert.NotEmpty(t, result.Message)
			}
		})
	}
}

func TestRoomEntryOrchestrator_NewRoomEntryOrchestrator(t *testing.T) {
	app := &WorkspaceApp{
		ProcessedUserId:          "test-user",
		ProcessedUserDisplayName: "Test User",
		ProcessedUserIsMember:    false,
		Configs: &Configs{
			Constants: repository.ConstantsConfigDoc{
				YoutubeMembershipEnabled: true,
				MinWorkTimeMin:          10,
				MaxWorkTimeMin:          480,
				DefaultWorkTimeMin:      60,
			},
		},
	}

	orchestrator := NewRoomEntryOrchestrator(app)

	assert.NotNil(t, orchestrator)
	assert.NotNil(t, orchestrator.validator)
	assert.NotNil(t, orchestrator.seatService)
	assert.NotNil(t, orchestrator.userStateService)
	assert.Equal(t, app, orchestrator.app)
}

func TestRoomEntryRequest_StructureValidation(t *testing.T) {
	request := &RoomEntryRequest{
		UserId: "test-user-123",
		InOption: &utils.InOption{
			IsSeatIdSet:  true,
			SeatId:       5,
			IsMemberSeat: false,
		},
		IsMemberSeat: false,
	}

	assert.Equal(t, "test-user-123", request.UserId)
	assert.NotNil(t, request.InOption)
	assert.Equal(t, 5, request.InOption.SeatId)
	assert.True(t, request.InOption.IsSeatIdSet)
	assert.False(t, request.IsMemberSeat)
}

func TestRoomEntryResult_StructureValidation(t *testing.T) {
	result := &RoomEntryResult{
		Success:        true,
		Message:        "Entry successful",
		Error:          nil,
		AssignedSeatId: 5,
		UserSettings: &UserSettings{
			WorkDurationMin: 60,
		},
		ShouldReturn: false,
	}

	assert.True(t, result.Success)
	assert.Equal(t, "Entry successful", result.Message)
	assert.Nil(t, result.Error)
	assert.Equal(t, 5, result.AssignedSeatId)
	assert.NotNil(t, result.UserSettings)
	assert.Equal(t, 60, result.UserSettings.WorkDurationMin)
	assert.False(t, result.ShouldReturn)
}

func TestRoomStatus_StructureValidation(t *testing.T) {
	status := &RoomStatus{
		GeneralSeatsAvailable: 10,
		MemberSeatsAvailable:  5,
		TotalUsers:           15,
	}

	assert.Equal(t, 10, status.GeneralSeatsAvailable)
	assert.Equal(t, 5, status.MemberSeatsAvailable)
	assert.Equal(t, 15, status.TotalUsers)
}

func TestRoomEntryOrchestrator_GetRoomStatus(t *testing.T) {
	app := &WorkspaceApp{
		ProcessedUserId:          "test-user",
		ProcessedUserDisplayName: "Test User",
		ProcessedUserIsMember:    false,
		Configs: &Configs{
			Constants: repository.ConstantsConfigDoc{
				YoutubeMembershipEnabled: true,
				MinWorkTimeMin:          10,
				MaxWorkTimeMin:          480,
				DefaultWorkTimeMin:      60,
			},
		},
	}

	orchestrator := NewRoomEntryOrchestrator(app)
	ctx := context.Background()

	status, err := orchestrator.GetRoomStatus(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	// Currently returns placeholder values
	assert.Equal(t, 0, status.GeneralSeatsAvailable)
	assert.Equal(t, 0, status.MemberSeatsAvailable)
	assert.Equal(t, 0, status.TotalUsers)
}

// Integration test placeholder (will be implemented when we have more complete mocking)
func TestRoomEntryOrchestrator_ProcessRoomEntry_Placeholder(t *testing.T) {
	t.Skip("Integration test requires database mocking - to be implemented")
	
	// This test would verify the complete ProcessRoomEntry workflow
	// including transaction handling and service coordination
	// We'll implement this once we have the database operations completed
}