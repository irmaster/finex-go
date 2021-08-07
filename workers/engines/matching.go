package engines

import (
	"encoding/json"
	"errors"

	"github.com/shopspring/decimal"
	"github.com/zsmartex/finex/config"
	"github.com/zsmartex/finex/matching"
	"github.com/zsmartex/finex/models"
)

type MatchingPayloadMessage struct {
	Action matching.PayloadAction `json:"action"`
	Order  *matching.Order        `json:"order"`
	Market string                 `json:"market"`
}

type MatchingWorker struct {
	Engines map[string]*matching.Engine
}

func NewMatchingWorker() *MatchingWorker {
	worker := &MatchingWorker{
		Engines: make(map[string]*matching.Engine),
	}

	worker.Reload("all")

	return worker
}

func (w MatchingWorker) Process(payload []byte) error {
	var matching_payload MatchingPayloadMessage
	if err := json.Unmarshal(payload, &matching_payload); err != nil {
		return err
	}

	switch matching_payload.Action {
	case matching.ActionSubmit:
		order := matching_payload.Order
		return w.SubmitOrder(order)
	case matching.ActionCancel:
		order := matching_payload.Order
		return w.CancelOrder(order)
	case matching.ActionReload:
		w.Reload(matching_payload.Market)
	default:
		config.Logger.Fatalf("Unknown action: %s", matching_payload.Action)
	}

	return nil
}

func (w MatchingWorker) SubmitOrder(order *matching.Order) error {
	engine := w.Engines[order.Symbol]

	if engine == nil {
		return errors.New("engine not found")
	}

	if !engine.Initialized {
		return errors.New("engine is not ready")
	}

	engine.Submit(order)
	return nil
}

func (w MatchingWorker) CancelOrder(order *matching.Order) error {
	engine := w.Engines[order.Symbol]

	if engine == nil {
		return errors.New("engine not found")
	}

	if !engine.Initialized {
		return errors.New("engine is not ready")
	}

	engine.Cancel(order)
	return nil
}

func (w MatchingWorker) GetEngineByMarket(market string) *matching.Engine {
	engine, found := w.Engines[market]

	if found {
		return engine
	}

	return nil
}

func (w MatchingWorker) Reload(market string) {
	if market == "all" {
		var markets []models.Market
		config.DataBase.Where("state = ?", "enabled").Find(&markets)
		for _, market := range markets {
			w.InitializeEngine(market.Symbol)
		}
		config.Logger.Info("All engines reloaded.")
	} else {
		w.InitializeEngine(market)
	}
}

func (w MatchingWorker) InitializeEngine(market string) {
	engine := matching.NewEngine(market, decimal.Zero)
	w.Engines[market] = engine

	w.LoadOrders(engine)
	engine.Initialized = true
	config.Logger.Infof("%v engine reloaded.\n", market)
}

func (w MatchingWorker) BuildOrder(order map[string]interface{}) *matching.Order {
	mapOrderInterfaceJSON, _ := json.Marshal(order)

	var mOrder *matching.Order
	json.Unmarshal(mapOrderInterfaceJSON, &mOrder)

	return mOrder
}

func (w MatchingWorker) LoadOrders(engine *matching.Engine) {
	var orders []models.Order
	config.DataBase.Where("market_id = ? AND state = ?", engine.Market, models.StateWait).Order("id asc").Find(&orders)

	for _, order := range orders {
		engine.Submit(order.ToMatchingAttributes())
	}
}
