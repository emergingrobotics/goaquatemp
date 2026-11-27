package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Response is the common envelope returned by every endpoint:
//
//	{"error_code":"0","error_msg":"Success","isReusltSuc":true,"objectResult":...}
//
// error_code is a JSON string on the real API; numbers are accepted too.
type Response struct {
	ErrorCode    int
	ErrorMessage string
	Result       json.RawMessage
}

// IsSuccess returns true if the response indicates success.
func (r *Response) IsSuccess() bool {
	return r.ErrorCode == 0
}

// ParseResponse decodes the envelope, picking the result field by dialect.
func (d Dialect) ParseResponse(body []byte) (*Response, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	resp := &Response{Result: raw[d.ObjectResult]}

	if v, ok := raw["error_code"]; ok {
		code, err := parseFlexInt(v)
		if err != nil {
			return nil, fmt.Errorf("invalid error_code %s: %w", string(v), err)
		}
		resp.ErrorCode = code
	}
	if v, ok := raw["error_msg"]; ok {
		_ = json.Unmarshal(v, &resp.ErrorMessage)
	}
	return resp, nil
}

// LoginResult holds the fields used from a login response.
type LoginResult struct {
	Token  string
	UserID string
}

// ParseLogin extracts the token and user id from a login result.
func (d Dialect) ParseLogin(result json.RawMessage) (*LoginResult, error) {
	obj, err := decodeObject(result)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token:  stringify(obj["x-token"]),
		UserID: stringify(obj[d.UserID]),
	}, nil
}

// ParseUserID extracts the user id from a getUserInfo result.
func (d Dialect) ParseUserID(result json.RawMessage) (string, error) {
	obj, err := decodeObject(result)
	if err != nil {
		return "", err
	}
	return stringify(obj[d.UserID]), nil
}

// DeviceInfo is a device as returned by the device list endpoints.
type DeviceInfo struct {
	DeviceCode string
	Nickname   string
	ProductID  string
	Status     string
	Raw        map[string]any
}

// IsOnline reports whether the device status indicates it is online.
func (d *DeviceInfo) IsOnline() bool {
	s := strings.ToUpper(d.Status)
	return s == "ONLINE" || s == "1"
}

// ParseDevices extracts the device list from a device list result.
func (d Dialect) ParseDevices(result json.RawMessage) ([]DeviceInfo, error) {
	if len(result) == 0 || string(result) == "null" {
		return nil, nil
	}
	var items []map[string]any
	if err := json.Unmarshal(result, &items); err != nil {
		return nil, fmt.Errorf("unexpected device list format: %w", err)
	}
	devices := make([]DeviceInfo, 0, len(items))
	for _, item := range items {
		devices = append(devices, DeviceInfo{
			DeviceCode: stringify(item[d.DeviceCode]),
			Nickname:   stringify(item[d.DeviceNickName]),
			ProductID:  stringify(item[d.ProductID]),
			Status:     stringify(item[d.DeviceStatus]),
			Raw:        item,
		})
	}
	return devices, nil
}

// ParseDataParams converts a getDataByCode result ([{code, value}, ...]) into
// a code -> value map.
func ParseDataParams(result json.RawMessage) (map[string]string, error) {
	params := make(map[string]string)
	if len(result) == 0 || string(result) == "null" {
		return params, nil
	}
	var items []struct {
		Code  string `json:"code"`
		Value any    `json:"value"`
	}
	if err := json.Unmarshal(result, &items); err != nil {
		return nil, fmt.Errorf("unexpected device data format: %w", err)
	}
	for _, it := range items {
		params[it.Code] = stringify(it.Value)
	}
	return params, nil
}

// ParsedDeviceData holds the device values the SDK interprets. Numeric
// pointer fields are nil when the device did not report the value.
type ParsedDeviceData struct {
	Power       bool
	Mode        string // set mode: APIMode*
	ModeState   string // operating state: APIState*, "" if unknown
	TargetTemp  float64
	TargetMin   *float64
	TargetMax   *float64
	InletTemp   float64
	OutletTemp  float64
	AmbientTemp float64
	CoilTemp    float64
	SuctionTemp *float64
	ExhaustTemp *float64
	FanSpeed    string

	CompressorRunning *bool    // from the Sys1CompState bit
	CompressorFreq    *float64 // Hz
	CompressorMaxFreq *float64 // Hz, for the current direction
	CompressorCurrent *float64 // A
	CompressorType    *float64

	InverterInputVoltage *float64 // V (T14)
	RealPowerRaw         *float64 // Z04, unit unverified
	EnergyTotal          *float64 // Z06, accumulated electricity, probably kWh

	WaterFlowing *bool
	ExternalStop *bool
	SensorFaults []string // names of status bits reporting a sensor fault
	RawParams    map[string]string
}

