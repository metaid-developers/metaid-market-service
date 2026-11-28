package orderbook_service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"metaid-market-service/conf"
)

func makeDummyOrderScript(
	codePrefix []byte,
	side uint8,
	makerHash []byte,
	tokenID []byte,
	tokenCodeHash []byte,
	priceNum, priceDen, remainAmount, minFill uint64,
	feeHash []byte,
	feeBps uint16,
	salt []byte,
) string {
	script := make([]byte, 0, len(codePrefix)+1+TailLen)
	script = append(script, codePrefix...)
	script = append(script, 0x6a) // OP_RETURN
	tail := BuildOrderTailHex(side, makerHash, tokenID, tokenCodeHash, priceNum, priceDen, remainAmount, minFill, feeHash, feeBps, salt)
	script = append(script, tail...)
	return hex.EncodeToString(script)
}

func repeatByte(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

func TestParseOrderScriptAndMath(t *testing.T) {
	askPrefix := []byte{0x01, 0x02, 0x03, 0x04, 0x76, 0xa9}
	makerHash := repeatByte(0x11, 20)
	feeHash := repeatByte(0x22, 20)
	tokenID := repeatByte(0xaa, 20)
	tokenCodeHash := repeatByte(0xbb, 20)
	salt := []byte{1, 2, 3, 4, 5, 6, 7, 8}

	scriptHex := makeDummyOrderScript(
		askPrefix, SideAsk, makerHash, tokenID, tokenCodeHash,
		105, 10, 10000, 500, feeHash, 25, salt,
	)

	parsed, err := ParseOrderScriptHex(scriptHex, "livenet")
	if err != nil {
		t.Fatalf("ParseOrderScriptHex failed: %v", err)
	}
	if parsed.OrderSide != SideAsk {
		t.Fatalf("expected SideAsk, got %d", parsed.OrderSide)
	}
	if parsed.PriceNum != 105 || parsed.PriceDen != 10 {
		t.Fatalf("unexpected price %d/%d", parsed.PriceNum, parsed.PriceDen)
	}
	if parsed.PriceDecimal.String() != "10.5" {
		t.Fatalf("expected price 10.5, got %s", parsed.PriceDecimal.String())
	}
	if parsed.RemainAmount != 10000 || parsed.MinFillAmount != 500 || parsed.FeeBps != 25 {
		t.Fatalf("unexpected amounts/fee: %+v", parsed)
	}
	if parsed.SaltHex != "0102030405060708" {
		t.Fatalf("unexpected salt %s", parsed.SaltHex)
	}
	if !strings.HasPrefix(parsed.MakerAddr, "1") {
		t.Fatalf("expected mainnet base58 address, got %q", parsed.MakerAddr)
	}
	wantCodeHash := hex.EncodeToString(Hash160(append(append([]byte{}, askPrefix...), 0x6a)))
	if parsed.ScriptCodeHash != wantCodeHash {
		t.Fatalf("codehash mismatch: got %s want %s", parsed.ScriptCodeHash, wantCodeHash)
	}

	// Ceil / Floor rounding checks
	if got := CalcAskGrossSats(3, 10, 4); got != 8 { // 30/4 = 7.5 -> ceil = 8
		t.Fatalf("CalcAskGrossSats expected 8, got %d", got)
	}
	if got := CalcBidSpendSats(3, 10, 4); got != 7 { // 30/4 = 7.5 -> floor = 7
		t.Fatalf("CalcBidSpendSats expected 7, got %d", got)
	}
	if got := CalcFeeSats(100000, 25); got != 250 {
		t.Fatalf("CalcFeeSats expected 250, got %d", got)
	}

	// Overflow-safe price comparison
	if ComparePrice(1<<62, 3, 1<<62, 4) <= 0 {
		t.Fatalf("ComparePrice failed for large uint64")
	}

	// BucketPrice Ask Ceil vs Bid Floor
	if got := BucketPrice(1051, 100, 1, SideAsk).String(); got != "10.6" { // 10.51 -> ceil(0.1) = 10.6
		t.Fatalf("Ask BucketPrice(10.51, prec=1) expected 10.6, got %s", got)
	}
	if got := BucketPrice(1059, 100, 1, SideBid).String(); got != "10.5" { // 10.59 -> floor(0.1) = 10.5
		t.Fatalf("Bid BucketPrice(10.59, prec=1) expected 10.5, got %s", got)
	}
	if got := BucketPrice(105, 1, -1, SideAsk).String(); got != "110" { // 105 -> ceil(10) = 110
		t.Fatalf("Ask BucketPrice(105, prec=-1) expected 110, got %s", got)
	}
	if got := BucketPrice(105, 1, -1, SideBid).String(); got != "100" { // 105 -> floor(10) = 100
		t.Fatalf("Bid BucketPrice(105, prec=-1) expected 100, got %s", got)
	}

	// Golden cross-language parity vector generated directly from compiled OrderAskV2 + OrderProto.buildOrderDataPart
	const realCompiledTail151 = "6a4c94010203040506070819002222222222222222222222222222222222222222f40100000000000010270000000000000a000000000000006900000000000000bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa11111111111111111111111111111111111111110102000000640000006d657461636f6e747261637491000000ff"
	goBuiltTail151 := "6a" + hex.EncodeToString(BuildOrderTailHex(SideAsk, makerHash, tokenID, tokenCodeHash, 105, 10, 10000, 500, feeHash, 25, salt))
	if goBuiltTail151 != realCompiledTail151 {
		t.Fatalf("Go BuildOrderTailHex drifted from TS compiled OrderAskV2 tail:\ngot:  %s\nwant: %s", goBuiltTail151, realCompiledTail151)
	}
	goldenParsed, err := ParseOrderScriptHex("010203"+realCompiledTail151, "livenet")
	if err != nil {
		t.Fatalf("ParseOrderScriptHex failed on real compiled OrderAskV2 tail: %v", err)
	}
	if goldenParsed.PriceNum != 105 || goldenParsed.PriceDen != 10 || goldenParsed.RemainAmount != 10000 || goldenParsed.MinFillAmount != 500 || goldenParsed.FeeBps != 25 || goldenParsed.SaltHex != "0102030405060708" {
		t.Fatalf("golden parsed mismatch: %+v", goldenParsed)
	}

	// Invalid script rejection
	if _, err := ParseOrderScriptHex("deadbeef", "livenet"); err == nil {
		t.Fatalf("expected error for short script")
	}
}

func TestL2DepthAndKlines(t *testing.T) {
	book := NewOrderBook("livenet", NewMemorySoftLocker())
	tokenID := repeatByte(0xaa, 20)
	tokenCodeHash := repeatByte(0xbb, 20)
	pairKey := MakePairKey(hex.EncodeToString(tokenCodeHash), hex.EncodeToString(tokenID))
	maker := repeatByte(0x11, 20)
	fee := repeatByte(0x22, 20)

	addAsk := func(txid string, pNum, pDen, amt uint64, saltByte byte) {
		s := makeDummyOrderScript([]byte{0x01}, SideAsk, maker, tokenID, tokenCodeHash, pNum, pDen, amt, 100, fee, 25, repeatByte(saltByte, 8))
		lo, err := ParseLiveOrder(txid, 0, DustLimit, s, 100, false, 1000, "livenet")
		if err != nil {
			t.Fatalf("ParseLiveOrder ask: %v", err)
		}
		book.UpsertOrder(lo)
	}
	addBid := func(txid string, pNum, pDen, sats uint64, saltByte byte) {
		s := makeDummyOrderScript([]byte{0x02}, SideBid, maker, tokenID, tokenCodeHash, pNum, pDen, sats, 100, fee, 25, repeatByte(saltByte, 8))
		lo, err := ParseLiveOrder(txid, 0, sats, s, 100, false, 1000, "livenet")
		if err != nil {
			t.Fatalf("ParseLiveOrder bid: %v", err)
		}
		book.UpsertOrder(lo)
	}

	// Asks at 10.12 (2000) and 10.19 (3000) -> precision=1 ceil => both bucket into 10.2 (5000)
	// Ask at 10.30 (1000) -> bucket 10.3
	addAsk(strings.Repeat("a", 64), 1012, 100, 2000, 1)
	addAsk(strings.Repeat("b", 64), 1019, 100, 3000, 2)
	addAsk(strings.Repeat("c", 64), 1030, 100, 1000, 3)

	// Bids at 9.98 (99800 sats -> 10000 tok) and 9.91 (49550 sats -> 5000 tok) -> precision=1 floor => both bucket into 9.9 (15000 tok)
	// Bid at 9.75 (97500 sats -> 10000 tok) -> bucket 9.7
	addBid(strings.Repeat("d", 64), 998, 100, 99800, 4)
	addBid(strings.Repeat("e", 64), 991, 100, 49550, 5)
	addBid(strings.Repeat("f", 64), 975, 100, 97500, 6)

	depth := book.GetDepth(pairKey, 1, 20)
	if len(depth.Asks) != 2 || depth.Asks[0].Price != "10.2" || depth.Asks[0].TokenAmount != 5000 || depth.Asks[0].OrderCount != 2 {
		t.Fatalf("unexpected ask depth: %+v", depth.Asks)
	}
	if depth.Asks[1].Price != "10.3" || depth.Asks[1].CumulativeTok != 6000 {
		t.Fatalf("unexpected ask cumulative: %+v", depth.Asks[1])
	}
	if len(depth.Bids) != 2 || depth.Bids[0].Price != "9.9" || depth.Bids[0].TokenAmount != 15000 || depth.Bids[0].OrderCount != 2 {
		t.Fatalf("unexpected bid depth: %+v", depth.Bids)
	}
	if depth.BestAsk != "10.2" || depth.BestBid != "9.9" || depth.Spread != "0.3" {
		t.Fatalf("unexpected spread/top: bestBid=%s bestAsk=%s spread=%s", depth.BestBid, depth.BestAsk, depth.Spread)
	}

	// K-lines aggregation
	baseTs := int64(1700000400000) // aligned to 1m
	trades := []TradeRecord{
		{TradeKey: "1", PairKey: pairKey, PriceNum: 100, PriceDen: 1, FillToken: 100, SatsAmount: 10000, TradeTime: baseTs + 1000},
		{TradeKey: "2", PairKey: pairKey, PriceNum: 120, PriceDen: 1, FillToken: 200, SatsAmount: 24000, TradeTime: baseTs + 15000},
		{TradeKey: "3", PairKey: pairKey, PriceNum: 90, PriceDen: 1, FillToken: 50, SatsAmount: 4500, TradeTime: baseTs + 30000},
		{TradeKey: "4", PairKey: pairKey, PriceNum: 110, PriceDen: 1, FillToken: 100, SatsAmount: 11000, TradeTime: baseTs + 45000},
		{TradeKey: "5", PairKey: pairKey, PriceNum: 115, PriceDen: 1, FillToken: 100, SatsAmount: 11500, TradeTime: baseTs + 65000},
	}
	bars := BuildKlines(trades, "1m", 10)
	if len(bars) != 2 {
		t.Fatalf("expected 2 1m kline bars, got %d", len(bars))
	}
	if bars[0].Open != "100" || bars[0].High != "120" || bars[0].Low != "90" || bars[0].Close != "110" || bars[0].VolumeToken != 450 {
		t.Fatalf("unexpected bar[0]: %+v", bars[0])
	}
}

func TestSmartOrderRouterAndSoftLock(t *testing.T) {
	locker := NewMemorySoftLocker()
	book := NewOrderBook("livenet", locker)
	tokenID := repeatByte(0xaa, 20)
	tokenCodeHash := repeatByte(0xbb, 20)
	pairKey := MakePairKey(hex.EncodeToString(tokenCodeHash), hex.EncodeToString(tokenID))
	maker := repeatByte(0x11, 20)
	fee := repeatByte(0x22, 20)

	addOrder := func(txid string, side uint8, pNum, pDen, amt, minFill uint64, createdAt int64, saltByte byte) string {
		s := makeDummyOrderScript([]byte{side}, side, maker, tokenID, tokenCodeHash, pNum, pDen, amt, minFill, fee, 25, repeatByte(saltByte, 8))
		sats := DustLimit
		if side == SideBid {
			sats = amt
		}
		lo, err := ParseLiveOrder(txid, 0, sats, s, 100, false, createdAt, "livenet")
		if err != nil {
			t.Fatalf("ParseLiveOrder: %v", err)
		}
		book.UpsertOrder(lo)
		return lo.Outpoint
	}

	// Same price 100: small order (3000, older) vs large order (10000, newer) -> router prefers larger (fewest legs)!
	opSmall := addOrder(strings.Repeat("1", 64), SideAsk, 100, 1, 3000, 500, 1000, 1)
	opLarge := addOrder(strings.Repeat("2", 64), SideAsk, 100, 1, 10000, 500, 2000, 2)
	// Better price 95 (4000, minFill 1000) -> always first!
	opCheap := addOrder(strings.Repeat("3", 64), SideAsk, 95, 1, 4000, 1000, 3000, 3)

	plan, err := book.RouteTake(RouteRequest{PairKey: pairKey, Side: "buy", TargetToken: 8000})
	if err != nil {
		t.Fatalf("RouteTake buy failed: %v", err)
	}
	if plan.FilledToken != 8000 || len(plan.Legs) != 2 {
		t.Fatalf("expected 2 legs filling 8000, got %+v", plan)
	}
	if plan.Legs[0].Outpoint != opCheap || plan.Legs[0].FillToken != 4000 || plan.Legs[0].IsPartial {
		t.Fatalf("leg[0] should fully fill cheapest order: %+v", plan.Legs[0])
	}
	if plan.Legs[1].Outpoint != opLarge || plan.Legs[1].FillToken != 4000 || !plan.Legs[1].IsPartial {
		t.Fatalf("leg[1] should partially fill largest same-price order: %+v", plan.Legs[1])
	}

	// Soft-lock opCheap and opLarge -> second concurrent Taker automatically routes to opSmall!
	if err := locker.Acquire([]string{opCheap, opLarge}, "res-1", 25*time.Second); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if err := locker.Acquire([]string{opCheap}, "res-2", 25*time.Second); !errors.Is(err, ErrOutpointLocked) {
		t.Fatalf("expected ErrOutpointLocked for conflicting lock, got %v", err)
	}
	plan2, err := book.RouteTake(RouteRequest{PairKey: pairKey, Side: "buy", TargetToken: 2000})
	if err != nil {
		t.Fatalf("second taker RouteTake failed: %v", err)
	}
	if len(plan2.Legs) != 1 || plan2.Legs[0].Outpoint != opSmall || plan2.Legs[0].FillToken != 2000 {
		t.Fatalf("second taker should skip soft-locked orders and pick opSmall: %+v", plan2)
	}
	_ = locker.Release([]string{opCheap, opLarge}, "res-1")

	// Bid zombie-dust guard test:
	// Bid has 1,000,000 sats @ 100 sat/tok (max 10,000 tok), minFill = 2,000 tok.
	// Taker wants to sell 9,000 tok -> remaining would be 100,000 sats (only 1,000 tok < minFill 2,000!).
	// Router must clamp partial fill to 8,000 tok so remaining 200,000 sats still affords 2,000 tok!
	opBid := addOrder(strings.Repeat("4", 64), SideBid, 100, 1, 1_000_000, 2000, 1000, 4)
	bidPlan, err := book.RouteTake(RouteRequest{PairKey: pairKey, Side: "sell", TargetToken: 9000})
	if err != nil {
		t.Fatalf("RouteTake sell failed: %v", err)
	}
	if len(bidPlan.Legs) != 1 || bidPlan.Legs[0].Outpoint != opBid || bidPlan.Legs[0].FillToken != 8000 || bidPlan.Legs[0].RemainAfter != 200000 {
		t.Fatalf("expected Bid partial fill clamped to 8000 (remain 200000 sats), got %+v", bidPlan.Legs)
	}
}

func TestSyncerSnapshotEventsResetAndWS(t *testing.T) {
	askPrefix := []byte{0x01, 0x02, 0x03}
	bidPrefix := []byte{0x04, 0x05, 0x06}
	askCodeHash := hex.EncodeToString(Hash160(append(append([]byte{}, askPrefix...), 0x6a)))
	bidCodeHash := hex.EncodeToString(Hash160(append(append([]byte{}, bidPrefix...), 0x6a)))

	tokenID := repeatByte(0xaa, 20)
	tokenCodeHash := repeatByte(0xbb, 20)
	pairKey := MakePairKey(hex.EncodeToString(tokenCodeHash), hex.EncodeToString(tokenID))
	maker := repeatByte(0x11, 20)
	fee := repeatByte(0x22, 20)

	askTx0 := strings.Repeat("a", 64)
	askTx1 := strings.Repeat("b", 64)
	askScript0 := makeDummyOrderScript(askPrefix, SideAsk, maker, tokenID, tokenCodeHash, 100, 1, 10000, 1000, fee, 25, repeatByte(0x09, 8))
	askScript1 := makeDummyOrderScript(askPrefix, SideAsk, maker, tokenID, tokenCodeHash, 100, 1, 6000, 1000, fee, 25, repeatByte(0x09, 8))

	var mu sync.Mutex
	phase := 0

	higunSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		p := phase
		mu.Unlock()
		switch r.URL.Path {
		case "/orderbook/utxos":
			ch := r.URL.Query().Get("codehash")
			items := []HigunOrderbookUTXO{}
			seq := uint64(10)
			if p != 0 {
				seq = 50
			}
			if ch == askCodeHash {
				if p == 0 {
					items = append(items, HigunOrderbookUTXO{TxID: askTx0, Vout: 0, Codehash: ch, Satoshis: DustLimit, ScriptHex: askScript0, Height: 100})
				} else {
					items = append(items, HigunOrderbookUTXO{TxID: askTx1, Vout: 0, Codehash: ch, Satoshis: DustLimit, ScriptHex: askScript1, Height: 101})
				}
			}
			_ = json.NewEncoder(w).Encode(HigunOrderbookUTXOsPage{Seq: seq, Items: items})
		case "/orderbook/events":
			if p == 0 {
				// Partial fill event: askTx0:0 spent by askTx1, askTx1:0 created with same salt & remain 6000
				_ = json.NewEncoder(w).Encode(HigunOrderbookEventsPage{
					LatestSeq: 12,
					Events: []HigunOrderbookEvent{
						{Seq: 11, Type: "spent", Codehash: askCodeHash, TxID: askTx0, Vout: 0, SpentBy: askTx1, Height: 101, Timestamp: time.Now().UnixMilli()},
						{Seq: 12, Type: "created", Codehash: askCodeHash, TxID: askTx1, Vout: 0, Satoshis: DustLimit, ScriptHex: askScript1, Height: 101, Timestamp: time.Now().UnixMilli()},
					},
				})
			} else {
				_ = json.NewEncoder(w).Encode(HigunOrderbookEventsPage{LatestSeq: 50, ResetRequired: true})
			}
		}
	}))
	defer higunSrv.Close()

	cfg := conf.DefaultOrderbookConfig()
	cfg.HigunUrl = higunSrv.URL
	cfg.AskCodeHash = askCodeHash
	cfg.BidCodeHash = bidCodeHash

	hub := NewWSHub()
	book := NewOrderBook("livenet", NewMemorySoftLocker())
	syncer := NewSyncer(cfg, "livenet", book, hub, false)

	// Start WS server and connect a real RFC 6455 client
	wsSrv := httptest.NewServer(hub)
	defer wsSrv.Close()

	rawConn, err := net.DialTimeout("tcp", strings.TrimPrefix(wsSrv.URL, "http://"), 2*time.Second)
	if err != nil {
		t.Fatalf("ws dial failed: %v", err)
	}
	defer rawConn.Close()
	_, _ = rawConn.Write([]byte("GET /?pair=" + pairKey + " HTTP/1.1\r\n" +
		"Host: localhost\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"))
	br := bufio.NewReader(rawConn)
	statusLine, err := br.ReadString('\n')
	if err != nil || !strings.Contains(statusLine, "101") {
		t.Fatalf("expected 101 Switching Protocols, got %q err=%v", statusLine, err)
	}
	for {
		line, _ := br.ReadString('\n')
		if line == "\r\n" || line == "" {
			break
		}
	}
	// Read welcome frame
	readWSFrame := func() string {
		_ = rawConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		hdr := make([]byte, 2)
		if _, err := io.ReadFull(br, hdr); err != nil {
			t.Fatalf("readWSFrame hdr: %v", err)
		}
		plen := uint64(hdr[1] & 0x7f)
		if plen == 126 {
			var b [2]byte
			_, _ = io.ReadFull(br, b[:])
			plen = uint64(binary.BigEndian.Uint16(b[:]))
		}
		buf := make([]byte, plen)
		if _, err := io.ReadFull(br, buf); err != nil {
			t.Fatalf("readWSFrame payload: %v", err)
		}
		return string(buf)
	}
	if welcome := readWSFrame(); !strings.Contains(welcome, `"connected"`) {
		t.Fatalf("unexpected ws welcome: %s", welcome)
	}

	// 1. Snapshot bootstrap
	ctx := context.Background()
	if err := syncer.BootstrapSnapshot(ctx); err != nil {
		t.Fatalf("BootstrapSnapshot failed: %v", err)
	}
	if _, ok := book.GetOrder(askTx0 + ":0"); !ok {
		t.Fatalf("expected askTx0:0 in book after snapshot")
	}
	if depthMsg := readWSFrame(); !strings.Contains(depthMsg, `"channel":"depth"`) {
		t.Fatalf("expected depth WS broadcast after bootstrap, got %s", depthMsg)
	}

	// 2. Poll incremental partial fill event
	if err := syncer.PollOnce(ctx); err != nil {
		t.Fatalf("PollOnce failed: %v", err)
	}
	if _, ok := book.GetOrder(askTx0 + ":0"); ok {
		t.Fatalf("askTx0:0 should have been removed after spend")
	}
	contOrder, ok := book.GetOrder(askTx1 + ":0")
	if !ok || contOrder.RemainAmount != 6000 {
		t.Fatalf("expected continuation order askTx1:0 remain=6000, got %+v", contOrder)
	}
	trades := book.GetRecentTrades(pairKey, 10)
	if len(trades) != 1 || trades[0].FillToken != 4000 || !trades[0].IsPartial || trades[0].SatsAmount != 400000 {
		t.Fatalf("expected 1 partial fill trade (4000 tok / 400000 sats), got %+v", trades)
	}
	// Verify trade + depth frames arrived over WS
	msg1 := readWSFrame()
	msg2 := readWSFrame()
	if !(strings.Contains(msg1+msg2, `"channel":"trades"`) && strings.Contains(msg1+msg2, `"channel":"depth"`)) {
		t.Fatalf("expected both trades and depth WS broadcasts, got %s | %s", msg1, msg2)
	}

	// 3. Higun signals resetRequired=true -> clean reload
	mu.Lock()
	phase = 1
	mu.Unlock()
	if err := syncer.PollOnce(ctx); err != nil {
		t.Fatalf("PollOnce resetRequired failed: %v", err)
	}
	if syncer.ResetCount() != 1 || syncer.LastSeq() != 50 {
		t.Fatalf("expected resetCount=1 lastSeq=50, got resetCount=%d lastSeq=%d", syncer.ResetCount(), syncer.LastSeq())
	}
}

