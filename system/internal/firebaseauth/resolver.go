package firebaseauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"app.modules/core/mypage"
	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultLinkedAccountsCollection = "mypage-users"
	authorizationHeader             = "Authorization"
	bearerPrefix                    = "Bearer "
)

type Resolver struct {
	authClient               *auth.Client
	repo                     repository.Repository
	linkedAccountsCollection string
	nowFunc                  func() time.Time // テストの時刻注入用
}

type LinkedYouTubeAccountDoc struct {
	YouTubeChannelID string    `firestore:"youtube-channel-id"`
	DisplayName      string    `firestore:"display-name"`
	ProfileImageURL  string    `firestore:"profile-image-url"`
	LinkedAt         time.Time `firestore:"linked-at"`
	UpdatedAt        time.Time `firestore:"updated-at"`
}

func NewResolver(
	ctx context.Context,
	clientOption option.ClientOption,
	repo repository.Repository,
) (*Resolver, error) {
	return NewResolverWithCollection(ctx, clientOption, repo, defaultLinkedAccountsCollection)
}

func NewResolverWithCollection(
	ctx context.Context,
	clientOption option.ClientOption,
	repo repository.Repository,
	linkedAccountsCollection string,
) (*Resolver, error) {
	if repo == nil {
		return nil, errors.New("repository is nil")
	}

	if linkedAccountsCollection == "" {
		linkedAccountsCollection = defaultLinkedAccountsCollection
	}

	app, err := firebase.NewApp(ctx, nil, clientOption)
	if err != nil {
		return nil, fmt.Errorf("initialize firebase app: %w", err)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize firebase auth client: %w", err)
	}

	return &Resolver{
		authClient:               authClient,
		repo:                     repo,
		linkedAccountsCollection: linkedAccountsCollection,
	}, nil
}

func (r *Resolver) currentTime() time.Time {
	if r.nowFunc != nil {
		return r.nowFunc()
	}
	return timeutil.JstNow()
}

func (r *Resolver) Resolve(ctx context.Context, req *http.Request) (mypage.Identity, error) {
	authenticatedUser, err := r.Authenticate(ctx, requestAuthHeader{request: req})
	if err != nil {
		return mypage.Identity{}, err
	}

	linkedAccount, err := r.readLinkedYouTubeAccount(ctx, authenticatedUser.FirebaseUID)
	if err != nil {
		return mypage.Identity{}, err
	}

	if linkedAccount.YouTubeChannelID == "" {
		return mypage.Identity{}, fmt.Errorf("%w: youtube channel id is empty", mypage.ErrYouTubeLinkRequired)
	}

	return mypage.Identity{
		FirebaseUID:      authenticatedUser.FirebaseUID,
		YouTubeChannelID: linkedAccount.YouTubeChannelID,
		DisplayName:      linkedAccount.DisplayName,
		ProfileImageURL:  linkedAccount.ProfileImageURL,
	}, nil
}

func (r *Resolver) Authenticate(ctx context.Context, req mypage.FirebaseIDTokenRequest) (mypage.AuthenticatedFirebaseUser, error) {
	idToken, err := bearerTokenFromAuthorizationHeader(req.AuthorizationHeader())
	if err != nil {
		return mypage.AuthenticatedFirebaseUser{}, err
	}

	token, err := r.authClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		return mypage.AuthenticatedFirebaseUser{}, fmt.Errorf("%w: verify firebase id token: %v", mypage.ErrUnauthorized, err)
	}

	if token.UID == "" {
		return mypage.AuthenticatedFirebaseUser{}, fmt.Errorf("%w: firebase uid is empty", mypage.ErrUnauthorized)
	}

	return mypage.AuthenticatedFirebaseUser{
		FirebaseUID: token.UID,
	}, nil
}

