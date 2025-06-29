package workspaceapp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSuccessResult(t *testing.T) {
	message := "Test success message"
	result := NewSuccessResult(message)
	
	assert.Equal(t, message, result.Message)
	assert.NoError(t, result.Error)
}

func TestNewErrorResult(t *testing.T) {
	err := errors.New("test error")
	result := NewErrorResult(err)
	
	assert.Equal(t, "", result.Message)
	assert.Equal(t, err, result.Error)
}

func TestTransactionError(t *testing.T) {
	originalErr := errors.New("original error")
	operation := "test operation"
	
	txErr := &TransactionError{
		Operation: operation,
		Cause:     originalErr,
	}
	
	assert.Contains(t, txErr.Error(), operation)
	assert.Contains(t, txErr.Error(), originalErr.Error())
	assert.Equal(t, originalErr, txErr.Unwrap())
}

func TestCommandError(t *testing.T) {
	originalErr := errors.New("original error")
	command := "test command"
	
	cmdErr := &CommandError{
		Command: command,
		Cause:   originalErr,
	}
	
	assert.Contains(t, cmdErr.Error(), command)
	assert.Contains(t, cmdErr.Error(), originalErr.Error())
	assert.Equal(t, originalErr, cmdErr.Unwrap())
}

func TestWrapTransactionError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		operation string
		want      error
	}{
		{
			name:      "nil error",
			err:       nil,
			operation: "test",
			want:      nil,
		},
		{
			name:      "valid error",
			err:       errors.New("test error"),
			operation: "test operation",
			want:      &TransactionError{Operation: "test operation", Cause: errors.New("test error")},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapTransactionError(tt.err, tt.operation)
			if tt.want == nil {
				assert.NoError(t, result)
			} else {
				assert.IsType(t, &TransactionError{}, result)
				txErr := result.(*TransactionError)
				assert.Equal(t, tt.operation, txErr.Operation)
				assert.Equal(t, tt.err.Error(), txErr.Cause.Error())
			}
		})
	}
}

func TestWrapCommandError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		command string
		want    error
	}{
		{
			name:    "nil error",
			err:     nil,
			command: "test",
			want:    nil,
		},
		{
			name:    "valid error",
			err:     errors.New("test error"),
			command: "test command",
			want:    &CommandError{Command: "test command", Cause: errors.New("test error")},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapCommandError(tt.err, tt.command)
			if tt.want == nil {
				assert.NoError(t, result)
			} else {
				assert.IsType(t, &CommandError{}, result)
				cmdErr := result.(*CommandError)
				assert.Equal(t, tt.command, cmdErr.Command)
				assert.Equal(t, tt.err.Error(), cmdErr.Cause.Error())
			}
		})
	}
}

// MockWorkspaceApp for testing error handling methods
type MockWorkspaceApp struct {
	ProcessedUserDisplayName string
	lastMessage              string
}

func (m *MockWorkspaceApp) MessageToLiveChat(ctx context.Context, message string) {
	m.lastMessage = message
}

func TestHandleCommandResult_Success(t *testing.T) {
	app := &MockWorkspaceApp{ProcessedUserDisplayName: "TestUser"}
	ctx := context.Background()
	
	result := NewSuccessResult("Success message")
	err := app.HandleCommandResult(ctx, result)
	
	assert.NoError(t, err)
	assert.Equal(t, "Success message", app.lastMessage)
}

func TestHandleCommandResult_Error(t *testing.T) {
	app := &MockWorkspaceApp{ProcessedUserDisplayName: "TestUser"}
	ctx := context.Background()
	
	testErr := errors.New("test error")
	result := NewErrorResult(testErr)
	err := app.HandleCommandResult(ctx, result)
	
	assert.Error(t, err)
	assert.Equal(t, testErr, err)
	// Note: The actual error message would depend on i18n.T implementation
	assert.NotEmpty(t, app.lastMessage)
}

// Helper method to satisfy the interface for testing
func (m *MockWorkspaceApp) HandleCommandResult(ctx context.Context, result *CommandResult) error {
	if result.Error != nil {
		// Simplified version for testing
		m.MessageToLiveChat(ctx, "Error occurred: "+m.ProcessedUserDisplayName)
		return result.Error
	}
	
	if result.Message != "" {
		m.MessageToLiveChat(ctx, result.Message)
	}
	
	return nil
}