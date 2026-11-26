package testutil

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"time"
)

// FakeServer simulates the modern (crmservice) Aqua Temp API for testing.
// Its wire format mirrors the real API: no ".json" suffix, camelCase keys,
// string error codes, a bare x-token header, and "-100" for invalid tokens.
type FakeServer struct {
	Server        *httptest.Server
	Token         string
	UserID        string
	Username      string
	Password      string // plain text; the client must send its MD5
	Devices       []DeviceInfo
	SharedDevices []DeviceInfo
	DeviceData    map[string][]Parameter

	// Status endpoints.
	DeviceStatus map[string]string // deviceCode -> ONLINE/OFFLINE (default ONLINE)
	DeviceFaults map[string][]map[string]any

	// Parameter catalogs.
	ProtocalID     string
	StatusParams   []map[string]any
	SettingsParams []map[string]any

	// History: address -> frequency -> values. An address absent here has no
	// history configured.
	History map[string]map[string][]HistoryValue

	LastDataRequest    map[string]any
	LastDataEndpoint   string
	LastHistoryRequest map[string]any
	ProductInfoCalls   int

	mu             sync.Mutex
	LoginCount     int
	ControlCalls   []ControlCall
	ControlReqs    int
	FailLogin      bool
	FailNextNPosts int
}

// DeviceInfo matches a device list entry.
type DeviceInfo struct {
	DeviceCode     string `json:"deviceCode"`
	DeviceNickName string `json:"deviceNickName"`
	DeviceStatus   string `json:"deviceStatus"`
	ProductID      string `json:"productId"`
}

// Parameter matches a getDataByCode result entry.
type Parameter struct {
	Code  string `json:"code"`
	Value string `json:"value"`
}

// HistoryValue is one listCollectData sample.
type HistoryValue struct {
	DateTime     string `json:"dateTime"`
	AddressValue string `json:"addressValue"`
}

// ControlCall records one entry of a control request's "param" list.
type ControlCall struct {
	DeviceCode   string
	ProtocolCode string
	Value        any
}

// NewFakeServer creates and starts a new fake Aqua Temp API server.
func NewFakeServer() *FakeServer {
	f := &FakeServer{
		Token:        "fake-token-12345",
		UserID:       "user-123",
		Username:     "test@example.com",
		Password:     "testpass",
		DeviceData:   make(map[string][]Parameter),
		DeviceStatus: make(map[string]string),
		DeviceFaults: make(map[string][]map[string]any),
		ProtocalID:   "PROTO1",
		History:      make(map[string]map[string][]HistoryValue),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/app/user/login", f.handleLogin)
	mux.HandleFunc("/app/device/deviceList", f.handleDeviceList)
	mux.HandleFunc("/app/device/getMyAppectDeviceShareDataList", f.handleSharedDeviceList)
	mux.HandleFunc("/app/device/getDataByCode", f.handleDeviceData)
	mux.HandleFunc("/app/device/control", f.handleControl)
	mux.HandleFunc("/app/device/v5/getDataByCode", f.handleDeviceDataV5)
	mux.HandleFunc("/app/device/getDeviceStatus", f.handleDeviceStatus)
	mux.HandleFunc("/app/device/getFaultDataByDeviceCode", f.handleFaults)
	mux.HandleFunc("/app/product/queryProductById", f.handleProductInfo)
	mux.HandleFunc("/app/device/getControlDetailStatusByDeviceCode", f.handleStatusParams)
	mux.HandleFunc("/app/device/getControlParamConfigByDeviceCode", f.handleSettingsParams)
	mux.HandleFunc("/device/snapshot/listCollectData", f.handleHistory)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r) // the real server answers unknown paths with 404
	})

	f.Server = httptest.NewServer(mux)
	return f
}

// Close shuts down the fake server.
func (f *FakeServer) Close() { f.Server.Close() }

// URL returns the server's base URL.
func (f *FakeServer) URL() string { return f.Server.URL }

// InvalidateToken simulates the token being revoked (e.g. by an app login).
func (f *FakeServer) InvalidateToken(newToken string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Token = newToken
}

