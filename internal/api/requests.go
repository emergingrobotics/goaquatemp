package api

import (
	"strconv"
	"strings"
)

// Dialect captures the per-API-version differences in paths and JSON key
// names. The modern (crmservice) API uses camelCase keys and no path suffix;
// the legacy (cloudservice) API uses snake_case keys and a ".json" suffix.
type Dialect struct {
	Suffix         string
	UserName       string
	UserID         string
	ObjectResult   string
	ProductID      string
	ProductIDs     string
	PageIndex      string
	PageSize       string
	ToUser         string
	DeviceCode     string
	ProtocalCodes  string // sic: the API spells it "protocal" here
	ProtocolCode   string
	DeviceNickName string
	DeviceStatus   string
	IsFault        string

	// DataEndpoint is where device values are read; DataCodes is the field
	// holding the requested codes there, and BitCodes the field for status
	// bits ("" when the dialect cannot read bits).
	DataEndpoint string
	DataCodes    string
	BitCodes     string
}

// Modern is used by the crmservice API (modern and HiTemp accounts).
var Modern = Dialect{
	Suffix:         "",
	UserName:       "userName",
	UserID:         "userId",
	ObjectResult:   "objectResult",
	ProductID:      "productId",
	ProductIDs:     "productIds",
	PageIndex:      "pageIndex",
	PageSize:       "pageSize",
	ToUser:         "toUser",
	DeviceCode:     "deviceCode",
	ProtocalCodes:  "protocalCodes",
	ProtocolCode:   "protocolCode",
	DeviceNickName: "deviceNickName",
	DeviceStatus:   "deviceStatus",
	IsFault:        "isFault",
	DataEndpoint:   EndpointDeviceDataV5,
	DataCodes:      "codes",
	BitCodes:       "bitCodes",
}

// Legacy is used by the old cloudservice API.
var Legacy = Dialect{
	Suffix:         ".json",
	UserName:       "user_name",
	UserID:         "user_id",
	ObjectResult:   "object_result",
	ProductID:      "product_id",
	ProductIDs:     "product_ids",
	PageIndex:      "page_index",
	PageSize:       "page_size",
	ToUser:         "to_user",
	DeviceCode:     "device_code",
	ProtocalCodes:  "protocal_codes",
	ProtocolCode:   "protocol_code",
	DeviceNickName: "device_nick_name",
	DeviceStatus:   "device_status",
	IsFault:        "is_fault",
	DataEndpoint:   EndpointDeviceData,
	DataCodes:      "protocal_codes",
	BitCodes:       "",
}

// DialectFor returns the dialect for an API version name.
func DialectFor(version string) Dialect {
	if version == "legacy" {
		return Legacy
	}
	return Modern
}

// Path returns the endpoint path with the dialect's suffix, placed before
// any query string.
func (d Dialect) Path(endpoint string) string {
	if i := strings.IndexByte(endpoint, '?'); i >= 0 {
		return endpoint[:i] + d.Suffix + endpoint[i:]
	}
	return endpoint + d.Suffix
}

// SupportsBits reports whether the dialect can read status bits.
func (d Dialect) SupportsBits() bool {
	return d.BitCodes != ""
}

// LoginRequest builds the login body. password must already be MD5-hashed.
func (d Dialect) LoginRequest(username, password string) map[string]any {
	return map[string]any{
		d.UserName: username,
		"password": password,
	}
}

// DeviceListRequest builds the body for listing devices the account owns.
func (d Dialect) DeviceListRequest() map[string]any {
	return map[string]any{
		d.ProductIDs: ProductIDs,
		d.PageIndex:  1,
		d.PageSize:   999,
	}
}

// SharedDeviceListRequest builds the body for listing devices shared to the
// account (and accepted).
func (d Dialect) SharedDeviceListRequest(userID string) map[string]any {
	return map[string]any{
		d.ProductIDs: ProductIDs,
		d.ToUser:     userID,
		d.PageIndex:  1,
		d.PageSize:   999,
	}
}

// DeviceDataRequest builds the body for reading protocol codes (and, where
// supported, status bits) from a device. Send it to d.DataEndpoint.
func (d Dialect) DeviceDataRequest(deviceCode string, codes, bits []string) map[string]any {
	req := map[string]any{
		d.DeviceCode: deviceCode,
		d.DataCodes:  codes,
	}
	if d.SupportsBits() && len(bits) > 0 {
		req[d.BitCodes] = bits
	}
	return req
}

// ProductInfoRequest builds the body for queryProductById.
func (d Dialect) ProductInfoRequest(productID string) map[string]any {
	return map[string]any{"id": productID}
}

// StatusParametersRequest builds the body for the status parameter catalog.
func (d Dialect) StatusParametersRequest(deviceCode, protocalID string) map[string]any {
	return map[string]any{
		d.DeviceCode: deviceCode,
		"protocalId": protocalID,
		d.PageIndex:  1,
		d.PageSize:   9999,
		"paramName":  "",
		"num":        "",
	}
}

// ControlParametersRequest builds the body for the settings catalog.
func (d Dialect) ControlParametersRequest(deviceCode string) map[string]any {
	return map[string]any{
		d.DeviceCode: deviceCode,
		d.PageIndex:  1,
		d.PageSize:   9999,
		"paramName":  "",
		"num":        "",
	}
}

// HistoryRequest builds the body for listCollectData. start and end must
// already be formatted as "2006-01-02 15:04:05" in the zone whose UTC offset
// (in whole hours) is tzHours.
func (d Dialect) HistoryRequest(deviceCode, address, start, end, frequency string, tzHours int) map[string]any {
	return map[string]any{
		d.DeviceCode: deviceCode,
		"address":    address,
		"startTime":  start,
		"endTime":    end,
		"frequency":  frequency,
		"timeZone":   strconv.Itoa(tzHours),
	}
}

// DeviceStatusRequest builds the body for getDeviceStatus.
func (d Dialect) DeviceStatusRequest(deviceCode string) map[string]any {
	return map[string]any{d.DeviceCode: deviceCode}
}

// ControlValue is a single protocol-code assignment in a control request.
// Value is sent as-is: strings for Power/Mode/Manual-mute, numbers for
// temperatures.
type ControlValue struct {
	ProtocolCode string
	Value        any
}

// ControlRequest builds the control body: {"param": [{deviceCode, protocolCode, value}, ...]}.
func (d Dialect) ControlRequest(deviceCode string, values []ControlValue) map[string]any {
	params := make([]map[string]any, len(values))
	for i, v := range values {
		params[i] = map[string]any{
			d.DeviceCode:   deviceCode,
			d.ProtocolCode: v.ProtocolCode,
			"value":        v.Value,
		}
	}
	return map[string]any{"param": params}
}
