package own_service

type OwnResp struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

type OwnRespV3 struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Result  interface{} `json:"result"`
}

type BalanceInfo struct {
	Balance float64 `json:"balance"`
	Block   struct {
		IncomeFee float64 `json:"incomeFee"`
		SpendFee  float64 `json:"spendFee"`
	} `json:"block"`
	Mempool struct {
		IncomeFee float64 `json:"incomeFee"`
		SpendFee  float64 `json:"spendFee"`
	} `json:"mempool"`
}

type UtxoInfo struct {
	TxId         string      `json:"txId"`
	Vout         int64       `json:"vout"`
	Satoshi      int64       `json:"satoshi"`
	Confirmed    bool        `json:"confirmed"`
	Inscriptions interface{} `json:"inscriptions"`
}

type InscriptionInfo struct {
	Confirmed    bool `json:"confirmed"`
	Inscriptions []struct {
		Id  string `json:"id"`
		Num int64  `json:"num"`
	} `json:"inscriptions"`
	Satoshi int64  `json:"satoshi"`
	TxId    string `json:"txId"`
	Vout    int64  `json:"vout"`
}

//type TokenSummaryInfo struct {
//	TokenBalance struct {
//		Ticker                 string `json:"ticker"`
//		AvailableBalance       string `json:"availableBalance"`
//		TransferableBalance    string `json:"transferableBalance"`
//		OverallBalance         string `json:"overallBalance"`
//		AvailableBalanceSafe   string `json:"availableBalanceSafe"`
//		AvailableBalanceUnSafe string `json:"availableBalanceUnSafe"`
//	} `json:"tokenBalance"`
//	HistoryList      []interface{} `json:"historyList"`
//	TransferableList []struct {
//		InscriptionId     string `json:"inscriptionId"`
//		InscriptionNumber int64  `json:"inscriptionNumber"`
//		Amount            string `json:"amount"`
//		Ticker            string `json:"ticker"`
//	} `json:"transferableList"`
//	TokenInfo struct {
//		TotalSupply string `json:"totalSupply"`
//		TotalMinted string `json:"totalMinted"`
//	} `json:"tokenInfo"`
//}

type TokenSummaryInfo struct {
	TokenBalance struct {
		Ticker                 string `json:"ticker"`
		AvailableBalance       string `json:"availableBalance"`
		TransferableBalance    string `json:"transferableBalance"`
		OverallBalance         string `json:"overallBalance"`
		AvailableBalanceSafe   string `json:"availableBalanceSafe"`
		AvailableBalanceUnSafe string `json:"availableBalanceUnSafe"`
	} `json:"tokenBalance"`
	HistoryList      []interface{} `json:"historyList"`
	TransferableList []struct {
		InscriptionId     string `json:"inscriptionId"`
		InscriptionNumber int64  `json:"inscriptionNumber"`
		Amount            string `json:"amount"`
		Ticker            string `json:"ticker"`
	} `json:"transferableList"`
	TokenInfo struct {
		TotalSupply string `json:"totalSupply"`
		TotalMinted string `json:"totalMinted"`
	} `json:"tokenInfo"`
}

type TokenBalanceInfo struct {
	List []struct {
		Ticker                 string `json:"ticker"`
		OverallBalance         string `json:"overallBalance"`
		TransferableBalance    string `json:"transferableBalance"`
		AvailableBalance       string `json:"availableBalance"`
		AvailableBalanceSafe   string `json:"availableBalanceSafe"`
		AvailableBalanceUnSafe string `json:"availableBalanceUnSafe"`
		Decimal                int64  `json:"decimal"`
	} `json:"list"`
	Total int64 `json:"total"`
}

type OwnUtxoInfo struct {
	IsExist     bool   `json:"isExist"`
	TxConfirm   bool   `json:"txConfirm"`
	SpendStatus string `json:"spendStatus"`
	Height      int64  `json:"height"`
	Date        int64  `json:"date"`
	Value       int64  `json:"value"`
	Where       string `json:"where"`
	Address     string `json:"address"`
	SpendInfo   struct {
		SpendTx string `json:"spendTx"`
		Height  int64  `json:"height"`
		Date    int64  `json:"date"`
		Where   string `json:"where"`
	} `json:"spendInfo"`
}

type CheckTxInfo struct {
	TxHash    string   `json:"tx_hash"`
	Address   []string `json:"address"`
	Value     []int    `json:"value"`
	Input     []string `json:"input"`
	Height    int64    `json:"height"`
	Date      int64    `json:"date"`
	Confirmed bool     `json:"confirmed"`
}