func (f *FakeServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.LoginCount++
	failLogin := f.FailLogin
	f.mu.Unlock()

	var req struct {
		UserName string `json:"userName"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeResult(w, "-1", "invalid request body", nil)
		return
	}

	sum := md5.Sum([]byte(f.Password))
	if failLogin || req.UserName != f.Username || req.Password != hex.EncodeToString(sum[:]) {
		writeResult(w, "-1", "用户名或密码错误", nil)
		return
	}

	f.mu.Lock()
	token := f.Token
	f.mu.Unlock()
	writeResult(w, "0", "Success", map[string]any{
		"x-token": token,
		"userId":  f.UserID,
	})
}

func (f *FakeServer) handleDeviceList(w http.ResponseWriter, r *http.Request) {
	if !f.checkToken(w, r) || f.checkServerError(w) {
		return
	}
	writeResult(w, "0", "Success", f.Devices)
}

func (f *FakeServer) handleSharedDeviceList(w http.ResponseWriter, r *http.Request) {
	if !f.checkToken(w, r) {
		return
	}
	var req struct {
		ToUser string `json:"toUser"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.ToUser != f.UserID {
		writeResult(w, "0", "Success", []any{})
		return
	}
	writeResult(w, "0", "Success", f.SharedDevices)
}

func (f *FakeServer) handleDeviceData(w http.ResponseWriter, r *http.Request) {
	if !f.checkToken(w, r) {
		return
	}
	f.mu.Lock()
	f.LastDataEndpoint = r.URL.Path
	f.mu.Unlock()
	var req struct {
		DeviceCode    string   `json:"deviceCode"`
		ProtocalCodes []string `json:"protocalCodes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeResult(w, "-1", "invalid request body", nil)
		return
	}
	params := f.DeviceData[req.DeviceCode]
	if params == nil {
		params = []Parameter{}
	}
	writeResult(w, "0", "Success", params)
}

func (f *FakeServer) handleControl(w http.ResponseWriter, r *http.Request) {
	if !f.checkToken(w, r) {
		return
	}
	var req struct {
		Param []struct {
			DeviceCode   string `json:"deviceCode"`
			ProtocolCode string `json:"protocolCode"`
			Value        any    `json:"value"`
		} `json:"param"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Param) == 0 {
		writeResult(w, "-1", "invalid request body", nil)
		return
	}

	f.mu.Lock()
	f.ControlReqs++
	for _, p := range req.Param {
		f.ControlCalls = append(f.ControlCalls, ControlCall{
			DeviceCode:   p.DeviceCode,
			ProtocolCode: p.ProtocolCode,
			Value:        p.Value,
		})
	}
	f.mu.Unlock()

	writeResult(w, "0", "Success", nil)
}

// handleDeviceDataV5 answers the requested codes and bits; unknown ones come
// back with an empty value and dataType, as on the real API.
func (f *FakeServer) handleDeviceDataV5(w http.ResponseWriter, r *http.Request) {
	if !f.checkToken(w, r) {
		return
	}
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeResult(w, "-1", "invalid request body", nil)
		return
	}
	f.mu.Lock()
	f.LastDataRequest = raw
	f.LastDataEndpoint = r.URL.Path
	f.mu.Unlock()

	code, _ := raw["deviceCode"].(string)
	known := make(map[string]string)
	for _, p := range f.DeviceData[code] {
		known[p.Code] = p.Value
	}

	items := []map[string]any{}
	for _, field := range []string{"codes", "bitCodes"} {
		list, _ := raw[field].([]any)
		for _, c := range list {
			name, _ := c.(string)
			v, ok := known[name]
			item := map[string]any{"code": name, "value": v, "dataType": "TEMP"}
			if field == "bitCodes" {
				item["dataType"] = nil
			}
			if !ok {
				item["value"], item["dataType"] = "", ""
			}
			items = append(items, item)
		}
	}
	writeResult(w, "0", "Success", items)
}

func (f *FakeServer) handleDeviceStatus(w http.ResponseWriter, r *http.Request) {
	if !f.checkToken(w, r) {
		return
	}
	code := decodeDeviceCode(r)
	status := f.DeviceStatus[code]
	if status == "" {
		status = "ONLINE"
	}
	fault := len(f.DeviceFaults[code]) > 0
	writeResult(w, "0", "Success", map[string]any{"status": status, "isFault": fault, "is_fault": fault})
}

