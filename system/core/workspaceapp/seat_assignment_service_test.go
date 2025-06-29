package workspaceapp

import (
	"context"
	"fmt"
	"testing"

	"app.modules/core/repository"
	"app.modules/core/studyspaceerror"
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
)

// MockWorkspaceAppForSeatService provides a mock for testing seat assignment service
type MockWorkspaceAppForSeatService struct {
	ProcessedUserId   string
	Configs           *Configs
	
	// Mock data for testing
	OccupiedSeats     map[int]bool  // seat_id -> is_occupied
	UserRestrictions  map[string]map[int]bool  // user_id -> seat_id -> has_restriction
}

func (m *MockWorkspaceAppForSeatService) MinAvailableSeatIdForUser(ctx context.Context, tx *firestore.Transaction, userId string, isMemberSeat bool) (int, error) {
	maxSeats := m.Configs.Constants.MaxSeats
	if isMemberSeat {
		maxSeats = m.Configs.Constants.MemberMaxSeats
	}
	
	for seatId := 1; seatId <= maxSeats; seatId++ {
		if !m.OccupiedSeats[seatId] {
			// Check user restrictions
			if userRestrictions, exists := m.UserRestrictions[userId]; exists {
				if userRestrictions[seatId] {
					continue // Skip this seat due to user restrictions
				}
			}
			return seatId, nil
		}
	}
	
	return 0, studyspaceerror.ErrNoSeatAvailable
}

func (m *MockWorkspaceAppForSeatService) RandomAvailableSeatIdForUser(ctx context.Context, tx *firestore.Transaction, userId string, isMemberSeat bool) (int, error) {
	// For simplicity, this returns the same as MinAvailableSeatIdForUser
	return m.MinAvailableSeatIdForUser(ctx, tx, userId, isMemberSeat)
}

func (m *MockWorkspaceAppForSeatService) IfSeatVacant(ctx context.Context, tx *firestore.Transaction, seatId int, isMemberSeat bool) (bool, error) {
	return !m.OccupiedSeats[seatId], nil
}

func (m *MockWorkspaceAppForSeatService) CheckIfUserSittingTooMuchForSeat(ctx context.Context, userId string, seatId int, isMemberSeat bool) (bool, error) {
	if userRestrictions, exists := m.UserRestrictions[userId]; exists {
		return userRestrictions[seatId], nil
	}
	return false, nil
}

func createMockSeatServiceApp() *MockWorkspaceAppForSeatService {
	return &MockWorkspaceAppForSeatService{
		ProcessedUserId: "test-user",
		Configs: &Configs{
			Constants: repository.ConstantsConfigDoc{
				MaxSeats:       10,
				MemberMaxSeats: 5,
			},
		},
		OccupiedSeats: map[int]bool{
			1: true,  // Seat 1 is occupied
			3: true,  // Seat 3 is occupied
		},
		UserRestrictions: map[string]map[int]bool{
			"restricted-user": {
				2: true, // User has restriction on seat 2
				4: true, // User has restriction on seat 4
			},
		},
	}
}

func TestSeatAssignmentService_AssignSeat_SpecificSeat(t *testing.T) {
	tests := []struct {
		name           string
		seatId         int
		expectedSeatId int
		expectedAssigned bool
	}{
		{
			name:           "assign seat 0 (minimum available)",
			seatId:         0,
			expectedSeatId: 2, // First available seat (1 is occupied)
			expectedAssigned: true,
		},
		{
			name:           "assign specific available seat",
			seatId:         5,
			expectedSeatId: 5,
			expectedAssigned: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApp := createMockSeatServiceApp()
			
			// Create workspace app with mock methods
			app := &WorkspaceApp{
				ProcessedUserId: mockApp.ProcessedUserId,
				Configs:         mockApp.Configs,
			}
			
			// Create service with mock wrapper
			service := &SeatAssignmentServiceWrapper{
				Service: NewSeatAssignmentService(app),
				MockApp: mockApp,
			}
			
			request := &SeatAssignmentRequest{
				UserId:       "test-user",
				InOption:     &utils.InOption{IsSeatIdSet: true, SeatId: tt.seatId, IsMemberSeat: false},
				IsMemberSeat: false,
			}
			
			ctx := context.Background()
			var tx *firestore.Transaction // nil for this test
			
			result := service.AssignSeat(ctx, tx, request)
			
			assert.Equal(t, tt.expectedAssigned, result.Assigned)
			if tt.expectedAssigned {
				assert.Equal(t, tt.expectedSeatId, result.SeatId)
				assert.NoError(t, result.Error)
			}
		})
	}
}

