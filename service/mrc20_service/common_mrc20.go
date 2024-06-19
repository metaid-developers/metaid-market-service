package mrc20_service

import (
	"github.com/godaddy-x/freego/utils/decimal"
	"metaid-market-service/tool"
)

type Mrc20DataItem struct {
	Id     string `json:"id"`
	Amount string `json:"amount"`
	Vout   int64  `json:"vout"`
}

func MakeTransferPayload(tickerId string, transferMrc20s []*TransferMrc20, mrc20Outs []*Mrc20OutInfo) (string, error) {
	var (
		payload       string           = ""
		totalAmountDe decimal.Decimal  = decimal.New(0, 0)
		dataItems     []*Mrc20DataItem = make([]*Mrc20DataItem, 0)
	)
	for _, v := range transferMrc20s {
		mrc20AmountDe, err := decimal.NewFromString(v.Mrc20Amount)
		if err != nil {
			return "", err
		}
		totalAmountDe = totalAmountDe.Add(mrc20AmountDe)
	}

	for i, v := range mrc20Outs {
		dataItem := &Mrc20DataItem{
			Id:     tickerId,
			Amount: v.Amount,
			Vout:   int64(i + 1),
		}
		dataItems = append(dataItems, dataItem)
	}
	payload = tool.AnyToStr(dataItems)

	return payload, nil
}
