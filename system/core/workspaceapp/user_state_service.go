package workspaceapp

import (
	"context"
	"fmt"
	"time"

	"app.modules/core/repository"
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
)

// UserStateService handles user state management and configuration
type UserStateService struct {
	app *WorkspaceApp
}

// NewUserStateService creates a new instance of UserStateService
func NewUserStateService(app *WorkspaceApp) *UserStateService {
	return &UserStateService{
		app: app,
	}
}

// UserRoomStatus represents the current room status of a user
type UserRoomStatus struct {
	IsInMemberRoom  bool
	IsInGeneralRoom bool
	IsInAnyRoom     bool
	CurrentSeatId   int
	IsMemberSeat    bool
}

// UserSettings represents user configuration for room entry
type UserSettings struct {
	UserDoc          repository.UserDoc
	WorkDurationMin  int
	SeatAppearance   repository.SeatAppearance
	OrderInfo        *OrderInfo
}

// OrderInfo represents menu order information
type OrderInfo struct {
	HasOrder     bool
	MenuCode     string
	OrderMessage string
}

// EntryContext represents the context for room entry
type EntryContext struct {
	UserId       string
	InOption     *utils.InOption
	IsMemberSeat bool
}

// GetUserRoomStatus retrieves the current room status of a user
func (u *UserStateService) GetUserRoomStatus(ctx context.Context, userId string) (*UserRoomStatus, error) {
	isInMemberRoom, isInGeneralRoom, err := u.app.IsUserInRoom(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to check user room status: %w", err)
	}

	status := &UserRoomStatus{
		IsInMemberRoom:  isInMemberRoom,
		IsInGeneralRoom: isInGeneralRoom,
		IsInAnyRoom:     isInMemberRoom || isInGeneralRoom,
		CurrentSeatId:   0,
		IsMemberSeat:    false,
	}

	// If user is in a room, get the current seat information
	if status.IsInAnyRoom {
		// TODO: Implement method to get current seat ID
		// This would require additional logic to determine which seat the user is currently in
	}

	return status, nil
}

// PrepareUserSettings prepares all necessary user settings for room entry
func (u *UserStateService) PrepareUserSettings(ctx context.Context, tx *firestore.Transaction, entryCtx *EntryContext) (*UserSettings, error) {
	// Read user document
	userDoc, err := u.app.Repository.ReadUser(ctx, tx, entryCtx.UserId)
	if err != nil {
		return nil, fmt.Errorf("failed to read user document: %w", err)
	}

	// Determine work duration
	workDurationMin, err := u.determineWorkDuration(entryCtx.InOption, userDoc)
	if err != nil {
		return nil, fmt.Errorf("failed to determine work duration: %w", err)
	}

	// Get seat appearance
	seatAppearance, err := u.getSeatAppearance(ctx, tx, entryCtx.UserId)
	if err != nil {
		return nil, fmt.Errorf("failed to get seat appearance: %w", err)
	}

	// Prepare order information
	orderInfo, err := u.prepareOrderInfo(entryCtx.InOption)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare order info: %w", err)
	}

	settings := &UserSettings{
		UserDoc:         userDoc,
		WorkDurationMin: workDurationMin,
		SeatAppearance:  seatAppearance,
		OrderInfo:       orderInfo,
	}

	return settings, nil
}

// determineWorkDuration determines the work duration based on user preferences and options
func (u *UserStateService) determineWorkDuration(inOption *utils.InOption, userDoc repository.UserDoc) (int, error) {
	// If duration is explicitly set in the command, use it
	if inOption.MinWorkOrderOption != nil && inOption.MinWorkOrderOption.IsDurationMinSet {
		return inOption.MinWorkOrderOption.DurationMin, nil
	}

	// If user has a default study time set, use it
	if userDoc.DefaultStudyMin > 0 {
		return userDoc.DefaultStudyMin, nil
	}

	// Otherwise, use system default
	return u.app.Configs.Constants.DefaultWorkTimeMin, nil
}

// getSeatAppearance retrieves the seat appearance for the user
func (u *UserStateService) getSeatAppearance(ctx context.Context, tx *firestore.Transaction, userId string) (repository.SeatAppearance, error) {
	return u.app.GetUserRealtimeSeatAppearance(ctx, tx, userId)
}