func TestOrderTokenV2ProtoBookAndRouter(t *testing.T) {
	mcCH := bytes.Repeat([]byte{0x11}, 20)
	mcID := bytes.Repeat([]byte{0x22}, 20)
	wbtcCH := bytes.Repeat([]byte{0x33}, 20)
	wbtcID := bytes.Repeat([]byte{0x44}, 20)
	makerHash := bytes.Repeat([]byte{0xaa}, 20)
	feeHash := bytes.Repeat([]byte{0xbb}, 20)

	wbtcKey := MakePairKey(hex.EncodeToString(wbtcCH), hex.EncodeToString(wbtcID))
	SetQuoteTokenPriority([]string{wbtcKey})
	defer SetQuoteTokenPriority(nil)

	codePrefix := []byte{0x01, 0x02, 0x03, 0x6a}

	// 1. Maker sells 10,000 MC for WBTC at 500 WBTC / 1 MC -> Canonical ASK
	askTail := BuildOrderTokenTailHex(makerHash, mcID, mcCH, wbtcID, wbtcCH, 500, 1, 10000, 1000, feeHash, 25, []byte{1, 2, 3, 4, 5, 6, 7, 8})
	askScriptHex := hex.EncodeToString(append(append([]byte{}, codePrefix...), askTail...))
	askOrder, err := ParseLiveOrder(strings.Repeat("a", 64), 0, DustLimit, askScriptHex, 100, false, 1700000000000, "mainnet")
	if err != nil {
		t.Fatalf("ParseLiveOrder token ask failed: %v", err)
	}
	if !askOrder.IsTokenOrder || askOrder.Side != SideAsk || askOrder.Price != "500" {
		t.Fatalf("unexpected token ask: %+v", askOrder)
	}

	// 2. Maker locks 2,000,000 WBTC to buy MC at 400 WBTC / 1 MC (on-chain rawPrice = 1 MC / 400 WBTC) -> Canonical BID
	bidTail := BuildOrderTokenTailHex(makerHash, wbtcID, wbtcCH, mcID, mcCH, 1, 400, 2000000, 400000, feeHash, 25, []byte{8, 7, 6, 5, 4, 3, 2, 1})
	bidScriptHex := hex.EncodeToString(append(append([]byte{}, codePrefix...), bidTail...))
	bidOrder, err := ParseLiveOrder(strings.Repeat("b", 64), 0, DustLimit, bidScriptHex, 101, false, 1700000001000, "mainnet")
	if err != nil {
		t.Fatalf("ParseLiveOrder token bid failed: %v", err)
	}
	if !bidOrder.IsTokenOrder || bidOrder.Side != SideBid || bidOrder.Price != "400" {
		t.Fatalf("unexpected token bid: %+v", bidOrder)
	}
	if askOrder.PairKey != bidOrder.PairKey {
		t.Fatalf("expected identical canonical pairKey, got ask=%s bid=%s", askOrder.PairKey, bidOrder.PairKey)
	}

	book := NewOrderBook("mainnet", NewMemorySoftLocker())
	book.SetPairMetadataFull(askOrder.PairKey, "MC", 8, "WBTC", 8)
	book.UpsertOrder(askOrder)
	book.UpsertOrder(bidOrder)

	depth := book.GetDepth(askOrder.PairKey, 0, 10)
	if depth.BestAsk != "500" || depth.BestBid != "400" || depth.Spread != "100" {
		t.Fatalf("unexpected FT/FT depth: %+v", depth)
	}

	// Route BUY 4,000 MC (takes canonical Ask -> pays WBTC)
	buyPlan, err := book.RouteTake(RouteRequest{
		PairKey:     askOrder.PairKey,
		Side:        "buy",
		TargetToken: 4000,
	})
	if err != nil {
		t.Fatalf("RouteTake FT/FT buy failed: %v", err)
	}
	if !buyPlan.IsTokenPair || buyPlan.FilledToken != 4000 || buyPlan.TotalSats != 2000000 || buyPlan.Legs[0].FillSellAmount != 4000 {
		t.Fatalf("unexpected FT/FT buyPlan: %+v", buyPlan)
	}
	if buyPlan.BuyTokenID != hex.EncodeToString(wbtcID) {
		t.Fatalf("expected buyTokenID=WBTC, got %s", buyPlan.BuyTokenID)
	}

	// Route SELL 3,000 MC (takes canonical Bid -> pays MC, receives 1,200,000 WBTC)
	sellPlan, err := book.RouteTake(RouteRequest{
		PairKey:     askOrder.PairKey,
		Side:        "sell",
		TargetToken: 3000,
	})
	if err != nil {
		t.Fatalf("RouteTake FT/FT sell failed: %v", err)
	}
	if !sellPlan.IsTokenPair || sellPlan.FilledToken != 3000 || sellPlan.TotalSats != 1200000 || sellPlan.Legs[0].FillSellAmount != 1200000 {
		t.Fatalf("unexpected FT/FT sellPlan: %+v", sellPlan)
	}
	if sellPlan.BuyTokenID != hex.EncodeToString(mcID) {
		t.Fatalf("expected buyTokenID=MC, got %s", sellPlan.BuyTokenID)
	}

	pairs := book.ListPairs()
	if len(pairs) != 1 || !pairs[0].IsTokenPair || pairs[0].Symbol != "MC" || pairs[0].QuoteSymbol != "WBTC" {
		t.Fatalf("unexpected FT/FT pair summary: %+v", pairs)
	}
}

