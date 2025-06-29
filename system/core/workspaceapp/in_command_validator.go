package workspaceapp

import (
	"context"
	"fmt"

	"app.modules/core/i18n"
	"app.modules/core/studyspaceerror"
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
	"github.com/pkg/errors"
)

// InCommandValidator handles validation logic for seat entry commands
type InCommandValidator struct {
	app *WorkspaceApp
}

// NewInCommandValidator creates a new instance of InCommandValidator
func NewInCommandValidator(app *WorkspaceApp) *InCommandValidator {
	return &InCommandValidator{
		app: app,
	}
}

// ValidationResult represents the result of validation
type ValidationResult struct {
	IsValid      bool
	ErrorMessage string
	ShouldReturn bool // Whether the command should return early
}

// ValidateAccessPermission validates if the user can access the requested seat type
func (v *InCommandValidator) ValidateAccessPermission(inOption *utils.InOption) *ValidationResult {
	t := i18n.GetTFunc("command-in")
	
	if inOption.IsMemberSeat && !v.app.ProcessedUserIsMember {
		var message string
		if v.app.Configs.Constants.YoutubeMembershipEnabled {
			message = t("member-seat-forbidden", v.app.ProcessedUserDisplayName)
		} else {
			message = t("membership-disabled", v.app.ProcessedUserDisplayName)
		}
		
		return &ValidationResult{
			IsValid:      false,
			ErrorMessage: message,
			ShouldReturn: true,
		}
	}
	
	return &ValidationResult{
		IsValid:      true,
		ErrorMessage: "",
		ShouldReturn: false,
	}
}

// ValidateSeatAvailability validates if the requested seat is available for the user
func (v *InCommandValidator) ValidateSeatAvailability(ctx context.Context, tx *firestore.Transaction, inOption *utils.InOption) *ValidationResult {
	t := i18n.GetTFunc("command-in")
	
	// If seat is not specified, skip availability check (will be handled by assignment logic)
	if !inOption.IsSeatIdSet || inOption.SeatId == 0 {
		return &ValidationResult{
			IsValid:      true,
			ErrorMessage: "",
			ShouldReturn: false,
		}
	}
	
	// Check if the seat is vacant
	isVacant, err := v.app.IfSeatVacant(ctx, tx, inOption.SeatId, inOption.IsMemberSeat)
	if err != nil {
		return &ValidationResult{
			IsValid:      false,
			ErrorMessage: fmt.Sprintf("failed to check seat vacancy: %v", err),
			ShouldReturn: false,
		}
	}
	
	if !isVacant {
		return &ValidationResult{
			IsValid:      false,
			ErrorMessage: t("no-seat", v.app.ProcessedUserDisplayName, utils.InCommand),
			ShouldReturn: true,
		}
	}
	
	return &ValidationResult{
		IsValid:      true,
		ErrorMessage: "",
		ShouldReturn: false,
	}
}

// ValidateUserSeatRestrictions validates if the user has any restrictions for the specific seat
func (v *InCommandValidator) ValidateUserSeatRestrictions(ctx context.Context, inOption *utils.InOption) *ValidationResult {
	t := i18n.GetTFunc("command-in")
	
	// Skip restriction check if seat is not specified or is seat 0
	if !inOption.IsSeatIdSet || inOption.SeatId == 0 {
		return &ValidationResult{
			IsValid:      true,
			ErrorMessage: "",
			ShouldReturn: false,
		}
	}
	
	// Check if user has been sitting too much in this seat recently
	isTooMuch, err := v.app.CheckIfUserSittingTooMuchForSeat(ctx, v.app.ProcessedUserId, inOption.SeatId, inOption.IsMemberSeat)
	if err != nil {
		return &ValidationResult{
			IsValid:      false,
			ErrorMessage: fmt.Sprintf("failed to check user seat restrictions: %v", err),
			ShouldReturn: false,
		}
	}
	
	if isTooMuch {
		return &ValidationResult{
			IsValid:      false,
			ErrorMessage: t("no-availability", v.app.ProcessedUserDisplayName, utils.InCommand),
			ShouldReturn: true,
		}
	}
	
	return &ValidationResult{
		IsValid:      true,
		ErrorMessage: "",
		ShouldReturn: false,
	}
}

// ValidateAll performs comprehensive validation for the In command
func (v *InCommandValidator) ValidateAll(ctx context.Context, tx *firestore.Transaction, inOption *utils.InOption) *ValidationResult {
	// 1. Check access permission (no transaction needed)
	if result := v.ValidateAccessPermission(inOption); !result.IsValid {
		return result
	}
	
	// 2. Check seat availability (requires transaction)
	if result := v.ValidateSeatAvailability(ctx, tx, inOption); !result.IsValid {
		return result
	}
	
	// 3. Check user seat restrictions (no transaction needed)
	if result := v.ValidateUserSeatRestrictions(ctx, inOption); !result.IsValid {
		return result
	}
	
	return &ValidationResult{
		IsValid:      true,
		ErrorMessage: "",
		ShouldReturn: false,
	}
}

// ValidateWorkTimeSettings validates work time related settings
func (v *InCommandValidator) ValidateWorkTimeSettings(inOption *utils.InOption) *ValidationResult {
	if inOption.MinWorkOrderOption == nil {
		return &ValidationResult{
			IsValid:      false,
			ErrorMessage: "work time option is required",
			ShouldReturn: false,
		}
	}
	
	if inOption.MinWorkOrderOption.IsDurationMinSet {
		duration := inOption.MinWorkOrderOption.DurationMin
		constants := v.app.Configs.Constants
		
		if duration < constants.MinWorkTimeMin || duration > constants.MaxWorkTimeMin {
			t := i18n.GetTFunc("command-in")
			return &ValidationResult{
				IsValid:      false,
				ErrorMessage: t("invalid-work-time", v.app.ProcessedUserDisplayName, constants.MinWorkTimeMin, constants.MaxWorkTimeMin),
				ShouldReturn: true,
			}
		}
	}
	
	return &ValidationResult{
		IsValid:      true,
		ErrorMessage: "",
		ShouldReturn: false,
	}
}

// ValidateSystemCapacity validates if the system can accommodate new entries
func (v *InCommandValidator) ValidateSystemCapacity(ctx context.Context, tx *firestore.Transaction, inOption *utils.InOption) *ValidationResult {
	// This validation is for when no seat is specified and we need to find an available seat
	if inOption.IsSeatIdSet && inOption.SeatId != 0 {
		return &ValidationResult{
			IsValid:      true,
			ErrorMessage: "",
			ShouldReturn: false,
		}
	}
	
	// Check if there are any available seats for this user
	_, err := v.app.RandomAvailableSeatIdForUser(ctx, tx, v.app.ProcessedUserId, inOption.IsMemberSeat)
	if err != nil {
		if errors.Is(err, studyspaceerror.ErrNoSeatAvailable) {
			t := i18n.GetTFunc("command-in")
			return &ValidationResult{
				IsValid:      false,
				ErrorMessage: t("room-full", v.app.ProcessedUserDisplayName),
				ShouldReturn: true,
			}
		}
		return &ValidationResult{
			IsValid:      false,
			ErrorMessage: fmt.Sprintf("failed to check system capacity: %v", err),
			ShouldReturn: false,
		}
	}
	
	return &ValidationResult{
		IsValid:      true,
		ErrorMessage: "",
		ShouldReturn: false,
	}
}