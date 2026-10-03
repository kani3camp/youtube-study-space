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
	defaultChannelOwnersCollection  = "mypage-youtube-channel-owners"
	authorizationHeader             = "Authorization"
	bearerPrefix                    = "Bearer "
)

type Resolver struct {
	authClient               *auth.Client
	firestoreClient          repository.DBClient
	linkedAccountsCollection string
	channelOwnersCollection  string
	nowFunc                  func() time.Time // テストの時刻注入用
}

type LinkedYouTubeAccountDoc struct {
	YouTubeChannelID string    `firestore:"youtube-channel-id"`
	DisplayName      string    `firestore:"display-name"`
	ProfileImageURL  string    `firestore:"profile-image-url"`
	LinkedAt         time.Time `firestore:"linked-at"`
	UpdatedAt        time.Time `firestore:"updated-at"`
}

type YouTubeChannelOwnerDoc struct {
	FirebaseUID string    `firestore:"firebase-uid"`
	LinkedAt    time.Time `firestore:"linked-at"`
	UpdatedAt   time.Time `firestore:"updated-at"`
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
		firestoreClient:          repo.FirestoreClient(),
		linkedAccountsCollection: linkedAccountsCollection,
		channelOwnersCollection:  channelOwnersCollectionName(linkedAccountsCollection),
	}, nil
}

func channelOwnersCollectionName(linkedAccountsCollection string) string {
	if linkedAccountsCollection == defaultLinkedAccountsCollection {
		return defaultChannelOwnersCollection
	}
	return linkedAccountsCollection + "-youtube-channel-owners"
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

func (r *Resolver) LinkYouTubeAccount(ctx context.Context, firebaseUID string, viewer mypage.Viewer) error {
	if firebaseUID == "" {
		return mypage.ErrUnauthorized
	}
	viewer.YouTubeChannelID = strings.TrimSpace(viewer.YouTubeChannelID)
	if viewer.YouTubeChannelID == "" {
		return fmt.Errorf("%w: youtube channel id is empty", mypage.ErrInvalidIdentity)
	}

	accountRef := r.firestoreClient.
		Collection(r.linkedAccountsCollection).
		Doc(firebaseUID)
	ownerRef := r.firestoreClient.
		Collection(r.channelOwnersCollection).
		Doc(viewer.YouTubeChannelID)

	now := r.currentTime()
	err := r.firestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		ownerSnapshot, ownerExists, err := transactionGetOptional(tx, ownerRef)
		if err != nil {
			return fmt.Errorf("read youtube channel owner: %w", err)
		}

		var owner YouTubeChannelOwnerDoc
		ownerLinkedAt := now
		if ownerExists {
			if err := ownerSnapshot.DataTo(&owner); err != nil {
				return fmt.Errorf("decode youtube channel owner: %w", err)
			}
			if owner.FirebaseUID == "" {
				return errors.New("youtube channel owner firebase uid is empty")
			}
			if owner.FirebaseUID != firebaseUID {
				return fmt.Errorf(
					"%w: youtubeChannelID=%s",
					mypage.ErrYouTubeChannelAlreadyLinked,
					viewer.YouTubeChannelID,
				)
			}
			if !owner.LinkedAt.IsZero() {
				ownerLinkedAt = owner.LinkedAt
			}
		} else {
			// Existing rows created before the reverse index was introduced are checked
			// while the index is lazily backfilled by this transaction.
			legacyOwners, err := tx.Documents(
				r.firestoreClient.Collection(r.linkedAccountsCollection).
					Where("youtube-channel-id", "==", viewer.YouTubeChannelID).
					Limit(2),
			).GetAll()
			if err != nil {
				return fmt.Errorf("query legacy youtube channel owner: %w", err)
			}
			switch len(legacyOwners) {
			case 0:
			case 1:
				if legacyOwners[0].Ref.ID != firebaseUID {
					return fmt.Errorf(
						"%w: youtubeChannelID=%s",
						mypage.ErrYouTubeChannelAlreadyLinked,
						viewer.YouTubeChannelID,
					)
				}
			default:
				return fmt.Errorf(
					"duplicate legacy youtube channel links: youtubeChannelID=%s ownerCandidates=%d",
					viewer.YouTubeChannelID,
					len(legacyOwners),
				)
			}
		}

		currentSnapshot, currentExists, err := transactionGetOptional(tx, accountRef)
		if err != nil {
			return fmt.Errorf("read existing linked youtube account: %w", err)
		}

		linkedAt := now
		var oldOwnerRef *firestore.DocumentRef
		if currentExists {
			var current LinkedYouTubeAccountDoc
			if err := currentSnapshot.DataTo(&current); err != nil {
				return fmt.Errorf("decode existing linked youtube account: %w", err)
			}
			if !current.LinkedAt.IsZero() {
				linkedAt = current.LinkedAt
			}

			oldChannelID := strings.TrimSpace(current.YouTubeChannelID)
			if oldChannelID != "" && oldChannelID != viewer.YouTubeChannelID {
				candidate := r.firestoreClient.Collection(r.channelOwnersCollection).Doc(oldChannelID)
				oldOwnerSnapshot, oldOwnerExists, err := transactionGetOptional(tx, candidate)
				if err != nil {
					return fmt.Errorf("read previous youtube channel owner: %w", err)
				}
				if oldOwnerExists {
					var oldOwner YouTubeChannelOwnerDoc
					if err := oldOwnerSnapshot.DataTo(&oldOwner); err != nil {
						return fmt.Errorf("decode previous youtube channel owner: %w", err)
					}
					if oldOwner.FirebaseUID != firebaseUID {
						return errors.New("previous youtube channel owner does not match linked account")
					}
					oldOwnerRef = candidate
				}
			}
		}

		if oldOwnerRef != nil {
			if err := tx.Delete(oldOwnerRef); err != nil {
				return fmt.Errorf("delete previous youtube channel owner: %w", err)
			}
		}

		if err := tx.Set(ownerRef, YouTubeChannelOwnerDoc{
			FirebaseUID: firebaseUID,
			LinkedAt:    ownerLinkedAt,
			UpdatedAt:   now,
		}, firestore.MergeAll); err != nil {
			return fmt.Errorf("set youtube channel owner: %w", err)
		}

		if err := tx.Set(accountRef, map[string]any{
			"youtube-channel-id": viewer.YouTubeChannelID,
			"display-name":       viewer.DisplayName,
			"profile-image-url":  viewer.ProfileImageURL,
			"linked-at":          linkedAt,
			"updated-at":         now,
		}, firestore.MergeAll); err != nil {
			return fmt.Errorf("set linked youtube account: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("link youtube account transaction: %w", err)
	}

	return nil
}

func transactionGetOptional(
	tx *firestore.Transaction,
	ref *firestore.DocumentRef,
) (*firestore.DocumentSnapshot, bool, error) {
	snapshot, err := tx.Get(ref)
	if status.Code(err) == codes.NotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return snapshot, true, nil
}

func (r *Resolver) readLinkedYouTubeAccount(ctx context.Context, firebaseUID string) (LinkedYouTubeAccountDoc, error) {
	doc, err := r.firestoreClient.
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