func TestSubUnitPricesDistinctAtDefaultPrecision(t *testing.T) {
	tokenCodeHash := bytes.Repeat([]byte{0xa2}, 20)
	tokenID := bytes.Repeat([]byte{0xbb}, 20)
	maker := bytes.Repeat([]byte{0x11}, 20)
	fee := make([]byte, 20)
	pairKey := MakePairKey(hex.EncodeToString(tokenCodeHash), hex.EncodeToString(tokenID))

	book := NewOrderBook("livenet", NewMemorySoftLocker())
	prices := []struct {
		txByte byte
		num    uint64
		den    uint64
		sats   uint64
		want   string
	}{
		{0x01, 3, 1000, 15000000, "0.003"},
		{0x02, 2, 1000, 20000000, "0.002"},
		{0x03, 1, 1000, 10000000, "0.001"},
	}
	for _, p := range prices {
		script := makeDummyOrderScript([]byte{0x51, 0x52, 0x53}, SideBid, maker, tokenID, tokenCodeHash, p.num, p.den, p.sats, 546, fee, 0, bytes.Repeat([]byte{p.txByte}, 8))
		lo, err := ParseLiveOrder(strings.Repeat(hex.EncodeToString([]byte{p.txByte}), 32), 0, p.sats, script, 200000, false, 1000, "livenet")
		if err != nil {
			t.Fatalf("ParseLiveOrder bid %s failed: %v", p.want, err)
		}
		book.UpsertOrder(lo)
	}

	depth := book.GetDepth(pairKey, DefaultDepthPrecision, 50)
	if len(depth.Bids) != 3 {
		t.Fatalf("expected 3 distinct bid buckets at DefaultDepthPrecision=%d, got %d: %+v", DefaultDepthPrecision, len(depth.Bids), depth.Bids)
	}
	for i, want := range []string{"0.003", "0.002", "0.001"} {
		if depth.Bids[i].Price != want {
			t.Fatalf("bid[%d] expected price %s, got %s", i, want, depth.Bids[i].Price)
		}
		if depth.Bids[i].OrderCount != 1 {
			t.Fatalf("bid[%d] expected orderCount=1, got %d", i, depth.Bids[i].OrderCount)
		}
	}
}

