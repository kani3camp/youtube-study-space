package workspaceapp

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"app.modules/core/repository"
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
)

// RoomEntryOrchestrator coordinates the entire room entry process
type RoomEntryOrchestrator struct {
	app                *WorkspaceApp
	validator          *InCommandValidator
	seatService        *SeatAssignmentService
	userStateService   *UserStateService
}

// NewRoomEntryOrchestrator creates a new instance of RoomEntryOrchestrator
func NewRoomEntryOrchestrator(app *WorkspaceApp) *RoomEntryOrchestrator {
	return &RoomEntryOrchestrator{
		app:                app,
		validator:          NewInCommandValidator(app),
		seatService:        NewSeatAssignmentService(app),
		userStateService:   NewUserStateService(app),
	}
}

// RoomEntryRequest represents a comprehensive request for room entry
type RoomEntryRequest struct {
	UserId       string
	InOption     *utils.InOption
	IsMemberSeat bool
}

// RoomEntryResult represents the complete result of room entry processing
type RoomEntryResult struct {
	Success         bool
	Message         string
	Error           error
	AssignedSeatId  int
	UserSettings    *UserSettings
	ShouldReturn    bool
}

// ProcessRoomEntry handles the complete room entry workflow
func (r *RoomEntryOrchestrator) ProcessRoomEntry(ctx context.Context, request *RoomEntryRequest) *RoomEntryResult {
	// Execute the room entry process within a Firestore transaction
	var result *RoomEntryResult
	var resultErr error
	
	txErr := r.app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		var err error
		result, err = r.processRoomEntryInTransaction(ctx, tx, request)
		resultErr = err
		return err
	})
	
	if txErr != nil {
		return &RoomEntryResult{
			Success: false,
			Message: "Transaction failed",
			Error:   txErr,
		}
	}
	
	if resultErr != nil {
		return &RoomEntryResult{
			Success: false,
			Message: "Room entry processing failed",
			Error:   resultErr,
		}
	}
	
	return result
}

// processRoomEntryInTransaction handles the room entry process within a transaction
func (r *RoomEntryOrchestrator) processRoomEntryInTransaction(ctx context.Context, tx *firestore.Transaction, request *RoomEntryRequest) (*RoomEntryResult, error) {
	// Phase 1: Comprehensive Validation
	validationResult := r.validator.ValidateAll(ctx, tx, request.InOption)
	if !validationResult.IsValid {
		return &RoomEntryResult{
			Success:      false,
			Message:      validationResult.ErrorMessage,
			Error:        nil,
			ShouldReturn: validationResult.ShouldReturn,
		}, nil
	}

	// Phase 2: Prepare User Settings
	entryContext := &EntryContext{
		UserId:       request.UserId,
		InOption:     request.InOption,
		IsMemberSeat: request.IsMemberSeat,
	}

	userSettings, err := r.userStateService.PrepareUserSettings(ctx, tx, entryContext)
	if err != nil {
		return &RoomEntryResult{
			Success: false,
			Message: "Failed to prepare user settings",
			Error:   err,
		}, err
	}

	// Phase 3: Update work duration in option if needed
	r.userStateService.UpdateWorkDurationInOption(request.InOption, userSettings)

	// Phase 4: Validate work time settings
	workTimeValidation := r.validator.ValidateWorkTimeSettings(request.InOption)
	if !workTimeValidation.IsValid {
		return &RoomEntryResult{
			Success:      false,
			Message:      workTimeValidation.ErrorMessage,
			Error:        nil,
			ShouldReturn: workTimeValidation.ShouldReturn,
		}, nil
	}

	// Phase 5: Check system capacity if no specific seat is requested
	capacityValidation := r.validator.ValidateSystemCapacity(ctx, tx, request.InOption)
	if !capacityValidation.IsValid {
		return &RoomEntryResult{
			Success:      false,
			Message:      capacityValidation.ErrorMessage,
			Error:        nil,
			ShouldReturn: capacityValidation.ShouldReturn,
		}, nil
	}

	// Phase 6: Assign Seat
	seatRequest := &SeatAssignmentRequest{
		UserId:       request.UserId,
		InOption:     request.InOption,
		IsMemberSeat: request.IsMemberSeat,
	}

	seatResult := r.seatService.AssignSeat(ctx, tx, seatRequest)
	if !seatResult.Assigned {
		return &RoomEntryResult{
			Success: false,
			Message: seatResult.Message,
			Error:   seatResult.Error,
		}, seatResult.Error
	}

	// Phase 7: Prepare entry time information
	entryTime, exitTime := r.userStateService.PrepareEntryTime(userSettings)

	// Phase 8: Execute the actual seat assignment in the database
	err = r.executeSeatAssignment(ctx, tx, request, userSettings, seatResult, entryTime, exitTime)
	if err != nil {
		return &RoomEntryResult{
			Success: false,
			Message: "Failed to execute seat assignment",
			Error:   err,
		}, err
	}

	// Success!
	return &RoomEntryResult{
		Success:        true,
		Message:        "Room entry completed successfully",
		Error:          nil,
		AssignedSeatId: seatResult.SeatId,
		UserSettings:   userSettings,
		ShouldReturn:   false,
	}, nil
}

