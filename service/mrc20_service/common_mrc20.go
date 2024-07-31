package mrc20_service

import (
	"fmt"
	"github.com/godaddy-x/freego/utils/decimal"
	"metaid-market-service/tool"
	"strconv"
)

type Mrc20DataItem struct {
	Id     string `json:"id"`
	Amount string `json:"amount"`
	Vout   int64  `json:"vout"`
}

func MakeTransferPayload(tickId string, transferMrc20s []*TransferMrc20, mrc20Outs []*Mrc20OutInfo) (string, error) {
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
			Id:     tickId,
			Amount: v.Amount,
			Vout:   int64(i + 1),
		}
		dataItems = append(dataItems, dataItem)
	}
	payload = tool.AnyToStr(dataItems)

	return payload, nil
}

type Mrc20DeployData struct {
	Tick         string      `json:"tick"`
	TokenName    string      `json:"tokenName"`
	Decimals     string      `json:"decimals"`
	AmtPerMint   string      `json:"amtPerMint"`
	MintCount    string      `json:"mintCount"`
	PremineCount string      `json:"premineCount"`
	BeginHeight  string      `json:"beginHeight"`
	EndHeight    string      `json:"endHeight"`
	Metadata     string      `json:"metadata"`
	PinCheck     interface{} `json:"pinCheck"`
	PayCheck     *PayCheck   `json:"payCheck"`
}

type PayCheck struct {
	PayAmount string `json:"payAmount"`
	PayTo     string `json:"payTo"`
}

func MakeDeployPayload(tick, tokenName, metaData, amtPerMint string) (string, *Mrc20DeployData, int64) {
	var (
		payload         string           = ""
		totalSupply     int64            = 0
		mrc20DeployData *Mrc20DeployData = &Mrc20DeployData{
			Tick:         tick,
			TokenName:    tokenName,
			Decimals:     "8",
			AmtPerMint:   amtPerMint,
			MintCount:    "1",
			PremineCount: "1",
			BeginHeight:  "",
			EndHeight:    "",
			Metadata:     metaData,
			PinCheck: map[string]interface{}{
				"creator": "",
				"path":    "",
				"lvl":     "",
				"count":   "",
			},
		}
	)

	amtPerMintDe, _ := decimal.NewFromString(mrc20DeployData.AmtPerMint)
	mintCountDe, _ := decimal.NewFromString(mrc20DeployData.MintCount)
	totalSupply = amtPerMintDe.Mul(mintCountDe).IntPart()

	payload = tool.AnyToStr(mrc20DeployData)
	return payload, mrc20DeployData, totalSupply
}

func MakeDeployPayloadForIdCoins(tick, tokenName, metaId, metadata, payTo string, followersNum, amountPerMint, liquidityPerMint int64) (string, *Mrc20DeployData, int64) {
	var (
		payload         string           = ""
		totalSupply     int64            = 0
		mrc20DeployData *Mrc20DeployData = &Mrc20DeployData{
			Tick:         tick,
			TokenName:    tokenName,
			Decimals:     "8",
			AmtPerMint:   strconv.FormatInt(amountPerMint, 10),
			MintCount:    strconv.FormatInt(followersNum, 10),
			PremineCount: "",
			BeginHeight:  "",
			EndHeight:    "",
			Metadata:     metadata,
			PinCheck: map[string]interface{}{
				"creator": "",
				"path":    fmt.Sprintf("/follow['%s']", metaId),
				"lvl":     "",
				"count":   "1",
			},
			PayCheck: &PayCheck{
				PayAmount: strconv.FormatInt(liquidityPerMint, 10),
				PayTo:     payTo,
			},
		}
	)

	amtPerMintDe, _ := decimal.NewFromString(mrc20DeployData.AmtPerMint)
	mintCountDe, _ := decimal.NewFromString(mrc20DeployData.MintCount)
	totalSupply = amtPerMintDe.Mul(mintCountDe).IntPart()

	payload = tool.AnyToStr(mrc20DeployData)
	return payload, mrc20DeployData, totalSupply
}
