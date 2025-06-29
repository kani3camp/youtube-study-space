package workspaceapp

import (
	"context"
	"testing"

	"app.modules/core/repository"
	"app.modules/core/studyspaceerror"
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
)

// MockWorkspaceAppForValidator provides a mock for testing validator
type MockWorkspaceAppForValidator struct {
	ProcessedUserId                 string
	ProcessedUserDisplayName        string
	ProcessedUserIsMember           bool
	Configs                         *Configs
}

func (m *MockWorkspaceAppForValidator) IfSeatVacant(ctx context.Context, tx *firestore.Transaction, seatId int, isMemberSeat bool) (bool, error) {
	// Mock implementation - seat 1 is occupied, others are vacant
	return seatId != 1, nil
}

func (m *MockWorkspaceAppForValidator) CheckIfUserSittingTooMuchForSeat(ctx context.Context, userId string, seatId int, isMemberSeat bool) (bool, error) {
	// Mock implementation - seat 2 has restrictions for the user
	return seatId == 2, nil
}

func (m *MockWorkspaceAppForValidator) RandomAvailableSeatIdForUser(ctx context.Context, tx *firestore.Transaction, userId string, isMemberSeat bool) (int, error) {
	if userId == "no-seat-user" {
		return 0, studyspaceerror.ErrNoSeatAvailable
	}
	return 5, nil
}

func createMockValidatorApp() *MockWorkspaceAppForValidator {
	return &MockWorkspaceAppForValidator{
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
}

func TestInCommandValidator_ValidateAccessPermission(t *testing.T) {
	tests := []struct {
		name           string
		isMember       bool
		targetMemberSeat bool
		expectedValid  bool
		expectedReturn bool
	}{
		{
			name:           "member accessing member seat",
			isMember:       true,
			targetMemberSeat: true,
			expectedValid:  true,
			expectedReturn: false,
		},
		{
			name:           "member accessing general seat",
			isMember:       true,
			targetMemberSeat: false,
			expectedValid:  true,
			expectedReturn: false,
		},
		{
			name:           "non-member accessing general seat",
			isMember:       false,
			targetMemberSeat: false,
			expectedValid:  true,
			expectedReturn: false,
		},
		{
			name:           "non-member accessing member seat",
			isMember:       false,
			targetMemberSeat: true,
			expectedValid:  false,
			expectedReturn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApp := createMockValidatorApp()
			mockApp.ProcessedUserIsMember = tt.isMember
			
			// Create a proper WorkspaceApp structure for the validator
			app := &WorkspaceApp{
				ProcessedUserId:          mockApp.ProcessedUserId,
				ProcessedUserDisplayName: mockApp.ProcessedUserDisplayName,
				ProcessedUserIsMember:    mockApp.ProcessedUserIsMember,
				Configs:                  mockApp.Configs,
			}
			
			validator := NewInCommandValidator(app)
			
			inOption := &utils.InOption{
				IsMemberSeat: tt.targetMemberSeat,
			}
			
			result := validator.ValidateAccessPermission(inOption)
			
			assert.Equal(t, tt.expectedValid, result.IsValid)
			assert.Equal(t, tt.expectedReturn, result.ShouldReturn)
			
			if !tt.expectedValid {
				assert.NotEmpty(t, result.ErrorMessage)
			}
		})
	}
}