func (r *Resolver) SaveLinkedYouTubeAccount(ctx context.Context, firebaseUID string, viewer mypage.Viewer) error {
	if firebaseUID == "" {
		return mypage.ErrUnauthorized
	}
	if viewer.YouTubeChannelID == "" {
		return fmt.Errorf("%w: youtube channel id is empty", mypage.ErrInvalidIdentity)
	}

	ref := r.repo.FirestoreClient().
		Collection(r.linkedAccountsCollection).
		Doc(firebaseUID)

	now := r.currentTime()
	linkedAt := now

	current, err := ref.Get(ctx)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return fmt.Errorf("read existing linked youtube account: %w", err)
		}
	} else {
		var existing LinkedYouTubeAccountDoc
		if err := current.DataTo(&existing); err != nil {
			return fmt.Errorf("decode existing linked youtube account: %w", err)
		}
		if !existing.LinkedAt.IsZero() {
			linkedAt = existing.LinkedAt
		}
	}

	_, err = ref.Set(ctx, map[string]any{
		"youtube-channel-id": viewer.YouTubeChannelID,
		"display-name":       viewer.DisplayName,
		"profile-image-url":  viewer.ProfileImageURL,
		"linked-at":          linkedAt,
		"updated-at":         now,
	}, firestore.MergeAll)
	if err != nil {
		return fmt.Errorf("set linked youtube account: %w", err)
	}

	return nil
}

func (r *Resolver) FindFirebaseUIDByYouTubeChannelID(ctx context.Context, youtubeChannelID string) (string, error) {
	youtubeChannelID = strings.TrimSpace(youtubeChannelID)
	if youtubeChannelID == "" {
		return "", fmt.Errorf("%w: youtube channel id is empty", mypage.ErrInvalidRequest)
	}

	// TODO: This lookup does not provide strict uniqueness guarantees under concurrent link requests.
	// Consider introducing a reverse-index document keyed by YouTube channel ID and enforcing ownership in a Firestore transaction.
	docs, err := r.repo.FirestoreClient().
		Collection(r.linkedAccountsCollection).
		Where("youtube-channel-id", "==", youtubeChannelID).
		Limit(2).
		Documents(ctx).
		GetAll()
	if err != nil {
		return "", fmt.Errorf("query linked youtube account owner: %w", err)
	}

	switch len(docs) {
	case 0:
		return "", nil
	case 1:
		return docs[0].Ref.ID, nil
	default:
		return "", fmt.Errorf(
			"duplicate youtube channel link detected: youtubeChannelID=%s ownerCandidates=%d",
			youtubeChannelID,
			len(docs),
		)
	}
}

func (r *Resolver) readLinkedYouTubeAccount(ctx context.Context, firebaseUID string) (LinkedYouTubeAccountDoc, error) {
	doc, err := r.repo.FirestoreClient().
		Collection(r.linkedAccountsCollection).
		Doc(firebaseUID).
		Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return LinkedYouTubeAccountDoc{}, fmt.Errorf("%w: firebaseUID=%s", mypage.ErrYouTubeLinkRequired, firebaseUID)
		}
		return LinkedYouTubeAccountDoc{}, fmt.Errorf("read linked youtube account: %w", err)
	}

	var linkedAccount LinkedYouTubeAccountDoc
	if err := doc.DataTo(&linkedAccount); err != nil {
		return LinkedYouTubeAccountDoc{}, fmt.Errorf("decode linked youtube account: %w", err)
	}

	return linkedAccount, nil
}

func bearerTokenFromAuthorizationHeader(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%w: authorization header is empty", mypage.ErrUnauthorized)
	}

	if !strings.HasPrefix(value, bearerPrefix) {
		return "", fmt.Errorf("%w: authorization header is not bearer token", mypage.ErrUnauthorized)
	}

	token := strings.TrimSpace(strings.TrimPrefix(value, bearerPrefix))
	if token == "" {
		return "", fmt.Errorf("%w: bearer token is empty", mypage.ErrUnauthorized)
	}

	return token, nil
}

type requestAuthHeader struct {
	request *http.Request
}

func (r requestAuthHeader) AuthorizationHeader() string {
	if r.request == nil {
		return ""
	}
	return r.request.Header.Get(authorizationHeader)
}
