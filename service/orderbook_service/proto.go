package orderbook_service

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/ripemd160"
)

const (
	OrderProtoType    uint32 = 100
	OrderProtoVersion uint32 = 2
	OrderProtoFlag           = "metacontract"

	SideAsk uint8 = 1
	SideBid uint8 = 2

	DustLimit  uint64 = 546
	MaxFeeBps  uint16 = 1000
	FeeDenom   uint64 = 10000
	OpPushLen         = 2
	TailLen           = 150
	RawDataLen        = 148

	// Right-to-left byte offsets matching contracts/orderbook/orderProto.scrypt
	protoHeaderOffset   = 25
	orderSideOffset     = 26
	makerAddrOffset     = 46
	tokenIDOffset       = 66
	tokenCodeHashOffset = 86
	priceNumOffset      = 94
	priceDenOffset      = 102
	remainAmountOffset  = 110
	minFillOffset       = 118
	feeAddrOffset       = 138
	feeBpsOffset        = 140
	saltOffset          = 148
)

var (
	expectedProtoHeader = append([]byte{0x02, 0x00, 0x00, 0x00, 0x64, 0x00, 0x00, 0x00}, []byte(OrderProtoFlag)...)
	nullAddressBytes    = make([]byte, 20)
)

// ParsedOrderTail holds every field decoded from the fixed 150-byte OrderProto tail.
type ParsedOrderTail struct {
	ScriptCodeHash string
	OrderSide      uint8 // 1 = Ask, 2 = Bid
	MakerHash20    string
	MakerAddr      string
	TokenID        string
	TokenCodeHash  string
	PairKey        string
	PriceNum       uint64
	PriceDen       uint64
	PriceDecimal   decimal.Decimal
	RemainAmount   uint64 // Ask: remaining token units; Bid: remaining SPACE satoshis
	MinFillAmount  uint64 // Minimum token units per partial fill
	FeeHash20      string
	FeeAddr        string
	FeeBps         uint16
	SaltHex        string
}

func MakePairKey(tokenCodeHash, tokenID string) string {
	return strings.ToLower(strings.TrimSpace(tokenCodeHash)) + ":" + strings.ToLower(strings.TrimSpace(tokenID))
}

func Hash160(data []byte) []byte {
	s := sha256.Sum256(data)
	r := ripemd160.New()
	_, _ = r.Write(s[:])
	return r.Sum(nil)
}

func Hash20ToAddress(hash20 []byte, net string) string {
	if len(hash20) != 20 || bytes.Equal(hash20, nullAddressBytes) {
		return ""
	}
	params := &chaincfg.MainNetParams
	if strings.EqualFold(net, "testnet") || strings.EqualFold(net, "regtest") {
		params = &chaincfg.TestNet3Params
	}
	addr, err := btcutil.NewAddressPubKeyHash(hash20, params)
	if err != nil {
		return ""
	}
	return addr.EncodeAddress()
}

// ParseOrderScriptHex decodes a hex locking script's 150-byte OrderProto tail.
func ParseOrderScriptHex(scriptHex string, net string) (*ParsedOrderTail, error) {
	raw, err := hex.DecodeString(strings.TrimSpace(scriptHex))
	if err != nil {
		return nil, fmt.Errorf("invalid script hex: %w", err)
	}
	return ParseOrderScript(raw, net)
}

