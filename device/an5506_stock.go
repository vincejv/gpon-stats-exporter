package device

import (
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/vincejv/gpon-parser/model"
	"github.com/vincejv/gpon-parser/util"
	"golang.org/x/net/publicsuffix"
)

func (o AN5506_Stock) GetGponUrl() string {
	host := util.Getenv("ONT_WEB_HOST", "globebroadband.net")
	port := util.Getenv("ONT_WEB_PORT", "80")
	webProtocol := util.Getenv("ONT_WEB_PROTOCOL", "http")
	return fmt.Sprintf("%s://%s:%s", webProtocol, host, port)
}

// cron job
func (o AN5506_Stock) UpdateCachedPage() {
	// clear old cache
	cachedPage.SetPage(nil)
	cachedPage2.SetPage(nil)
	SvcHealth.SetFlag(false)

	options := cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	}
	jar, err := cookiejar.New(&options)
	if err != nil {
		log.Println(err)
		return
	}

	client := http.Client{Jar: jar}
	form := url.Values{}
	form.Add("User", util.Getenv("ONT_WEB_USER", "user"))          // webgui username
	form.Add("Passwd", util.Getenv("ONT_WEB_PASS", "tattoo@home")) // webgui password

	// 1. Login to UI
	req, err := http.NewRequest("POST", o.GetGponUrl()+"/goform/webLogin", strings.NewReader(form.Encode()))
	if err != nil {
		log.Println(err)
		return
	}

	req.Header.Set("Referer", "http://127.0.0.1/gpon-parser") // must define a referer or it will fail

	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()

	// 2. Get optical power
	o.parsePage(client, GponSvc.GetGponUrl()+"/state/opt_power.asp", cachedPage)

	// 3. Get device information
	o.parsePage(client, GponSvc.GetGponUrl()+"/state/deviceInfor.asp", cachedPage2)
	SvcHealth.SetFlag(true)

	// 4. Logout from UI
	req, err = http.NewRequest("GET", o.GetGponUrl()+"/goform/webLogout", nil)
	if err != nil {
		log.Println(err)
		return
	}

	req.Header.Set("Referer", o.GetGponUrl()+"/menu_ph_globe.asp") // must define a referer to logout cleanly
	resp, err = client.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
}

func (o AN5506_Stock) parsePage(client http.Client, url string, docPage *util.DocPage) {
	resp, err := client.Get(url)
	if err != nil {
		log.Println(err)
	}
	if err != nil {
		log.Println(err)
	}

	doc, err := htmlquery.Parse(resp.Body)
	if err == nil {
		docPage.SetPage(doc)
	} else {
		log.Println(err)
		docPage.SetPage(nil)
	}

	defer resp.Body.Close()
}

func (o AN5506_Stock) GetOpticalInfo() *model.OpticalStats {
	var opticalInfo *model.OpticalStats

	if cachedPage.GetPage() != nil {
		opticalInfo = new(model.OpticalStats)

		parsedList := make([]string, 0, 5)
		for i := 2; i < 7; i++ {
			if util.ParseHtmlPage(&parsedList, cachedPage, fmt.Sprintf("/html/body/form/table[3]/tbody/tr[%d]/td[2]", i)) != nil {
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

func (o AN5506_Stock) GetDeviceInfo() *model.DeviceStats {
	var deviceInfo *model.DeviceStats

	if cachedPage2.GetPage() != nil {
		deviceInfo = new(model.DeviceStats)

		parsedList := make([]string, 0, 6)
		for i := 2; i < 16; i++ {
			if util.ParseHtmlPage(&parsedList, cachedPage2, fmt.Sprintf("/html/body/form/table[3]/tbody/tr[%d]/td[2]", i)) != nil {
				return deviceInfo
			}
		}

		deviceInfo.DeviceModel = parsedList[2]
		deviceInfo.ModelSerial = strings.ReplaceAll(parsedList[4], "FFFFFF", "")
		deviceInfo.SoftwareVersion = parsedList[0]
		deviceInfo.MemoryUsage = util.ParseFloat(util.RemoveLastNChars(parsedList[10], 1))
		deviceInfo.CpuUsage = util.ParseFloat(util.RemoveLastNChars(parsedList[9], 1))
		deviceInfo.Uptime = util.ParseDuration(parsedList[13])
	}

	return deviceInfo
}
