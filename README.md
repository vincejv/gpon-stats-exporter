# GPON Parser (GPON Stats Exporter)

GPON Parser is a Telegraf exporter written in Go that parses and exports GPON ONT/ONU statistics via REST API (JSON format). It is designed for seamless integration with Telegraf, InfluxDB, and Grafana for monitoring and visualization of key performance metrics such as CPU usage, RAM usage, optical RX/TX power, bias current, voltage, and temperature.

## Features

- Fetches GPON ONT/ONU statistics via REST API
- Monitors CPU usage, RAM usage, optical RX/TX power, bias current, voltage, and temperature
- Outputs data in JSON format for easy processing
- Compatible with Telegraf for InfluxDB ingestion
- Supports visualization in Grafana

## Installation

1. Clone the repository:
   ```sh
   git clone https://github.com/vincejv/gpon-parser
   cd gpon-parser
   ```
2. Build the binary (see in [detail](#building-from-source))
   ```sh
   go build -ldflags "-s -w" -o gpon-parser
   ```
3. Configure the [environment variables](#environment-variables) for a specific ONT model or config.

## Usage

Run the exporter with:

```sh
./gpon-parser
```

## Integration with Telegraf

Configure Telegraf to scrape the exported JSON data using the `inputs.http` plugin.
`telegraf/telegraf.d/gpon-monitoring.conf`
```
[[inputs.http]]
  name_override="gpon_monitoring"

  urls = [
    "http://gpon-monitoring.docker.internal:8092/gpon/deviceInfo",
    "http://gpon-monitoring.docker.internal:8092/gpon/opticalInfo",
  ]

  data_format = "json"

  json_string_fields = [
    "deviceModel",
    "modelSerial",
    "softwareVersion",
  ]

  [inputs.http.tags]
    area = "home"
```

## ONT Model supported

* FiberHome HG6245D (Globe Telecom Philippines firmware)
  * `ONT_MODEL: hg6245d_globe`
* FiberHome AN5506_04F1A (Globe Telecom Philippines firmware) and other generic FH ONT with generic firmware
  * `ONT_MODEL: an5506_stock`
* ZTE F660 and F670
  * `ONT_MODEL: zte_f670`
* ZLT G3000A (Globe Telecom Philippines firmware)
  * `ONT_MODEL: zlt_g3000a`
* ZLT G202 (Globe Telecom Philippines firmware)
  * `ONT_MODEL: zlt_g202`
* Skyworth GN630V (Stock firmware)
  * `ONT_MODEL: skyworth_gn630v`
* Nokia G-010S-A/P (OpenWrt firmware) (must add [this script](deps/g010s.sh) to `rootfs` and add it as a Publicly accessed "custom command" through LuCi)
  * `ONT_MODEL: nokia_g010s`

## Environment variables
* `ONT_WEB_HOST`
  * IP address of ONT
  * Default: depends on modem
* `ONT_WEB_PORT`
  * Port on which ONT Web UI is listening to
  * Default: depends on modem
* `ONT_WEB_PROTOCOL`
  * Web protocol which the ONT web gui uses, typically set as `http` or `https`
  * Default: `http`
* `ONT_WEB_USER`
  * ONT Web UI username
  * Default: depends on modem
* `ONT_WEB_PASS`
  * ONT Web UI password
  * Default: depends on modem
* `ONT_TELNET_HOST`
  * ONT telnet console hostname or ip address
  * Default: `ONT_WEB_HOST`
* `ONT_TELNET_PORT`
  * ONT telnet console password
  * Default: `23`
* `ONT_POLL_SEC`
  * Specifies the frequency on how often the GPON stats are pulled from the ONT
  * Default: `60`
* `LISTEN_PORT`
  * Port on which the exporter listens to
  * Default: `8092`
* `LISTEN_IP`
  * Ip address on which the exporter listens to
  * Default: `0.0.0.0`
* `G010S_SCRIPT_URL`
  * OpenWrt url path to fetch the system and GPON stats, it is formatted as `ONT_WEB_PROTOCOL://ONT_WEB_HOST:ONT_WEB_PORT/cgi-bin/luci/command/G010S_SCRIPT_URL
`
  * Default: `cfg149944`

## Running
Docker Pull
```sh
docker pull vincejv/gpon-parser:latest
```
Docker Run
```sh
docker run -d \
  --name gpon-parser \
  --restart unless-stopped \
  vincejv/gpon-parser:latest
```
Docker Compose
```yaml
version: '3'

services:
  gpon-parser:
    image: vincejv/gpon-parser:latest
    container_name: gpon-parser
    restart: unless-stopped
    environment:
      ONT_MODEL: "zte_f670"
```

## REST API Paths
`/gpon/allInfo`
```json
{
  "deviceStats": {
    "memoryUsage": 54.885117384596136,
    "cpuUsage": 1.31,
    "cpuDtlUsage": [
      0.1,
      2.52
    ],
    "deviceModel": "F660",
    "modelSerial": "FHTTXXXXXX",
    "softwareVersion": "V1.1.20P3N6B",
    "uptime": 86673
  },
  "opticalStats": {
    "rxPower": -26.5757,
    "txPower": 2.7781,
    "temperature": 44,
    "supplyVoltage": 3.229,
    "biasCurrent": 13.5
  }
}
```
`/gpon/deviceInfo`
```json
{
  "memoryUsage": 54.880947416704885,
  "cpuUsage": 2.4749999999999996,
  "cpuDtlUsage": [
    0.1,
    4.85
  ],
  "deviceModel": "F660",
  "modelSerial": "FHTTXXXXXX",
  "softwareVersion": "V1.1.20P3N6B",
  "uptime": 86748
}
```
`/gpon/opticalInfo`
```json
{
  "rxPower": -26.5757,
  "txPower": 2.7781,
  "temperature": 44,
  "supplyVoltage": 3.229,
  "biasCurrent": 13.55
}
```

## Building from source

### Building the package
```sh
go build -ldflags "-s -w"
```

### Running
```sh
go run .
```

### ARM Build on Windows
```powershell
$env:GOARCH='arm'
$env:GOOS='linux'
```

### ARM Build on Linux
```sh
export GOARCH='arm'
export GOOS='linux'
```
