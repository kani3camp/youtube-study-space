package wordsreader

type WordsReader interface {
	ReadBlockRegexes() (chatRegexes []string, channelRegexes []string, err error)
	ReadNotificationRegexes() (chatRegexes []string, channelRegexes []string, err error)
}
