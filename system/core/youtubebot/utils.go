package youtubebot

import "google.golang.org/api/youtube/v3"

const (
	TextMessageEventString   = "textMessageEvent"
	SuperChatEventString     = "superChatEvent"
	MemberMilestoneChatEvent = "memberMilestoneChatEvent"
)

// HasTextMessageByAuthor HasTextMessageContent returns true when the chatMessage has a text message content by the author.
func HasTextMessageByAuthor(chat *youtube.LiveChatMessage) bool {
	if chat.Snippet.Type == TextMessageEventString && chat.Snippet.TextMessageDetails != nil {
		return true
	}
	if chat.Snippet.Type == SuperChatEventString && chat.Snippet.SuperChatDetails != nil {
		return true
	}
	if chat.Snippet.Type == MemberMilestoneChatEvent && chat.Snippet.MemberMilestoneChatDetails != nil {
		return true
	}
	return false
}

func ExtractTextMessageByAuthor(chat *youtube.LiveChatMessage) string {
	if chat.Snippet.Type == TextMessageEventString && chat.Snippet.TextMessageDetails != nil {
		return chat.Snippet.TextMessageDetails.MessageText
	}
	if chat.Snippet.Type == SuperChatEventString && chat.Snippet.SuperChatDetails != nil {
		return chat.Snippet.SuperChatDetails.UserComment
	}
	if chat.Snippet.Type == MemberMilestoneChatEvent && chat.Snippet.MemberMilestoneChatDetails != nil {
		return chat.Snippet.MemberMilestoneChatDetails.UserComment
	}
	return ""
}

func IsChatMessageByModerator(chat *youtube.LiveChatMessage) bool {
	return chat.AuthorDetails.IsChatModerator
}

func IsChatMessageByOwner(chat *youtube.LiveChatMessage) bool {
	return chat.AuthorDetails.IsChatOwner
}

func IsChatMessageByMember(chat *youtube.LiveChatMessage) bool {
	return chat.AuthorDetails.IsChatSponsor
}

func ExtractAuthorChannelId(chat *youtube.LiveChatMessage) string {
	return chat.AuthorDetails.ChannelId
}

func ExtractAuthorDisplayName(chat *youtube.LiveChatMessage) string {
	return chat.AuthorDetails.DisplayName
}
