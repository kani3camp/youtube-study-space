package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/youtube/v3"

	"app.modules/core/repository"
	"app.modules/core/workspaceapp"
	"app.modules/core/youtubebot"
)

const durableLiveChatIngestEnabledEnv = "DURABLE_LIVE_CHAT_INGEST_ENABLED"

type durableLiveChatPageIngester interface {
	IngestLiveChatSourcePage(
		ctx context.Context,
		liveChatID string,
		expectedPageToken string,
		nextPageToken string,
		messages []repository.LiveChatIngestMessage,
		ingestedAt time.Time,
	) error
}

func readDurableLiveChatIngestEnabled() (bool, error) {
	raw := strings.TrimSpace(os.Getenv(durableLiveChatIngestEnabledEnv))
	if raw == "" {
		return false, nil
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", durableLiveChatIngestEnabledEnv, err)
	}
	return enabled, nil
}

func ingestFetchedLiveChatPage(
	ctx context.Context,
	ingester durableLiveChatPageIngester,
	liveChatID string,
	expectedPageToken string,
	nextPageToken string,
	chatMessages []*youtube.LiveChatMessage,
	ingestedAt time.Time,
) error {
	sourceMessages := make([]repository.LiveChatIngestMessage, 0, len(chatMessages))
	for i, chatMessage := range chatMessages {
		if chatMessage == nil {
			return fmt.Errorf("live chat message at index %d is nil", i)
		}
		if chatMessage.Snippet == nil {
			return fmt.Errorf("live chat message at index %d has nil snippet", i)
		}
		if !youtubebot.HasTextMessageByAuthor(chatMessage) {
			continue
		}
		sourceMessage, err := workspaceapp.BuildLiveChatIngestMessage(chatMessage)
		if err != nil {
			return fmt.Errorf("build durable live chat ingest message at index %d: %w", i, err)
		}
		sourceMessages = append(sourceMessages, sourceMessage)
	}

	if err := ingester.IngestLiveChatSourcePage(
		ctx,
		liveChatID,
		expectedPageToken,
		nextPageToken,
		sourceMessages,
		ingestedAt,
	); err != nil {
		return fmt.Errorf("ingest durable live chat page: %w", err)
	}
	return nil
}
