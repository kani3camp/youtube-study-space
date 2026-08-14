package workspaceapp

import (
	"context"
	"errors"

	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/utils"
)

// ProcessMessage 入力コマンドを解析して実行
func (app *WorkspaceApp) ProcessMessage(
	ctx context.Context,
	ngWordConfig NGWordConfig,
	commandString string,
	userID string,
	userDisplayName string,
	userProfileImageURL string,
	isChatModerator bool,
	isChatOwner bool,
	isChatMember bool,
) error {
	prepared, err := app.prepareMessage(
		ctx,
		ngWordConfig,
		commandString,
		userID,
		userDisplayName,
		userProfileImageURL,
		isChatModerator,
		isChatOwner,
		isChatMember,
	)
	if err != nil {
		app.MessageToLiveChat(ctx, i18nmsg.CommandError(app.ProcessedUserDisplayName))
		return err
	}
	if prepared.ImmediateReply != "" {
		app.MessageToLiveChat(ctx, prepared.ImmediateReply)
	}
	if prepared.SkipExecution {
		return nil
	}
	return app.executeCommand(ctx, prepared.CommandDetails, commandString)
}

// executeCommand 解析済みのコマンドを実行する
func (app *WorkspaceApp) executeCommand(ctx context.Context, commandDetails *utils.CommandDetails, commandString string) error {
	// commandDetailsに基づいて命令処理
	switch commandDetails.CommandType {
	case utils.NotCommand:
		return nil
	case utils.InvalidCommand:
		return nil
	case utils.In:
		return app.In(ctx, &commandDetails.InOption)
	case utils.Out:
		return app.Out(ctx)
	case utils.Info:
		return app.ShowUserInfo(ctx, &commandDetails.InfoOption)
	case utils.My:
		return app.My(ctx, commandDetails.MyOptions)
	case utils.Change:
		return app.Change(ctx, &commandDetails.ChangeOption)
	case utils.Seat:
		return app.ShowSeatInfo(ctx, &commandDetails.SeatOption)
	case utils.Report:
		return app.Report(ctx, &commandDetails.ReportOption)
	case utils.Kick:
		return app.Kick(ctx, &commandDetails.KickOption)
	case utils.Check:
		return app.Check(ctx, &commandDetails.CheckOption)
	case utils.Block:
		return app.Block(ctx, &commandDetails.BlockOption)
	case utils.More:
		return app.More(ctx, &commandDetails.MoreOption)
	case utils.Break:
		return app.Break(ctx, &commandDetails.BreakOption)
	case utils.Resume:
		return app.Resume(ctx, &commandDetails.ResumeOption)
	case utils.Rank:
		return app.Rank(ctx, commandDetails)
	case utils.Order:
		return app.Order(ctx, &commandDetails.OrderOption)
	case utils.Clear:
		return app.Clear(ctx)
	default:
		return errors.New("Unknown command: " + commandString)
	}
}
