package wallet_service

type WalletMessage struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Msg     string      `json:"msg"`
	Data    interface{} `json:"data"`
}

type SignPsbtRequest struct {
	Net          string         `json:"net"`
	SwapOrderId  string         `json:"swapOrderId"`
	PsbtHex      string         `json:"psbtHex"`
	ToSignInputs []*ToSignInput `json:"toSignInputs"`
}

type ToSignInput struct {
	Index        int      `json:"index"`
	Address      string   `json:"address"`
	PublicKey    string   `json:"publicKey"`
	SigHashTypes []uint32 `json:"sigHashTypes"`
}

type SignPsbtResponse struct {
	PsbtHex string `json:"psbtHex"`
}

type PoolKey struct {
	Btc      string `json:"btc"`      //public key
	Protocol string `json:"protocol"` // public key
}
