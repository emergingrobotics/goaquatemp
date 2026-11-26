# Aqua Temp / Linked-Go Cloud API

This document describes the cloud API used by the Aqua Temp app (and the
HiTemp / WarmLink apps, which run on the same Linked-Go "Go Heating IoT"
platform). There is no official documentation. Everything here was either
probed against the live API or taken from working open-source clients.

## Contents

1. [Confidence and sources](#1-confidence-and-sources)
2. [Conventions](#2-conventions)
3. [Authentication](#3-authentication)
4. [Walkthrough: first device → status → on/off](#4-walkthrough-first-device--status--onoff)
5. [Listing devices](#5-listing-devices)
6. [Online and fault status](#6-online-and-fault-status)
7. [Reading live values (`getDataByCode`)](#7-reading-live-values-getdatabycode)
8. [The key readings](#8-the-key-readings)
9. [Compressor status in detail](#9-compressor-status-in-detail)
10. [Mode: set mode vs. what the unit is doing](#10-mode-set-mode-vs-what-the-unit-is-doing)
11. [Control: power, mode, temperature, fan](#11-control-power-mode-temperature-fan)
12. [History / graphs (`listCollectData`)](#12-history--graphs-listcollectdata)
13. [Parameter catalogs](#13-parameter-catalogs)
14. [Other endpoints](#14-other-endpoints)
15. [Legacy API differences](#15-legacy-api-differences)
16. [What goaquatemp implements today](#16-what-goaquatemp-implements-today)
17. [Sources](#17-sources)

---

## 1. Confidence and sources

Each statement is tagged with how it is known:

| Tag | Meaning |
|---|---|
| **Probed** | Seen in live responses on 2026-11-17, on one pool heat pump (product `1132174963097280512`, controller model `PASRW050-P-BP122-R410A-D`), modern API. The vendor can change the backend at any time. |
| **Integration** | What a working open-source client sends and expects (mainly [MatiasFernandez/aquatemp](https://github.com/MatiasFernandez/aquatemp), a maintained fork of [radical-squared/aquatemp](https://github.com/radical-squared/aquatemp)). Works against the live API, but was not re-probed here. |
| **Capture** | Seen in a traffic capture of the WarmLink iOS app ([FoxAir_Control #209](https://github.com/dosordie/FoxAir_Control/issues/209)). |
| **Inferred** | Our reading of the evidence; unconfirmed. |

Control endpoints were **not** called while writing this document, so
everything in [§11](#11-control-power-mode-temperature-fan) is tagged
Integration.

---

## 2. Conventions

### Base URL

| API | Base URL | Notes |
|---|---|---|
| Modern (Aqua Temp app ≥ 1.5.9, HiTemp) | `https://cloud.linked-go.com:449/crmservice/api` | camelCase fields, no path suffix. **Use this.** |
| Legacy (old Aqua Temp accounts) | `https://cloud.linked-go.com/cloudservice/api` | snake_case fields, `.json` suffix. See [§15](#15-legacy-api-differences). |

Most paths are under `/app/...`. The history endpoint is the exception: it is
under `/device/snapshot/...` (no `app/`).

### Requests

- Every call is `POST` with a JSON body and
  `Content-Type: application/json; charset=utf-8`.
- After login, send the session token in an `x-token` header, **as the bare
  token** (no `Bearer ` prefix).
- Some endpoints take `?lang=en` to get English labels in the parameter
  catalogs. It does no harm elsewhere.

### Response envelope

Every response, success or failure, has the same envelope:

```json
{
  "error_code": "0",
  "error_msg": "Success",
  "error_msg_code": "",
  "isReusltSuc": true,
  "objectResult": { },
  "sessionid": null,
  "totalSize": null,
  "totalPage": null,
  "nextPage": null
}
```

- `error_code` is a **string** (`"0"`, `"-1"`, `"-100"`). Parse it leniently
  (accept a string or a number). **Probed.**
- `error_msg` is often Chinese. The ones seen:

| `error_code` | `error_msg` | Meaning |
|---|---|---|
| `"0"` | `Success` | OK |
| `"-100"` | `请重新登录` ("please log in again") | Token missing, expired or revoked. Also returned for a login body in the wrong format (e.g. legacy field names on the modern host). **Probed** |
| `"-1"` (`error_msg_code` `1400034`) | `密码错误` ("wrong password") | Bad password. The app may cut passwords longer than 16 characters to the first 16. **Probed / Integration** |
| `"-1"` (HTTP 400) | `请求参数格式错误` ("request parameter format error") | Body fields of the wrong type or format. **Probed** |
| `"-1"` on `control` | (varies) | The cloud gave up waiting (~50 s) for the device to acknowledge. **Integration** |
| `"0"` on `getDeviceStatus` | `离线` ("offline") | Device offline. Check `objectResult.status`, not the message. **Integration** |

- An unknown path returns **HTTP 404** with no envelope. **Probed.**
- Many objects carry both camelCase and snake_case copies of each field
  (`deviceCode` and `device_code`, `isFault` and `is_fault`). **Probed.**

### One session per account

The cloud allows **one session per account**. Logging in anywhere (this
client, the phone app, Home Assistant) invalidates the previous token, and
the other client then gets `-100`. For automation, create a second account and
share the heat pump with it from the owner account. **Integration.**

### Rate limiting

After about ten requests to non-existent paths in quick succession, the server
started timing out TLS handshakes for a short while. Normal polling is fine.
Don't hammer the API or brute-force paths. **Probed.**

---

## 3. Authentication

### Login: `POST /app/user/login`

Request (**Probed**):

```json
{ "userName": "you@example.com", "password": "<md5 hex of the password>" }
```

- `password` is the lowercase hex MD5 of the plain-text password.
- Do **not** send the legacy `user_name` or `type` fields. The modern host
  answers them with `-100`.

Response `objectResult` (**Probed**, secrets elided):

```json
{
  "x-token": "<session token>",
  "userId": "1730…",           "user_id": "1730…",
  "userName": "g****n@…",      "nickName": "…",
  "user_type": "Customer",
  "accessKey": "<secret>",
  "appId": "14"
}
```

- `x-token` goes in the `x-token` header of every later request.
- `userId` is needed for the shared-device list (`toUser`).
- No expiry is returned. Tokens live until revoked (by another login) or
  expired server-side. Handle `-100` by logging in again and retrying once.
- Treat `x-token` and `accessKey` as secrets. Never log them.

### User info: `POST /app/user/getUserInfo`

No body. `objectResult.userId` is the same as at login. **Probed.** Use it if
the login response lacked `userId`.

---

## 4. Walkthrough: first device → status → on/off

```bash
BASE='https://cloud.linked-go.com:449/crmservice/api'
H='Content-Type: application/json; charset=utf-8'

# 1. Login
PW_MD5=$(printf '%s' "$AQUATEMP_PASSWORD" | md5sum | cut -c1-32)
TOKEN=$(curl -s -X POST "$BASE/app/user/login" -H "$H" \
  -d "{\"userName\":\"$AQUATEMP_USERNAME\",\"password\":\"$PW_MD5\"}" \
  | jq -r '.objectResult["x-token"]')

# 2. First device (owned). If the device is shared to you, use
#    getMyAppectDeviceShareDataList instead (see §5).
DEV=$(curl -s -X POST "$BASE/app/device/deviceList" -H "$H" -H "x-token: $TOKEN" \
  -d '{"productIds":["1132174963097280512"],"pageIndex":1,"pageSize":999}' \
  | jq -r '.objectResult[0].deviceCode')

# 3. Online?
curl -s -X POST "$BASE/app/device/getDeviceStatus" -H "$H" -H "x-token: $TOKEN" \
  -d "{\"deviceCode\":\"$DEV\"}" | jq '.objectResult.status'        # "ONLINE"

# 4. Status: power, mode, operating state, compressor, temperatures
curl -s -X POST "$BASE/app/device/v5/getDataByCode" -H "$H" -H "x-token: $TOKEN" -d "{
  \"deviceCode\": \"$DEV\",
  \"codes\": [\"Power\",\"Mode\",\"ModeState\",\"Set_Temp\",\"R01\",\"R02\",\"R03\",
            \"T02\",\"T03\",\"T05\",\"O07\",\"H08\",\"H09\"],
  \"bitCodes\": [\"Sys1CompState\",\"WaterFlowSwitch\"]
}" | jq -r '.objectResult[] | "\(.code)=\(.value)"'

# 5. Turn on ("1") / off ("0")  -- WRITES TO THE HEAT PUMP
curl -s -X POST "$BASE/app/device/control" -H "$H" -H "x-token: $TOKEN" \
  -d "{\"param\":[{\"deviceCode\":\"$DEV\",\"protocolCode\":\"power\",\"value\":\"1\"}]}"
```

Output of step 4 on the probed unit while heating (**Probed**):

```
Power=1  Mode=1  ModeState=1  Set_Temp=32.2  R01=28.9  R02=32.2  R03=27.0
T02=28.5  T03=30.5  T05=26.0  O07=70  H08=85  H09=52
Sys1CompState=1  WaterFlowSwitch=0
```

Reading it: power on, set to heat, actually heating, target 32.2 °C, inlet
28.5 °C, outlet 30.5 °C, ambient 26.0 °C, compressor running at 70 Hz of an
85 Hz heating maximum (82 %), water flowing.

---

## 5. Listing devices

### Owned devices: `POST /app/device/deviceList`

```json
{ "productIds": ["1132174963097280512", "…"], "pageIndex": 1, "pageSize": 999 }
```

`productIds` filters by product. Send every known heat pump product ID (the
list is in `internal/api/endpoints.go`, and [MatiasFernandez's
consts.py](https://github.com/MatiasFernandez/aquatemp/blob/main/custom_components/aqua_temp/cloud/consts.py)
has 31). **Probed** with one ID; **Integration** for the full list.

Each `objectResult[]` item (**Probed**, abridged):

| Field | Example | Notes |
|---|---|---|
| `deviceCode` | `289C…` | **The ID used by every other device call** |
| `deviceId` | `1730…` | Used by `queryDeviceById` |
| `deviceNickName` | (user-set; defaults to the device code) | |
| `deviceStatus` | `ONLINE` / `OFFLINE` | |
| `productId` | `1132174963097280512` | Selects the parameter profile |
| `custModel` | `Inter Heat 58IS` | Marketing model |
| `model` | `PASRW050-P-BP122-R410A-D` | Controller model |
| `sn` | serial number | |
| `isFault` | `false` | |
| `wifiSoftwareVer`, `wifiSoftwareCode` | `ver0002`, `hf230Aliyun` | Wi-Fi module |

### Devices shared to you

| Endpoint | Body | |
|---|---|---|
| `POST /app/device/getMyAppectDeviceShareDataList` | `{productIds, toUser: <userId>, pageIndex, pageSize}` | Accepted shares. **Probed** (empty list on this account) |
| `POST /app/device/getMyTobeAppectDeviceShareDataList` | `{productIds, toUser}` | Pending invitations. **Integration** |

Items have the same shape as `deviceList`.

---

## 6. Online and fault status

### `POST /app/device/getDeviceStatus`

```json
{ "deviceCode": "<deviceCode>" }
```

Response `objectResult` (**Probed**):

```json
{ "status": "ONLINE", "isFault": false, "is_fault": false }
```

- `status` is `ONLINE` or `OFFLINE`. The cloud takes about 4–5 minutes to
  notice a dropped device. **Integration.**
- `isFault` is a JSON boolean.
- Don't send control commands to an `OFFLINE` device. The cloud waits about
  50 s before failing. **Integration.**

### `POST /app/device/getFaultDataByDeviceCode`

Same body. `objectResult` is a list of active faults, and `[0].description` is
human-readable text. **Probed** (empty list, no faults); description field from
**Integration**.

---

## 7. Reading live values (`getDataByCode`)

Device values are named by **protocol codes**: short register names like `T02`,
or newer **semantic** names like `InletWaterTemp`. Both work for many values.

### v5 (preferred): `POST /app/device/v5/getDataByCode`

```json
{
  "deviceCode": "<deviceCode>",
  "codes":    ["Power", "Mode", "T02", "InletWaterTemp", "…"],
  "bitCodes": ["Sys1CompState", "WaterFlowSwitch", "…"]
}
```

- This is the endpoint the official app uses. **Integration / Probed.**
- Register and semantic codes go in `codes`. Sending them as `protocalCodes`
  here returns `Success` with an **empty** list. **Integration.**
- `bitCodes` takes the names of single status bits (see [§9](#9-compressor-status-in-detail)).
  They are **only** readable through this array. **Probed.**

### v1: `POST /app/device/getDataByCode`

```json
{ "deviceCode": "<deviceCode>", "protocalCodes": ["Power", "T02", "…"] }
```

(`protocal` is the API's own spelling.) Same values as v5 `codes`, but no bits.
**Probed.**

### Response

`objectResult` is a list, one item per requested code (**Probed**):

```json
{ "code": "T02", "value": "28.5", "dataType": "TEMP", "rangeStart": "", "rangeEnd": "" }
```

- `value` is always a **string**. Parse numbers yourself.
- A code the device doesn't know still comes back, with `value: ""` and
  `dataType: ""`. On this unit, `O01`–`O05` come back empty through this
  endpoint even though the status catalog has them. **Probed.**
- A known **bit** comes back with `dataType: null` (not `""`). Keep the two
  apart. **Probed.**
- `dataType` is `ENUM`, `TEMP`, `DIGI1` (integer), `DIGI5` (one decimal) or
  `BINARY`.

---

## 8. The key readings

All **Probed** on product `1132174963097280512` unless marked. "Alias" means a
semantic code that returned the same value.

### Power and mode

| What | Code (alias) | Values |
|---|---|---|
| Power (setting) | `Power` (`PowerState`, `Power_State`) | `"0"` off, `"1"` on |
| Mode (setting) | `Mode` | `"0"` cool, `"1"` heat, `"2"` auto. No "off" value: off is `Power=0` |
| **Operating state** (what it is doing) | `ModeState` (`Current mode`) | `"0"` cooling, `"1"` heating, `"2"` defrosting |
| Silent / low fan | `Manual-mute` | `"0"` off (auto fan), `"1"` on (silent) |

### Temperatures (°C, see the unit note below)

| What | Code (alias) | Live sample |
|---|---|---|
| **Inlet water** (pool water in; the app's "current" temperature) | `T02` (`InletWaterTemp`) | 28.5 |
| **Outlet water** (heated water out) | `T03` (`OutletWaterTemp`) | 30.5 |
| **Ambient air** | `T05` (`AT`) | 26.0 |
| Suction | `T01` (`Sys1SuctionTemp`) | 17.0 |
| Coil 1 | `T04` (`Sys1Coil1Temp`) | 12.5 |
| Exhaust (compressor discharge) | `T06` (`Sys1ExhaustTemp`) | 66.0 |
| IPM (inverter module) | `O09` (`IPMTemp`) | 43.0 (status catalog only) |

### Targets and limits

| What | Code (alias) | Live sample |
|---|---|---|
| Active target (follows the mode) | `Set_Temp` (`SetCurrentTemp`) | 32.2 |
| Cool target | `R01` | 28.9 (range `R08`–`R09`) |
| Heat target | `R02` (`SetHeatTargetTemp`) | 32.2 (range `R10`–`R11`) |
| Auto target | `R03` | 27.0 (range `R08`–`R11`) |
| Min/max cool target | `R08` / `R09` (`SetCoolMin/MaxTargetTemp`) | 8.0 / 35.0 |
| Min/max heat target | `R10` / `R11` (`SetHeatMin/MaxTargetTemp`) | 15.0 / 40.0 |
| Restart / shutdown hysteresis | `R04` / `R05` | 0.5 / 1.0 |

### Compressor, electrical, energy

| What | Code (alias) | Live sample | Notes |
|---|---|---|---|
| **Compressor running** | bit `Sys1CompState` (v5 `bitCodes` only) | `"1"` | See [§9](#9-compressor-status-in-detail) |
| Compressor output frequency | `O07` (`Sys1CompWorkingFreq`) | 70 Hz | 0 when stopped |
| Target frequency | `T28` (`TargetFreq`) | 70 Hz | |
| Heat / cool max frequency | `H08` / `H09` | 85 / 52 Hz | |
| Compressor type | `H12` | 3 | Non-zero = inverter (Inferred) |
| Compressor current | `O08` (`CompCurrent`) | 6.9 A | |
| Inverter AC input voltage | `T14` (`Sys1InvBoardInputVol`) | 221 V | Catalog wrongly shows unit `A` |
| DC bus voltage | `T19` (`BusVoltage`) | 369 | |
| Fan speed | `T17` (`ACFan1RealSpeed`) | 700 rpm | |
| Real power | `Z04` (`RealPower`) | 24.3 | Unit unclear; catalog says "0–100 kW". Unverified |
| Accumulated energy | `Z06` (`ACC`) | 460.6 | Probably kWh. Unverified |
| COP / capacity / flow rate | `Z01` / `Z02` / `Z03` | 0.0 | Not populated on this unit |

### Unit note

Control parameter `H03` ("Temp. unit", `0` = °C, `1` = °F) reads `1` on the
probed unit. The values above are clearly °C, though (28.5 water, ranges
15–40), and the status catalog labels them `℃`. Treat the values as °C. Don't
convert based on `H03` until this is verified. **Inferred.** The SDK keeps
°C and offers `CelsiusToFahrenheit`; the CLI prints both units.

---

## 9. Compressor status in detail

There are three ways to tell whether the compressor is running.

1. **`Sys1CompState` bit, v5 only (recommended).** `"1"` = running,
   `"0"` = stopped. This is what the official app reads to show
   "Running"/"Standby". **Probed** (`1` while running at 70 Hz); semantics from
   **Integration**.

   ```json
   POST /app/device/v5/getDataByCode
   { "deviceCode": "<deviceCode>", "codes": ["Mode"], "bitCodes": ["Sys1CompState"] }
   ```

   Keep at least one ordinary code in `codes`. A request with only unknown
   codes returns `Success` with an empty list, which looks like a dead
   device. **Integration.**

2. **Output frequency `O07` > 0.** Works through v1 too. 0 Hz while idle.
   **Probed.**

3. **`O01` "Compressor" relay in the status catalog**
   (`getControlDetailStatusByDeviceCode`, [§13](#13-parameter-catalogs)).
   Reads `1`/ON. That endpoint also has the other relay outputs: `O02`
   circulation pump, `O03` 4-way valve, `O04` high fan, `O05` low fan, `O13`
   chassis heater. **Probed.** `O01` is **not** readable through
   `getDataByCode` on this unit (empty value).

### Compressor speed as a percentage

The app shows compressor speed as `O07 / max`, where max is `H08` in heat,
`H09` in cool, and in auto whichever matches the direction (see
[§10](#10-mode-set-mode-vs-what-the-unit-is-doing)). The app shows the
percentage only when `H12` ≠ 0. **Integration.** Live: 70 / 85 = 82 %.

### Other bits (v5 `bitCodes`)

| Bit | Meaning | Polarity |
|---|---|---|
| `Sys1CompState` | Compressor running | `1` = running |
| `WaterFlowSwitch` | Water flow | **`0` = flowing**, `1` = no flow (pump off) |
| `EmergencySwitch` | External stop input holding the unit off | `1` = held off (inverted on some products) |
| `Sys1HighPressureSwitch` / `Sys1LowPressureSwitch` | Pressure protections | Unknown; `0` on a healthy unit |
| `InletWaterTempSensorFault` / `OutletWaterTempSensorFault` / `ATSensorFault` | Sensor faults | `1` = faulty |
| `ModeSwitch`, `Master/SlaveSwitch` | Wired inputs | Unknown |

All read `0`, except `Sys1CompState`=`1`, on the probed unit. Polarities from
**Integration**.

### Water temperatures need flow

The inlet (`T02`) and outlet (`T03`) sensors sit in the heat pump's own
plumbing. When `WaterFlowSwitch` = `1` (no flow, circulation pump off) they read
the water standing in the pipe, which drifts toward the air temperature. The
API still returns a number, but it is not the pool temperature.

The SDK keeps the reported numbers in `InletTemp`, `OutletTemp` and
`CurrentTemp` and sets `DeviceData.WaterTempStatus`:

| `WaterTempStatus` | Flow bit | Meaning |
|---|---|---|
| `valid` | `0` | Water flowing; temperatures usable |
| `no_flow` | `1` | Stagnant; **do not use** |
| `unknown` | absent (legacy API) | Can't be checked; treated as usable |

`ValidInletTemp()`, `ValidOutletTemp()` and `ValidCurrentTemp()` return `nil`
when there is no flow, and `WaterTempsValid()` is false. The CLI prints `NA`
for these values (text and CSV; `null` in hand-built JSON) and shows the
stagnant reading separately (`NA (no flow; stagnant reading …)` in text,
`*_raw` columns in CSV, `*_raw` keys in `temp history` JSON).

Ambient, coil, suction and exhaust temperatures don't depend on water flow
and are always reported as-is. **Not handled:** history (`listCollectData`)
has no flow series, so recorded inlet/outlet history includes stagnant
periods.

---

## 10. Mode: set mode vs. what the unit is doing

- **`Mode`** is what the user selected: cool `0` / heat `1` / auto `2`. It
  doesn't change when the unit is idle or off.
- **`ModeState`** is what the unit is doing now: cooling `0` / heating `1` /
  defrosting `2`. **Probed** (`1` while heating; labels from the enum catalog).
- **Power** off overrides both. `Mode` keeps its last value while powered off.
- **Auto mode direction**, as the app decides it: heating when `R03` (auto
  target) > `T02` (inlet), otherwise cooling. **Integration / Inferred.**

A good "what is it doing" summary:

```
Power == 0                         -> off
Sys1CompState == 0                 -> idle (on, at/near target, or waiting)
ModeState == 2                     -> defrosting
ModeState == 1                     -> heating
ModeState == 0                     -> cooling
```

---

## 11. Control: power, mode, temperature, fan

> **These write to the real heat pump.** Not exercised while writing this
> document. Everything here is **Integration** unless noted.

### `POST /app/device/control`

```json
{
  "param": [
    { "deviceCode": "<deviceCode>", "protocolCode": "<code>", "value": <value> }
  ]
}
```

- Field name is `protocolCode` (spelled correctly here, unlike `protocalCodes`).
- Several entries can go in one request. They are applied together.
- The cloud waits for the device to acknowledge (~5 s when online). On
  timeout it returns `error_code "-1"` after about 50 s. **Don't retry
  timeouts**: the command may still land. Retry only connection errors, 5xx,
  and `-100` (after logging in again).

### Turn on / off

```json
{ "param": [ { "deviceCode": "<deviceCode>", "protocolCode": "power", "value": "1" } ] }
```

- `"1"` on, `"0"` off, as **strings**.
- The code is lowercase **`power`** for control, even though it reads back as
  `Power`.
- If bit `EmergencySwitch` = `1`, an external input is holding the unit off and
  "on" will not take effect.

### Set mode

```json
{ "param": [
  { "deviceCode": "<deviceCode>", "protocolCode": "Set_Temp", "value": 32.2 },
  { "deviceCode": "<deviceCode>", "protocolCode": "Mode",     "value": "1" }
] }
```

- `"0"` cool, `"1"` heat, `"2"` auto. To turn "off", use power `"0"`.
- The reference client re-sends the current target as `Set_Temp` alongside
  `Mode`, and powers the unit on first if it was off.

### Set target temperature

Write the mode's own target register **and** `Set_Temp`, as numbers:

```json
{ "param": [
  { "deviceCode": "<deviceCode>", "protocolCode": "R02",      "value": 30.0 },
  { "deviceCode": "<deviceCode>", "protocolCode": "Set_Temp", "value": 30.0 }
] }
```

| Current `Mode` | Target register | Valid range |
|---|---|---|
| cool `0` | `R01` | `R08`–`R09` (8–35 on this unit) |
| heat `1` | `R02` | `R10`–`R11` (15–40 on this unit) |
| auto `2` | `R03` | `R08`–`R11` |

### Silent fan

```json
{ "param": [ { "deviceCode": "<deviceCode>", "protocolCode": "Manual-mute", "value": "1" } ] }
```

`"0"` = normal (auto), `"1"` = silent (low). There is no low/medium/high.

### Writing other parameters

The control-parameter catalog ([§13](#13-parameter-catalogs)) lists about 175
writable settings (timers, pump mode `P01`, defrost `D..`, PV/eco `EM..`).
They can presumably be written the same way by `num`, but no client does this.
Treat it as installer territory. **Inferred.**

---

## 12. History / graphs (`listCollectData`)

The app's temperature-curve screen is backed by a history endpoint that none
of the open-source integrations use. It was found in a WarmLink app capture
(**Capture**), and its request format was worked out by probing (**Probed**).

### `POST /device/snapshot/listCollectData`

Note the path: `{base}/device/snapshot/listCollectData`, **not** under `/app/`.

```json
{
  "deviceCode": "<deviceCode>",
  "address":    "2046",
  "startTime":  "2026-11-16 00:00:00",
  "endTime":    "2026-11-16 23:59:59",
  "frequency":  "day",
  "timeZone":   "-8"
}
```

| Field | Format | Notes |
|---|---|---|
| `address` | string | A register **address** from the status catalog (not the code). See below |
| `startTime` / `endTime` | string, `yyyy-MM-dd HH:mm:ss` | Wall-clock time in `timeZone`. **Align `startTime` to a bucket boundary**: slots are laid out from it as sent, so `04:22` produces slots `04:22, 04:27, …` that never match stored samples (all `0.00`). Epoch seconds or millis return `Success` with **no data**. Numeric (non-string) times → HTTP 400 |
| `frequency` | `day`, `week`, `month`, `year` | Selects resolution (table below) |
| `timeZone` | string, UTC offset in hours, e.g. `"-8"`, `"0"`, `"8"` | An IANA name (`America/Los_Angeles`) → HTTP 400 `请求参数格式错误` |

| `frequency` | Resolution | `dateTime` format | Label time zone |
|---|---|---|---|
| `week` | 5 minutes | `2026-11-17 22:50:00` | **Must be queried in UTC+8**: send the window in China time with `timeZone: "8"` (see below) |
| `day` | 1 hour | `2026-11-16 13` | `timeZone` |
| `month` | 1 day | `2026-11-16` | `timeZone` (Inferred) |
| `year` | 1 month | `2026-11` | `timeZone` (Inferred) |

**The 5-minute series is stored in UTC+8.** The server lays out slots from the
window exactly as sent, but fills them from samples keyed by China-time
timestamps. Values only land in the slots when the window is sent in UTC+8 with
`timeZone: "8"`. With any other `timeZone`, a short window comes back all
`0.00`. A long enough window partly overlaps by accident, which is why the first
probes seemed to work with `"-8"`. The hourly series honours `timeZone` (the
same day with `"-8"` and `"0"` is shifted by 8 hours, and matches live readings
in local time). **Probed 2026-11-17.**

(The names describe the app's chart span, not the bucket size: `frequency=day`
gives one day of hourly points. WarmLink saw the same: "`frequency=day`
returned hourly values, `month` daily values." **Capture.**)

### Response

```json
{
  "title": "2046 进水温度",
  "total": "24",
  "productId": "1132174963097280512",
  "deviceCode": "<deviceCode>",
  "address": "2046",
  "configId": "1694585444006588418",
  "valueList": [
    { "dateTime": "2026-11-16 13", "addressValue": "27.50" },
    { "dateTime": "2026-11-16 14", "addressValue": "28.00" }
  ],
  "fengList": [], "guList": [], "pingList": []
}
```

- `valueList[].addressValue` is a string with two decimals.
- `title` is `<address> <Chinese label>`. `configId` is non-empty only for
  addresses that have history.
- `fengList` / `guList` / `pingList` (峰/谷/平 = peak/valley/flat tariff
  periods) are for energy series. Empty here. **Inferred.**
- An address **without** history configured returns `Success` with all fields
  empty (`title: ""`, `configId: ""`, `valueList: []`).

### Which addresses have history (product `1132174963097280512`)

The `address` comes from the `address` field of the status catalog
([§13](#13-parameter-catalogs)).

| Address | Code | Title | History? |
|---|---|---|---|
| `2046` | `T02` | 进水温度 (inlet water temp) | **yes** |
| `2047` | `T03` | 出水温度 (outlet water temp) | **yes** |
| `2049` | `T05` | 环境温度 (ambient temp) | **yes** |
| `2021` | `O07` | 压缩机输出频率 (compressor output frequency) | **yes** |
| `2045` | `T01` | (suction) | no |
| `2011` / `2012` | Power state / `ModeState` | | no |
| `2019` | `O01` (compressor relay) | | no |
| `2055` | `Z04` (real power) | | no |
| `2038+2039` | `Z06` (accumulated energy) | | no (the WarmLink app reads kWh series this way on its products: `2078+9`) |

These four series are exactly what the app's curve screen shows: inlet and
outlet water, ambient, and compressor frequency (which doubles as a
compressor on/off history).

### Caveats (all **Probed**)

- **Gaps read `0.00`.** The 5-minute series alternates real values with
  `0.00` where no sample was stored (the device reports roughly every
  10 minutes). Drop zeros for temperatures. For compressor frequency, `0` is
  also a real "stopped" value, so prefer the hourly series or treat a lone
  zero between two non-zero samples as a gap.
- **Future buckets are `0.00`.** `day`/`month`/`year` return the full period,
  including buckets after now. `year` returned `2026-12: 0.00`.
- Hourly, daily and monthly values are **samples at the start of the bucket,
  not averages**: the hourly value equals the 5-minute sample at :00, and the
  daily value equals the 00:00 hourly value. Use the 5-minute series for
  anything that needs the shape within an hour.
- How far back history goes wasn't measured. `month` over 2026-10-01 to
  2026-11-17 returned a value for every day, and `year` returned 11 months.
- Probed values seemed odd at times (ambient `46.00` for hours 00–10 on
  2026-11-16). The cloud returned them as-is.

### Examples

Last 24 h of inlet temperature, hourly, in local time:

```bash
curl -s -X POST "$BASE/device/snapshot/listCollectData" -H "$H" -H "x-token: $TOKEN" -d "{
  \"deviceCode\": \"$DEV\", \"address\": \"2046\",
  \"startTime\": \"$(date -d '-1 day' '+%F %H:00:00')\", \"endTime\": \"$(date '+%F %T')\",
  \"frequency\": \"day\", \"timeZone\": \"$(date +%z | sed -E 's/^([+-])0?([0-9]+)[0-9]{2}$/\1\2/; s/^\+//')\"
}" | jq -r '.objectResult.valueList[] | "\(.dateTime)\t\(.addressValue)"'
```

Last 2 h at 5-minute resolution. The window goes out in China time, and the
returned labels are China time:

```bash
M=$(( $(date +%s) / 300 * 300 ))   # align to 5 minutes
curl -s -X POST "$BASE/device/snapshot/listCollectData" -H "$H" -H "x-token: $TOKEN" -d "{
  \"deviceCode\": \"$DEV\", \"address\": \"2046\",
  \"startTime\": \"$(TZ=Asia/Shanghai date -d @$((M-7200)) '+%F %T')\",
  \"endTime\": \"$(TZ=Asia/Shanghai date -d @$M '+%F %T')\",
  \"frequency\": \"week\", \"timeZone\": \"8\"
}" | jq -r '.objectResult.valueList[] | select(.addressValue != "0.00") | "\(.dateTime)\t\(.addressValue)"'
```

`goaquatemp` does all of this for you: `client.GetHistory` and `aquatemp history`.

---

## 13. Parameter catalogs

These return the device's full parameter lists with English labels, ranges and
register addresses. They are how codes and addresses can be discovered for any
product. All **Probed** unless noted.

### Product → protocol ID: `POST /app/product/queryProductById?lang=en`

```json
{ "id": "<productId>" }
```

`objectResult.protocalId` (here `1135730985732685824`) is needed by the status
catalog. It also returns `productName` (`POOL_TEMP`) and a `productKey`
(secret).

### Status (read-only) catalog: `POST /app/device/getControlDetailStatusByDeviceCode?lang=en`

```json
{ "deviceCode": "<deviceCode>", "protocalId": "<protocalId>",
  "pageIndex": 1, "pageSize": 9999, "paramName": "", "num": "" }
```

`objectResult.dataList[]` lists 69 entries on this unit. Each has:

| Field | Example (T02) |
|---|---|
| `num` | `T02` (the protocol code) |
| `standardNum` | `InletWaterTemp` (the semantic code; not unique) |
| `content` | `Inlet water Temp.` |
| `description` | `-30-100℃` (ranges are only free text) |
| `dataValue` | `28.5` (the **current value**) |
| `address` | `2046` (**the history address**) |
| `tranferUnit` | `TEMP` |
| `showUnit` | `℃` (sometimes wrong) |
| `detailId` | used by the enum lookup |

This is the only place the relay outputs (`O01`–`O05`, `O13`) and switch
inputs (`S01`–`S06`) are readable on this unit.

### Settings (writable) catalog: `POST /app/device/getControlParamConfigByDeviceCode?lang=en`

```json
{ "deviceCode": "<deviceCode>", "pageIndex": 1, "pageSize": 9999, "paramName": "", "num": "" }
```

175 entries on this unit, the same shape with `value` instead of `dataValue`.
Includes `Power`, `Mode`, `R01`–`R14`, `H01`–`H43`, `F..` fan, `D..`
defrost, `E..` expansion valve, `P01`–`P10` circulation pump schedule, timers
`1150`–`1161`, spot timers `KW1`–`KW30`, PV/eco `EM..`.

### Enum labels: `POST /app/protocal/getEnumDataByDetailId?lang=en`

```json
{ "detailId": "<detailId of an ENUM item>" }
```

Returns `[{value, meaning, enMeaning}]`. **Integration** (Bruno collection).
WarmLink's capture shows the path as `/app/device/protocal/getEnumDataByDetailId`.
The `description` field of enum items usually spells the options out anyway
(`0-cooling/1-Heating/2-Defrosting`).

### Device details: `POST /app/device/queryDeviceById?lang=en`

```json
{ "deviceId": "<deviceId>" }
```

Returns about 100 fields, including `productId`, `deviceStatus`, `inWaterTemp`,
`outWaterTemp`, and also **secrets and personal data**: `deviceSecret`,
`productKey` (Wi-Fi module credentials), owner, customer and dealer names,
emails and phone numbers. Never log or share this response.

---

## 14. Other endpoints

| Endpoint | Body | Status |
|---|---|---|
| `/device/sendDevicePassthroughInstruction` | `{device_code, query_instruction: "630300040001CD89"}` | Legacy API keep-alive. **Integration** |
| `/app/device/ota/judgeUpgradeList` | ? | Firmware update check (read-only). **Capture** |
| `/app/device/control` with `app_heartbeat` | ? | App heartbeat. **Capture** |

---

## 15. Legacy API differences

For accounts created before app v1.5.8 (`cloudservice` host):

- Base `https://cloud.linked-go.com/cloudservice/api`, every path ends in `.json`.
- snake_case fields: `user_name`, `device_code`, `protocal_codes`,
  `protocol_code`, `object_result`, `product_ids`, `page_index`, `page_size`,
  `to_user`.
- Needs the passthrough keep-alive ([§14](#14-other-endpoints)) before reads.
- No v5 endpoint, so no bit codes (no `Sys1CompState`). Use `O07`.

**Integration.** Not probed.

---

## 16. What goaquatemp implements today

| Feature | SDK | CLI |
|---|---|---|
| Login, `-100` re-login and retry | `Login` (no-op when already logged in) | `aquatemp login` |
| Owned + accepted shared devices | `GetDevices`, `GetDevice` | `devices list`, `devices info` |
| Live values via v5 `getDataByCode` + `bitCodes` (legacy: v1) | `GetDeviceData` | `status`, `temp get` |
| Operating state (heating / cooling / defrosting / idle / off) | `DeviceData.State` | `status` |
| Compressor: running bit, Hz, % of max, current | `DeviceData.Compressor*` | `status` |
| Water flow, external stop, sensor faults | `DeviceData.WaterFlowing`, `ExternalStop`, `SensorFaults` | `status` |
| Water temps marked invalid (NA) with no flow | `DeviceData.WaterTempStatus`, `Valid*Temp()` | `status`, `temp get`, `temp history` |
| Online status and faults | `GetDeviceStatus` | `status` |
| History (5 min / hourly / daily / monthly) | `GetHistory` | `history` |
| Parameter catalogs | `GetParameters` | `params` |
| Control: power, mode, target, silent fan | `SetPower`, `SetMode`, `SetTemperature`, `SetFanSpeed`, `Control` | `control …` |

Without `--device`, CLI commands use the account's first device.

Control uses the reference client's wire format and is covered by tests
against the fake server, but it has **not been exercised against a real heat
pump** by this project.

## 17. Sources

- Live probing of the modern API, 2026-11-17 (read-only; no control calls).
- [MatiasFernandez/aquatemp](https://github.com/MatiasFernandez/aquatemp): its
  [Bruno API collection](https://github.com/MatiasFernandez/aquatemp/tree/main/docs/api),
  [cloud client](https://github.com/MatiasFernandez/aquatemp/tree/main/custom_components/aqua_temp/cloud)
  and [pool heat pump notes](https://github.com/MatiasFernandez/aquatemp/blob/main/docs/entities/pool-heat-pumps.md)
  ([PR #40](https://github.com/MatiasFernandez/aquatemp/pull/40),
  [PR #37](https://github.com/MatiasFernandez/aquatemp/pull/37)).
- [radical-squared/aquatemp](https://github.com/radical-squared/aquatemp), the original Home Assistant integration.
- [dosordie/FoxAir_Control #209](https://github.com/dosordie/FoxAir_Control/issues/209): WarmLink iOS capture, the source of `listCollectData`.
- [europer/hitemp-aquatemp-home_assisent](https://github.com/europer/hitemp-aquatemp-home_assisent),
  [dst6se/aquatemp](https://github.com/dst6se/aquatemp/blob/main/heatpump),
  [Afraskai/cfgroup](https://github.com/Afraskai/cfgroup): other clients, same endpoints.
- [ioBroker Midas (Aquatemp) script thread](https://forum.iobroker.net/topic/54765/javascript-midas-aquatemp-poolheizung): API levels and base URLs.
- Manuals describing the app's curve screen: [Oasis AquaTemp module manual](https://sunloverheating.com.au/wp-content/uploads/2024/09/Oasis-AquaTemp-Module-Installation-and-Operation-Manual-2024.pdf),
  [EVOHEAT AquaTemp Wi-Fi module manual](https://manuals.plus/evoheat/aquatemp-wi-fi-module-controller-manual).
