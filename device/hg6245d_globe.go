package device

import (
	"fmt"

	"github.com/antchfx/htmlquery"
	"github.com/vincejv/gpon-parser/model"
	"github.com/vincejv/gpon-parser/util"
)

func (o HG6245D_Globe) GetGponUrl() string {
	host := util.Getenv("ONT_WEB_HOST", "globebroadband.net")
	port := util.Getenv("ONT_WEB_PORT", "80")
	return fmt.Sprintf("http://%s:%s/login_new_globe.asp", host, port)
}

// cron job
func (o HG6245D_Globe) UpdateCachedPage() {
	cachedPage.SetPage(nil)
	SvcHealth.SetFlag(false)
	doc, err := htmlquery.LoadURL(GponSvc.GetGponUrl())
	if err == nil {
		cachedPage.SetPage(doc)
		SvcHealth.SetFlag(true)
	}
}

func (o HG6245D_Globe) GetOpticalInfo() *model.OpticalStats {
	var opticalInfo *model.OpticalStats

	if cachedPage.GetPage() != nil {
		opticalInfo = new(model.OpticalStats)
		parsedList := make([]string, 0, 5)
		for i := 1; i < 6; i++ {
			if util.ParseHtmlPage(&parsedList, cachedPage, fmt.Sprintf("/html/body/div[1]/div[1]/div[2]/div/div[4]/ul/li[%d]/span[2]", i)) != nil {
				return opticalInfo
			}
		}

		opticalInfo.TxPower = util.ParseFloat(util.RemoveLastNChars(parsedList[0], 4))
		opticalInfo.RxPower = util.ParseFloat(util.RemoveLastNChars(parsedList[1], 4))
		opticalInfo.Temperature = util.ParseFloat(util.RemoveLastNChars(parsedList[2], 4))
		opticalInfo.SupplyVoltage = util.ParseFloat(util.RemoveLastNChars(parsedList[3], 2))
		opticalInfo.BiasCurrent = util.ParseFloat(util.RemoveLastNChars(parsedList[4], 3))
	}

	return opticalInfo
}

func (o HG6245D_Globe) GetDeviceInfo() *model.DeviceStats {
	var deviceInfo *model.DeviceStats

	if cachedPage.GetPage() != nil {
		deviceInfo = new(model.DeviceStats)
		parsedList := make([]string, 0, 6)
		for i := 1; i < 7; i++ {
			if util.ParseHtmlPage(&parsedList, cachedPage, fmt.Sprintf("/html/body/div[1]/div[1]/div[2]/div/div[5]/ul/li[%d]/span[2]", i)) != nil {
				return deviceInfo
			}
		}

		deviceInfo.DeviceModel = parsedList[0]
		deviceInfo.ModelSerial = parsedList[1]
		deviceInfo.SoftwareVersion = parsedList[2]
		deviceInfo.MemoryUsage = util.ParseFloat(util.RemoveLastNChars(parsedList[3], 1))
		deviceInfo.CpuUsage = util.ParseFloat(util.RemoveLastNChars(parsedList[4], 1))
		deviceInfo.Uptime = util.ParseDuration(parsedList[5])
	}

	return deviceInfo
}
