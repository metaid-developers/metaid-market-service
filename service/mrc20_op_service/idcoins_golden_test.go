package mrc20_op_service

import (
	"encoding/json"
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/controller/request"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

const defaultIdCoinsGoldenDir = "/Users/tusm/Documents/MetaID_Projects/_tmp/idcoins-golden-20261128"

type idCoinsGoldenDiff struct {
	File     string      `json:"file"`
	Path     string      `json:"path"`
	Expected interface{} `json:"expected"`
	Actual   interface{} `json:"actual"`
}

func jsonValue(t *testing.T, v interface{}) interface{} {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal actual: %v", err)
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("normalize actual: %v", err)
	}
	return out
}

func readJSONFile(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("parse golden %s: %v", path, err)
	}
	return out
}

func goldenData(doc map[string]interface{}) map[string]interface{} {
	if response, ok := doc["response"].(map[string]interface{}); ok {
		if data, ok := response["data"].(map[string]interface{}); ok {
			return data
		}
	}
	if data, ok := doc["data"].(map[string]interface{}); ok {
		return data
	}
	return doc
}

func diffJSONValues(path string, expected, actual interface{}, skip func(string) bool, out *[]idCoinsGoldenDiff, file string) {
	if skip != nil && skip(path) {
		return
	}
	if reflect.DeepEqual(expected, actual) {
		return
	}
	em, eok := expected.(map[string]interface{})
	am, aok := actual.(map[string]interface{})
	if eok && aok {
		keys := map[string]bool{}
		for k := range em {
			keys[k] = true
		}
		for k := range am {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			diffJSONValues(joinDiffPath(path, k), em[k], am[k], skip, out, file)
		}
		return
	}
	el, eok := expected.([]interface{})
	al, aok := actual.([]interface{})
	if eok && aok {
		if len(el) != len(al) {
			*out = append(*out, idCoinsGoldenDiff{File: file, Path: path + ".length", Expected: len(el), Actual: len(al)})
			return
		}
		for i := range el {
			diffJSONValues(fmt.Sprintf("%s[%d]", path, i), el[i], al[i], skip, out, file)
		}
		return
	}
	*out = append(*out, idCoinsGoldenDiff{File: file, Path: path, Expected: expected, Actual: actual})
}

func joinDiffPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

func idCoinsGoldenSkip(path string) bool {
	marketFields := map[string]bool{
		"marketPrice": true, "marketPriceUsd": true, "floorPrice": true, "floorPriceUsd": true,
		"change24h": true, "marketCap": true, "marketCapUsd": true, "totalVolume": true,
	}
	parts := strings.Split(path, ".")
	last := parts[len(parts)-1]
	if last == "ordersPrice" || last == "ordersPool" || marketFields[last] {
		return true
	}
	if strings.Contains(path, "metaData") && strings.Contains(strings.ToLower(path), "shabi") {
		return true
	}
	return false
}

func diffGolden(t *testing.T, file string, expected, actual interface{}) []idCoinsGoldenDiff {
	t.Helper()
	diffs := make([]idCoinsGoldenDiff, 0)
	skip := func(path string) bool {
		if idCoinsGoldenSkip(path) {
			return true
		}
		// ShaBi's legacy metaData was truncated in the retired database; MAN now
		// returns the correct full value. Detect by file name because list paths do
		// not carry the tick in their path.
		if strings.HasSuffix(path, "metaData") && strings.Contains(file, "ShaBi") {
			return true
		}
		return false
	}
	diffJSONValues("", expected, actual, skip, &diffs, file)
	filtered := diffs[:0]
	for _, d := range diffs {
		// Legacy truncated metaData at 512 chars in its database; MAN returns the
		// correct full value. Treat an exact 512-char prefix as an allowed diff.
		if strings.HasSuffix(d.Path, "metaData") {
			expStr, expOk := d.Expected.(string)
			actStr, actOk := d.Actual.(string)
			if expOk && actOk && len([]rune(expStr)) == 512 && strings.HasPrefix(actStr, expStr) && actStr != expStr {
				continue
			}
		}
		filtered = append(filtered, d)
	}
	return filtered
}

