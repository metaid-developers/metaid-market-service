package man_service

type ManResp struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

//
//type PinInfo struct {
//	Id                 string `json:"id"`
//	Number             int    `json:"number"`
//	RootTxId           string `json:"rootTxId"`
//	Address            string `json:"address"`
//	Output             string `json:"output"`
//	OutputValue        int    `json:"outputValue"`
//	Timestamp          int    `json:"timestamp"`
//	GenesisFee         int    `json:"genesisFee"`
//	GenesisHeight      int    `json:"genesisHeight"`
//	GenesisTransaction string `json:"genesisTransaction"`
//	TxInIndex          int    `json:"txInIndex"`
//	TxInOffset         int    `json:"txInOffset"`
//	Operation          string `json:"operation"`
//	Path               string `json:"path"`
//	ParentPath         string `json:"parentPath"`
//	Encryption         string `json:"encryption"`
//	Version            string `json:"version"`
//	ContentType        string `json:"contentType"`
//	ContentTypeDetect  string `json:"contentTypeDetect"`
//	ContentBody        string `json:"contentBody"`
//	ContentLength      int    `json:"contentLength"`
//	ContentSummary     string `json:"contentSummary"`
//	Preview            string `json:"preview"`
//	Content            string `json:"content"`
//}

type PinUtxoTotalValue struct {
	UtxoNum int64 `json:"utxoNum"`
	UtxoSum int64 `json:"utxoSum"`
}

type PinInfoList struct {
	Total int64      `json:"total"`
	List  []*PinInfo `json:"list"`
}

type PinInfo struct {
	Id                 string `json:"id"`
	Number             int64  `json:"number"`
	Metaid             string `json:"metaid"`
	Address            string `json:"address"`
	CreateAddress      string `json:"createAddress"`
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
	PopLv              int    `json:"popLv"`
}

type MetaIDUserInfo struct {
	//Number int    `json:"number"`
	Metaid string `json:"metaid"`
	Name   string `json:"name"`
	//NameId        string `json:"nameId"`
	Address string `json:"address"`
	Avatar  string `json:"avatar"`
	//AvatarId      string `json:"avatarId"`
	//Bio           string `json:"bio"`
	//BioId         string `json:"bioId"`
	//SoulbondToken string `json:"soulbondToken"`
	//IsInit        bool   `json:"isInit"`
	//Unconfirmed   string `json:"unconfirmed"`
}
