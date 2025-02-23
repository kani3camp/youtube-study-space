package moderatorbot

type MessageBot interface {
	SendMessage(message string) error
	SendMessageWithError(message string, err error) error
}
