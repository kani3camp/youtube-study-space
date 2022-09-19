package myspreadsheet

import (
	"context"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"strings"
)

type SpreadsheetController struct {
	client                     *sheets.Service
	spreadsheetId              string
	blockRegexSheetName        string
	notificationRegexSheetName string
}

func NewSpreadsheetController(
	ctx context.Context,
	clientOption option.ClientOption,
	spreadsheetId string,
	blockRegexSheetNamePrefix,
	notificationRegexSheetNamePrefix string,
) (*SpreadsheetController, error) {
	service, err := sheets.NewService(ctx, clientOption)
	if err != nil {
		return nil, err
	}
	
	ss, err := service.Spreadsheets.Get(spreadsheetId).Do()
	if err != nil {
		return nil, err
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
	
	return &SpreadsheetController{
		client:                     service,
		spreadsheetId:              spreadsheetId,
		blockRegexSheetName:        blockRegexSheetName,
		notificationRegexSheetName: notificationRegexSheetName,
	}, nil
}

func (sc *SpreadsheetController) GetRegexForBlock() ([]string, []string, error) {
	readRange := sc.blockRegexSheetName + "!" + "A2:C999" // 「有効, 文字列, チャンネル名にも適用」2行目スタート。999行目まで。
	
	resp, err := sc.client.Spreadsheets.Values.Get(sc.spreadsheetId, readRange).Do()
	if err != nil {
		return nil, nil, err
	}
	
	var regexListForChatMessage []string
	var regexListForChannelName []string
	var regex string
	var enabled, applyForChannelName bool
	for _, row := range resp.Values {
		enabled = row[0] == "TRUE"
		regex = row[1].(string)
		applyForChannelName = row[2] == "TRUE"
		
		if regex == "" {
			continue // skip vacant cell
		}
		if !enabled {
			continue
		}
		
		regexListForChatMessage = append(regexListForChatMessage, regex)
		if applyForChannelName {
			regexListForChannelName = append(regexListForChannelName, regex)
		}
	}
	
	return regexListForChatMessage, regexListForChannelName, nil
}

func (sc *SpreadsheetController) GetRegexForNotification() ([]string, []string, error) {
	readRange := sc.notificationRegexSheetName + "!" + "A2:C999" // 「有効, 文字列, チャンネル名にも適用」2行目スタート。999行目まで。
	
	resp, err := sc.client.Spreadsheets.Values.Get(sc.spreadsheetId, readRange).Do()
	if err != nil {
		return nil, nil, err
	}
	
	var regexListForChatMessage []string
	var regexListForChannelName []string
	var regex string
	var enabled, applyForChannelName bool
	for _, row := range resp.Values {
		enabled = row[0] == "TRUE"
		regex = row[1].(string)
		applyForChannelName = row[2] == "TRUE"
		
		if regex == "" {
			continue // skip vacant cell
		}
		if !enabled {
			continue
		}
		
		regexListForChatMessage = append(regexListForChatMessage, regex)
		if applyForChannelName {
			regexListForChannelName = append(regexListForChannelName, regex)
		}
	}
	
	return regexListForChatMessage, regexListForChannelName, nil
}
