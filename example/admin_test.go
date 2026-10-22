package example

import (
	"metaid-market-service/common"
	"metaid-market-service/controller/request"
	"metaid-market-service/models"
	"metaid-market-service/tool"
	"testing"
)

func TestAdminColddownDummyUtox(t *testing.T) {
	var (
		net string = "livenet"

		// chain     string = "btc"
		// priKeyHex string = "7939199684693942e52f58f08b13242cb8e58f72e92cbd5599288a6c3334ea1e"
		// address   string = "bc1qxnqkhzr98pehxyfl27z362nr6yv38rktqyenc5"

		chain     string = "doge"
		priKeyHex string = "7939199684693942e52f58f08b13242cb8e58f72e92cbd5599288a6c3334ea1e"
		address   string = "D9x3PJ2GQkeEUz7FZUatJBY8KHTbv1QLDq"

		url         string = domain + "/api/v1/admin/utxo/colddown"
		pkScript, _        = common.AddressToPkScript(net, chain, address)
		req                = &request.ColdDownDummyUtxoRequest{
			// btc
			// Chain: chain,
			// //UtxoType: models.UtxoTypeDummy600,
			// UtxoType:  models.UtxoTypeDummy1200,
			// TxId:      "95638d987fb0625eb3b8886486077f796a6f6137a4d5b1190f95ad5cc63d279c",
			// Index:     0,
			// Amount:    60000,
			// PkScript:  pkScript,
			// Address:   address,
			// PriKeyHex: priKeyHex,
			// PerAmount: 600,
			// Count:     15,
			// ChangeAddress: "bc1q7u6frlfjyuu4gugl7ugdj767m3rw7z0f7qdn0g",
			// FeeRate:       35,

			// doge
			Chain: chain,
			// UtxoType: models.UtxoTypeDummy600,
			UtxoType:      models.UtxoTypeDummy1200,
			TxId:          "22a5071bafff6e96c9be4f06d56f4bfc32b9db8712a7aa29022c3658b8297529",
			Index:         0,
			Amount:        10000000,
			PkScript:      pkScript,
			Address:       address,
			PriKeyHex:     priKeyHex,
			PerAmount:     1000000,
			Count:         3,
			ChangeAddress: "DCoo7zdQh2KNkVxgezTaibwML8XZc5tSnN",
			FeeRate:       4000,
			SignMode:      common.SignModeLegacy,
			// 839f2557fbc1eb882ffea27711f415e4cbf1997c9ea98ceef3bcce4a66a8fc83
		}
	)
	if req.UtxoType == models.UtxoTypeDummy1200 {
		if chain == "doge" {
			req.PerAmount = 2000000
		} else {
			req.PerAmount = 1200
		}
	}

	resp, err := tool.PostUrl(url, req, nil)
	if err != nil {
		t.Errorf("PostUrl() error = %v,", err)
		return
	}
	t.Logf("resp = %v,", resp)
}
