package goaquatemp

// ParameterKind selects a parameter catalog.
type ParameterKind string

const (
	// ParametersStatus is the read-only status catalog: sensors, relay
	// outputs, switch inputs, versions. Its Address values are the register
	// addresses usable as HistorySeries.
	ParametersStatus ParameterKind = "status"
	// ParametersSettings is the writable settings catalog.
	ParametersSettings ParameterKind = "settings"
)

// Parameter is one entry of a device's parameter catalog, as the cloud
// describes it (labels in English).
type Parameter struct {
	// Code is the protocol code (e.g. "T02"), usable with getDataByCode.
	Code string `json:"code"`
	// SemanticCode is the newer name for the same value (e.g.
	// "InletWaterTemp"), when there is one. Not unique.
	SemanticCode string `json:"semanticCode,omitempty"`
	Label        string `json:"label"`
	// Description usually holds the range or the enum options as text.
	Description string `json:"description,omitempty"`
	Value       string `json:"value"`
	Unit        string `json:"unit,omitempty"`
	// Type is the cloud's data type: ENUM, TEMP, DIGI1, DIGI5, BINARY.
	Type string `json:"type,omitempty"`
	// Address is the register address, used for history.
	Address  string `json:"address,omitempty"`
	DetailID string `json:"detailId,omitempty"`
}
