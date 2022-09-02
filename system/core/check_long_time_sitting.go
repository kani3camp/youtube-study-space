package core

import (
	"app.modules/core/utils"
	"context"
	"log"
	"time"
)

// GoroutineCheckLongTimeSitting 居座り検出ループ
func (s *System) GoroutineCheckLongTimeSitting(ctx context.Context) {
	minimumInterval := time.Duration(s.Configs.Constants.MinimumCheckLongTimeSittingIntervalMinutes) * time.Minute
	log.Printf("居座りチェックの最小間隔: %v\n", minimumInterval)
	
	for {
		log.Println("checking long time sitting")
		start := utils.JstNow()
		
		err := s.CheckLongTimeSitting(ctx)
		if err != nil {
			s.MessageToLineBotWithError("failed to CheckLongTimeSitting", err)
			log.Println(err)
		}
		
		end := utils.JstNow()
		duration := end.Sub(start)
		if duration < minimumInterval {
			time.Sleep(minimumInterval - duration)
		}
	}
}