// prepareOrderInfo prepares menu order information if applicable
func (u *UserStateService) prepareOrderInfo(inOption *utils.InOption) (*OrderInfo, error) {
	orderInfo := &OrderInfo{
		HasOrder:     false,
		MenuCode:     "",
		OrderMessage: "",
	}

	// Check if there's an order in the command
	if inOption.MinWorkOrderOption != nil && inOption.MinWorkOrderOption.IsOrderSet {
		orderInfo.HasOrder = true
		orderInfo.MenuCode = fmt.Sprintf("%d", inOption.MinWorkOrderOption.OrderNum)
		// orderMessage will be built later by the message service
	}

	return orderInfo, nil
}

// ValidateUserState validates the user's current state for room entry
func (u *UserStateService) ValidateUserState(ctx context.Context, entryCtx *EntryContext) error {
	// Check if user is registered
	isRegistered, err := u.app.IfUserRegistered(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to check user registration: %w", err)
	}

	if !isRegistered {
		return fmt.Errorf("user is not registered")
	}

	return nil
}

// UpdateWorkDurationInOption updates the work duration in the InOption based on user settings
func (u *UserStateService) UpdateWorkDurationInOption(inOption *utils.InOption, settings *UserSettings) {
	if inOption.MinWorkOrderOption == nil {
		inOption.MinWorkOrderOption = &utils.MinWorkOrderOption{}
	}

	if !inOption.MinWorkOrderOption.IsDurationMinSet {
		inOption.MinWorkOrderOption.DurationMin = settings.WorkDurationMin
		inOption.MinWorkOrderOption.IsDurationMinSet = true
	}
}

// UserActivityInfo represents user activity information
type UserActivityInfo struct {
	TotalStudyTime   int
	DailyStudyTime   int
	RankPoints       int
	IsRankVisible    bool
	LastEntered      *time.Time
	LastExited       *time.Time
	RegistrationDate *time.Time
}

// GetUserActivityInfo retrieves comprehensive user activity information
func (u *UserStateService) GetUserActivityInfo(ctx context.Context, tx *firestore.Transaction, userId string) (*UserActivityInfo, error) {
	userDoc, err := u.app.Repository.ReadUser(ctx, tx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to read user document: %w", err)
	}

	info := &UserActivityInfo{
		TotalStudyTime: userDoc.TotalStudySec,
		DailyStudyTime: userDoc.DailyTotalStudySec,
		RankPoints:     userDoc.RankPoint,
		IsRankVisible:  userDoc.RankVisible,
	}

	// Set time pointers if they're not zero values
	if !userDoc.LastEntered.IsZero() {
		info.LastEntered = &userDoc.LastEntered
	}
	if !userDoc.LastExited.IsZero() {
		info.LastExited = &userDoc.LastExited
	}
	if !userDoc.RegistrationDate.IsZero() {
		info.RegistrationDate = &userDoc.RegistrationDate
	}

	return info, nil
}

// PrepareEntryTime prepares the entry time and duration for seat assignment
func (u *UserStateService) PrepareEntryTime(settings *UserSettings) (entryTime, exitTime time.Time) {
	now := utils.JstNow()
	duration := time.Duration(settings.WorkDurationMin) * time.Minute
	
	return now, now.Add(duration)
}

// CanUserEnterRoom checks if the user can enter the requested room type
func (u *UserStateService) CanUserEnterRoom(ctx context.Context, entryCtx *EntryContext) (bool, string, error) {
	// Check membership requirements for member seats
	if entryCtx.IsMemberSeat && !u.app.ProcessedUserIsMember {
		if !u.app.Configs.Constants.YoutubeMembershipEnabled {
			return false, "Membership is disabled", nil
		}
		return false, "Member seat access requires membership", nil
	}

	// Check if user is already in a room
	_, err := u.GetUserRoomStatus(ctx, entryCtx.UserId)
	if err != nil {
		return false, "", fmt.Errorf("failed to get room status: %w", err)
	}

	// User can enter if they're not in any room, or if they're moving between rooms
	return true, "", nil
}

// GetDefaultSettings returns default settings for a user
func (u *UserStateService) GetDefaultSettings() *UserSettings {
	return &UserSettings{
		UserDoc:         repository.UserDoc{},
		WorkDurationMin: u.app.Configs.Constants.DefaultWorkTimeMin,
		SeatAppearance:  repository.SeatAppearance{},
		OrderInfo: &OrderInfo{
			HasOrder: false,
		},
	}
}