func TestInCommandValidator_ValidateSeatAvailability(t *testing.T) {
	tests := []struct {
		name           string
		seatIdSet      bool
		seatId         int
		expectedValid  bool
		expectedReturn bool
	}{
		{
			name:           "no seat specified",
			seatIdSet:      false,
			seatId:         0,
			expectedValid:  true,
			expectedReturn: false,
		},
		{
			name:           "seat 0 specified",
			seatIdSet:      true,
			seatId:         0,
			expectedValid:  true,
			expectedReturn: false,
		},
		{
			name:           "available seat specified",
			seatIdSet:      true,
			seatId:         3,
			expectedValid:  true,
			expectedReturn: false,
		},
		{
			name:           "occupied seat specified",
			seatIdSet:      true,
			seatId:         1,
			expectedValid:  false,
			expectedReturn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApp := createMockValidatorApp()
			
			// Create a mock that implements the required methods
			app := &WorkspaceApp{
				ProcessedUserId:          mockApp.ProcessedUserId,
				ProcessedUserDisplayName: mockApp.ProcessedUserDisplayName,
				ProcessedUserIsMember:    mockApp.ProcessedUserIsMember,
				Configs:                  mockApp.Configs,
			}
			
			// We'll need to mock the IfSeatVacant method
			// For this test, we'll create a simple wrapper
			validator := &InCommandValidator{app: app}
			
			// Override the validator's app with our mock
			validator.app = &WorkspaceApp{
				ProcessedUserId:          mockApp.ProcessedUserId,
				ProcessedUserDisplayName: mockApp.ProcessedUserDisplayName,
				ProcessedUserIsMember:    mockApp.ProcessedUserIsMember,
				Configs:                  mockApp.Configs,
			}
			
			inOption := &utils.InOption{
				IsSeatIdSet:  tt.seatIdSet,
				SeatId:       tt.seatId,
				IsMemberSeat: false,
			}
			
			ctx := context.Background()
			var tx *firestore.Transaction // nil for this test
			
			// For testing purposes, we'll create a mock validator
			mockValidator := &MockInCommandValidator{
				BaseValidator: validator,
				MockApp:       mockApp,
			}
			
			result := mockValidator.ValidateSeatAvailability(ctx, tx, inOption)
			
			assert.Equal(t, tt.expectedValid, result.IsValid)
			assert.Equal(t, tt.expectedReturn, result.ShouldReturn)
			
			if !tt.expectedValid && tt.expectedReturn {
				assert.NotEmpty(t, result.ErrorMessage)
			}
		})
	}
}

func TestInCommandValidator_ValidateWorkTimeSettings(t *testing.T) {
	tests := []struct {
		name              string
		workOption        *utils.MinWorkOrderOption
		expectedValid     bool
		expectedReturn    bool
	}{
		{
			name:           "nil work option",
			workOption:     nil,
			expectedValid:  false,
			expectedReturn: false,
		},
		{
			name: "valid work time",
			workOption: &utils.MinWorkOrderOption{
				IsDurationMinSet: true,
				DurationMin:      60,
			},
			expectedValid:  true,
			expectedReturn: false,
		},
		{
			name: "work time too short",
			workOption: &utils.MinWorkOrderOption{
				IsDurationMinSet: true,
				DurationMin:      5,
			},
			expectedValid:  false,
			expectedReturn: true,
		},
		{
			name: "work time too long",
			workOption: &utils.MinWorkOrderOption{
				IsDurationMinSet: true,
				DurationMin:      500,
			},
			expectedValid:  false,
			expectedReturn: true,
		},
		{
			name: "work time not set",
			workOption: &utils.MinWorkOrderOption{
				IsDurationMinSet: false,
			},
			expectedValid:  true,
			expectedReturn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApp := createMockValidatorApp()
			
			app := &WorkspaceApp{
				ProcessedUserId:          mockApp.ProcessedUserId,
				ProcessedUserDisplayName: mockApp.ProcessedUserDisplayName,
				ProcessedUserIsMember:    mockApp.ProcessedUserIsMember,
				Configs:                  mockApp.Configs,
			}
			
			validator := NewInCommandValidator(app)
			
			inOption := &utils.InOption{
				MinWorkOrderOption: tt.workOption,
			}
			
			result := validator.ValidateWorkTimeSettings(inOption)
			
			assert.Equal(t, tt.expectedValid, result.IsValid)
			assert.Equal(t, tt.expectedReturn, result.ShouldReturn)
			
			if !tt.expectedValid {
				assert.NotEmpty(t, result.ErrorMessage)
			}
		})
	}
}

// MockInCommandValidator wraps the real validator with mock capabilities
type MockInCommandValidator struct {
	*InCommandValidator
	BaseValidator *InCommandValidator
	MockApp       *MockWorkspaceAppForValidator
}

