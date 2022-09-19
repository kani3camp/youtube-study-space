package myspreadsheet

import (
	"context"
	"golang.org/x/oauth2"
	"google.golang.org/api/sheets/v4"
)

type SpreadSheetController struct {
	Client *
}


func NewSpreadSheetController(ctx context.Context) (*SpreadSheetController, error) {
	service, err = sheets.NewService(ctx, )
	
}