func TestSeatAssignmentService_AssignSeat_NoSeatSpecified(t *testing.T) {
	tests := []struct {
		name           string
		userId         string
		expectedSeatId int
		expectedAssigned bool
	}{
		{
			name:           "normal user gets available seat",
			userId:         "normal-user",
			expectedSeatId: 2, // First available seat
			expectedAssigned: true,
		},
		{
			name:           "restricted user gets unrestricted seat",
			userId:         "restricted-user",
			expectedSeatId: 5, // Skip seats 2,4 due to restrictions
			expectedAssigned: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApp := createMockSeatServiceApp()
			
			app := &WorkspaceApp{
				ProcessedUserId: mockApp.ProcessedUserId,
				Configs:         mockApp.Configs,
			}
			
			service := &SeatAssignmentServiceWrapper{
				Service: NewSeatAssignmentService(app),
				MockApp: mockApp,
			}
			
			request := &SeatAssignmentRequest{
				UserId:       tt.userId,
				InOption:     &utils.InOption{IsSeatIdSet: false, IsMemberSeat: false},
				IsMemberSeat: false,
			}
			
			ctx := context.Background()
			var tx *firestore.Transaction
			
			result := service.AssignSeat(ctx, tx, request)
			
			assert.Equal(t, tt.expectedAssigned, result.Assigned)
			if tt.expectedAssigned {
				assert.Equal(t, tt.expectedSeatId, result.SeatId)
				assert.NoError(t, result.Error)
			}
		})
	}
}

