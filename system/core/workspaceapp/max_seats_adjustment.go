package workspaceapp

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"app.modules/core/repository"
	"app.modules/core/utils"
)

// AdjustMaxSeats 一般席とメンバー席の数を調整する
func (app *WorkspaceApp) AdjustMaxSeats(ctx context.Context) error {
	slog.Info(utils.NameOf(app.AdjustMaxSeats))
	// UpdateDesiredMaxSeats()などはLambdaからも並列で実行される可能性があるが、競合が起こってもそこまで深刻な問題にはならないためトランザクションは使用しない。

	constants, err := app.Repository.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return fmt.Errorf("in ReadSystemConstantsConfig(): %w", err)
	}

	// 一般席の調整
	if err := app.adjustGeneralSeats(ctx, constants); err != nil {
		return fmt.Errorf("in adjustGeneralSeats(): %w", err)
	}

	// メンバー席の調整
	if err := app.adjustMemberSeats(ctx, constants); err != nil {
		return fmt.Errorf("in adjustMemberSeats(): %w", err)
	}

	return nil
}

// adjustGeneralSeats 一般席の数を調整する
func (app *WorkspaceApp) adjustGeneralSeats(ctx context.Context, constants repository.ConstantsConfigDoc) error {
	// 一般席
	if constants.DesiredMaxSeats > constants.MaxSeats { // 一般席を増やす
		app.MessageToLiveChat(ctx, "席を増やします↗")
		if err := app.Repository.UpdateMaxSeats(ctx, nil, constants.DesiredMaxSeats); err != nil {
			return fmt.Errorf("in UpdateMaxSeats(): %w", err)
		}
	} else if constants.DesiredMaxSeats < constants.MaxSeats { // 一般席を減らす
		if constants.FixedMaxSeatsEnabled { // 空席率に関係なく、max_seatsをdesiredに合わせる
			// なくなる座席にいるユーザーは退出させる
			seats, err := app.Repository.ReadGeneralSeats(ctx)
			if err != nil {
				return err
			}
			app.MessageToLiveChat(ctx, "座席数を"+strconv.Itoa(constants.DesiredMaxSeats)+"に固定します↘ 必要な場合は退出してもらうことがあります。")
			for _, seat := range seats {
				if seat.SeatId > constants.DesiredMaxSeats {
					app.SetProcessedUser(seat.UserId, seat.UserDisplayName, seat.UserProfileImageUrl, false, false, false)
					// 退出させる
					if err := app.Out(ctx); err != nil {
						return fmt.Errorf("in Out(): %w", err)
					}
				}
			}

			// max_seatsを更新
			if err := app.Repository.UpdateMaxSeats(ctx, nil, constants.DesiredMaxSeats); err != nil {
				return fmt.Errorf("in UpdateMaxSeats(): %w", err)
			}
		} else {
			// max_seatsを減らしても、空席率が設定値以上か確認
			seats, err := app.Repository.ReadGeneralSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadGeneralSeats(): %w", err)
			}
			if int(float32(constants.DesiredMaxSeats)*(1.0-constants.MinVacancyRate)) < len(seats) {
				slog.Info("減らそうとしすぎ。desiredは却下します。",
					"desired", constants.DesiredMaxSeats,
					"current max seats", constants.MaxSeats,
					"current seats length", len(seats))
				if err := app.Repository.UpdateDesiredMaxSeats(ctx, nil, constants.MaxSeats); err != nil {
					return fmt.Errorf("in UpdateDesiredMaxSeats(): %w", err)
				}
			} else {
				// 消えてしまう席にいるユーザーを移動させる
				app.MessageToLiveChat(ctx, "人数が減ったため席を減らします↘ 必要な場合は席を移動してもらうことがあります。")
				for _, seat := range seats {
					if seat.SeatId > constants.DesiredMaxSeats {
						app.SetProcessedUser(seat.UserId, seat.UserDisplayName, seat.UserProfileImageUrl, false, false, false)
						// 移動させる
						inCommandDetails := &utils.CommandDetails{
							CommandType: utils.In,
							InOption: utils.InOption{
								IsSeatIdSet: true,
								SeatId:      0,
								// TODO: メニューも引き継げるようにする。ただし、システム側の都合なので注文回数はカウントしないようにしたい
								MinWorkOrderOption: &utils.MinWorkOrderOption{
									IsWorkNameSet:    true,
									IsDurationMinSet: true,
									WorkName:         seat.WorkName,
									DurationMin:      int(utils.NoNegativeDuration(seat.Until.Sub(utils.JstNow())).Minutes()),
								},
								IsMemberSeat: false,
							},
						}
						if err := app.In(ctx, &inCommandDetails.InOption); err != nil {
							return fmt.Errorf("in In(): %w", err)
						}
					}
				}
				// max_seatsを更新
				if err := app.Repository.UpdateMaxSeats(ctx, nil, constants.DesiredMaxSeats); err != nil {
					return fmt.Errorf("in UpdateMaxSeats(): %w", err)
				}
			}
		}
	}

	return nil
}

