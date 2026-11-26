package goaquatemp

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrHistoryUnavailable is returned when the cloud keeps no history for the
// requested series on this device.
var ErrHistoryUnavailable = errors.New("no history recorded for this series")

// HistorySeries identifies a recorded series by its register address. The
// constants cover the series the official app charts; any address from the
// status parameter catalog (Parameter.Address) can be tried.
type HistorySeries string

const (
	SeriesInletTemp           HistorySeries = "2046"
	SeriesOutletTemp          HistorySeries = "2047"
	SeriesAmbientTemp         HistorySeries = "2049"
	SeriesCompressorFrequency HistorySeries = "2021"
)

// AllHistorySeries lists the standard series in display order.
var AllHistorySeries = []HistorySeries{
	SeriesInletTemp, SeriesOutletTemp, SeriesAmbientTemp, SeriesCompressorFrequency,
}

var seriesNames = map[HistorySeries]string{
	SeriesInletTemp:           "inlet",
	SeriesOutletTemp:          "outlet",
	SeriesAmbientTemp:         "ambient",
	SeriesCompressorFrequency: "compressor",
}

// Name returns a short name for the series ("inlet", "outlet", "ambient",
// "compressor"), or the address for other series.
func (s HistorySeries) Name() string {
	if n, ok := seriesNames[s]; ok {
		return n
	}
	return string(s)
}

// Unit returns the unit of the series' values, if known.
func (s HistorySeries) Unit() string {
	switch s {
	case SeriesInletTemp, SeriesOutletTemp, SeriesAmbientTemp:
		return "°C"
	case SeriesCompressorFrequency:
		return "Hz"
	}
	return ""
}

// ParseHistorySeries accepts a short name (inlet, outlet, ambient,
// compressor) or a register address.
func ParseHistorySeries(s string) (HistorySeries, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	for series, name := range seriesNames {
		if s == name {
			return series, nil
		}
	}
	if s != "" && strings.Trim(s, "0123456789+") == "" {
		return HistorySeries(s), nil
	}
	return "", &ValidationError{Field: "series", Value: s,
		Message: "series must be inlet, outlet, ambient, compressor or a register address"}
}

// HistoryResolution is the spacing of history points.
type HistoryResolution string

const (
	// ResolutionAuto picks a resolution from the requested span.
	ResolutionAuto    HistoryResolution = ""
	Resolution5Min    HistoryResolution = "5m"
	ResolutionHourly  HistoryResolution = "hour"
	ResolutionDaily   HistoryResolution = "day"
	ResolutionMonthly HistoryResolution = "month"
)

// ParseHistoryResolution accepts 5m, hour, day, month or auto.
func ParseHistoryResolution(s string) (HistoryResolution, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return ResolutionAuto, nil
	case "5m", "5min":
		return Resolution5Min, nil
	case "hour", "hourly", "1h":
		return ResolutionHourly, nil
	case "day", "daily", "1d":
		return ResolutionDaily, nil
	case "month", "monthly":
		return ResolutionMonthly, nil
	}
	return "", &ValidationError{Field: "resolution", Value: s,
		Message: "resolution must be auto, 5m, hour, day or month"}
}

// autoResolution picks a resolution that keeps the number of points sensible.
func autoResolution(span time.Duration) HistoryResolution {
	switch {
	case span <= 48*time.Hour:
		return Resolution5Min
	case span <= 31*24*time.Hour:
		return ResolutionHourly
	case span <= 400*24*time.Hour:
		return ResolutionDaily
	default:
		return ResolutionMonthly
	}
}

// HistoryQuery selects a history series and time range.
type HistoryQuery struct {
	Series HistorySeries
	Start  time.Time
	End    time.Time // zero means now
	// Resolution defaults to ResolutionAuto.
	Resolution HistoryResolution
	// KeepGaps keeps the 0 values the cloud returns where no sample was
	// stored and for buckets in the future. By default they are dropped.
	KeepGaps bool
}

// History is a recorded series.
type History struct {
	DeviceCode string            `json:"deviceCode"`
	Series     HistorySeries     `json:"series"`
	Name       string            `json:"name"`
	Title      string            `json:"title"` // the cloud's label (Chinese)
	Unit       string            `json:"unit,omitempty"`
	Resolution HistoryResolution `json:"resolution"`
	Points     []HistoryPoint    `json:"points"`
}

// HistoryPoint is one value of a series; Time is the start of its bucket.
type HistoryPoint struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
}

// historyFrequency maps a resolution to the API's frequency value and the
// layout of returned labels.
func historyFrequency(r HistoryResolution) (frequency, layout string, err error) {
	switch r {
	case Resolution5Min:
		return "week", "2006-01-02 15:04:05", nil
	case ResolutionHourly:
		return "day", "2006-01-02 15", nil
	case ResolutionDaily:
		return "month", "2006-01-02", nil
	case ResolutionMonthly:
		return "year", "2006-01", nil
	}
	return "", "", fmt.Errorf("unknown resolution %q", r)
}

// bucketLength returns the duration of a bucket at resolution r starting at t.
func bucketLength(r HistoryResolution, t time.Time) time.Duration {
	switch r {
	case Resolution5Min:
		return 5 * time.Minute
	case ResolutionHourly:
		return time.Hour
	case ResolutionDaily:
		return t.AddDate(0, 0, 1).Sub(t)
	default:
		return t.AddDate(0, 1, 0).Sub(t)
	}
}

// chinaTime is the zone the 5-minute series is stored in (probed 2026-11-17).
var chinaTime = time.FixedZone("UTC+8", 8*3600)

// alignToBucket truncates t to the start of its bucket at resolution r, in
// t's location.
func alignToBucket(r HistoryResolution, t time.Time) time.Time {
	y, mo, d := t.Date()
	switch r {
	case Resolution5Min:
		return time.Date(y, mo, d, t.Hour(), t.Minute()/5*5, 0, 0, t.Location())
	case ResolutionHourly:
		return time.Date(y, mo, d, t.Hour(), 0, 0, 0, t.Location())
	case ResolutionDaily:
		return time.Date(y, mo, d, 0, 0, 0, 0, t.Location())
	default:
		return time.Date(y, mo, 1, 0, 0, 0, 0, t.Location())
	}
}

// requestZone returns the whole-hour UTC offset of t's location (the API
// takes whole hours) and a fixed zone for it.
func requestZone(t time.Time) (int, *time.Location) {
	_, offset := t.Zone()
	hours := offset / 3600
	return hours, time.FixedZone(fmt.Sprintf("UTC%+d", hours), hours*3600)
}
