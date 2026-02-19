package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
)

// FakeServer simulates the Aqua Temp API for testing.
type FakeServer struct {
	Server     *httptest.Server
	Token      string
	Username   string
	Password   string
	Devices    []DeviceInfo
	DeviceData map[string][]Parameter

	mu             sync.Mutex
	LoginCount     int
	ControlCalls   []ControlCall
	FailLogin      bool
	FailNextNPosts int
}

// DeviceInfo matches the API device list response structure.
type DeviceInfo struct {
	DeviceCode   string `json:"device_code"`
	DeviceNick   string `json:"device_nick"`
	DeviceType   string `json:"device_type"`
	OnlineStatus string `json:"online_status"`
	ProductCode  string `json:"product_code"`
	CreateTime   string `json:"create_time"`
}

// Parameter matches the API device data response structure.
type Parameter struct {
	Code  string `json:"protocal_code"`
	Value string `json:"value"`
}

// ControlCall records a control request for verification.
type ControlCall struct {
	DeviceCode   string
	ProtocolCode string
	Value        string
}

// NewFakeServer creates and starts a new fake Aqua Temp API server.
func NewFakeServer() *FakeServer {
	f := &FakeServer{
		Token:      "fake-token-12345",
		Username:   "test@example.com",
		Password:   "testpass",
		DeviceData: make(map[string][]Parameter),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/app/user/login.json", f.handleLogin)
	mux.HandleFunc("/app/device/deviceList.json", f.handleDeviceList)
	mux.HandleFunc("/app/device/getDataByCode.json", f.handleDeviceData)
	mux.HandleFunc("/app/device/control.json", f.handleControl)

	f.Server = httptest.NewServer(mux)
	return f
}

// Close shuts down the fake server.
func (f *FakeServer) Close() {
	f.Server.Close()
}

// URL returns the server's base URL.
func (f *FakeServer) URL() string {
	return f.Server.URL
}

func (f *FakeServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.LoginCount++
	failLogin := f.FailLogin
	f.mu.Unlock()

	if r.Method != http.MethodPost {
		writeError(w, 405, "method not allowed")
		return
	}

	var req struct {
		UserName string `json:"user_name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, -1, "invalid request body")
		return
	}

	if failLogin || req.UserName != f.Username || req.Password != f.Password {
		writeJSON(w, map[string]any{
			"error_code": 401,
			"error_msg":  "invalid credentials",
		})
		return
	}

	writeJSON(w, map[string]any{
		"error_code": 0,
		"error_msg":  "",
		"objectResult": map[string]any{
			"x-token":   f.Token,
			"userId":    "user-123",
			"expiresIn": 3600,
		},
	})
}

func (f *FakeServer) handleDeviceList(w http.ResponseWriter, r *http.Request) {
	if !f.checkToken(w, r) {
		return
	}

	if f.checkServerError(w) {
		return
	}

	writeJSON(w, map[string]any{
		"error_code":   0,
		"error_msg":    "",
		"objectResult": f.Devices,
	})
}

func (f *FakeServer) handleDeviceData(w http.ResponseWriter, r *http.Request) {
	if !f.checkToken(w, r) {
		return
	}

	var req struct {
		DeviceCode string `json:"device_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, -1, "invalid request body")
		return
	}

	params, ok := f.DeviceData[req.DeviceCode]
	if !ok {
		writeJSON(w, map[string]any{
			"error_code":   0,
			"error_msg":    "",
			"objectResult": []any{},
		})
		return
	}

	writeJSON(w, map[string]any{
		"error_code":   0,
		"error_msg":    "",
		"objectResult": params,
	})
}

func (f *FakeServer) handleControl(w http.ResponseWriter, r *http.Request) {
	if !f.checkToken(w, r) {
		return
	}

	var req struct {
		DeviceCode   string `json:"device_code"`
		ProtocolCode string `json:"protocal_code"`
		Value        string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, -1, "invalid request body")
		return
	}

	f.mu.Lock()
	f.ControlCalls = append(f.ControlCalls, ControlCall{
		DeviceCode:   req.DeviceCode,
		ProtocolCode: req.ProtocolCode,
		Value:        req.Value,
	})
	f.mu.Unlock()

	writeJSON(w, map[string]any{
		"error_code": 0,
		"error_msg":  "",
	})
}

func (f *FakeServer) checkToken(w http.ResponseWriter, r *http.Request) bool {
	token := r.Header.Get("x-token")
	if token == "" || token != "bearer "+f.Token {
		writeJSON(w, map[string]any{
			"error_code": 401,
			"error_msg":  "token invalid or expired",
		})
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

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, map[string]any{
		"error_code": code,
		"error_msg":  message,
	})
}
