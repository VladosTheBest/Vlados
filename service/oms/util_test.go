package oms

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ericlagergren/decimal"
	"github.com/rs/zerolog/log"
	kafkaGo "github.com/segmentio/kafka-go"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/config"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/net/kafka"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/net/manager"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"math/rand"
	"time"

	psql "github.com/ericlagergren/decimal/sql/postgres"
)

func setupConfig() config.Config {
	cfg := config.Config{
		Preprocessors: config.Preprocessors{
			QueueManager: config.QueueManager{
				Type: "kafka",
				Patterns: map[string]string{
					"sync_data": "sync_data",
				},
				Inputs:  []string{"sync_data"},
				Outputs: []string{"sync_data"},
			},
		},
		Kafka: kafka.Config{
			UseTLS:  false,
			Brokers: []string{"localhost:9092"},
			Reader: kafka.ReaderConfig{
				QueueCapacity:  2,
				MaxWait:        500,
				MinBytes:       100,
				MaxBytes:       10000,
				ReadBackoffMin: 100,
				ReadBackoffMax: 1000,
				ChannelSize:    2,
			},
			Writer: kafka.WriterConfig{
				QueueCapacity: 1,
				BatchSize:     1,
				BatchBytes:    10000,
				BatchTimeout:  500,
				Async:         true,
			},
		},
	}
	return cfg
}

type positionHandler struct {
}

func (p *positionHandler) Load(topics []string) (map[string]int64, error) {
	m := make(map[string]int64)
	for _, t := range topics {
		m[t] = 0
	}
	return m, nil
}
func (p *positionHandler) Save(positions map[string]int64) error { return nil }

func setupDM() *manager.DataManager {
	cfg := setupConfig()
	dm := manager.NewDataManager(cfg)
	dm.SetPositionHandler(&positionHandler{})
	return dm
}

func setupDB() (*gorm.DB, sqlmock.Sqlmock) {
	logger := log.With().Str("test", "OMS").Str("method", "setupDB").Logger()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		logger.Fatal().Msgf("can't create sqlmock: %s", err)
	}

	dialector := postgres.New(postgres.Config{
		DSN:                  "postgres-mock",
		DriverName:           "postgres",
		Conn:                 db,
		PreferSimpleProtocol: true,
	})

	gormDB, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		logger.Fatal().Msgf("can't open gorm connection: %s", err)
	}
	return gormDB, mock
}

func setupRepo() (*queries.Repo, sqlmock.Sqlmock) {
	db, mock := setupDB()
	return &queries.Repo{
		Conn:            db,
		ConnReader:      db,
		ConnReaderAdmin: db,
	}, mock
}

func getTestOrderWithPrice(orderID uint64, marketID string, ownerID uint64, price *decimal.Big) *model.Order {
	order := getTestOrder(orderID, marketID, ownerID)
	order.Price.V = price
	return order
}

// getTestOrder create returns a dummy order instance
func getTestOrder(orderID uint64, marketID string, ownerID uint64) *model.Order {
	price := conv.NewDecimalWithPrecision()
	price.SetString(conv.FromUnits(1000, 8))
	var parentOrderId uint64 = 0
	orderTYpe := model.OrderType_Market
	stopPrice := model.TrailingStopPriceType_Absolute
	order := model.NewOrder(
		ownerID,
		marketID,
		model.MarketSide_Buy,
		model.OrderType_Market,
		model.OrderStop_None,
		price,
		price,
		price,
		price,
		price,
		price,
		price,
		price,
		price,
		price,
		price,
		price,
		price,
		&parentOrderId,
		&orderTYpe,
		0,
		model.UIType_Api,
		"0",
		price,
		price,
		&stopPrice,
	)
	order.ID = orderID
	return order
}

func getTestMarket(marketId string) *model.Market {
	big := conv.NewDecimalWithPrecision()
	return model.NewMarket(
		marketId,
		"Test",
		"BTC",
		"USDT",
		model.MarketStatusActive,
		8, 8, 8, 8,
		big,
		big,
		big,
		big,
		big,
	)
}

func getEventFromDataManager(topic string, dm *manager.MockDataManager) kafkaGo.Message {
	return <-dm.Queues[topic]
}

func getTestTrade(ID uint64, marketID string, bidOwnerID, askOwnerID uint64, side model.MarketSide) *model.Trade {
	timestamp := time.Now().Unix()
	price := conv.NewDecimalWithPrecision()
	price.SetString(conv.FromUnits(1000, 8))
	seqID := rand.Uint64()
	eventSeqID := rand.Uint64()
	askID := rand.Uint64()
	bidID := rand.Uint64()
	trade := model.NewTrade(seqID, marketID, eventSeqID, side, price, price, price, askID, bidID, askOwnerID, bidOwnerID, timestamp)
	trade.ID = ID
	return trade
}

func getTestRevenue(id uint64, opType model.OperationType, userID uint64) *model.Revenue {
	price := conv.NewDecimalWithPrecision()
	price.SetString(conv.FromUnits(1000, 8))
	revenue := model.NewRevenue("btc",
		model.AccountType_Main,
		opType,
		"",
		userID,
		price,
		price,
		0,
		1)
	revenue.ID = id

	return revenue
}

func getTestLiability(id uint64, opType model.OperationType, userID uint64) *model.Liability {
	price := conv.NewDecimalWithPrecision()
	price.SetString(conv.FromUnits(1000, 8))
	liability := model.NewLiability("btc",
		model.AccountType_Main,
		opType,
		"",
		userID,
		price,
		price,
		0,
		1)
	liability.ID = id

	return liability
}

func getReferralEarning(id uint64, referralType model.OperationType, userID uint64) *model.ReferralEarning {
	price := conv.NewDecimalWithPrecision()
	price.SetString(conv.FromUnits(1000, 8))
	referralEarning := model.ReferralEarning{
		Id:                id,
		RefId:             "",
		ReferralId:        1,
		UserId:            userID,
		RelatedObjectType: model.ReferralEarningsType_Order,
		Level:             model.ReferralEarningLevel1,
		Type:              referralType,
		Amount:            &psql.Decimal{V: price},
		CoinSymbol:        "btc",
		CreatedAt:         time.Now(),
	}

	return &referralEarning
}

func getTestOperation(id uint64, opType model.OperationType, status model.OperationStatus) *model.Operation {
	operation := model.NewOperation(opType, status)
	operation.ID = id

	return operation
}

// func printAlloc() {
// 	fmt.Printf("%d MB\n", getAllocatedMemoryInMB())
// }
