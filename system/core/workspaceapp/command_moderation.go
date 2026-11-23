package workspaceapp

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp/presenter"
)

func (app *WorkspaceApp) Report(ctx context.Context, reportOption *utils.ReportOption) error {
	if reportOption.Message == "" { // !reportのみは不可
		app.MessageToLiveChat(ctx, i18nmsg.CommandReportNoMessage(app.ProcessedUserDisplayName))
		return nil
	}

	ownerMessage := i18nmsg.CommandReportOwner(utils.ReportCommand, app.ProcessedUserID, app.ProcessedUserDisplayName, reportOption.Message)
	app.MessageToOwner(ctx, ownerMessage)

	messageForModerators := i18nmsg.CommandReportModerators(utils.ReportCommand, app.ProcessedUserDisplayName, reportOption.Message)
	if err := app.MessageToModerators(ctx, messageForModerators); err != nil {
		app.MessageToOwnerWithError(ctx, "モデレーターへメッセージが送信できませんでした: \""+messageForModerators+"\"", err)
	}

	app.MessageToLiveChat(ctx, i18nmsg.CommandReportAlert(app.ProcessedUserDisplayName))
	return nil
}

func (app *WorkspaceApp) Kick(ctx context.Context, kickOption *utils.KickOption) error {
	targetSeatID := kickOption.SeatID
	isTargetMemberSeat := kickOption.IsTargetMemberSeat
	var replyMessage string

	// commanderはモデレーターもしくはチャットオーナーか
	if !app.ProcessedUserIsModeratorOrOwner {
		app.MessageToLiveChat(ctx, i18nmsg.CommandPermission(app.ProcessedUserDisplayName, utils.KickCommand))
		return nil
	}

	var moderatorLog string // コミット後にモデレーターへ送るログ。空なら送らない。
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// リトライされても前の試行の結果を持ち越さない
		replyMessage = ""
		moderatorLog = ""

		// ターゲットの座席は誰か使っているか
		{
			isSeatAvailable, err := app.IfSeatVacant(ctx, tx, targetSeatID, isTargetMemberSeat)
			if err != nil {
				return fmt.Errorf("in IfSeatVacant(): %w", err)
			}
			if isSeatAvailable {
				replyMessage = i18nmsg.CommandUnused(app.ProcessedUserDisplayName)
				return nil
			}
		}

		// ユーザーを強制退室させる
		targetSeat, err := app.Repository.ReadSeat(ctx, tx, targetSeatID, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = i18nmsg.CommandUnused(app.ProcessedUserDisplayName)
				return nil
			}
			return fmt.Errorf("in ReadSeat: %w", err)
		}

		workSegments, err := app.Repository.ReadWorkStateSegmentsBySessionID(ctx, targetSeat.SessionID)
		if err != nil {
			return fmt.Errorf("in ReadWorkStateSegmentsBySessionID: %w", err)
		}

		seatIDStr := presenter.SeatIDStr(targetSeatID, isTargetMemberSeat)
		replyMessage = i18nmsg.CommandKickKick(app.ProcessedUserDisplayName, seatIDStr, targetSeat.UserDisplayName)

		// app.ProcessedUserが処理の対象ではないことに注意。
		userDoc, err := app.Repository.ReadUser(ctx, tx, targetSeat.UserID)
		if err != nil {
			return fmt.Errorf("in ReadUser: %w", err)
		}

		workedTimeSec, addedRP, exitErr := app.exitRoom(ctx, tx, isTargetMemberSeat, targetSeat, &userDoc, workSegments)
		if exitErr != nil {
			return fmt.Errorf("%sさんのkick退室処理中にエラーが発生しました: %w", app.ProcessedUserDisplayName, exitErr)
		}
		var rpEarned string
		if userDoc.RankVisible {
			rpEarned = i18nmsg.CommandRpEarned(addedRP)
		}
		replyMessage += i18nmsg.CommandExit(targetSeat.UserDisplayName, workedTimeSec/60, seatIDStr, rpEarned)

		moderatorLog = app.ProcessedUserDisplayName + "さん、" + strconv.Itoa(targetSeat.
			SeatID) + "番席のユーザーをkickしました。\n" +
			"チャンネル名: " + targetSeat.UserDisplayName + "\n" +
			"作業名: " + targetSeat.WorkName + "\n" +
			"入室時間: " + strconv.Itoa(workedTimeSec/60) + "分\n" +
			"チャンネルURL: https://youtube.com/channel/" + targetSeat.UserID
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Kick()", "txErr", txErr)
		app.MessageToLiveChat(ctx, i18nmsg.CommandError(app.ProcessedUserDisplayName))
		return txErr
	}
	// 退室はコミット済みなので、ログ送信に失敗しても返信は退室結果のままにする
	var logErr error
	if moderatorLog != "" {
		if err := app.LogToModerators(ctx, moderatorLog); err != nil {
			logErr = fmt.Errorf("failed LogToModerators(): %w", err)
		}
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return logErr
}

