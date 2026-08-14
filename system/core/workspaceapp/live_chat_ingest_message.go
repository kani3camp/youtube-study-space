package workspaceapp

import (
	"errors"
	"fmt"
	"time"

	"google.golang.org/api/youtube/v3"

	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/core/youtubebot"
)

// BuildLiveChatIngestMessage converts one YouTube source message into the
// durable ingest shape while preserving the existing live-chat-history schema.
// The returned History is intentionally equivalent to AddLiveChatHistoryDoc's
// current document, and owner/sponsor metadata is carried separately for Inbox
// command processing.
func BuildLiveChatIngestMessage(chatMessage *youtube.LiveChatMessage) (repository.LiveChatIngestMessage, error) {
	if chatMessage == nil {
		return repository.LiveChatIngestMessage{}, errors.New("live chat message is nil")
	}
	if chatMessage.Snippet == nil {
		return repository.LiveChatIngestMessage{}, errors.New("live chat message snippet is nil")
	}
	if chatMessage.AuthorDetails == nil {
		return repository.LiveChatIngestMessage{}, errors.New("live chat message author details are nil")
	}
	if !youtubebot.HasTextMessageByAuthor(chatMessage) {
		return repository.LiveChatIngestMessage{}, errors.New("live chat message has no author text message")
	}

	publishedAt, err := time.Parse(time.RFC3339Nano, chatMessage.Snippet.PublishedAt)
	if err != nil {
		return repository.LiveChatIngestMessage{}, fmt.Errorf("parse live chat published at: %w", err)
	}
	publishedAt = publishedAt.In(timeutil.JapanLocation())

	return repository.LiveChatIngestMessage{
		History: repository.LiveChatHistoryDoc{
			AuthorChannelID:       chatMessage.AuthorDetails.ChannelId,
			AuthorDisplayName:     chatMessage.AuthorDetails.DisplayName,
			AuthorProfileImageURL: chatMessage.AuthorDetails.ProfileImageUrl,
			AuthorIsChatModerator: chatMessage.AuthorDetails.IsChatModerator,
			ID:                    chatMessage.Id,
			LiveChatID:            chatMessage.Snippet.LiveChatId,
			MessageText:           youtubebot.ExtractTextMessageByAuthor(chatMessage),
			PublishedAt:           publishedAt,
			Type:                  chatMessage.Snippet.Type,
		},
		AuthorIsChatOwner:   youtubebot.IsChatMessageByOwner(chatMessage),
		AuthorIsChatSponsor: youtubebot.IsChatMessageByMember(chatMessage),
	}, nil
}