// ParseOrderScript decodes an OrderAskV2 or OrderBidV2 locking script in pure Go.
func ParseOrderScript(script []byte, net string) (*ParsedOrderTail, error) {
	slen := len(script)
	if slen <= TailLen+1 {
		return nil, errors.New("script too short for OrderProto tail")
	}
	if script[slen-TailLen-1] != 0x6a || script[slen-TailLen] != 0x4c || script[slen-TailLen+1] != byte(RawDataLen) {
		return nil, errors.New("invalid OrderProto OP_RETURN / OP_PUSHDATA1 tail header")
	}
	hdr := script[slen-protoHeaderOffset : slen-protoHeaderOffset+20]
	if !bytes.Equal(hdr, expectedProtoHeader) {
		return nil, errors.New("invalid OrderProto version/type/flag")
	}

	side := script[slen-orderSideOffset]
	if side != SideAsk && side != SideBid {
		return nil, fmt.Errorf("invalid orderSide %d", side)
	}

	makerHash := script[slen-makerAddrOffset : slen-orderSideOffset]
	tokenID := script[slen-tokenIDOffset : slen-makerAddrOffset]
	tokenCodeHash := script[slen-tokenCodeHashOffset : slen-tokenIDOffset]
	priceNum := binary.LittleEndian.Uint64(script[slen-priceNumOffset : slen-tokenCodeHashOffset])
	priceDen := binary.LittleEndian.Uint64(script[slen-priceDenOffset : slen-priceNumOffset])
	remainAmount := binary.LittleEndian.Uint64(script[slen-remainAmountOffset : slen-priceDenOffset])
	minFill := binary.LittleEndian.Uint64(script[slen-minFillOffset : slen-remainAmountOffset])
	feeHash := script[slen-feeAddrOffset : slen-minFillOffset]
	feeBps := binary.LittleEndian.Uint16(script[slen-feeBpsOffset : slen-feeAddrOffset])
	salt := script[slen-saltOffset : slen-feeBpsOffset]

	if priceNum == 0 || priceDen == 0 {
		return nil, errors.New("invalid zero price numerator or denominator")
	}
	if feeBps > MaxFeeBps {
		return nil, fmt.Errorf("feeBps %d exceeds max %d", feeBps, MaxFeeBps)
	}

	codeHash := Hash160(script[:slen-TailLen])
	tokenCodeHashHex := hex.EncodeToString(tokenCodeHash)
	tokenIDHex := hex.EncodeToString(tokenID)

	priceDec := PriceToDecimal(priceNum, priceDen)

	return &ParsedOrderTail{
		ScriptCodeHash: hex.EncodeToString(codeHash),
		OrderSide:      side,
		MakerHash20:    hex.EncodeToString(makerHash),
		MakerAddr:      Hash20ToAddress(makerHash, net),
		TokenID:        tokenIDHex,
		TokenCodeHash:  tokenCodeHashHex,
		PairKey:        MakePairKey(tokenCodeHashHex, tokenIDHex),
		PriceNum:       priceNum,
		PriceDen:       priceDen,
		PriceDecimal:   priceDec,
		RemainAmount:   remainAmount,
		MinFillAmount:  minFill,
		FeeHash20:      hex.EncodeToString(feeHash),
		FeeAddr:        Hash20ToAddress(feeHash, net),
		FeeBps:         feeBps,
		SaltHex:        hex.EncodeToString(salt),
	}, nil
}

// BuildOrderTailHex builds a 150-byte OrderProto tail (0x4c 0x94 + 148B payload),
// primarily used in tests and verification tools.
func BuildOrderTailHex(
	side uint8,
	makerHash20 []byte,
	tokenID []byte,
	tokenCodeHash []byte,
	priceNum, priceDen, remainAmount, minFill uint64,
	feeHash20 []byte,
	feeBps uint16,
	salt []byte,
) []byte {
	payload := make([]byte, 0, RawDataLen)
	payload = append(payload, padOrSlice(salt, 8)...)
	b2 := make([]byte, 2)
	binary.LittleEndian.PutUint16(b2, feeBps)
	payload = append(payload, b2...)
	payload = append(payload, padOrSlice(feeHash20, 20)...)

	b8 := make([]byte, 8)
	binary.LittleEndian.PutUint64(b8, minFill)
	payload = append(payload, b8...)
	binary.LittleEndian.PutUint64(b8, remainAmount)
	payload = append(payload, b8...)
	binary.LittleEndian.PutUint64(b8, priceDen)
	payload = append(payload, b8...)
	binary.LittleEndian.PutUint64(b8, priceNum)
	payload = append(payload, b8...)

	payload = append(payload, padOrSlice(tokenCodeHash, 20)...)
	payload = append(payload, padOrSlice(tokenID, 20)...)
	payload = append(payload, padOrSlice(makerHash20, 20)...)
	payload = append(payload, side)

	// protoVersion(4)=2, protoType(4)=100, flag(12)="metacontract", dataLen(4)=145, version(1)=0xff
	payload = append(payload, expectedProtoHeader...)
	b4 := make([]byte, 4)
	binary.LittleEndian.PutUint32(b4, uint32(RawDataLen-3))
	payload = append(payload, b4...)
	payload = append(payload, 0xff)

	out := make([]byte, 0, TailLen)
	out = append(out, 0x4c, byte(RawDataLen))
	out = append(out, payload...)
	return out
}

