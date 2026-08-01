package main

import (
	"context"
	"log/slog"
	"os"

	"app.modules/core/mypage"
	"app.modules/core/repository"
	"app.modules/internal/apigatewayhttp"
	"app.modules/internal/awsruntime"
	"app.modules/internal/firebaseauth"
	"app.modules/internal/logging"
	"app.modules/internal/youtubeauth"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func init() {
	logging.InitLogger()
}

func handle(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	gracefulCtx, cancel := awsruntime.CreateGracefulContext(ctx, awsruntime.DefaultGraceSeconds)
	defer cancel()

	clientOption, err := awsruntime.FirestoreClientOption()
	if err != nil {
		slog.ErrorContext(ctx, "failed to get Firestore client option", "err", err)
		return apigatewayhttp.JSONError(500, "internal_error", "internal server error"), nil
	}

	repo, err := repository.NewFirestoreController(gracefulCtx, clientOption)
	if err != nil {
		slog.ErrorContext(ctx, "failed to initialize Firestore", "err", err)
		return apigatewayhttp.JSONError(500, "internal_error", "internal server error"), nil
	}
	defer func() {
		if err := repo.FirestoreClient().Close(); err != nil {
			slog.ErrorContext(ctx, "failed to close Firestore client", "err", err)
		}
	}()

	authResolver, err := firebaseauth.NewResolver(gracefulCtx, clientOption, repo)
	if err != nil {
		slog.ErrorContext(ctx, "failed to initialize Firebase auth resolver", "err", err)
		return apigatewayhttp.JSONError(500, "internal_error", "internal server error"), nil
	}

	store := mypage.NewRepositoryStore(repo)
	service, err := mypage.NewService(store, nil)
	if err != nil {
		slog.ErrorContext(ctx, "failed to initialize mypage service", "err", err)
		return apigatewayhttp.JSONError(500, "internal_error", "internal server error"), nil
	}
	youtubeClient := youtubeauth.NewClient()

	handler, err := mypage.NewHandler(mypage.HandlerOptions{
		MeGetter:              service,
		YouTubeLinker:         service,
		IdentityResolver:      authResolver,
		FirebaseAuthenticator: authResolver,
		ChannelFetcher:        youtubeClient,
		LinkedAccountStore:    authResolver,
		AllowedOrigin:         os.Getenv("MYPAGE_ALLOWED_ORIGIN"),
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to initialize mypage handler", "err", err)
		return apigatewayhttp.JSONError(500, "internal_error", "internal server error"), nil
	}

	return apigatewayhttp.Serve(gracefulCtx, req, handler), nil
}

func main() {
	lambda.Start(handle)
}