func (app *WorkspaceApp) Check(ctx context.Context, checkOption *utils.CheckOption) error {
	jstNow := app.currentTime()
	targetSeatID := checkOption.SeatID
	isTargetMemberSeat := checkOption.IsTargetMemberSeat

	var replyMessage string
	var moderatorLog string // コミット後にモデレーターへ送る座席情報。空なら送らない。
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// リトライされても前の試行の結果を持ち越さない
		replyMessage = ""
		moderatorLog = ""

		// commanderはモデレーターかチャットオーナーか
		if !app.ProcessedUserIsModeratorOrOwner {
			replyMessage = i18nmsg.CommandPermission(app.ProcessedUserDisplayName, utils.CheckCommand)
			return nil
		}

		// ターゲットの座席は誰か使っているか
		{
			isSeatVacant, err := app.IfSeatVacant(ctx, tx, targetSeatID, isTargetMemberSeat)
			if err != nil {
				return fmt.Errorf("in IfSeatVacant: %w", err)
			}
			if isSeatVacant {
				replyMessage = i18nmsg.CommandUnused(app.ProcessedUserDisplayName)
				return nil
			}
		}
		// 座席情報を表示する
		seat, err := app.Repository.ReadSeat(ctx, tx, targetSeatID, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = i18nmsg.CommandUnused(app.ProcessedUserDisplayName)
				return nil
			}
			return fmt.Errorf("in ReadSeat: %w", err)
		}
		sinceMinutes := int(timeutil.NoNegativeDuration(jstNow.Sub(seat.EnteredAt)).Minutes())
		untilMinutes := seat.RemainingWorkMin(jstNow)
		seatIDStr := presenter.SeatIDStr(targetSeatID, isTargetMemberSeat)
		moderatorLog = app.ProcessedUserDisplayName + "さん、" + seatIDStr + "番席のユーザー情報です。\n" +
			"チャンネル名: " + seat.UserDisplayName + "\n" + "入室時間: " + strconv.Itoa(sinceMinutes) + "分\n" +
			"作業名: " + seat.WorkName + "\n" +
			"自動退室まで" + strconv.Itoa(untilMinutes) + "分\n" +
			"チャンネルURL: https://youtube.com/channel/" + seat.UserID
		replyMessage = i18nmsg.CommandSent(app.ProcessedUserDisplayName)
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Check()", "txErr", txErr)
		app.MessageToLiveChat(ctx, i18nmsg.CommandError(app.ProcessedUserDisplayName))
		return txErr
	}
	if moderatorLog != "" {
		if err := app.LogToModerators(ctx, moderatorLog); err != nil {
			app.MessageToLiveChat(ctx, i18nmsg.CommandError(app.ProcessedUserDisplayName))
			return fmt.Errorf("failed LogToModerators(): %w", err)
		}
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return nil
}