// adjustMemberSeats メンバー席の数を調整する
func (app *WorkspaceApp) adjustMemberSeats(ctx context.Context, constants repository.ConstantsConfigDoc) error {
	// メンバー席
	if constants.DesiredMemberMaxSeats > constants.MemberMaxSeats { // メンバー席を増やす
		app.MessageToLiveChat(ctx, "メンバー限定の席を増やします↗")
		if err := app.Repository.UpdateMemberMaxSeats(ctx, nil, constants.DesiredMemberMaxSeats); err != nil {
			return fmt.Errorf("in UpdateMemberMaxSeats(): %w", err)
		}
	} else if constants.DesiredMemberMaxSeats < constants.MemberMaxSeats { // メンバー席を減らす
		if constants.FixedMaxSeatsEnabled { // 空席率に関係なく、member_max_seatsをdesiredに合わせる
			// なくなる座席にいるユーザーは退出させる
			seats, err := app.Repository.ReadMemberSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadMemberSeats(): %w", err)
			}
			app.MessageToLiveChat(ctx, "メンバー限定の座席数を"+strconv.Itoa(constants.DesiredMemberMaxSeats)+"に固定します↘ 必要な場合は退出してもらうことがあります。")
			for _, seat := range seats {
				if seat.SeatId > constants.DesiredMemberMaxSeats {
					app.SetProcessedUser(seat.UserId, seat.UserDisplayName, seat.UserProfileImageUrl, false, false, false)
					// 退出させる
					if err := app.Out(ctx); err != nil {
						return fmt.Errorf("in Out(): %w", err)
					}
				}
			}
			// member_max_seatsを更新
			if err := app.Repository.UpdateMemberMaxSeats(ctx, nil, constants.DesiredMemberMaxSeats); err != nil {
				return fmt.Errorf("in UpdateMemberMaxSeats(): %w", err)
			}
		} else {
			// member_max_seatsを減らしても、空席率が設定値以上か確認
			seats, err := app.Repository.ReadMemberSeats(ctx)
			if err != nil {
				return fmt.Errorf("in ReadMemberSeats(): %w", err)
			}
			if int(float32(constants.DesiredMemberMaxSeats)*(1.0-constants.MinVacancyRate)) < len(seats) {
				slog.Warn("減らそうとしすぎ。desiredは却下します。",
					"desired", constants.DesiredMaxSeats,
					"current member max seats", constants.MemberMaxSeats,
					"current seats length", len(seats))
				if err := app.Repository.UpdateDesiredMemberMaxSeats(ctx, nil, constants.MemberMaxSeats); err != nil {
					return fmt.Errorf("in UpdateDesiredMemberMaxSeats(): %w", err)
				}
			} else {
				// 消えてしまう席にいるユーザーを移動させる
				app.MessageToLiveChat(ctx, "人数が減ったためメンバー限定席を減らします↘ 必要な場合は席を移動してもらうことがあります。")
				for _, seat := range seats {
					if seat.SeatId > constants.DesiredMemberMaxSeats {
						app.SetProcessedUser(seat.UserId, seat.UserDisplayName, seat.UserProfileImageUrl, false, false, true)
						// 移動させる
						inCommandDetails := &utils.CommandDetails{
							CommandType: utils.In,
							InOption: utils.InOption{
								IsSeatIdSet: true,
								SeatId:      0,
								// TODO: メニューも引き継げるようにする。ただし、システム側の都合なので注文回数はカウントしないようにしたい
								MinWorkOrderOption: &utils.MinWorkOrderOption{
									IsWorkNameSet:    true,
									IsDurationMinSet: true,
									WorkName:         seat.WorkName,
									DurationMin:      int(utils.NoNegativeDuration(seat.Until.Sub(utils.JstNow())).Minutes()),
								},
								IsMemberSeat: true,
							},
						}

						if err := app.In(ctx, &inCommandDetails.InOption); err != nil {
							return fmt.Errorf("in In(): %w", err)
						}
					}
				}
				// member_max_seatsを更新
				if err := app.Repository.UpdateMemberMaxSeats(ctx, nil, constants.DesiredMemberMaxSeats); err != nil {
					return fmt.Errorf("in UpdateMemberMaxSeats(): %w", err)
				}
			}
		}
	}

	return nil
}