func padOrSlice(b []byte, n int) []byte {
	out := make([]byte, n)
	copy(out, b)
	return out
}

// CalcAskGrossSats computes ceil(fillToken * priceNum / priceDen) using big.Int (no overflow).
func CalcAskGrossSats(fillToken, priceNum, priceDen uint64) uint64 {
	if fillToken == 0 || priceNum == 0 || priceDen == 0 {
		return 0
	}
	num := new(big.Int).Mul(new(big.Int).SetUint64(fillToken), new(big.Int).SetUint64(priceNum))
	den := new(big.Int).SetUint64(priceDen)
	num.Add(num, new(big.Int).Sub(den, big.NewInt(1)))
	return new(big.Int).Div(num, den).Uint64()
}

// CalcBidSpendSats computes floor(fillToken * priceNum / priceDen) using big.Int (no overflow).
func CalcBidSpendSats(fillToken, priceNum, priceDen uint64) uint64 {
	if fillToken == 0 || priceNum == 0 || priceDen == 0 {
		return 0
	}
	num := new(big.Int).Mul(new(big.Int).SetUint64(fillToken), new(big.Int).SetUint64(priceNum))
	den := new(big.Int).SetUint64(priceDen)
	return new(big.Int).Div(num, den).Uint64()
}

// CalcBidMaxToken computes floor(remainSats * priceDen / priceNum).
func CalcBidMaxToken(remainSats, priceNum, priceDen uint64) uint64 {
	if remainSats == 0 || priceNum == 0 || priceDen == 0 {
		return 0
	}
	num := new(big.Int).Mul(new(big.Int).SetUint64(remainSats), new(big.Int).SetUint64(priceDen))
	den := new(big.Int).SetUint64(priceNum)
	return new(big.Int).Div(num, den).Uint64()
}

// CalcFeeSats computes floor(sats * feeBps / 10000).
func CalcFeeSats(sats uint64, feeBps uint16) uint64 {
	if sats == 0 || feeBps == 0 {
		return 0
	}
	num := new(big.Int).Mul(new(big.Int).SetUint64(sats), new(big.Int).SetUint64(uint64(feeBps)))
	return new(big.Int).Div(num, big.NewInt(int64(FeeDenom))).Uint64()
}

// ComparePrice compares aNum/aDen vs bNum/bDen exactly via cross-multiplication.
// Returns -1 if a < b, 0 if a == b, +1 if a > b.
func ComparePrice(aNum, aDen, bNum, bDen uint64) int {
	lhs := new(big.Int).Mul(new(big.Int).SetUint64(aNum), new(big.Int).SetUint64(bDen))
	rhs := new(big.Int).Mul(new(big.Int).SetUint64(bNum), new(big.Int).SetUint64(aDen))
	return lhs.Cmp(rhs)
}

// PriceToDecimal converts priceNum/priceDen into a shopspring/decimal with 12 fractional digits max.
func PriceToDecimal(priceNum, priceDen uint64) decimal.Decimal {
	if priceDen == 0 {
		return decimal.Zero
	}
	n := decimal.NewFromBigInt(new(big.Int).SetUint64(priceNum), 0)
	d := decimal.NewFromBigInt(new(big.Int).SetUint64(priceDen), 0)
	return n.DivRound(d, 12)
}

// BucketPrice buckets an exact rational priceNum/priceDen at the requested decimal precision
// (can be negative for integer ticks like 10, 100, or positive for 0.1, 0.01).
// Spec rule: Ask rounds UP (Ceil), Bid rounds DOWN (Floor) so L2 quotes never mislead Takers.
func BucketPrice(priceNum, priceDen uint64, precision int32, side uint8) decimal.Decimal {
	if priceDen == 0 {
		return decimal.Zero
	}
	// Exact rational computation: scale = 10^precision (or divide by 10^-precision)
	num := new(big.Int).SetUint64(priceNum)
	den := new(big.Int).SetUint64(priceDen)

	ten := big.NewInt(10)
	if precision >= 0 {
		mul := new(big.Int).Exp(ten, big.NewInt(int64(precision)), nil)
		num.Mul(num, mul)
	} else {
		mul := new(big.Int).Exp(ten, big.NewInt(int64(-precision)), nil)
		den.Mul(den, mul)
	}

	quo, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	if side == SideAsk && rem.Sign() > 0 {
		quo.Add(quo, big.NewInt(1))
	}
	return decimal.NewFromBigInt(quo, -precision)
}
