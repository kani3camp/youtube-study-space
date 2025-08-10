package workspaceapp

import (
	"context"
	"fmt"
	"log/slog"

	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/repository"
	"app.modules/core/utils"

	"cloud.google.com/go/firestore"
)

func (app *WorkspaceApp) ShowUserInfo(ctx context.Context, infoOption *utils.InfoOption) error {
	// no-op
	var replyMessage string
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		totalStudyDuration, dailyTotalStudyDuration, err := app.GetUserRealtimeTotalStudyDurations(ctx, tx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in app.GetUserRealtimeTotalStudyDurations(): %w", err)
		}
		dailyTotalTimeStr := utils.DurationToString(dailyTotalStudyDuration)
		totalTimeStr := utils.DurationToString(totalStudyDuration)
		replyMessage += i18nmsg.CommandUserInfoBase(app.ProcessedUserDisplayName, dailyTotalTimeStr, totalTimeStr)

		userDoc, err := app.Repository.ReadUser(ctx, tx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in app.Repository.ReadUser: %w", err)
		}

		if userDoc.RankVisible {
			replyMessage += i18nmsg.CommandUserInfoRank(userDoc.RankPoint)
		}

		if infoOption.ShowDetails {
			switch userDoc.RankVisible {
			case true:
				replyMessage += i18nmsg.CommandUserInfoRankOn()
			case false:
				replyMessage += i18nmsg.CommandUserInfoRankOff()
			}

			if userDoc.IsContinuousActive {
				continuousActiveDays := int(utils.JstNow().Sub(userDoc.CurrentActivityStateStarted).Hours() / 24)
				replyMessage += i18nmsg.CommandUserInfoRankOnContinuous(continuousActiveDays+1, continuousActiveDays)
			}

			if userDoc.DefaultStudyMin == 0 {
				replyMessage += i18nmsg.CommandUserInfoDefaultWorkOff()
			} else {
				replyMessage += i18nmsg.CommandUserInfoDefaultWork(userDoc.DefaultStudyMin)
			}

			if userDoc.FavoriteColor == "" {
				replyMessage += i18nmsg.CommandUserInfoFavoriteColorOff()
			} else {
				replyMessage += i18nmsg.CommandUserInfoFavoriteColor(utils.ColorCodeToColorName(userDoc.FavoriteColor))
			}

			replyMessage += i18nmsg.CommandUserInfoRegisterDate(userDoc.RegistrationDate.In(utils.JapanLocation()).Format("2006年01月02日"))
		}
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in ShowUserInfo()", "txErr", txErr)
		replyMessage = i18nmsg.CommandError(app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (app *WorkspaceApp) My(ctx context.Context, myOptions []utils.MyOption) error {
	// ユーザードキュメントはすでにあり、登録されていないプロパティだった場合、そのままプロパティを保存したら自動で作成される。
	// また、読み込みのときにそのプロパティがなくても大丈夫。自動で初期値が割り当てられる。
	// ただし、ユーザードキュメントがそもそもない場合は、書き込んでもエラーにはならないが、登録日が記録されないため、要登録。

	// オプションが1つ以上指定されているか？
	if len(myOptions) == 0 {
		app.MessageToLiveChat(ctx, i18nmsg.CommandOptionWarn(app.ProcessedUserDisplayName))
		return nil
	}

	// no-op

	replyMessage := ""
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 変更前のuserDocを読み込んでおく
		userDoc, err := app.Repository.ReadUser(ctx, tx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in ReadUser: %w", err)
		}

		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom

		var seats []repository.SeatDoc
		if isInMemberRoom {
			seats, err = app.Repository.ReadMemberSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadMemberSeats: %w", err)
			}
		}
		if isInGeneralRoom {
			seats, err = app.Repository.ReadGeneralSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadGeneralSeats: %w", err)
			}
		}
		realTimeTotalStudyDuration, _, err := app.GetUserRealtimeTotalStudyDurations(ctx, tx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in RetrieveRealtimeTotalStudyDuration: %w", err)
		}
		realTimeTotalStudySec := int(realTimeTotalStudyDuration.Seconds())

		// これ以降は書き込みのみ

		replyMessage = i18nmsg.CommonSir(app.ProcessedUserDisplayName)
		currentRankVisible := userDoc.RankVisible
		for _, myOption := range myOptions {
			if myOption.Type == utils.RankVisible {
				newRankVisible := myOption.BoolValue
				// 現在の値と、設定したい値が同じなら、変更なし
				if userDoc.RankVisible == newRankVisible {
					var rankVisibleStr string
					if userDoc.RankVisible {
						rankVisibleStr = i18nmsg.CommonOn()
					} else {
						rankVisibleStr = i18nmsg.CommonOff()
					}
					replyMessage += i18nmsg.CommandMyAlreadyRank(rankVisibleStr)
				} else { // 違うなら、切替
					if err := app.Repository.UpdateUserRankVisible(tx, app.ProcessedUserId, newRankVisible); err != nil {
						return fmt.Errorf("in UpdateUserRankVisible: %w", err)
					}
					var newValueStr string
					if newRankVisible {
						newValueStr = i18nmsg.CommonOn()
					} else {
						newValueStr = i18nmsg.CommonOff()
					}
					replyMessage += i18nmsg.CommandMySetRank(newValueStr)

					// 入室中であれば、座席の色も変える
					if isInRoom {
						seatAppearance, err := utils.GetSeatAppearance(realTimeTotalStudySec, newRankVisible, userDoc.RankPoint, userDoc.FavoriteColor)
						if err != nil {
							return fmt.Errorf("in GetSeatAppearance: %w", err)
						}

						// 席の色を更新
						newSeat, err := utils.GetSeatByUserId(seats, app.ProcessedUserId)
						if err != nil {
							return fmt.Errorf("in GetSeatByUserId: %w", err)
						}
						newSeat.Appearance = seatAppearance
						if err := app.Repository.UpdateSeat(ctx, tx, newSeat, isInMemberRoom); err != nil {
							return fmt.Errorf("in app.Repository.UpdateSeats: %w", err)
						}
					}
				}
				currentRankVisible = newRankVisible
			} else if myOption.Type == utils.DefaultStudyMin {
				if err := app.Repository.UpdateUserDefaultStudyMin(tx, app.ProcessedUserId, myOption.IntValue); err != nil {
					return fmt.Errorf("in UpdateUserDefaultStudyMin: %w", err)
				}
				// 値が0はリセットのこと。
				if myOption.IntValue == 0 {
					replyMessage += i18nmsg.CommandMyResetDefaultWork()
				} else {
					replyMessage += i18nmsg.CommandMySetDefaultWork(myOption.IntValue)
				}
			} else if myOption.Type == utils.FavoriteColor {
				// 値が""はリセットのこと。
				colorCode := utils.ColorNameToColorCode(myOption.StringValue)
				if err := app.Repository.UpdateUserFavoriteColor(tx, app.ProcessedUserId, colorCode); err != nil {
					return fmt.Errorf("in UpdateUserFavoriteColor: %w", err)
				}
				replyMessage += i18nmsg.CommandMySetFavoriteColor()
				if !utils.CanUseFavoriteColor(realTimeTotalStudySec) {
					replyMessage += i18nmsg.CommandMyAlertFavoriteColor(utils.FavoriteColorAvailableThresholdHours)
				}

				// 入室中であれば、座席の色も変える
				if isInRoom {
					newSeat, err := utils.GetSeatByUserId(seats, app.ProcessedUserId)
					if err != nil {
						return fmt.Errorf("in GetSeatByUserId: %w", err)
					}
					seatAppearance, err := utils.GetSeatAppearance(realTimeTotalStudySec, currentRankVisible, userDoc.RankPoint, colorCode)
					if err != nil {
						return fmt.Errorf("in GetSeatAppearance: %w", err)
					}

					// 席の色を更新
					newSeat.Appearance = seatAppearance
					if err := app.Repository.UpdateSeat(ctx, tx, newSeat, isInMemberRoom); err != nil {
						return fmt.Errorf("in app.Repository.UpdateSeat(): %w", err)
					}
				}
			}
		}
		return nil
	})
	if txErr != nil {
		slog.Error("txErr in My()", "txErr", txErr)
		replyMessage = i18nmsg.CommandError(app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}

func (app *WorkspaceApp) Rank(ctx context.Context, _ *utils.CommandDetails) error {
	replyMessage := ""
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 変更前のuserDocを読み込んでおく
		userDoc, err := app.Repository.ReadUser(ctx, tx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("in ReadUser: %w", err)
		}

		isInMemberRoom, isInGeneralRoom, err := app.IsUserInRoom(ctx, app.ProcessedUserId)
		if err != nil {
			return fmt.Errorf("failed IsUserInRoom: %w", err)
		}
		isInRoom := isInMemberRoom || isInGeneralRoom

		var currentSeat repository.SeatDoc
		var realtimeTotalStudySec int
		if isInRoom {
			var err error
			currentSeat, err = app.CurrentSeat(ctx, app.ProcessedUserId, isInMemberRoom)
			if err != nil {
				return fmt.Errorf("failed app.CurrentSeat(): %w", err)
			}

			realtimeTotalStudyDuration, _, err := app.GetUserRealtimeTotalStudyDurations(ctx, tx, app.ProcessedUserId)
			if err != nil {
				return fmt.Errorf("in RetrieveRealtimeTotalStudyDuration: %w", err)
			}
			realtimeTotalStudySec = int(realtimeTotalStudyDuration.Seconds())
		}

		// 以降書き込みのみ

		// ランク表示設定のON/OFFを切り替える
		newRankVisible := !userDoc.RankVisible
		if err := app.Repository.UpdateUserRankVisible(tx, app.ProcessedUserId, newRankVisible); err != nil {
			return fmt.Errorf("in UpdateUserRankVisible: %w", err)
		}
		var newValueStr string
		if newRankVisible {
			newValueStr = i18nmsg.CommonOn()
		} else {
			newValueStr = i18nmsg.CommonOff()
		}
		replyMessage = i18nmsg.CommandRank(app.ProcessedUserDisplayName, newValueStr)

		// 入室中であれば、座席の色も変える
		if isInRoom {
			seatAppearance, err := utils.GetSeatAppearance(realtimeTotalStudySec, newRankVisible, userDoc.RankPoint, userDoc.FavoriteColor)
			if err != nil {
				return fmt.Errorf("in GetSeatAppearance: %w", err)
			}

			// 席の色を更新
			currentSeat.Appearance = seatAppearance
			if err := app.Repository.UpdateSeat(ctx, tx, currentSeat, isInMemberRoom); err != nil {
				return fmt.Errorf("in app.Repository.UpdateSeat(): %w", err)
			}
		}

		return nil
	})
	if txErr != nil {
		slog.Error("txErr in Rank()", "txErr", txErr)
		replyMessage = i18nmsg.CommandError(app.ProcessedUserDisplayName)
	}
	app.MessageToLiveChat(ctx, replyMessage)
	return txErr
}
