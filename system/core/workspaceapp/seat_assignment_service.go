package workspaceapp

import (
	"context"
	"fmt"

	"app.modules/core/studyspaceerror"
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
	"github.com/pkg/errors"
)

// SeatAssignmentService handles seat assignment and availability logic
type SeatAssignmentService struct {
	app *WorkspaceApp
}

// NewSeatAssignmentService creates a new instance of SeatAssignmentService
func NewSeatAssignmentService(app *WorkspaceApp) *SeatAssignmentService {
	return &SeatAssignmentService{
		app: app,
	}
}

// SeatAssignmentRequest represents a request for seat assignment
type SeatAssignmentRequest struct {
	UserId       string
	InOption     *utils.InOption
	PreferredId  int  // For specific seat requests
	IsMemberSeat bool
}

// SeatAssignmentResult represents the result of seat assignment
type SeatAssignmentResult struct {
	SeatId    int
	Assigned  bool
	Error     error
	Message   string
}

// AssignSeat handles the complete seat assignment process
func (s *SeatAssignmentService) AssignSeat(ctx context.Context, tx *firestore.Transaction, request *SeatAssignmentRequest) *SeatAssignmentResult {
	// If seat is specified
	if request.InOption.IsSeatIdSet {
		return s.assignSpecificSeat(ctx, tx, request)
	}
	
	// If no seat is specified, find any available seat
	return s.assignAvailableSeat(ctx, tx, request)
}

// assignSpecificSeat handles assignment when a specific seat is requested
func (s *SeatAssignmentService) assignSpecificSeat(ctx context.Context, tx *firestore.Transaction, request *SeatAssignmentRequest) *SeatAssignmentResult {
	// If seat ID is 0, find the minimum available seat
	if request.InOption.SeatId == 0 {
		return s.assignMinimumAvailableSeat(ctx, tx, request)
	}
	
	// For specific seat ID, the validation should already be done
	// We just return the assigned seat
	return &SeatAssignmentResult{
		SeatId:   request.InOption.SeatId,
		Assigned: true,
		Error:    nil,
		Message:  "",
	}
}

// assignMinimumAvailableSeat finds and assigns the minimum available seat ID
func (s *SeatAssignmentService) assignMinimumAvailableSeat(ctx context.Context, tx *firestore.Transaction, request *SeatAssignmentRequest) *SeatAssignmentResult {
	seatId, err := s.app.MinAvailableSeatIdForUser(ctx, tx, request.UserId, request.IsMemberSeat)
	if err != nil {
		if errors.Is(err, studyspaceerror.ErrNoSeatAvailable) {
			return &SeatAssignmentResult{
				SeatId:   0,
				Assigned: false,
				Error:    err,
				Message:  "No available seats",
			}
		}
		return &SeatAssignmentResult{
			SeatId:   0,
			Assigned: false,
			Error:    fmt.Errorf("failed to find minimum available seat: %w", err),
			Message:  "Failed to assign seat",
		}
	}
	
	// Update the inOption with the assigned seat
	request.InOption.SeatId = seatId
	
	return &SeatAssignmentResult{
		SeatId:   seatId,
		Assigned: true,
		Error:    nil,
		Message:  "",
	}
}

// assignAvailableSeat finds and assigns any available seat
func (s *SeatAssignmentService) assignAvailableSeat(ctx context.Context, tx *firestore.Transaction, request *SeatAssignmentRequest) *SeatAssignmentResult {
	seatId, err := s.app.RandomAvailableSeatIdForUser(ctx, tx, request.UserId, request.IsMemberSeat)
	if err != nil {
		if errors.Is(err, studyspaceerror.ErrNoSeatAvailable) {
			return &SeatAssignmentResult{
				SeatId:   0,
				Assigned: false,
				Error:    err,
				Message:  "Room is full",
			}
		}
		return &SeatAssignmentResult{
			SeatId:   0,
			Assigned: false,
			Error:    fmt.Errorf("failed to find available seat: %w", err),
			Message:  "Failed to assign seat",
		}
	}
	
	// Update the inOption with the assigned seat
	request.InOption.SeatId = seatId
	
	return &SeatAssignmentResult{
		SeatId:   seatId,
		Assigned: true,
		Error:    nil,
		Message:  "",
	}
}