func TestLimitCrossingAndBidEdgeCases(t *testing.T) {
	tokenCodeHash := bytes.Repeat([]byte{0xa2}, 20)
	tokenID := bytes.Repeat([]byte{0x07}, 20)
	maker := bytes.Repeat([]byte{0x12}, 20)
	fee := make([]byte, 20)
	pairKey := MakePairKey(hex.EncodeToString(tokenCodeHash), hex.EncodeToString(tokenID))

	book := NewOrderBook("livenet", NewMemorySoftLocker())

	// Two Asks: 100 MC @ 0.01, 20 MC @ 0.011
	ask1Script := makeDummyOrderScript([]byte{0x51, 0x52, 0x53}, SideAsk, maker, tokenID, tokenCodeHash, 1, 100, 100_0000_0000, 1_0000_0000, fee, 0, bytes.Repeat([]byte{0x11}, 8))
	ask1, err := ParseLiveOrder(strings.Repeat("a1", 32), 0, 546, ask1Script, 200000, false, 1000, "livenet")
	if err != nil {
		t.Fatalf("ask1 parse failed: %v", err)
	}
	book.UpsertOrder(ask1)

	ask2Script := makeDummyOrderScript([]byte{0x51, 0x52, 0x53}, SideAsk, maker, tokenID, tokenCodeHash, 11, 1000, 20_0000_0000, 2000_0000, fee, 0, bytes.Repeat([]byte{0x12}, 8))
	ask2, err := ParseLiveOrder(strings.Repeat("a2", 32), 0, 546, ask2Script, 200000, false, 1001, "livenet")
	if err != nil {
		t.Fatalf("ask2 parse failed: %v", err)
	}
	book.UpsertOrder(ask2)

	// Limit Buy at 0.01 for 150 MC -> fills 100 MC @ 0.01, leaves 50 MC unfilled
	planExact, err := book.RouteTake(RouteRequest{
		PairKey:     pairKey,
		Side:        "buy",
		TargetToken: 150_0000_0000,
		MaxPriceNum: 1,
		MaxPriceDen: 100,
	})
	if err != nil {
		t.Fatalf("limit buy @ 0.01 failed: %v", err)
	}
	if len(planExact.Legs) != 1 || planExact.FilledToken != 100_0000_0000 || planExact.UnfilledToken != 50_0000_0000 {
		t.Fatalf("unexpected exact limit buy plan: %+v", planExact)
	}

	// Limit Buy at 0.011 for 110 MC -> sweeps 100 MC @ 0.01 + 10 MC @ 0.011
	planHigher, err := book.RouteTake(RouteRequest{
		PairKey:     pairKey,
		Side:        "buy",
		TargetToken: 110_0000_0000,
		MaxPriceNum: 11,
		MaxPriceDen: 1000,
	})
	if err != nil {
		t.Fatalf("limit buy @ 0.011 failed: %v", err)
	}
	if len(planHigher.Legs) != 2 || planHigher.FilledToken != 110_0000_0000 || planHigher.UnfilledToken != 0 {
		t.Fatalf("unexpected higher limit buy plan: %+v", planHigher)
	}

	// Limit Buy at 0.009 -> no crossing liquidity
	if _, err := book.RouteTake(RouteRequest{
		PairKey:     pairKey,
		Side:        "buy",
		TargetToken: 10_0000_0000,
		MaxPriceNum: 9,
		MaxPriceDen: 1000,
	}); err == nil {
		t.Fatalf("expected error for non-crossing limit buy @ 0.009")
	}

	// Tiny Bid (5000 sats) should be skipped because proceeds < MinBidSettleMinerFee - DustLimit
	tinyBidScript := makeDummyOrderScript([]byte{0x51, 0x52, 0x53}, SideBid, maker, tokenID, tokenCodeHash, 1, 100, 5000, 1000, fee, 0, bytes.Repeat([]byte{0x21}, 8))
	tinyBid, err := ParseLiveOrder(strings.Repeat("b1", 32), 0, 5000, tinyBidScript, 200000, false, 1000, "livenet")
	if err != nil {
		t.Fatalf("tinyBid parse failed: %v", err)
	}
	book.UpsertOrder(tinyBid)
	if _, err := book.RouteTake(RouteRequest{
		PairKey:     pairKey,
		Side:        "sell",
		TargetToken: 1_0000_0000,
	}); err == nil {
		t.Fatalf("expected tiny 5000-sat bid to be rejected by settle miner-fee guard")
	}

	// Non-divisible Bid: 20,000 sats @ 0.003 (3/1000) -> maxTok=floor(20000*1000/3)=6666666 leaves 1 sat;
	// 6666667 spends exact 20,000 sats (remainSatsNext == 0 -> clean full fill!)
	oddBidScript := makeDummyOrderScript([]byte{0x51, 0x52, 0x53}, SideBid, maker, tokenID, tokenCodeHash, 3, 1000, 20000, 100000, fee, 0, bytes.Repeat([]byte{0x22}, 8))
	oddBid, err := ParseLiveOrder(strings.Repeat("b2", 32), 0, 20000, oddBidScript, 200000, false, 1001, "livenet")
	if err != nil {
		t.Fatalf("oddBid parse failed: %v", err)
	}
	book.UpsertOrder(oddBid)
	sellOdd, err := book.RouteTake(RouteRequest{
		PairKey:     pairKey,
		Side:        "sell",
		TargetToken: 7000000,
		MinPriceNum: 3,
		MinPriceDen: 1000,
	})
	if err != nil {
		t.Fatalf("oddBid full-fill route failed: %v", err)
	}
	if len(sellOdd.Legs) != 1 || sellOdd.Legs[0].IsPartial || sellOdd.Legs[0].SatsAmount != 20000 || sellOdd.Legs[0].FillToken != 6666667 {
		t.Fatalf("expected clean full fill of 20000 sats with fillToken=6666667, got %+v", sellOdd.Legs)
	}
}