func resetIdCoinsCachesForTest() {
	idCoinsTickCacheMu.Lock()
	idCoinsTickCache = make(map[string]idCoinsTickCacheEntry)
	idCoinsTickCacheMu.Unlock()
	idCoinsFollowersCacheMu.Lock()
	idCoinsFollowersCache = make(map[string]idCoinsFollowersCacheEntry)
	idCoinsFollowersCacheMu.Unlock()
	idCoinsUserInfoCacheMu.Lock()
	idCoinsUserInfoCache = make(map[string]idCoinsUserInfoCacheEntry)
	idCoinsUserInfoCacheMu.Unlock()
}

type idCoinsLatencyReport struct {
	Endpoint string `json:"endpoint"`
	ColdMs   int64  `json:"coldMs"`
	WarmMs   int64  `json:"warmMs"`
}

func measureIdCoinsEndpointLatency(t *testing.T, name string, call func() error) idCoinsLatencyReport {
	t.Helper()
	report := idCoinsLatencyReport{Endpoint: name}
	resetIdCoinsCachesForTest()
	start := time.Now()
	if err := call(); err != nil {
		t.Fatalf("%s cold call: %v", name, err)
	}
	report.ColdMs = time.Since(start).Milliseconds()
	start = time.Now()
	if err := call(); err != nil {
		t.Fatalf("%s warm call: %v", name, err)
	}
	report.WarmMs = time.Since(start).Milliseconds()
	return report
}