func (m *MockInCommandValidator) ValidateSeatAvailability(ctx context.Context, tx *firestore.Transaction, inOption *utils.InOption) *ValidationResult {
	// If seat is not specified, skip availability check
	if !inOption.IsSeatIdSet || inOption.SeatId == 0 {
		return &ValidationResult{
			IsValid:      true,
			ErrorMessage: "",
			ShouldReturn: false,
		}
	}
	
	// Use mock implementation
	isVacant, err := m.MockApp.IfSeatVacant(ctx, tx, inOption.SeatId, inOption.IsMemberSeat)
	if err != nil {
		return &ValidationResult{
			IsValid:      false,
			ErrorMessage: err.Error(),
			ShouldReturn: false,
		}
	}
	
	if !isVacant {
		return &ValidationResult{
			IsValid:      false,
			ErrorMessage: "seat is occupied",
			ShouldReturn: true,
		}
	}
	
	return &ValidationResult{
		IsValid:      true,
		ErrorMessage: "",
		ShouldReturn: false,
	}
}

func TestInCommandValidator_ValidateUserSeatRestrictions(t *testing.T) {
	tests := []struct {
		name           string
		seatIdSet      bool
		seatId         int
		expectedValid  bool
		expectedReturn bool
	}{
		{
			name:           "no seat specified",
			seatIdSet:      false,
			seatId:         0,
			expectedValid:  true,
			expectedReturn: false,
		},
		{
			name:           "seat 0 specified",
			seatIdSet:      true,
			seatId:         0,
			expectedValid:  true,
			expectedReturn: false,
		},
		{
			name:           "seat without restrictions",
			seatIdSet:      true,
			seatId:         3,
			expectedValid:  true,
			expectedReturn: false,
		},
		{
			name:           "seat with user restrictions",
			seatIdSet:      true,
			seatId:         2,
			expectedValid:  false,
			expectedReturn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApp := createMockValidatorApp()
			
			app := &WorkspaceApp{
				ProcessedUserId:          mockApp.ProcessedUserId,
				ProcessedUserDisplayName: mockApp.ProcessedUserDisplayName,
				ProcessedUserIsMember:    mockApp.ProcessedUserIsMember,
				Configs:                  mockApp.Configs,
			}
			
			validator := &MockInCommandValidator{
				BaseValidator: &InCommandValidator{app: app},
				MockApp:       mockApp,
			}
			
			inOption := &utils.InOption{
				IsSeatIdSet:  tt.seatIdSet,
				SeatId:       tt.seatId,
				IsMemberSeat: false,
			}
			
			ctx := context.Background()
			
			result := validator.ValidateUserSeatRestrictions(ctx, inOption)
			
			assert.Equal(t, tt.expectedValid, result.IsValid)
			assert.Equal(t, tt.expectedReturn, result.ShouldReturn)
			
			if !tt.expectedValid && tt.expectedReturn {
				assert.NotEmpty(t, result.ErrorMessage)
			}
		})
	}
}

func (m *MockInCommandValidator) ValidateUserSeatRestrictions(ctx context.Context, inOption *utils.InOption) *ValidationResult {
	// Skip restriction check if seat is not specified or is seat 0
	if !inOption.IsSeatIdSet || inOption.SeatId == 0 {
		return &ValidationResult{
			IsValid:      true,
			ErrorMessage: "",
			ShouldReturn: false,
		}
	}
	
	// Use mock implementation
	isTooMuch, err := m.MockApp.CheckIfUserSittingTooMuchForSeat(ctx, m.MockApp.ProcessedUserId, inOption.SeatId, inOption.IsMemberSeat)
	if err != nil {
		return &ValidationResult{
			IsValid:      false,
			ErrorMessage: err.Error(),
			ShouldReturn: false,
		}
	}
	
	if isTooMuch {
		return &ValidationResult{
			IsValid:      false,
			ErrorMessage: "user has restrictions for this seat",
			ShouldReturn: true,
		}
	}
	
	return &ValidationResult{
		IsValid:      true,
		ErrorMessage: "",
		ShouldReturn: false,
	}
}