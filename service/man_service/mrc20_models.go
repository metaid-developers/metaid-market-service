package man_service

type Mrc20BalanceResp struct {
	Total int64               `json:"total"`
	List  []*Mrc20BalanceInfo `json:"list"`
}

type Mrc20BalanceInfo struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Balance int64  `json:"balance"`
}

type Mrc20UtxoResp struct {
	Total int64        `json:"total"`
	List  []*Mrc20Utxo `json:"list"`
}

type Mrc20Utxo struct {
	Tick        string `json:"tick"`
	Mrc20Id     string `json:"mrc20Id"`
	TxPoint     string `json:"txPoint"`
	PinId       string `json:"pinId"`
	PinContent  string `json:"pinContent"`
	Verify      bool   `json:"verify"`
	BlockHeight int64  `json:"blockHeight"`
	MrcOption   string `json:"mrcOption"`
	FromAddress string `json:"fromAddress"`
	ToAddress   string `json:"toAddress"`
	ErrorMsg    string `json:"errorMsg"`
	AmtChange   int64  `json:"amtChange"`
	Status      int64  `json:"status"`
	Chain       string `json:"chain"`
	Index       int64  `json:"index"`
	Timestamp   int64  `json:"timestamp"`
}

type Mrc20TickListResp struct {
	Total int64            `json:"total"`
	List  []*Mrc20TickInfo `json:"list"`
}
type Mrc20TickInfo struct {
	Tick        string      `json:"tick"`
	TokenName   string      `json:"tokenName"`
	Decimals    string      `json:"decimals"`
	AmtPerMint  string      `json:"amtPerMint"`
	MintCount   string      `json:"mintCount"`
	BlockHeight string      `json:"blockheight"`
	MetaData    string      `json:"metadata"`
	Type        string      `json:"type"`
	Qual        interface{} `json:"qual"`
	TotalMinted int64       `json:"totalMinted"`
	Mrc20Id     string      `json:"mrc20Id"`
	PinNumber   int64       `json:"pinNumber"`
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
