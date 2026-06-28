package workspaceapp

import "slices"

type NGWordConfig struct {
	blockRegexesForChatMessage        []string
	blockRegexesForChannelName        []string
	notificationRegexesForChatMessage []string
	notificationRegexesForChannelName []string
}

func NewNGWordConfig(
	blockRegexesForChatMessage []string,
	blockRegexesForChannelName []string,
	notificationRegexesForChatMessage []string,
	notificationRegexesForChannelName []string,
) NGWordConfig {
	return NGWordConfig{
		blockRegexesForChatMessage:        slices.Clone(blockRegexesForChatMessage),
		blockRegexesForChannelName:        slices.Clone(blockRegexesForChannelName),
		notificationRegexesForChatMessage: slices.Clone(notificationRegexesForChatMessage),
		notificationRegexesForChannelName: slices.Clone(notificationRegexesForChannelName),
	}
}

func (c NGWordConfig) Count() int {
	return len(c.blockRegexesForChatMessage) +
		len(c.blockRegexesForChannelName) +
		len(c.notificationRegexesForChatMessage) +
		len(c.notificationRegexesForChannelName)
}
