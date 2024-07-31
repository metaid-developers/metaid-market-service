package mvcscan_service

type MvcScanResp struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
}

type MetaContractFtSummaryResp struct {
	Total       int                      `json:"total"`
	FtMarketCap string                   `json:"ftMarketCap"`
	Data        []*MetaContractFtSummary `json:"data"`
}
type MetaContractFtSummary struct {
	FtId                 string `json:"ftId"`
	Net                  string `json:"net"`
	Codehash             string `json:"codehash"`
	Genesis              string `json:"genesis"`
	TokenName            string `json:"tokenName"`
	Symbol               string `json:"symbol"`
	Icon                 string `json:"icon"`
	Name                 string `json:"name"`
	DecimalNum           int64  `json:"decimalNum"`
	MarketPrice          int64  `json:"marketPrice"`
	FullyMarketCap       string `json:"fullyMarketCap"`
	CirculatingMarketCap string `json:"circulatingMarketCap"`
	MarketPriceUsdt      string `json:"marketPriceUsdt"`
	Circulation          int64  `json:"circulation"`
	TotalSupply          int64  `json:"totalSupply"`
	MaxTokenAmount       int64  `json:"maxTokenAmount"`
	MaxMintAmount        int64  `json:"maxMintAmount"`
	MintedAmount         int64  `json:"mintedAmount"`
	HolderTotal          int64  `json:"holderTotal"`
	ContractTx           string `json:"contractTx"`
}
