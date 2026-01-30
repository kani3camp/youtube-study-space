package wordsreader

import "context"

type WordsReader interface {
	ReadBlockRegexes(ctx context.Context) (chatRegexes []string, channelRegexes []string, err error)
	ReadNotificationRegexes(ctx context.Context) (chatRegexes []string, channelRegexes []string, err error)
}