// TargetCodeForMode returns the protocol code holding the target temperature
// for an API mode value, or "" if unknown.
func TargetCodeForMode(mode string) string {
	switch mode {
	case APIModeCool:
		return ParamCoolSet
	case APIModeHeat:
		return ParamHeatSet
	case APIModeAuto:
		return ParamAutoSet
	}
	return ""
}

// targetBoundsForMode returns the min/max protocol codes for a mode.
func targetBoundsForMode(mode string) (string, string) {
	switch mode {
	case APIModeCool:
		return ParamCoolMin, ParamCoolMax
	case APIModeHeat:
		return ParamHeatMin, ParamHeatMax
	case APIModeAuto:
		return ParamCoolMin, ParamHeatMax
	}
	return "", ""
}

// ParseDeviceData interprets raw protocol-code values.
func ParseDeviceData(params map[string]string) *ParsedDeviceData {
	data := &ParsedDeviceData{
		Power:       params[ParamPower] == APIPowerOn,
		Mode:        params[ParamMode],
		ModeState:   params[ParamModeState],
		InletTemp:   parseFloat(params[ParamInletTemp]),
		OutletTemp:  parseFloat(params[ParamOutlet]),
		AmbientTemp: parseFloat(params[ParamAmbient]),
		CoilTemp:    parseFloat(params[ParamCoilTemp]),
		SuctionTemp: optFloat(params, ParamSuction),
		ExhaustTemp: optFloat(params, ParamExhaust),
		FanSpeed:    params[ParamFanSpeed],

		CompressorRunning: optBool(params, BitCompressor, "1"),
		CompressorFreq:    optFloat(params, ParamCompFreq),
		CompressorCurrent: optFloat(params, ParamCompCurrent),
		CompressorType:    optFloat(params, ParamCompType),

		InverterInputVoltage: optFloat(params, ParamInverterVolts),
		RealPowerRaw:         optFloat(params, ParamRealPower),
		EnergyTotal:          optFloat(params, ParamEnergyTotal),
		WaterFlowing:         optBool(params, BitWaterFlow, "0"),
		ExternalStop:         optBool(params, BitEmergencyStop, "1"),
		RawParams:            params,
	}

	if code := TargetCodeForMode(data.Mode); code != "" && params[code] != "" {
		data.TargetTemp = parseFloat(params[code])
	} else {
		data.TargetTemp = parseFloat(params[ParamSetTemp])
	}
	if lo, hi := targetBoundsForMode(data.Mode); lo != "" {
		data.TargetMin = optFloat(params, lo)
		data.TargetMax = optFloat(params, hi)
	}

	// Max compressor frequency for the direction the unit is working in.
	maxCode := ""
	switch {
	case data.ModeState == APIStateCooling || (data.ModeState == "" && data.Mode == APIModeCool):
		maxCode = ParamCoolMaxFreq
	case data.ModeState == APIStateHeating || data.ModeState == APIStateDefrosting ||
		(data.ModeState == "" && data.Mode == APIModeHeat):
		maxCode = ParamHeatMaxFreq
	}
	if maxCode != "" {
		data.CompressorMaxFreq = optFloat(params, maxCode)
	}

	for _, bit := range []string{BitInletSensorFault, BitOutletSensorFault, BitAmbientSensorFault} {
		if params[bit] == "1" {
			data.SensorFaults = append(data.SensorFaults, bit)
		}
	}
	return data
}

// optFloat returns the parsed value of code, or nil if absent, empty or not a number.
func optFloat(params map[string]string, code string) *float64 {
	v, ok := params[code]
	if !ok || v == "" {
		return nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil
	}
	return &f
}

// optBool returns whether code equals onValue, or nil if absent or empty.
func optBool(params map[string]string, code, onValue string) *bool {
	v, ok := params[code]
	if !ok || v == "" {
		return nil
	}
	b := v == onValue
	return &b
}

func decodeObject(result json.RawMessage) (map[string]any, error) {
	if len(result) == 0 || string(result) == "null" {
		return map[string]any{}, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		return nil, fmt.Errorf("unexpected result format: %w", err)
	}
	return obj, nil
}

// parseFlexInt accepts a JSON number or a JSON string containing an integer.
func parseFlexInt(v json.RawMessage) (int, error) {
	var n int
	if err := json.Unmarshal(v, &n); err == nil {
		return n, nil
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return 0, err
	}
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(s)
}

// stringify renders a decoded JSON scalar as a string.
func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func parseFloat(value string) float64 {
	f, _ := strconv.ParseFloat(value, 64)
	return f
}

// DeviceStatusResult is the result of getDeviceStatus.
type DeviceStatusResult struct {
	Status  string // ONLINE or OFFLINE
	IsFault bool
}

