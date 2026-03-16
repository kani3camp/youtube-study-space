package mock_moderatorbot

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination ./interface.go -package mock_moderatorbot app.modules/core/moderatorbot MessageBot
