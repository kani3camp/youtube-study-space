package workspaceapp

import (
	"context"
	"log/slog"

	"app.modules/core/i18n"
)

// CommandResult represents the result of a command execution
type CommandResult struct {
	Message string
	Error   error
}

// NewSuccessResult creates a successful command result
func NewSuccessResult(message string) *CommandResult {
	return &CommandResult{
		Message: message,
		Error:   nil,
	}
}

// NewErrorResult creates an error command result
func NewErrorResult(err error) *CommandResult {
	return &CommandResult{
		Message: "",
		Error:   err,
	}
}

// HandleCommandResult processes a command result and sends appropriate messages
func (app *WorkspaceApp) HandleCommandResult(ctx context.Context, result *CommandResult) error {
	if result.Error != nil {
		slog.Error("Command execution failed", "error", result.Error, "user", app.ProcessedUserDisplayName)
		errorMessage := i18n.T("command:error", app.ProcessedUserDisplayName)
		app.MessageToLiveChat(ctx, errorMessage)
		return result.Error
	}
	
	if result.Message != "" {
		app.MessageToLiveChat(ctx, result.Message)
	}
	
	return nil
}

// HandleTransactionResult processes transaction execution with consistent error handling
func (app *WorkspaceApp) HandleTransactionResult(ctx context.Context, txErr error, operationName string) error {
	if txErr != nil {
		slog.Error("Transaction failed", "operation", operationName, "error", txErr, "user", app.ProcessedUserDisplayName)
		errorMessage := i18n.T("command:error", app.ProcessedUserDisplayName)
		app.MessageToLiveChat(ctx, errorMessage)
		return txErr
	}
	
	return nil
}

// ExecuteWithErrorHandling executes a function that returns CommandResult and handles the result
func (app *WorkspaceApp) ExecuteWithErrorHandling(ctx context.Context, executor func() *CommandResult) error {
	result := executor()
	return app.HandleCommandResult(ctx, result)
}

// WrapTransactionError wraps a transaction error with consistent formatting
func WrapTransactionError(err error, operation string) error {
	if err == nil {
		return nil
	}
	return &TransactionError{
		Operation: operation,
		Cause:     err,
	}
}

// TransactionError represents an error that occurred during a transaction
type TransactionError struct {
	Operation string
	Cause     error
}

func (e *TransactionError) Error() string {
	return "transaction failed in " + e.Operation + ": " + e.Cause.Error()
}

func (e *TransactionError) Unwrap() error {
	return e.Cause
}

// CommandError represents an error that occurred during command execution
type CommandError struct {
	Command string
	Cause   error
}

func (e *CommandError) Error() string {
	return "command failed: " + e.Command + ": " + e.Cause.Error()
}

func (e *CommandError) Unwrap() error {
	return e.Cause
}

// WrapCommandError wraps a command error with consistent formatting
func WrapCommandError(err error, command string) error {
	if err == nil {
		return nil
	}
	return &CommandError{
		Command: command,
		Cause:   err,
	}
}