package poloniex

import (
	"github.com/shopspring/decimal"
)

type Currency struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	MaxDailyWithdrawal decimal.Decimal `json:"maxDailyWithdrawal"`
	TxFee              decimal.Decimal `json:"txFee"`
	MinConf            int             `json:"minConf"`
	DepositAddress     *string         `json:"depositAddress"`
	Disabled           int             `json:"disabled"`
	Delisted           int             `json:"delisted"`
	Frozen             int             `json:"frozen"`
}

type Currencies struct {
	Pair map[string]Currency
}