// ParseDeviceStatus extracts the online status and fault flag.
func (d Dialect) ParseDeviceStatus(result json.RawMessage) (*DeviceStatusResult, error) {
	obj, err := decodeObject(result)
	if err != nil {
		return nil, err
	}
	status := stringify(obj["status"])
	if status == "" {
		status = stringify(obj[d.DeviceStatus])
	}
	fault := obj[d.IsFault]
	if fault == nil {
		fault = obj["isFault"]
	}
	return &DeviceStatusResult{Status: status, IsFault: truthy(fault)}, nil
}

// FaultInfo is one active fault.
type FaultInfo struct {
	Description string
	Raw         map[string]any
}

// ParseFaults extracts the active faults from getFaultDataByDeviceCode.
func ParseFaults(result json.RawMessage) ([]FaultInfo, error) {
	if len(result) == 0 || string(result) == "null" {
		return nil, nil
	}
	var items []map[string]any
	if err := json.Unmarshal(result, &items); err != nil {
		return nil, fmt.Errorf("unexpected fault list format: %w", err)
	}
	faults := make([]FaultInfo, 0, len(items))
	for _, it := range items {
		faults = append(faults, FaultInfo{Description: stringify(it["description"]), Raw: it})
	}
	return faults, nil
}

// ParseProtocalID extracts protocalId from queryProductById.
func ParseProtocalID(result json.RawMessage) (string, error) {
	obj, err := decodeObject(result)
	if err != nil {
		return "", err
	}
	return stringify(obj["protocalId"]), nil
}

// ParameterInfo is one entry of a parameter catalog.
type ParameterInfo struct {
	Code         string `json:"code"`
	SemanticCode string `json:"semanticCode,omitempty"`
	Label        string `json:"label"`
	Description  string `json:"description,omitempty"`
	Value        string `json:"value"`
	Unit         string `json:"unit,omitempty"`
	Type         string `json:"type,omitempty"`
	Address      string `json:"address,omitempty"`
	DetailID     string `json:"detailId,omitempty"`
}

// ParseParameters extracts objectResult.dataList from a parameter catalog.
func ParseParameters(result json.RawMessage) ([]ParameterInfo, error) {
	var obj struct {
		DataList []map[string]any `json:"dataList"`
	}
	if len(result) == 0 || string(result) == "null" {
		return nil, nil
	}
	if err := json.Unmarshal(result, &obj); err != nil {
		return nil, fmt.Errorf("unexpected parameter list format: %w", err)
	}
	params := make([]ParameterInfo, 0, len(obj.DataList))
	for _, it := range obj.DataList {
		value := stringify(it["dataValue"])
		if _, ok := it["dataValue"]; !ok {
			value = stringify(it["value"])
		}
		semantic := stringify(it["standardNum"])
		if semantic == "undefined" {
			semantic = ""
		}
		params = append(params, ParameterInfo{
			Code:         strings.TrimSpace(stringify(it["num"])),
			SemanticCode: semantic,
			Label:        strings.TrimSpace(stringify(it["content"])),
			Description:  strings.TrimSpace(stringify(it["description"])),
			Value:        value,
			Unit:         stringify(it["showUnit"]),
			Type:         stringify(it["tranferUnit"]),
			Address:      stringify(it["address"]),
			DetailID:     stringify(it["detailId"]),
		})
	}
	return params, nil
}

// HistoryResult is the result of listCollectData.
type HistoryResult struct {
	Title    string
	ConfigID string // empty when the address has no history configured
	Points   []HistoryValue
}

// HistoryValue is one raw history sample.
type HistoryValue struct {
	DateTime string
	Value    string
}

// ParseHistory extracts a listCollectData result.
func ParseHistory(result json.RawMessage) (*HistoryResult, error) {
	var obj struct {
		Title     any `json:"title"`
		ConfigID  any `json:"configId"`
		ValueList []struct {
			DateTime     any `json:"dateTime"`
			AddressValue any `json:"addressValue"`
		} `json:"valueList"`
	}
	if len(result) == 0 || string(result) == "null" {
		return &HistoryResult{}, nil
	}
	if err := json.Unmarshal(result, &obj); err != nil {
		return nil, fmt.Errorf("unexpected history format: %w", err)
	}
	h := &HistoryResult{Title: stringify(obj.Title), ConfigID: stringify(obj.ConfigID)}
	for _, v := range obj.ValueList {
		h.Points = append(h.Points, HistoryValue{DateTime: stringify(v.DateTime), Value: stringify(v.AddressValue)})
	}
	return h, nil
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "1" || strings.EqualFold(t, "true")
	case float64:
		return t != 0
	}
	return false
}