func (app *WorkspaceApp) Block(ctx context.Context, blockOption *utils.BlockOption) error {
	targetSeatID := blockOption.SeatID
	isTargetMemberSeat := blockOption.IsTargetMemberSeat

	var replyMessage string
	var bannedUserID string // コミット後にブロックするユーザー。空ならブロックしない。
	var moderatorLog string
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// リトライされても前の試行の結果を持ち越さない
		replyMessage = ""
		bannedUserID = ""
		moderatorLog = ""

		// commanderはモデレーターかチャットオーナーか
		if !app.ProcessedUserIsModeratorOrOwner {
			replyMessage = i18nmsg.CommandPermission(app.ProcessedUserDisplayName, utils.BlockCommand)
			return nil
		}

		// ターゲットの座席は誰か使っているか
		{
			isSeatAvailable, err := app.IfSeatVacant(ctx, tx, targetSeatID, isTargetMemberSeat)
			if err != nil {
				return fmt.Errorf("in IfSeatVacant(): %w", err)
			}
			if isSeatAvailable {
				replyMessage = i18nmsg.CommandUnused(app.ProcessedUserDisplayName)
				return nil
			}
		}

		// ユーザーを強制退室させる
		targetSeat, err := app.Repository.ReadSeat(ctx, tx, targetSeatID, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = i18nmsg.CommandUnused(app.ProcessedUserDisplayName)
				return nil
			}
			return fmt.Errorf("in ReadSeat: %w", err)
		}
		seatIDStr := presenter.SeatIDStr(targetSeatID, isTargetMemberSeat)
		replyMessage = i18nmsg.CommandBlockBlock(app.ProcessedUserDisplayName, seatIDStr, targetSeat.UserDisplayName)

		// app.ProcessedUserが処理の対象ではないことに注意。
		userDoc, err := app.Repository.ReadUser(ctx, tx, targetSeat.UserID)
		if err != nil {
			return fmt.Errorf("in ReadUser: %w", err)
		}

		workSegments, err := app.Repository.ReadWorkStateSegmentsBySessionID(ctx, targetSeat.SessionID)
		if err != nil {
			return fmt.Errorf("in ReadWorkStateSegmentsBySessionID: %w", err)
		}

		workedTimeSec, addedRP, exitErr := app.exitRoom(ctx, tx, isTargetMemberSeat, targetSeat, &userDoc, workSegments)
		if exitErr != nil {
			return fmt.Errorf("%sさんの強制退室処理中にエラーが発生しました: %w", app.ProcessedUserDisplayName, exitErr)
		}
		var rpEarned string
		if userDoc.RankVisible {
			rpEarned = i18nmsg.CommandRpEarned(addedRP)
		}
		replyMessage += i18nmsg.CommandExit(targetSeat.UserDisplayName, workedTimeSec/60, seatIDStr, rpEarned)

		bannedUserID = targetSeat.UserID
		moderatorLog = app.ProcessedUserDisplayName + "さん、" + strconv.Itoa(targetSeat.
			SeatID) + "番席のユーザーをblockしました。\n" +
			"チャンネル名: " + targetSeat.UserDisplayName + "\n" +
			"作業名: " + targetSeat.WorkName + "\n" +
			"入室時間: " + strconv.Itoa(workedTimeSec/60) + "分\n" +
			"チャンネルURL: https://youtube.com/channel/" + targetSeat.UserID
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Block()", "txErr", txErr)
		app.MessageToLiveChat(ctx, i18nmsg.CommandError(app.ProcessedUserDisplayName))
		return txErr
	}
	// YouTube APIによるブロックはロールバックできないため、退室のコミット後に1回だけ行う。
	// 退室はコミット済みで同じ座席への!blockは再実行できないため、ブロックに失敗した場合は
	// 対象のチャンネルIDを含むエラーを呼び出し元へ返し、管理者が手動でブロックできるようにする。
	if bannedUserID != "" {
		if err := app.BanUser(ctx, bannedUserID); err != nil {
			app.MessageToLiveChat(ctx, i18nmsg.CommandError(app.ProcessedUserDisplayName))
			return fmt.Errorf("ban user %s after exit was committed: %w", bannedUserID, err)
		}
	}
	var logErr error
	if moderatorLog != "" {
		if err := app.LogToModerators(ctx, moderatorLog); err != nil {
			logErr = fmt.Errorf("failed LogToModerators(): %w", err)
		}
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return logErr
}