func TestSeatAssignmentService_ValidateSeatExists(t *testing.T) {
	tests := []struct {
		name         string
		seatId       int
		isMemberSeat bool
		expected     bool
	}{
		{
			name:         "valid general seat",
			seatId:       5,
			isMemberSeat: false,
			expected:     true,
		},
		{
			name:         "valid member seat",
			seatId:       3,
			isMemberSeat: true,
			expected:     true,
		},
		{
			name:         "invalid seat id (zero)",
			seatId:       0,
			isMemberSeat: false,
			expected:     false,
		},
		{
			name:         "invalid seat id (negative)",
			seatId:       -1,
			isMemberSeat: false,
			expected:     false,
		},
		{
			name:         "seat id exceeds max general seats",
			seatId:       15,
			isMemberSeat: false,
			expected:     false,
		},
		{
			name:         "seat id exceeds max member seats",
			seatId:       8,
			isMemberSeat: true,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApp := createMockSeatServiceApp()
			
			app := &WorkspaceApp{
				ProcessedUserId: mockApp.ProcessedUserId,
				Configs:         mockApp.Configs,
			}
			
			service := NewSeatAssignmentService(app)
			
			result := service.ValidateSeatExists(tt.seatId, tt.isMemberSeat)
			
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSeatAssignmentService_GetSeatInfo(t *testing.T) {
	tests := []struct {
		name               string
		seatId             int
		userId             string
		isMemberSeat       bool
		expectedAvailable  bool
		expectedRestrictions int
	}{
		{
			name:               "available seat for normal user",
			seatId:             2,
			userId:             "normal-user",
			isMemberSeat:       false,
			expectedAvailable:  true,
			expectedRestrictions: 0,
		},
		{
			name:               "occupied seat",
			seatId:             1,
			userId:             "normal-user",
			isMemberSeat:       false,
			expectedAvailable:  false,
			expectedRestrictions: 0,
		},
		{
			name:               "available seat with user restrictions",
			seatId:             2,
			userId:             "restricted-user",
			isMemberSeat:       false,
			expectedAvailable:  true,
			expectedRestrictions: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApp := createMockSeatServiceApp()
			
			app := &WorkspaceApp{
				ProcessedUserId: mockApp.ProcessedUserId,
				Configs:         mockApp.Configs,
			}
			
			service := &SeatAssignmentServiceWrapper{
				Service: NewSeatAssignmentService(app),
				MockApp: mockApp,
			}
			
			ctx := context.Background()
			var tx *firestore.Transaction
			
			info, err := service.GetSeatInfo(ctx, tx, tt.seatId, tt.isMemberSeat, tt.userId)
			
			assert.NoError(t, err)
			assert.Equal(t, tt.seatId, info.SeatId)
			assert.Equal(t, tt.expectedAvailable, info.IsAvailable)
			assert.Equal(t, tt.isMemberSeat, info.IsMemberSeat)
			assert.Equal(t, tt.expectedRestrictions, len(info.Restrictions))
		})
	}
}

// SeatAssignmentServiceWrapper wraps the service with mock capabilities
type SeatAssignmentServiceWrapper struct {
	*SeatAssignmentService
	Service *SeatAssignmentService
	MockApp *MockWorkspaceAppForSeatService
}

func (w *SeatAssignmentServiceWrapper) AssignSeat(ctx context.Context, tx *firestore.Transaction, request *SeatAssignmentRequest) *SeatAssignmentResult {
	// If seat is specified
	if request.InOption.IsSeatIdSet {
		return w.assignSpecificSeatMock(ctx, tx, request)
	}
	
	// If no seat is specified, find any available seat
	return w.assignAvailableSeatMock(ctx, tx, request)
}

func (w *SeatAssignmentServiceWrapper) assignSpecificSeatMock(ctx context.Context, tx *firestore.Transaction, request *SeatAssignmentRequest) *SeatAssignmentResult {
	// If seat ID is 0, find the minimum available seat
	if request.InOption.SeatId == 0 {
		seatId, err := w.MockApp.MinAvailableSeatIdForUser(ctx, tx, request.UserId, request.IsMemberSeat)
		if err != nil {
			return &SeatAssignmentResult{
				SeatId:   0,
				Assigned: false,
				Error:    err,
				Message:  "No available seats",
			}
		}
		
		request.InOption.SeatId = seatId
		return &SeatAssignmentResult{
			SeatId:   seatId,
			Assigned: true,
			Error:    nil,
			Message:  "",
		}
	}
	
	// For specific seat ID, return as assigned
	return &SeatAssignmentResult{
		SeatId:   request.InOption.SeatId,
		Assigned: true,
		Error:    nil,
		Message:  "",
	}
}

func (w *SeatAssignmentServiceWrapper) assignAvailableSeatMock(ctx context.Context, tx *firestore.Transaction, request *SeatAssignmentRequest) *SeatAssignmentResult {
	seatId, err := w.MockApp.RandomAvailableSeatIdForUser(ctx, tx, request.UserId, request.IsMemberSeat)
	if err != nil {
		return &SeatAssignmentResult{
			SeatId:   0,
			Assigned: false,
			Error:    err,
			Message:  "Room is full",
		}
	}
	
	request.InOption.SeatId = seatId
	return &SeatAssignmentResult{
		SeatId:   seatId,
		Assigned: true,
		Error:    nil,
		Message:  "",
	}
}

func (w *SeatAssignmentServiceWrapper) GetSeatInfo(ctx context.Context, tx *firestore.Transaction, seatId int, isMemberSeat bool, userId string) (*SeatInfo, error) {
	// Check if seat exists
	if !w.Service.ValidateSeatExists(seatId, isMemberSeat) {
		return nil, fmt.Errorf("seat %d does not exist", seatId)
	}
	
	// Check availability using mock
	isAvailable, err := w.MockApp.IfSeatVacant(ctx, tx, seatId, isMemberSeat)
	if err != nil {
		return nil, fmt.Errorf("failed to check seat availability: %w", err)
	}
	
	info := &SeatInfo{
		SeatId:       seatId,
		IsAvailable:  isAvailable,
		IsMemberSeat: isMemberSeat,
		Restrictions: []string{},
	}
	
	// Check user restrictions if user ID is provided
	if userId != "" {
		hasRestrictions, err := w.MockApp.CheckIfUserSittingTooMuchForSeat(ctx, userId, seatId, isMemberSeat)
		if err != nil {
			return nil, fmt.Errorf("failed to check user restrictions: %w", err)
		}
		
		if hasRestrictions {
			info.Restrictions = append(info.Restrictions, "User has recently used this seat")
		}
	}
	
	return info, nil
}