func TestTokenPairMaxLegsCappedAtTwo(t *testing.T) {
	mcCH := bytes.Repeat([]byte{0xa2}, 20)
	mcID := bytes.Repeat([]byte{0x07}, 20)
	wbtcCH := bytes.Repeat([]byte{0xc9}, 20)
	wbtcID := bytes.Repeat([]byte{0x2e}, 20)
	SetQuoteTokenPriority([]string{MakePairKey(hex.EncodeToString(wbtcCH), hex.EncodeToString(wbtcID))})
	defer SetQuoteTokenPriority(nil)

	maker := bytes.Repeat([]byte{0x12}, 20)
	fee := make([]byte, 20)
	book := NewOrderBook("livenet", NewMemorySoftLocker())
	codePrefix := []byte{0x01, 0x02, 0x03, 0x6a}

	var pairKey string
	for i := byte(1); i <= 3; i++ {
		tail := BuildOrderTokenTailHex(maker, mcID, mcCH, wbtcID, wbtcCH, 1, 1000, 1000000, 10000, fee, 0, bytes.Repeat([]byte{i}, 8))
		sc := hex.EncodeToString(append(append([]byte{}, codePrefix...), tail...))
		lo, err := ParseLiveOrder(strings.Repeat(hex.EncodeToString([]byte{0xc0 + i}), 32), 0, 546, sc, 200000, false, 1000+int64(i), "livenet")
		if err != nil {
			t.Fatalf("token ask %d failed: %v", i, err)
		}
		pairKey = lo.PairKey
		book.UpsertOrder(lo)
	}

	plan, err := book.RouteTake(RouteRequest{
		PairKey:     pairKey,
		Side:        "buy",
		TargetToken: 3000000,
	})
	if err != nil {
		t.Fatalf("FT/FT buy route failed: %v", err)
	}
	if len(plan.Legs) != 2 {
		t.Fatalf("expected FT/FT buy to be capped at 2 legs (3-output TokenTransferCheck), got %d", len(plan.Legs))
	}
}

