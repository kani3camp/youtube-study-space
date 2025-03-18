package workspaceapp

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"app.modules/core/i18n"
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (app *WorkspaceApp) Report(ctx context.Context, reportOption *utils.ReportOption) error {
	t := i18n.GetTFunc("command-report")
	if reportOption.Message == "" { // !reportのみは不可
		app.MessageToLiveChat(ctx, t("no-message", app.ProcessedUserDisplayName))
		return nil
	}

	ownerMessage := t("owner", utils.ReportCommand, app.ProcessedUserId, app.ProcessedUserDisplayName, reportOption.Message)
	app.MessageToOwner(ctx, ownerMessage)

	messageForModerators := t("moderators", utils.ReportCommand, app.ProcessedUserDisplayName, reportOption.Message)
	if err := app.MessageToModerators(ctx, messageForModerators); err != nil {
		app.MessageToOwnerWithError(ctx, "モデレーターへメッセージが送信できませんでした: \""+messageForModerators+"\"", err)
	}

	app.MessageToLiveChat(ctx, t("alert", app.ProcessedUserDisplayName))
	return nil
}

func (app *WorkspaceApp) Kick(ctx context.Context, kickOption *utils.KickOption) error {
	t := i18n.GetTFunc("command-kick")
	targetSeatId := kickOption.SeatId
	isTargetMemberSeat := kickOption.IsTargetMemberSeat
	var replyMessage string

	// commanderはモデレーターもしくはチャットオーナーか
	if !app.ProcessedUserIsModeratorOrOwner {
		app.MessageToLiveChat(ctx, i18n.T("command:permission", app.ProcessedUserDisplayName, utils.KickCommand))
		return nil
	}

	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// ターゲットの座席は誰か使っているか
		{
			isSeatAvailable, err := app.IfSeatVacant(ctx, tx, targetSeatId, isTargetMemberSeat)
			if err != nil {
				return fmt.Errorf("in IfSeatVacant(): %w", err)
			}
			if isSeatAvailable {
				replyMessage = i18n.T("command:unused", app.ProcessedUserDisplayName)
				return nil
			}
		}

		// ユーザーを強制退室させる
		targetSeat, err := app.Repository.ReadSeat(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = i18n.T("command:unused", app.ProcessedUserDisplayName)
				return nil
			}
			return fmt.Errorf("in ReadSeat: %w", err)
		}

		seatIdStr := utils.SeatIdStr(targetSeatId, isTargetMemberSeat)
		replyMessage = t("kick", app.ProcessedUserDisplayName, seatIdStr, targetSeat.UserDisplayName)

		// app.ProcessedUserが処理の対象ではないことに注意。
		userDoc, err := app.Repository.ReadUser(ctx, tx, targetSeat.UserId)
		if err != nil {
			return fmt.Errorf("in ReadUser: %w", err)
		}

		workedTimeSec, addedRP, exitErr := app.exitRoom(ctx, tx, isTargetMemberSeat, targetSeat, &userDoc)
		if exitErr != nil {
			return fmt.Errorf("%sさんのkick退室処理中にエラーが発生しました: %w", app.ProcessedUserDisplayName, exitErr)
		}
		var rpEarned string
		if userDoc.RankVisible {
			rpEarned = i18n.T("command:rp-earned", addedRP)
		}
		replyMessage += i18n.T("command:exit", targetSeat.UserDisplayName, workedTimeSec/60, seatIdStr, rpEarned)

		{
			err := app.LogToModerators(ctx, app.ProcessedUserDisplayName+"さん、"+strconv.Itoa(targetSeat.
				SeatId)+"番席のユーザーをkickしました。\n"+
				"チャンネル名: "+targetSeat.UserDisplayName+"\n"+
				"作業名: "+targetSeat.WorkName+"\n休憩中の作業名: "+targetSeat.BreakWorkName+"\n"+
				"入室時間: "+strconv.Itoa(workedTimeSec/60)+"分\n"+
				"チャンネルURL: https://youtube.com/channel/"+targetSeat.UserId)
			if err != nil {
				return fmt.Errorf("failed LogToModerators(): %w", err)
			}
		}
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Kick()", "txErr", txErr)
		replyMessage = i18n.T("command:error", app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (app *WorkspaceApp) Check(ctx context.Context, checkOption *utils.CheckOption) error {
	targetSeatId := checkOption.SeatId
	isTargetMemberSeat := checkOption.IsTargetMemberSeat

	var replyMessage string
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// commanderはモデレーターかチャットオーナーか
		if !app.ProcessedUserIsModeratorOrOwner {
			replyMessage = i18n.T("command:permission", app.ProcessedUserDisplayName, utils.CheckCommand)
			return nil
		}

		// ターゲットの座席は誰か使っているか
		{
			isSeatVacant, err := app.IfSeatVacant(ctx, tx, targetSeatId, isTargetMemberSeat)
			if err != nil {
				return fmt.Errorf("in IfSeatVacant: %w", err)
			}
			if isSeatVacant {
				replyMessage = i18n.T("command:unused", app.ProcessedUserDisplayName)
				return nil
			}
		}
		// 座席情報を表示する
		seat, err := app.Repository.ReadSeat(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = i18n.T("command:unused", app.ProcessedUserDisplayName)
				return nil
			}
			return fmt.Errorf("in ReadSeat: %w", err)
		}
		sinceMinutes := int(utils.NoNegativeDuration(utils.JstNow().Sub(seat.EnteredAt)).Minutes())
		untilMinutes := int(utils.NoNegativeDuration(seat.Until.Sub(utils.JstNow())).Minutes())
		var seatIdStr string
		if isTargetMemberSeat {
			seatIdStr = i18n.T("common:vip-seat-id", targetSeatId)
		} else {
			seatIdStr = strconv.Itoa(targetSeatId)
		}
		message := app.ProcessedUserDisplayName + "さん、" + seatIdStr + "番席のユーザー情報です。\n" +
			"チャンネル名: " + seat.UserDisplayName + "\n" + "入室時間: " + strconv.Itoa(sinceMinutes) + "分\n" +
			"作業名: " + seat.WorkName + "\n" + "休憩中の作業名: " + seat.BreakWorkName + "\n" +
			"自動退室まで" + strconv.Itoa(untilMinutes) + "分\n" +
			"チャンネルURL: https://youtube.com/channel/" + seat.UserId
		if err := app.LogToModerators(ctx, message); err != nil {
			return fmt.Errorf("failed LogToModerators(): %w", err)
		}
		replyMessage = i18n.T("command:sent", app.ProcessedUserDisplayName)
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Check()", "txErr", txErr)
		replyMessage = i18n.T("command:error", app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (app *WorkspaceApp) Block(ctx context.Context, blockOption *utils.BlockOption) error {
	targetSeatId := blockOption.SeatId
	isTargetMemberSeat := blockOption.IsTargetMemberSeat

	var replyMessage string
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// commanderはモデレーターかチャットオーナーか
		if !app.ProcessedUserIsModeratorOrOwner {
			replyMessage = app.ProcessedUserDisplayName + "さんは" + utils.BlockCommand + "コマンドを使用できません"
			return nil
		}

		// ターゲットの座席は誰か使っているか
		{
			isSeatAvailable, err := app.IfSeatVacant(ctx, tx, targetSeatId, isTargetMemberSeat)
			if err != nil {
				return fmt.Errorf("in IfSeatVacant(): %w", err)
			}
			if isSeatAvailable {
				replyMessage = app.ProcessedUserDisplayName + "さん、その番号の座席は誰も使用していません"
				return nil
			}
		}

		// ユーザーを強制退室させる
		targetSeat, err := app.Repository.ReadSeat(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = app.ProcessedUserDisplayName + "さん、その番号の座席は誰も使用していません"
				return nil
			}
			app.MessageToOwnerWithError(ctx, "in ReadSeat", err)
			return fmt.Errorf("in ReadSeat: %w", err)
		}
		replyMessage = app.ProcessedUserDisplayName + "さん、" + strconv.Itoa(targetSeat.SeatId) + "番席の" + targetSeat.UserDisplayName + "さんをブロックします。"

		// app.ProcessedUserが処理の対象ではないことに注意。
		userDoc, err := app.Repository.ReadUser(ctx, tx, targetSeat.UserId)
		if err != nil {
			return fmt.Errorf("in ReadUser: %w", err)
		}

		workedTimeSec, addedRP, exitErr := app.exitRoom(ctx, tx, isTargetMemberSeat, targetSeat, &userDoc)
		if exitErr != nil {
			return fmt.Errorf("%sさんの強制退室処理中にエラーが発生しました: %w", app.ProcessedUserDisplayName, exitErr)
		}
		var rpEarned string
		var seatIdStr string
		if userDoc.RankVisible {
			rpEarned = "（+ " + strconv.Itoa(addedRP) + " RP）"
		}
		if isTargetMemberSeat {
			seatIdStr = i18n.T("common:vip-seat-id", targetSeatId)
		} else {
			seatIdStr = strconv.Itoa(targetSeatId)
		}
		replyMessage = i18n.T("command:exit", targetSeat.UserDisplayName, workedTimeSec/60, seatIdStr, rpEarned)

		// ブロック
		if err := app.BanUser(ctx, targetSeat.UserId); err != nil {
			return fmt.Errorf("in BanUser: %w", err)
		}

		{
			err := app.LogToModerators(ctx, app.ProcessedUserDisplayName+"さん、"+strconv.Itoa(targetSeat.
				SeatId)+"番席のユーザーをblockしました。\n"+
				"チャンネル名: "+targetSeat.UserDisplayName+"\n"+
				"作業名: "+targetSeat.WorkName+"\n休憩中の作業名: "+targetSeat.BreakWorkName+"\n"+
				"入室時間: "+strconv.Itoa(workedTimeSec/60)+"分\n"+
				"チャンネルURL: https://youtube.com/channel/"+targetSeat.UserId)
			if err != nil {
				return fmt.Errorf("failed LogToModerators(): %w", err)
			}
		}
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Block()", "txErr", txErr)
		replyMessage = i18n.T("command:error", app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}
