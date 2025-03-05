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

func (s *WorkspaceApp) Report(ctx context.Context, reportOption *utils.ReportOption) error {
	t := i18n.GetTFunc("command-report")
	if reportOption.Message == "" { // !reportのみは不可
		s.MessageToLiveChat(ctx, t("no-message", s.ProcessedUserDisplayName))
		return nil
	}

	ownerMessage := t("owner", utils.ReportCommand, s.ProcessedUserId, s.ProcessedUserDisplayName, reportOption.Message)
	s.MessageToOwner(ctx, ownerMessage)

	messageForModerators := t("moderators", utils.ReportCommand, s.ProcessedUserDisplayName, reportOption.Message)
	if err := s.MessageToModerators(ctx, messageForModerators); err != nil {
		s.MessageToOwnerWithError(ctx, "モデレーターへメッセージが送信できませんでした: \""+messageForModerators+"\"", err)
	}

	s.MessageToLiveChat(ctx, t("alert", s.ProcessedUserDisplayName))
	return nil
}

func (s *WorkspaceApp) Kick(ctx context.Context, kickOption *utils.KickOption) error {
	t := i18n.GetTFunc("command-kick")
	targetSeatId := kickOption.SeatId
	isTargetMemberSeat := kickOption.IsTargetMemberSeat
	var replyMessage string

	// commanderはモデレーターもしくはチャットオーナーか
	if !s.ProcessedUserIsModeratorOrOwner {
		s.MessageToLiveChat(ctx, i18n.T("command:permission", s.ProcessedUserDisplayName, utils.KickCommand))
		return nil
	}

	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// ターゲットの座席は誰か使っているか
		{
			isSeatAvailable, err := s.IfSeatVacant(ctx, tx, targetSeatId, isTargetMemberSeat)
			if err != nil {
				return fmt.Errorf("in IfSeatVacant(): %w", err)
			}
			if isSeatAvailable {
				replyMessage = i18n.T("command:unused", s.ProcessedUserDisplayName)
				return nil
			}
		}

		// ユーザーを強制退室させる
		targetSeat, err := s.Repository.ReadSeat(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = i18n.T("command:unused", s.ProcessedUserDisplayName)
				return nil
			}
			return fmt.Errorf("in ReadSeat: %w", err)
		}

		seatIdStr := utils.SeatIdStr(targetSeatId, isTargetMemberSeat)
		replyMessage = t("kick", s.ProcessedUserDisplayName, seatIdStr, targetSeat.UserDisplayName)

		// s.ProcessedUserが処理の対象ではないことに注意。
		userDoc, err := s.Repository.ReadUser(ctx, tx, targetSeat.UserId)
		if err != nil {
			return fmt.Errorf("in ReadUser: %w", err)
		}

		workedTimeSec, addedRP, exitErr := s.exitRoom(ctx, tx, isTargetMemberSeat, targetSeat, &userDoc)
		if exitErr != nil {
			return fmt.Errorf("%sさんのkick退室処理中にエラーが発生しました: %w", s.ProcessedUserDisplayName, exitErr)
		}
		var rpEarned string
		if userDoc.RankVisible {
			rpEarned = i18n.T("command:rp-earned", addedRP)
		}
		replyMessage += i18n.T("command:exit", targetSeat.UserDisplayName, workedTimeSec/60, seatIdStr, rpEarned)

		{
			err := s.LogToModerators(ctx, s.ProcessedUserDisplayName+"さん、"+strconv.Itoa(targetSeat.
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
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Check(ctx context.Context, checkOption *utils.CheckOption) error {
	targetSeatId := checkOption.SeatId
	isTargetMemberSeat := checkOption.IsTargetMemberSeat

	var replyMessage string
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// commanderはモデレーターかチャットオーナーか
		if !s.ProcessedUserIsModeratorOrOwner {
			replyMessage = i18n.T("command:permission", s.ProcessedUserDisplayName, utils.CheckCommand)
			return nil
		}

		// ターゲットの座席は誰か使っているか
		{
			isSeatVacant, err := s.IfSeatVacant(ctx, tx, targetSeatId, isTargetMemberSeat)
			if err != nil {
				return fmt.Errorf("in IfSeatVacant: %w", err)
			}
			if isSeatVacant {
				replyMessage = i18n.T("command:unused", s.ProcessedUserDisplayName)
				return nil
			}
		}
		// 座席情報を表示する
		seat, err := s.Repository.ReadSeat(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = i18n.T("command:unused", s.ProcessedUserDisplayName)
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
		message := s.ProcessedUserDisplayName + "さん、" + seatIdStr + "番席のユーザー情報です。\n" +
			"チャンネル名: " + seat.UserDisplayName + "\n" + "入室時間: " + strconv.Itoa(sinceMinutes) + "分\n" +
			"作業名: " + seat.WorkName + "\n" + "休憩中の作業名: " + seat.BreakWorkName + "\n" +
			"自動退室まで" + strconv.Itoa(untilMinutes) + "分\n" +
			"チャンネルURL: https://youtube.com/channel/" + seat.UserId
		if err := s.LogToModerators(ctx, message); err != nil {
			return fmt.Errorf("failed LogToModerators(): %w", err)
		}
		replyMessage = i18n.T("command:sent", s.ProcessedUserDisplayName)
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Check()", "txErr", txErr)
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (s *WorkspaceApp) Block(ctx context.Context, blockOption *utils.BlockOption) error {
	targetSeatId := blockOption.SeatId
	isTargetMemberSeat := blockOption.IsTargetMemberSeat

	var replyMessage string
	txErr := s.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// commanderはモデレーターかチャットオーナーか
		if !s.ProcessedUserIsModeratorOrOwner {
			replyMessage = s.ProcessedUserDisplayName + "さんは" + utils.BlockCommand + "コマンドを使用できません"
			return nil
		}

		// ターゲットの座席は誰か使っているか
		{
			isSeatAvailable, err := s.IfSeatVacant(ctx, tx, targetSeatId, isTargetMemberSeat)
			if err != nil {
				return fmt.Errorf("in IfSeatVacant(): %w", err)
			}
			if isSeatAvailable {
				replyMessage = s.ProcessedUserDisplayName + "さん、その番号の座席は誰も使用していません"
				return nil
			}
		}

		// ユーザーを強制退室させる
		targetSeat, err := s.Repository.ReadSeat(ctx, tx, targetSeatId, isTargetMemberSeat)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				replyMessage = s.ProcessedUserDisplayName + "さん、その番号の座席は誰も使用していません"
				return nil
			}
			s.MessageToOwnerWithError(ctx, "in ReadSeat", err)
			return fmt.Errorf("in ReadSeat: %w", err)
		}
		replyMessage = s.ProcessedUserDisplayName + "さん、" + strconv.Itoa(targetSeat.SeatId) + "番席の" + targetSeat.UserDisplayName + "さんをブロックします。"

		// s.ProcessedUserが処理の対象ではないことに注意。
		userDoc, err := s.Repository.ReadUser(ctx, tx, targetSeat.UserId)
		if err != nil {
			return fmt.Errorf("in ReadUser: %w", err)
		}

		workedTimeSec, addedRP, exitErr := s.exitRoom(ctx, tx, isTargetMemberSeat, targetSeat, &userDoc)
		if exitErr != nil {
			return fmt.Errorf("%sさんの強制退室処理中にエラーが発生しました: %w", s.ProcessedUserDisplayName, exitErr)
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
		if err := s.BanUser(ctx, targetSeat.UserId); err != nil {
			return fmt.Errorf("in BanUser: %w", err)
		}

		{
			err := s.LogToModerators(ctx, s.ProcessedUserDisplayName+"さん、"+strconv.Itoa(targetSeat.
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
		replyMessage = i18n.T("command:error", s.ProcessedUserDisplayName)
	}
	s.MessageToLiveChat(ctx, replyMessage)
	return txErr
}