// CheckSeatAvailability verifies if a specific seat is available
func (s *SeatAssignmentService) CheckSeatAvailability(ctx context.Context, tx *firestore.Transaction, seatId int, isMemberSeat bool) (bool, error) {
	return s.app.IfSeatVacant(ctx, tx, seatId, isMemberSeat)
}

// CheckUserSeatRestrictions verifies if a user has restrictions for a specific seat
func (s *SeatAssignmentService) CheckUserSeatRestrictions(ctx context.Context, userId string, seatId int, isMemberSeat bool) (bool, error) {
	return s.app.CheckIfUserSittingTooMuchForSeat(ctx, userId, seatId, isMemberSeat)
}

// GetAvailableSeatsCount returns the number of available seats
func (s *SeatAssignmentService) GetAvailableSeatsCount(ctx context.Context, tx *firestore.Transaction, isMemberSeat bool) (int, error) {
	// This would require implementing a method to count available seats
	// For now, we'll return a placeholder implementation
	maxSeats := s.app.Configs.Constants.MaxSeats
	if isMemberSeat {
		maxSeats = s.app.Configs.Constants.MemberMaxSeats
	}
	
	// TODO: Implement actual seat counting logic
	// This is a simplified implementation
	return maxSeats, nil
}

// ValidateSeatExists checks if a seat ID exists in the system
func (s *SeatAssignmentService) ValidateSeatExists(seatId int, isMemberSeat bool) bool {
	if seatId <= 0 {
		return false
	}
	
	maxSeats := s.app.Configs.Constants.MaxSeats
	if isMemberSeat {
		maxSeats = s.app.Configs.Constants.MemberMaxSeats
	}
	
	return seatId <= maxSeats
}

// SeatInfo represents information about a seat
type SeatInfo struct {
	SeatId       int
	IsAvailable  bool
	IsMemberSeat bool
	Restrictions []string
}

// GetSeatInfo returns comprehensive information about a seat
func (s *SeatAssignmentService) GetSeatInfo(ctx context.Context, tx *firestore.Transaction, seatId int, isMemberSeat bool, userId string) (*SeatInfo, error) {
	// Check if seat exists
	if !s.ValidateSeatExists(seatId, isMemberSeat) {
		return nil, fmt.Errorf("seat %d does not exist", seatId)
	}
	
	// Check availability
	isAvailable, err := s.CheckSeatAvailability(ctx, tx, seatId, isMemberSeat)
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
		hasRestrictions, err := s.CheckUserSeatRestrictions(ctx, userId, seatId, isMemberSeat)
		if err != nil {
			return nil, fmt.Errorf("failed to check user restrictions: %w", err)
		}
		
		if hasRestrictions {
			info.Restrictions = append(info.Restrictions, "User has recently used this seat")
		}
	}
	
	return info, nil
}

// GetRecommendedSeat returns a recommended seat for the user
func (s *SeatAssignmentService) GetRecommendedSeat(ctx context.Context, tx *firestore.Transaction, userId string, isMemberSeat bool) (*SeatAssignmentResult, error) {
	// Try to find the minimum available seat first (better for consistency)
	result := s.assignMinimumAvailableSeat(ctx, tx, &SeatAssignmentRequest{
		UserId:       userId,
		InOption:     &utils.InOption{IsSeatIdSet: true, SeatId: 0, IsMemberSeat: isMemberSeat},
		IsMemberSeat: isMemberSeat,
	})
	
	if result.Assigned {
		return result, nil
	}
	
	// If minimum seat assignment failed, try random assignment
	randomResult := s.assignAvailableSeat(ctx, tx, &SeatAssignmentRequest{
		UserId:       userId,
		InOption:     &utils.InOption{IsSeatIdSet: false, IsMemberSeat: isMemberSeat},
		IsMemberSeat: isMemberSeat,
	})
	
	if randomResult.Error != nil {
		return nil, randomResult.Error
	}
	
	return randomResult, nil
}