// executeSeatAssignment performs the actual database operations for seat assignment
func (r *RoomEntryOrchestrator) executeSeatAssignment(ctx context.Context, tx *firestore.Transaction, request *RoomEntryRequest, userSettings *UserSettings, seatResult *SeatAssignmentResult, entryTime, exitTime time.Time) error {
	// Determine menu code for the seat
	var menuCode string
	if userSettings.OrderInfo != nil && userSettings.OrderInfo.HasOrder {
		menuCodeInt, parseErr := strconv.Atoi(userSettings.OrderInfo.MenuCode)
		if parseErr != nil {
			return fmt.Errorf("failed to parse menu code: %w", parseErr)
		}
		targetMenuItem, err := r.app.GetMenuItemByNumber(menuCodeInt)
		if err != nil {
			return fmt.Errorf("failed to get menu item: %w", err)
		}
		menuCode = targetMenuItem.Code
		
		// Create order history if order is set
		orderHistoryDoc := repository.OrderHistoryDoc{
			UserId:       r.app.ProcessedUserId,
			MenuCode:     targetMenuItem.Code,
			SeatId:       seatResult.SeatId,
			IsMemberSeat: request.IsMemberSeat,
			OrderedAt:    entryTime,
		}
		if err := r.app.Repository.CreateOrderHistoryDoc(ctx, tx, orderHistoryDoc); err != nil {
			return fmt.Errorf("failed to create order history: %w", err)
		}
	}
	
	// Extract work name from the InOption
	workName := ""
	if request.InOption.MinWorkOrderOption != nil && request.InOption.MinWorkOrderOption.IsWorkNameSet {
		workName = request.InOption.MinWorkOrderOption.WorkName
	}
	
	// Call the existing enterRoom method to handle the actual database operations
	_, err := r.app.enterRoom(
		ctx,
		tx,
		r.app.ProcessedUserId,
		r.app.ProcessedUserDisplayName,
		r.app.ProcessedUserProfileImageUrl,
		seatResult.SeatId,
		request.IsMemberSeat,
		workName,
		"", // breakWorkName
		userSettings.WorkDurationMin,
		userSettings.SeatAppearance,
		menuCode,
		repository.WorkState,
		userSettings.UserDoc.IsContinuousActive,
		time.Time{}, // recentBreakStartedAt
		time.Time{}, // recentBreakUntil
		entryTime,
	)
	
	if err != nil {
		return fmt.Errorf("failed to enter room: %w", err)
	}
	
	return nil
}

// ValidatePreConditions performs lightweight validation before starting the transaction
func (r *RoomEntryOrchestrator) ValidatePreConditions(request *RoomEntryRequest) *RoomEntryResult {
	// Quick validation that doesn't require database access
	
	// Check access permission
	accessResult := r.validator.ValidateAccessPermission(request.InOption)
	if !accessResult.IsValid {
		return &RoomEntryResult{
			Success:      false,
			Message:      accessResult.ErrorMessage,
			Error:        nil,
			ShouldReturn: accessResult.ShouldReturn,
		}
	}

	// Check if user can enter the requested room type
	canEnter, reason, err := r.userStateService.CanUserEnterRoom(context.Background(), &EntryContext{
		UserId:       request.UserId,
		InOption:     request.InOption,
		IsMemberSeat: request.IsMemberSeat,
	})
	
	if err != nil {
		return &RoomEntryResult{
			Success: false,
			Message: "Failed to check room entry permissions",
			Error:   err,
		}
	}
	
	if !canEnter {
		return &RoomEntryResult{
			Success:      false,
			Message:      reason,
			Error:        nil,
			ShouldReturn: true,
		}
	}

	return &RoomEntryResult{
		Success: true,
		Message: "Pre-conditions validated successfully",
	}
}

// GetRoomStatus provides current room status information
func (r *RoomEntryOrchestrator) GetRoomStatus(ctx context.Context) (*RoomStatus, error) {
	// This would aggregate status from multiple sources
	return &RoomStatus{
		GeneralSeatsAvailable: 0, // TODO: Implement actual counting
		MemberSeatsAvailable:  0, // TODO: Implement actual counting
		TotalUsers:           0, // TODO: Implement actual counting
	}, nil
}

// RoomStatus represents the current status of the room
type RoomStatus struct {
	GeneralSeatsAvailable int
	MemberSeatsAvailable  int
	TotalUsers           int
}