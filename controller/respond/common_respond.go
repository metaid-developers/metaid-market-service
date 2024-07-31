package respond

type Mrc20TickInfo struct {
	Tick             string      `json:"tick"`
	TokenName        string      `json:"tokenName"`
	Decimals         string      `json:"decimals"`
	AmtPerMint       string      `json:"amtPerMint"`
	MintCount        string      `json:"mintCount"`
	PremineCount     string      `json:"premineCount"`
	TotalMinted      string      `json:"totalMinted"`
	BeginHeight      string      `json:"beginHeight"`
	EndHeight        string      `json:"endHeight"`
	MetaData         string      `json:"metaData"`
	Type             string      `json:"type"`
	Qual             interface{} `json:"qual"`
	PinCheck         interface{} `json:"pinCheck"`
	PayCheck         interface{} `json:"payCheck"`
	Mrc20Id          string      `json:"mrc20Id"`
	PinNumber        int64       `json:"pinNumber"`
	Holders          int64       `json:"holders"`
	TxCount          int64       `json:"txCount"`
	DeployerMetaId   string      `json:"deployerMetaId"`
	DeployerAddress  string      `json:"deployerAddress"`
	DeployerUserInfo *UserInfo   `json:"deployerUserInfo"`
	DeployTime       int64       `json:"deployTime"`
	Price            string      `json:"price"`
	PriceUsd         string      `json:"priceUsd"`
	FloorPrice       string      `json:"floorPrice"`
	FloorPriceUsd    string      `json:"floorPriceUsd"`
	Change24h        string      `json:"change24h"`
	MarketCap        string      `json:"marketCap"`
	MarketCapUsd     string      `json:"marketCapUsd"`
	TotalVolume      int64       `json:"totalVolume"`
	TotalSupply      string      `json:"totalSupply"`
	Supply           string      `json:"supply"`
	Mintable         bool        `json:"mintable"`
	Remaining        string      `json:"remaining"`
}

type Mrc20ShovelResp struct {
	Total int64         `json:"total"`
	List  []*ShovelInfo `json:"list"`
}

type ShovelInfo struct {
	Id                 string `json:"id"`
	Number             int64  `json:"number"`
	Metaid             string `json:"metaid"`
	Address            string `json:"address"`
	Creator            string `json:"creator"`
	InitialOwner       string `json:"initialOwner"`
	Output             string `json:"output"`
	OutputValue        int64  `json:"outputValue"`
	Timestamp          int64  `json:"timestamp"`
	GenesisFee         int64  `json:"genesisFee"`
	GenesisHeight      int64  `json:"genesisHeight"`
	GenesisTransaction string `json:"genesisTransaction"`
	TxIndex            int64  `json:"txIndex"`
	TxInIndex          int64  `json:"txInIndex"`
	Offset             int64  `json:"offset"`
	Location           string `json:"location"`
	Operation          string `json:"operation"`
	Path               string `json:"path"`
	ParentPath         string `json:"parentPath"`
	OriginalPath       string `json:"originalPath"`
	Encryption         string `json:"encryption"`
	Version            string `json:"version"`
	ContentType        string `json:"contentType"`
	ContentTypeDetect  string `json:"contentTypeDetect"`
	ContentBody        string `json:"contentBody"`
	ContentLength      int64  `json:"contentLength"`
	ContentSummary     string `json:"contentSummary"`
	Status             int64  `json:"status"`
	OriginalId         string `json:"originalId"`
	IsTransfered       bool   `json:"isTransfered"`
	Preview            string `json:"preview"`
	Content            string `json:"content"`
	Pop                string `json:"pop"`
	PopLv              int64  `json:"popLv"`
	ChainName          string `json:"chainName"`
	DataValue          int64  `json:"dataValue"`
	Mrc20Minted        bool   `json:"mrc20Minted"`
	Mrc20MintPin       string `json:"mrc20MintPin"`
}

type Mrc20UtxoResp struct {
	Total int64        `json:"total"`
	List  []*Mrc20Utxo `json:"list"`
}

//type Mrc20Utxo struct {
//	Tick        string `json:"tick"`
//	Mrc20Id     string `json:"mrc20Id"`
//	TxPoint     string `json:"txPoint"`
//	PinId       string `json:"pinId"`
//	PinContent  string `json:"pinContent"`
//	Verify      bool   `json:"verify"`
//	BlockHeight int64  `json:"blockHeight"`
//	MrcOption   string `json:"mrcOption"`
//	FromAddress string `json:"fromAddress"`
//	ToAddress   string `json:"toAddress"`
//	ErrorMsg    string `json:"errorMsg"`
//	AmtChange   int64  `json:"amtChange"`
//	Status      int64  `json:"status"`
//	Chain       string `json:"chain"`
//	Index       int64  `json:"index"`
//	Timestamp   int64  `json:"timestamp"`
//}

type Mrc20Utxo struct {
	Chain       string       `json:"chain"`
	BlockHeight int64        `json:"blockHeight"`
	Address     string       `json:"address"`
	Satoshi     int64        `json:"satoshi"`
	Satoshis    int64        `json:"satoshis"`
	ScriptPk    string       `json:"scriptPk"`
	TxId        string       `json:"txId"`
	Vout        int64        `json:"vout"`
	OutputIndex int64        `json:"outputIndex"`
	Mrc20s      []*Mrc20Info `json:"mrc20s"`
	Timestamp   int64        `json:"timestamp"`
	OrderId     string       `json:"orderId"`
	Tag         string       `json:"tag"` //id-coins
}

type Mrc20Info struct {
	Tick     string `json:"tick"`
	Mrc20Id  string `json:"mrc20Id"`
	TxPoint  string `json:"txPoint"`
	Amount   string `json:"amount"`
	Decimals string `json:"decimals"`
}

type Mrc20BalanceInfoResp struct {
	Total int64               `json:"total"`
	List  []*Mrc20BalanceInfo `json:"list"`
}

type Mrc20BalanceInfo struct {
	Tick      string `json:"tick"`
	TokenName string `json:"tokenName"`
	Mrc20Id   string `json:"mrc20Id"`
	Balance   string `json:"balance"`
	Decimals  string `json:"decimals"`
}

type Mrc20TickListResp struct {
	Total int64            `json:"total"`
	List  []*Mrc20TickInfo `json:"list"`
}

type BroadcastTxResp struct {
	TxId string `json:"txId"`
}

type Mrc20TickMarketPriceResp struct {
	TickId        string `json:"tickId"`
	Tick          string `json:"tick"`
	TokenName     string `json:"tokenName"`
	Decimals      int64  `json:"decimals"`
	Supply        string `json:"supply"`
	TotalVolume   int64  `json:"totalVolume"`
	MarketCap     string `json:"marketCap"`
	MarketCapUsd  string `json:"marketCapUsd"`
	LastPrice     string `json:"lastPrice"`
	LastPriceUsd  string `json:"lastPriceUsd"`
	Price         string `json:"price"`
	PriceUsd      string `json:"priceUsd"`
	FloorPrice    string `json:"floorPrice"`
	FloorPriceUsd string `json:"floorPriceUsd"`
}

type Mrc20TickHolderResp struct {
	Total int64         `json:"total"`
	List  []*HolderInfo `json:"list"`
}

type HolderInfo struct {
	TickId     string    `json:"tickId"`
	Tick       string    `json:"tick"`
	TokenName  string    `json:"tokenName"`
	MetaId     string    `json:"metaId"`
	Address    string    `json:"address"`
	UserInfo   *UserInfo `json:"userInfo"`
	Balance    string    `json:"balance"`
	Proportion string    `json:"proportion"`
}
