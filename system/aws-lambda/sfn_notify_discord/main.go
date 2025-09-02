package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"app.modules/aws-lambda/lambdautils"
	"app.modules/core/workspaceapp"

	"github.com/aws/aws-lambda-go/lambda"
)

type sfnErrorInput struct {
	// Step Functions context
	StateName    string `json:"stateName"`
	ExecutionArn string `json:"executionArn"`
	Workflow     string `json:"workflow"`

	// Error info
	Error string `json:"error"`
	Cause string `json:"cause"`
}

type notifyResponse struct {
	Result string `json:"result"`
}

func handler(input sfnErrorInput) (notifyResponse, error) {
	ctx := context.Background()

	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		slog.Error("failed to get Firestore client option", "err", err)
		return notifyResponse{}, err
	}

	app, err := workspaceapp.NewWorkspaceApp(ctx, false, clientOption)
	if err != nil {
		slog.Error("failed to init WorkspaceApp", "err", err)
		return notifyResponse{}, err
	}
	defer app.CloseFirestoreClient()

	// Compact cause (often a JSON string). Truncate if too long.
	compactCause := input.Cause
	var tmp map[string]any
	if json.Unmarshal([]byte(input.Cause), &tmp) == nil {
		if b, e := json.Marshal(tmp); e == nil {
			compactCause = string(b)
		}
	}
	if len(compactCause) > 1800 { // keep under Discord limit with some headroom
		compactCause = compactCause[:1800] + "... (truncated)"
	}

	message := fmt.Sprintf(
		"[%s] StepFunctions failure\nstate: %s\nerror: %s\ncause: %s\nexecution: %s",
		input.Workflow, input.StateName, input.Error, compactCause, input.ExecutionArn,
	)
	app.MessageToOwner(ctx, message)
	return notifyResponse{Result: "ok"}, nil
}

func main() {
	lambda.Start(handler)
}
