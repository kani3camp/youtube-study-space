package mock_moderatorbot

//go:generate go run go.uber.org/mock/mockgen -destination ./interface.go -package mock_moderatorbot app.modules/core/moderatorbot MessageBot
