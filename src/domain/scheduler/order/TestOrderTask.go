package order_scheduler

import (
	"context"
	"gitlab.faza.io/services/finance/app"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/future"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"go.mongodb.org/mongo-driver/bson"
	"strconv"
	"time"
)

func (scheduler OrderScheduler) TestOrderTask(ctx context.Context, startAt, endAt time.Time, sellerIds []uint64) future.IFuture {
	triggerHistory := entities.TriggerHistory{
		TriggerName:  "TestDummyTrigger__" + strconv.Itoa(int(time.Now().UTC().UnixNano())),
		ExecResult:   entities.TriggerExecResultNone,
		RunMode:      entities.TriggerRunModeRunning,
		TriggeredAt:  &endAt,
		IsMissedFire: false,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		DeletedAt:    nil,
	}

	iFutureData := app.Globals.TriggerHistoryRepository.Save(ctx, triggerHistory).Get()
	if iFutureData.Error() != nil {
		log.GLog.Logger.Error("TriggerHistoryRepository.Update failed",
			"fn", "doProcess",
			"trigger", triggerHistory,
			"error", iFutureData.Error().Reason())
		return future.FactorySyncDataOf(iFutureData).BuildAndSend()
	}

	testTriggerHistory := iFutureData.Data().(*entities.TriggerHistory)

	scheduler.financeTriggerInterval = endAt.Sub(startAt)
	iFutureData = scheduler.OrderSchedulerTask(ctx, *testTriggerHistory).Get()
	if iFutureData.Error() != nil {
		return future.FactorySyncDataOf(iFutureData).BuildAndSend()
	}

	financeList := make([]*entities.SellerFinance, 0, 16)
	if len(sellerIds) > 0 {
		for _, sellerId := range sellerIds {
			iFutureData = app.Globals.SellerFinanceRepository.FindByFilter(ctx, func() interface{} {
				return bson.D{{"sellerId", sellerId},
					{"ordersInfo.triggerName", testTriggerHistory.TriggerName},
					{"deletedAt", nil},
				}
			}).Get()

			if iFutureData.Error() != nil {
				if iFutureData.Error().Code() != future.NotFound {
					log.GLog.Logger.Error("SellerFinanceRepository.FindByFilter failed",
						"fn", "TestOrderTask",
						"sellerId", sellerId,
						"triggerName", testTriggerHistory.TriggerName,
						"error", iFutureData.Error())
					return future.FactorySyncDataOf(iFutureData).BuildAndSend()
				}
			} else {
				finances := iFutureData.Data().([]*entities.SellerFinance)
				financeList = append(financeList, finances...)
			}
		}

	} else {
		iFutureData = app.Globals.SellerFinanceRepository.FindByFilter(ctx, func() interface{} {
			return bson.D{{"ordersInfo.triggerName", testTriggerHistory.TriggerName},
				{"deletedAt", nil},
			}
		}).Get()

		if iFutureData.Error() != nil {
			if iFutureData.Error().Code() != future.NotFound {
				log.GLog.Logger.Error("SellerFinanceRepository.FindByFilter failed",
					"fn", "TestOrderTask",
					"triggerName", testTriggerHistory.TriggerName,
					"error", iFutureData.Error())
				return future.FactorySyncDataOf(iFutureData).BuildAndSend()
			}
		} else {
			finances := iFutureData.Data().([]*entities.SellerFinance)
			financeList = append(financeList, finances...)
		}
	}

	return future.FactorySync().SetData(financeList).BuildAndSend()
}