func TestIdCoinsGoldenComparison(t *testing.T) {
	if os.Getenv("IDCOINS_GOLDEN") != "1" {
		t.Skip("set IDCOINS_GOLDEN=1 to compare against live MAN and captured production goldens")
	}
	goldenDir := os.Getenv("IDCOINS_GOLDEN_DIR")
	if goldenDir == "" {
		goldenDir = defaultIdCoinsGoldenDir
	}
	conf.ManDomain = "https://v1man.metaid.io"
	conf.ManBaseDomain = "https://v1man.metaid.io"
	conf.Net = "mainnet"

	latencies := []idCoinsLatencyReport{
		measureIdCoinsEndpointLatency(t, "coins-info", func() error {
			_, err := fetchIdCoinsInfoLocally(&request.FetchOneIdCoinsRequest{TickId: "8ce590918a3d493631ab9d9e3bbc89e322f3dd08354af87f97c07218818b78f4i0"})
			return err
		}),
		measureIdCoinsEndpointLatency(t, "coins-list", func() error {
			_, err := fetchIdCoinsListLocally(&request.FetchIdCoinsListRequest{Cursor: 0, Size: 50, SortType: -1})
			return err
		}),
		measureIdCoinsEndpointLatency(t, "address/coins-info", func() error {
			_, err := fetchAddressIdCoinsInfoLocally(&request.FetchAddressIdCoinInfoRequest{IssuerAddress: "1BorJ1ZAtJUxT2aCpSBuLgejvaQ4i7CSyz"})
			return err
		}),
		measureIdCoinsEndpointLatency(t, "coins-simple-list", func() error {
			_, err := fetchIdCoinsSimpleInfoListLocally(0, 100)
			return err
		}),
	}
	for _, l := range latencies {
		t.Logf("latency %s: cold %dms warm %dms", l.Endpoint, l.ColdMs, l.WarmMs)
	}
	resetIdCoinsCachesForTest()

	allDiffs := make([]idCoinsGoldenDiff, 0)

	files, err := filepath.Glob(filepath.Join(goldenDir, "coins-info", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	for _, file := range files {
		doc := readJSONFile(t, file)
		reqDoc, _ := doc["request"].(map[string]interface{})
		params := map[string]string{}
		if rawParams, ok := reqDoc["params"].(map[string]interface{}); ok {
			for k, v := range rawParams {
				if s, ok := v.(string); ok {
					params[k] = s
				}
			}
		}
		req := &request.FetchOneIdCoinsRequest{
			TickId:        params["tickId"],
			Tick:          params["tick"],
			IssuerAddress: params["issuerAddress"],
			Address:       params["address"],
		}
		actual, err := fetchIdCoinsInfoLocally(req)
		if err != nil {
			allDiffs = append(allDiffs, idCoinsGoldenDiff{File: filepath.Base(file), Path: "_error", Expected: "success", Actual: err.Error()})
			continue
		}
		allDiffs = append(allDiffs, diffGolden(t, filepath.Base(file), goldenData(doc), jsonValue(t, actual))...)
	}

	doc := readJSONFile(t, filepath.Join(goldenDir, "coins-list_c0_s50.json"))
	actualList, err := fetchIdCoinsListLocally(&request.FetchIdCoinsListRequest{Cursor: 0, Size: 50, SortType: -1})
	if err != nil {
		allDiffs = append(allDiffs, idCoinsGoldenDiff{File: "coins-list_c0_s50.json", Path: "_error", Expected: "success", Actual: err.Error()})
	} else {
		allDiffs = append(allDiffs, diffGolden(t, "coins-list_c0_s50.json", goldenData(doc), jsonValue(t, actualList))...)
	}

	metasoDir := filepath.Join(goldenDir, "metaso-7071")
	simpleCases := []struct {
		file   string
		cursor int64
		size   int64
	}{
		{"coins-simple-list_c0_s10.json", 0, 10},
		{"coins-simple-list_c0_s100.json", 0, 100},
		{"coins-simple-list_c10_s5.json", 10, 5},
		{"coins-simple-list_c30_s10.json", 30, 10},
	}
	for _, tc := range simpleCases {
		doc = readJSONFile(t, filepath.Join(metasoDir, tc.file))
		actual, err := fetchIdCoinsSimpleInfoListLocally(tc.cursor, tc.size)
		if err != nil {
			allDiffs = append(allDiffs, idCoinsGoldenDiff{File: tc.file, Path: "_error", Expected: "success", Actual: err.Error()})
		} else {
			allDiffs = append(allDiffs, diffGolden(t, tc.file, goldenData(doc), jsonValue(t, actual))...)
		}
	}

	addressFiles, err := filepath.Glob(filepath.Join(metasoDir, "address-coins-info_*.json"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(addressFiles)
	for _, file := range addressFiles {
		base := filepath.Base(file)
		issuer := strings.TrimSuffix(strings.TrimPrefix(base, "address-coins-info_"), ".json")
		doc = readJSONFile(t, file)
		actual, err := fetchAddressIdCoinsInfoLocally(&request.FetchAddressIdCoinInfoRequest{IssuerAddress: issuer})
		if code, ok := doc["code"].(float64); ok && code != 0 {
			// The golden itself is an error response (e.g. unknown issuer).
			msg, _ := doc["message"].(string)
			if err == nil {
				allDiffs = append(allDiffs, idCoinsGoldenDiff{File: base, Path: "_error", Expected: msg, Actual: "success"})
			} else if err.Error() != msg {
				allDiffs = append(allDiffs, idCoinsGoldenDiff{File: base, Path: "_error", Expected: msg, Actual: err.Error()})
			}
			continue
		}
		if err != nil {
			allDiffs = append(allDiffs, idCoinsGoldenDiff{File: base, Path: "_error", Expected: "success", Actual: err.Error()})
		} else {
			allDiffs = append(allDiffs, diffGolden(t, base, goldenData(doc), jsonValue(t, actual))...)
		}
	}

	artifactDir := "/Users/tusm/Documents/MetaID_Projects/_tmp/idcoins-verification"
	_ = os.MkdirAll(artifactDir, 0755)
	raw, _ := json.MarshalIndent(allDiffs, "", "  ")
	artifact := filepath.Join(artifactDir, "golden-diffs.json")
	if err := os.WriteFile(artifact, raw, 0644); err != nil {
		t.Fatalf("write diff artifact: %v", err)
	}
	latRaw, _ := json.MarshalIndent(latencies, "", "  ")
	if err := os.WriteFile(filepath.Join(artifactDir, "latency.json"), latRaw, 0644); err != nil {
		t.Fatalf("write latency artifact: %v", err)
	}
	if len(allDiffs) > 0 {
		limit := len(allDiffs)
		if limit > 20 {
			limit = 20
		}
		preview, _ := json.MarshalIndent(allDiffs[:limit], "", "  ")
		t.Fatalf("%d unexpected golden diffs (artifact %s):\n%s", len(allDiffs), artifact, preview)
	}
}
