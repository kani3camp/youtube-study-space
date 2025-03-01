package wordsreader

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"strconv"
	"strings"
)

type SpreadsheetReader struct {
	client                     *sheets.Service
	spreadsheetId              string
	blockRegexSheetName        string
	notificationRegexSheetName string
}

func NewSpreadsheetReader(
	ctx context.Context,
	clientOption option.ClientOption,
	spreadsheetId string,
	blockRegexSheetNamePrefix,
	notificationRegexSheetNamePrefix string,
) (*SpreadsheetReader, error) {
	service, err := sheets.NewService(ctx, clientOption)
	if err != nil {
		return nil, fmt.Errorf("in sheets.NewService: %w", err)
	}

	ss, err := service.Spreadsheets.Get(spreadsheetId).Do()
	if err != nil {
		return nil, fmt.Errorf("in service.Spreadsheets.Get: %w", err)
	}

	var blockRegexSheetName string
	var notificationRegexSheetName string
	for _, sheet := range ss.Sheets {
		if strings.HasPrefix(sheet.Properties.Title, blockRegexSheetNamePrefix) {
			blockRegexSheetName = sheet.Properties.Title
		}
		if strings.HasPrefix(sheet.Properties.Title, notificationRegexSheetNamePrefix) {
			notificationRegexSheetName = sheet.Properties.Title
		}
	}
	if blockRegexSheetName == "" {
		return nil, errors.New("failed to find blockRegexSheetName")
	}
	if notificationRegexSheetName == "" {
		return nil, errors.New("failed to find notificationRegexSheetName")
	}

	return &SpreadsheetReader{
		client:                     service,
		spreadsheetId:              spreadsheetId,
		blockRegexSheetName:        blockRegexSheetName,
		notificationRegexSheetName: notificationRegexSheetName,
	}, nil
}

func (sc *SpreadsheetReader) ReadBlockRegexes() (chatRegexes []string, channelRegexes []string, err error) {
	readRange := fmt.Sprintf("%s!A2:C999", sc.blockRegexSheetName) // 「有効, 文字列, チャンネル名にも適用」2行目スタート。999行目まで。
	resp, err := sc.client.Spreadsheets.Values.Get(sc.spreadsheetId, readRange).Do()
	if err != nil {
		return nil, nil, fmt.Errorf("in sc.client.Spreadsheets.Values.Get: %w", err)
	}

	for _, row := range resp.Values {
		if len(row) < 3 {
			continue
		}

		enabledStr, ok1 := row[0].(string)
		regex, ok2 := row[1].(string)
		applyForChannelNameStr, ok3 := row[2].(string)
		if !ok1 || !ok2 || !ok3 {
			// 型が予想通りでなければスキップ
			continue
		}
		enabled, err := strconv.ParseBool(enabledStr)
		if err != nil {
			continue
		}
		applyForChannelName, err := strconv.ParseBool(applyForChannelNameStr)
		if err != nil {
			continue
		}

		// 空文字や無効な設定はスキップ
		if regex == "" || !enabled {
			continue
		}

		chatRegexes = append(chatRegexes, regex)
		if applyForChannelName {
			channelRegexes = append(channelRegexes, regex)
		}
	}

	return
}

func (sc *SpreadsheetReader) ReadNotificationRegexes() (chatRegexes []string, channelRegexes []string, err error) {
	readRange := fmt.Sprintf("%s!A2:C999", sc.notificationRegexSheetName) // 「有効, 文字列, チャンネル名にも適用」2行目スタート。999行目まで。
	resp, err := sc.client.Spreadsheets.Values.Get(sc.spreadsheetId, readRange).Do()
	if err != nil {
		return nil, nil, fmt.Errorf("in sc.client.Spreadsheets.Values.Get: %w", err)
	}

	for _, row := range resp.Values {
		if len(row) < 3 {
			continue
		}

		enabledStr, ok1 := row[0].(string)
		regex, ok2 := row[1].(string)
		applyForChannelNameStr, ok3 := row[2].(string)
		if !ok1 || !ok2 || !ok3 {
			// 型が予想通りでなければスキップ
			continue
		}
		enabled, err := strconv.ParseBool(enabledStr)
		if err != nil {
			continue
		}
		applyForChannelName, err := strconv.ParseBool(applyForChannelNameStr)
		if err != nil {
			continue
		}

		// 空文字や無効な設定はスキップ
		if regex == "" || !enabled {
			continue
		}

		chatRegexes = append(chatRegexes, regex)
		if applyForChannelName {
			channelRegexes = append(channelRegexes, regex)
		}
	}

	return
}