func (f *FakeServer) handleFaults(w http.ResponseWriter, r *http.Request) {
	if !f.checkToken(w, r) {
		return
	}
	faults := f.DeviceFaults[decodeDeviceCode(r)]
	if faults == nil {
		faults = []map[string]any{}
	}
	writeResult(w, "0", "Success", faults)
}

func (f *FakeServer) handleProductInfo(w http.ResponseWriter, r *http.Request) {
	if !f.checkToken(w, r) {
		return
	}
	f.mu.Lock()
	f.ProductInfoCalls++
	f.mu.Unlock()
	writeResult(w, "0", "Success", map[string]any{"protocalId": f.ProtocalID, "productKey": "secret"})
}

func (f *FakeServer) handleStatusParams(w http.ResponseWriter, r *http.Request) {
	if !f.checkToken(w, r) {
		return
	}
	var req struct {
		ProtocalID string `json:"protocalId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.ProtocalID != f.ProtocalID {
		writeResult(w, "0", "Success", map[string]any{"total": 0, "dataList": []any{}})
		return
	}
	writeResult(w, "0", "Success", map[string]any{"total": len(f.StatusParams), "dataList": f.StatusParams})
}

func (f *FakeServer) handleSettingsParams(w http.ResponseWriter, r *http.Request) {
	if !f.checkToken(w, r) {
		return
	}
	writeResult(w, "0", "Success", map[string]any{"total": len(f.SettingsParams), "dataList": f.SettingsParams})
}

// handleHistory mimics listCollectData, including its request validation:
// times must be "yyyy-MM-dd HH:mm:ss" strings and timeZone a whole-hour
// offset string, or the API answers HTTP 400.
func (f *FakeServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	if !f.checkToken(w, r) {
		return
	}
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w)
		return
	}
	f.mu.Lock()
	f.LastHistoryRequest = req
	f.mu.Unlock()

	start, ok1 := req["startTime"].(string)
	end, ok2 := req["endTime"].(string)
	tz, ok3 := req["timeZone"].(string)
	address, _ := req["address"].(string)
	frequency, _ := req["frequency"].(string)
	const layout = "2006-01-02 15:04:05"
	if !ok1 || !ok2 || !ok3 {
		writeBadRequest(w)
		return
	}
	if _, err := strconv.Atoi(tz); err != nil {
		writeBadRequest(w)
		return
	}
	if _, err := time.Parse(layout, start); err != nil {
		writeResult(w, "0", "Success", emptyHistory(""))
		return
	}
	if _, err := time.Parse(layout, end); err != nil {
		writeResult(w, "0", "Success", emptyHistory(""))
		return
	}

	series, ok := f.History[address]
	if !ok {
		writeResult(w, "0", "Success", emptyHistory(""))
		return
	}
	values := series[frequency]
	if values == nil {
		values = []HistoryValue{}
	}
	writeResult(w, "0", "Success", map[string]any{
		"title": address + " 进水温度", "total": strconv.Itoa(len(values)), "address": address,
		"configId": "CFG" + address, "valueList": values,
		"fengList": []any{}, "guList": []any{}, "pingList": []any{},
	})
}

func emptyHistory(address string) map[string]any {
	return map[string]any{"title": "", "total": "", "address": address, "configId": "",
		"valueList": []any{}, "fengList": []any{}, "guList": []any{}, "pingList": []any{}}
}

func decodeDeviceCode(r *http.Request) string {
	var req struct {
		DeviceCode string `json:"deviceCode"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	return req.DeviceCode
}

func writeBadRequest(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]any{
		"error_code": "-1", "error_msg": "请求参数格式错误", "isReusltSuc": false,
	})
}

func (f *FakeServer) checkToken(w http.ResponseWriter, r *http.Request) bool {
	f.mu.Lock()
	want := f.Token
	f.mu.Unlock()
	if r.Header.Get("x-token") != want {
		writeResult(w, "-100", "请重新登录", nil)
		return false
	}
	return true
}

func (f *FakeServer) checkServerError(w http.ResponseWriter) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FailNextNPosts > 0 {
		f.FailNextNPosts--
		w.WriteHeader(http.StatusInternalServerError)
		return true
	}
	return false
}

func writeResult(w http.ResponseWriter, code, msg string, result any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"error_code":   code,
		"error_msg":    msg,
		"isReusltSuc":  code == "0",
		"objectResult": result,
	})
}
