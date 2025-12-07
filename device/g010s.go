package device

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/vincejv/gpon-parser/model"
	"github.com/vincejv/gpon-parser/util"
)

func (o NOKIA_G010S) GetGponUrl() string {
	host := util.Getenv("ONT_WEB_HOST", "192.168.1.10")
	port := util.Getenv("ONT_WEB_PORT", "80")
	protocol := util.Getenv("ONT_WEB_PROTOCOL", "http")
	cfgCmd := util.Getenv("G010S_SCRIPT_URL", "cfg149944")
	return fmt.Sprintf("%s://%s:%s/cgi-bin/luci/command/%s", protocol, host, port, cfgCmd)
}

// cron job
func (o NOKIA_G010S) UpdateCachedPage() {
	cachedPage.SetStrPage("")
	SvcHealth.SetFlag(false)

	resp, err := http.Get(GponSvc.GetGponUrl())
	if err != nil {
		log.Println("Error fetching URL:", err)
		return
	}
	defer resp.Body.Close()

	// Ensure Content-Type is text/plain
	if resp.Header.Get("Content-Type") != "text/plain" {
		log.Println("Unexpected content type:", resp.Header.Get("Content-Type"))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading body:", err)
		return
	} else {
		cachedPage.SetStrPage(string(body))
		SvcHealth.SetFlag(true)
	}
}

func (o NOKIA_G010S) GetOpticalInfo() *model.OpticalStats {
	var opticalInfo *model.OpticalStats

	if len(cachedPage.GetStrPage()) > 0 {
		val := cachedPage.GetStrPage()
		opticalInfo = new(model.OpticalStats)

		// Use util.ParseFloat and RemoveLastNChars like your current style
		opticalInfo.TxPower = util.ParseFloat(util.ExtractAfter(val, "TX Power (dBm)        : ", "dBm"))
		opticalInfo.RxPower = util.ParseFloat(util.ExtractAfter(val, "RSSI 1490 Power (dBm) : ", "dBm"))
		opticalInfo.Temperature = util.ParseFloat(util.ExtractAfter(val, "Temp (Die/Laser) : ", "℃"))
		opticalInfo.SupplyVoltage = util.ParseFloat(util.ExtractAfter(val, "DDMI Voltage          : ", "mV")) / 1000
		opticalInfo.BiasCurrent = util.ParseFloat(util.ExtractAfter(val, "Bias Current     : ", "mA"))
	}

	return opticalInfo
}

func (o NOKIA_G010S) GetDeviceInfo() *model.DeviceStats {
	var deviceInfo *model.DeviceStats

	if len(cachedPage.GetStrPage()) > 0 {
		val := cachedPage.GetStrPage()
		deviceInfo = new(model.DeviceStats)

		deviceInfo.DeviceModel = util.ExtractLineValue(val, "Model              :")
		deviceInfo.ModelSerial = util.ExtractLineValue(val, "GPON Serial      :")
		deviceInfo.SoftwareVersion = util.ExtractLineValue(val, "omcid Version      :")
		deviceInfo.MemoryUsage = util.ParseFloat(util.RemoveLastNChars(util.ExtractLineValue(val, "Used RAM         :"), 1))
		deviceInfo.CpuUsage = util.ParseFloat(util.RemoveLastNChars(util.ExtractLineValue(val, "CPU Usage        :"), 1))
		deviceInfo.Uptime = util.ParseInt64(util.ExtractLineValue(val, "Uptime (secs)    :"))
	}

	return deviceInfo
}
