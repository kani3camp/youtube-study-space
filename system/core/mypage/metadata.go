package mypage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"app.modules/core/serviceaccess"
)

// PublicYouTubeMetadata uses an application API key, never a user's OAuth
// token. Key/uid request URLs and raw dependency errors must not be logged.
var ErrPublicChannelMissing = errors.New("public channel unavailable")

type PublicYouTubeMetadata struct {
	key    string
	client *http.Client
}

func NewPublicYouTubeMetadata(key string, client *http.Client) (*PublicYouTubeMetadata, error) {
	if key == "" {
		return nil, apiError("TEMPORARY_UNAVAILABLE")
	}
	bounded := http.Client{Timeout: time.Second}
	if client != nil {
		bounded = *client
	}
	if bounded.Timeout <= 0 || bounded.Timeout > time.Second {
		bounded.Timeout = time.Second
	}
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &PublicYouTubeMetadata{key: key, client: &bounded}, nil
}

func (p *PublicYouTubeMetadata) Read(ctx context.Context, uid string) (Channel, error) {
	if !youtubeChannelID.MatchString(uid) {
		return Channel{}, apiError("AUTH_REQUIRED")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	query := url.Values{"id": {uid}, "part": {"snippet"}, "key": {p.key}, "fields": {"nextPageToken,items(id,snippet(title,customUrl,thumbnails(default(url))))"}}.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeChannelsURL+"?"+query, nil)
	if err != nil {
		return Channel{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	response, err := p.client.Do(request)
	if err != nil {
		return Channel{}, unknownProviderError("TEMPORARY_UNAVAILABLE")
	}
	defer response.Body.Close() //nolint:errcheck // Read-only response close cannot change metadata and must not log credential URLs.
	if response.StatusCode != 200 {
		return Channel{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	body := &runtimeProviderBody{reader: response.Body}
	channel, err := decodeChannelResponse(body, true)
	if err != nil {
		if body.failed || ctx.Err() != nil {
			return Channel{}, unknownProviderError("TEMPORARY_UNAVAILABLE")
		}
		return Channel{}, err
	}
	if channel.ID != uid {
		return Channel{}, apiError("CHANNEL_UNAVAILABLE")
	}
	return channel, nil
}

type PublicMetadataReader interface {
	Read(context.Context, string) (Channel, error)
}
type MetadataWriter interface {
	SaveMetadata(context.Context, string, WebAccount, Channel, Policy, time.Time) (WebAccount, error)
}
type AccountMetadataRefresh struct {
	Runtime  *RuntimeRegistry
	Access   serviceaccess.Reader
	Provider PublicMetadataReader
	Store    MetadataWriter
	Policy   Policy
	Now      func() time.Time
}

func (r *AccountMetadataRefresh) Refresh(ctx context.Context, uid string, account WebAccount) (WebAccount, error) {
	ctx, work, runtimeErr := beginRuntime(ctx, r.Runtime, uid)
	if runtimeErr != nil {
		return WebAccount{}, runtimeErr
	}
	defer work.done()
	if r.Provider == nil || r.Store == nil || r.Now == nil || account.Revision.IsZero() {
		return WebAccount{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	checkpoint, err := readAccess(ctx, r.Access, uid, account.AccessCheckpoint)
	if err != nil {
		return WebAccount{}, err
	}
	account.AccessCheckpoint = checkpoint
	if err := checkMetadataAccount(account, r.Policy); err != nil {
		return WebAccount{}, err
	}
	if expiry, ok := r.Store.(interface {
		ExpireMetadata(context.Context, string, WebAccount, Policy, time.Time) (WebAccount, error)
	}); ok && !account.MetadataFetchedAt.IsZero() && r.Now().Sub(account.MetadataFetchedAt) >= 30*24*time.Hour {
		var stripped WebAccount
		err := runtimeMutation(ctx, work, func() error {
			var writeErr error
			stripped, writeErr = expiry.ExpireMetadata(ctx, uid, account, r.Policy, r.Now().UTC())
			if writeErr != nil {
				return fmt.Errorf("metadata store write: %w", writeErr)
			}
			return nil
		})
		if err != nil {
			return WebAccount{}, fmt.Errorf("expire channel metadata: %w", err)
		}
		account = stripped
	}
	channel, err := r.Provider.Read(ctx, uid)
	runtimeUnknownOutcome(ctx, work, err)
	if errors.Is(err, ErrPublicChannelMissing) {
		clearer, ok := r.Store.(interface {
			ClearMissingMetadata(context.Context, string, WebAccount, Policy, time.Time) (WebAccount, error)
		})
		if !ok {
			return WebAccount{}, ErrPublicChannelMissing
		}
		var cleared WebAccount
		clearErr := runtimeMutation(ctx, work, func() error {
			var writeErr error
			cleared, writeErr = clearer.ClearMissingMetadata(ctx, uid, account, r.Policy, r.Now().UTC())
			if writeErr != nil {
				return fmt.Errorf("metadata store write: %w", writeErr)
			}
			return nil
		})
		if clearErr != nil {
			return WebAccount{}, errors.Join(ErrPublicChannelMissing, fmt.Errorf("clear absent metadata: %w", clearErr))
		}
		return cleared, nil
	}
	if err != nil {
		return WebAccount{}, fmt.Errorf("refresh public metadata: %w", err)
	}
	if channel.ID != uid || channel.DisplayName == "" || ctx.Err() != nil {
		return WebAccount{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	var updated WebAccount
	err = runtimeMutation(ctx, work, func() error {
		var writeErr error
		updated, writeErr = r.Store.SaveMetadata(ctx, uid, account, channel, r.Policy, r.Now().UTC())
		if writeErr != nil {
			return fmt.Errorf("metadata store write: %w", writeErr)
		}
		return nil
	})
	if err != nil {
		return WebAccount{}, fmt.Errorf("commit refreshed metadata: %w", err)
	}
	return updated, nil
}

func checkMetadataAccount(account WebAccount, policy Policy) error {
	if policy.Privacy == "" || policy.Terms == "" || account.PrivacyPolicyVersion != policy.Privacy || account.TermsVersion != policy.Terms {
		return apiError("PRIVACY_RECONSENT_REQUIRED")
	}
	return nil
}
