package device

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/vincejv/gpon-parser/model"
	"github.com/vincejv/gpon-parser/util"
)

func (o SKYW_GN) GetGponUrl() string {
	host := util.Getenv("ONT_WEB_HOST", "192.168.1.1")
	port := util.Getenv("ONT_WEB_PORT", "80")
	return fmt.Sprintf("%s:%s", host, port)
}

func (o SKYW_GN) Login() {
	// Step 1: Perform login request
	loginURL := fmt.Sprintf("%s/cgi-bin/index2.asp", o.GetGponUrl())
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{
		Transport: tr,
	}

	usern := util.Getenv("ONT_WEB_USER", "user")
	passw := util.Getenv("ONT_WEB_PASS", "user")

	// Form data for login
	formData := url.Values{
		"Username":         {usern},
		"Password1":        {passw},
		"Password2":        {passw},
		"Logoff":           {"0"},
		"hLoginTimes":      {"1"},
		"hLoginTimes_Zero": {"0"},
		"value_one":        {"1"},
		"logintype":        {"usr"},
		"LanIP":            {"192.168.1.1"},
		"Ipv6LanIP":        {"fe80::1"},
		"AccessIP":         {"192.168.1.1"},
		"languageSwitch":   {"2"},
	}

	// Prepare the POST request
	req, err := http.NewRequest("POST", loginURL, strings.NewReader(formData.Encode()))
	if err != nil {
		log.Printf("Can't login: %s\n", err.Error())
		return
	}

	// Add necessary headers for login
	req.Header.Set("Cookie", fmt.Sprintf("SESSIONID=boasidf86124cb1f8adae618c48397d0addf12; LoginTimes=1; UID=%s; PSW=%s", usern, passw))

	// Perform the POST request to login
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Can't login: %s\n", err.Error())
		return
	}
	defer resp.Body.Close()

	// Check for successful login (e.g., status code)
	if resp.StatusCode != 200 {
		fmt.Printf("Login failed with status code: %d\n", resp.StatusCode)
		return
	}
}

func (o SKYW_GN) FetchPage(url string) string {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{
		Transport: tr,
	}

	// Step 2: Perform the GET request to fetch data after login
	getURL := fmt.Sprintf("%s%s", o.GetGponUrl(), url)
	req, err := http.NewRequest("GET", getURL, nil)
	if err != nil {
		log.Printf("Can't fetch page: %s\n", err.Error())
	}

	usern := util.Getenv("ONT_WEB_USER", "user")
	passw := util.Getenv("ONT_WEB_PASS", "user")

	// Add necessary headers for GET request
	req.Header.Set("Cookie", fmt.Sprintf("SESSIONID=boasidf86124cb1f8adae618c48397d0addf12; LoginTimes=1; UID=%s; PSW=%s", usern, passw))

	// Perform the GET request to fetch network data
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	// Read and parse the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	// Convert the body to string for regex parsing
	return string(body)
}

// cron job
func (o SKYW_GN) UpdateCachedPage() {
	// clear old cache
	cachedPage.SetPage(nil)
	cachedPage2.SetStrPage("")
	SvcHealth.SetFlag(false)

	// login
	o.Login()

	// device page
	doc, err := htmlquery.Parse(strings.NewReader(o.FetchPage("/cgi-bin/sta-device.asp")))
	if err == nil {
		cachedPage.SetPage(doc)
	}

	// gpon page
	buffPage := o.FetchPage("/cgi-bin/sta-network.asp")
	if len(buffPage) > 0 {
		cachedPage2.SetStrPage(buffPage)
		SvcHealth.SetFlag(true)
	}
}

// Function to extract the value of sysUpTime
func (o SKYW_GN) ExtractSysUpTime(rawInput string) string {
	// Using regex to extract the value
	re := regexp.MustCompile(`var sysUpTime = "(.*?)";`)
	match := re.FindStringSubmatch(rawInput)
	if len(match) > 1 {
		return match[1]
	}

	// Fallback using strings if regex fails
	key := `var sysUpTime = "`
	start := strings.Index(rawInput, key)
	if start != -1 {
		start += len(key)
		end := strings.Index(rawInput[start:], `";`)
		if end != -1 {
			return rawInput[start : start+end]
		}
	}

	// Plain uptime formats
	anyRe := regexp.MustCompile(`\d+\s+day[s]?\s+\d{2}:\d{2}:\d{2}|\d{2}:\d{2}:\d{2}`)
	plain := anyRe.FindString(rawInput)
	if plain != "" {
		return plain
	}

	return ""
}

// Function to convert sysUpTime string to seconds as int64
func (o SKYW_GN) ConvertToSeconds(sysUpTime string) (int64, error) {
	var totalSeconds int64

	// Check if "days" is present in the string
	if strings.Contains(sysUpTime, "days") {
		// Split the input into parts (e.g., "3", "days", "08:27:40")
		parts := strings.Split(sysUpTime, " ")

		// Convert days to seconds
		days, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid days value: %v", err)
		}
		totalSeconds += days * 24 * 3600

		// Extract the time part (e.g., "08:27:40")
		sysUpTime = parts[2]
	}

	// Parse the time (HH:mm:ss)
	timeParts := strings.Split(sysUpTime, ":")
	if len(timeParts) != 3 {
		return 0, fmt.Errorf("invalid time format")
	}

	// Convert hours, minutes, and seconds to integers
	hours, err := strconv.ParseInt(timeParts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid hours value: %v", err)
	}
	minutes, err := strconv.ParseInt(timeParts[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid minutes value: %v", err)
	}
	seconds, err := strconv.ParseInt(timeParts[2], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid seconds value: %v", err)
	}

	// Add time to total seconds
	totalSeconds += hours*3600 + minutes*60 + seconds

	return totalSeconds, nil
}

func (o SKYW_GN) GetOpticalInfo() *model.OpticalStats {
	var opticalInfo *model.OpticalStats

	if len(cachedPage2.GetStrPage()) > 0 {
		opticalInfo = new(model.OpticalStats)
		// Regex patterns for parsing values
		patterns := map[string]string{
			"SendPower":       `this\.SendPower\s+=\s+\(Math\.round\(Math\.log\(\(Number\((\d+)\)\)/10000\)`,
			"RecvPower":       `this\.RecvPower\s+=\s+\(Math\.round\(Math\.log\(\(Number\((\d+)\)\)/10000\)`,
			"WorkVoltage":     `Number\((\d+)\)/10`,
			"WorkElectric":    `Number\((\d+)\)\*2/1000`,
			"WorkTemperature": `transTemperature\((\d+)\)`,
			"PonState":        `this\.PonState\s+=\s+'(\w+)'`,
			"LoidStatus":      `this\.loidStatus\s+=\s+'(\w+)'`,
			"UpTime":          `this\.up_time\s+=\s+'(\d+)'`,
		}

		rawValues := make(map[string]string)

		// Extract raw values using regex
		for key, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			match := re.FindStringSubmatch(cachedPage2.GetStrPage())
			if len(match) > 1 {
				rawValues[key] = match[1]
			}
		}

		opticalInfo.RxPower, _ = util.ConvertPowerToDBm(rawValues["RecvPower"])
		opticalInfo.TxPower, _ = util.ConvertPowerToDBm(rawValues["SendPower"])
		opticalInfo.Temperature, _ = util.ConvertWorkTemperature(rawValues["WorkTemperature"])
		opticalInfo.SupplyVoltage = util.ParseFloat(rawValues["WorkVoltage"]) / 10000
		opticalInfo.BiasCurrent = util.ParseFloat(rawValues["WorkElectric"]) * 2 / 1000
	}

	return opticalInfo
}

func (o SKYW_GN) GetDeviceInfo() *model.DeviceStats {
	var deviceInfo *model.DeviceStats

	if cachedPage.GetPage() != nil {
		deviceInfo = new(model.DeviceStats)
		parsedList := make([]string, 0, 6)

		dvcInfoTbl := "/html/body/table/tbody/tr[2]/td/table/tbody/tr[2]/td[3]/table/tbody/tr/td[2]/table/tbody"
		dvcDtlTbl := "/html/body/table/tbody/tr[2]/td/table/tbody/tr[4]/td[3]/table/tbody/tr/td[2]/table/tbody"
		commonNode := "/td[2]"

		util.ParseHtmlPage(&parsedList, cachedPage, dvcInfoTbl+"/tr[6]"+commonNode)
		util.ParseHtmlPage(&parsedList, cachedPage, dvcInfoTbl+"/tr[5]"+commonNode)
		util.ParseHtmlPage(&parsedList, cachedPage, dvcDtlTbl+"/tr[2]"+commonNode)
		util.ParseHtmlPage(&parsedList, cachedPage, dvcDtlTbl+"/tr[1]"+commonNode)
		util.ParseHtmlPage(&parsedList, cachedPage, dvcDtlTbl+"/tr[3]"+commonNode)

		model := strings.ToLower(os.Getenv("ONT_MODEL"))

		if strings.Contains(model, "gn256") {
			deviceInfo.DeviceModel = "GN256"
		} else {
			deviceInfo.DeviceModel = "GN630V"
		}
		deviceInfo.ModelSerial = parsedList[0]
		deviceInfo.SoftwareVersion = parsedList[1]
		deviceInfo.MemoryUsage = util.ParseFloat(parsedList[2])
		deviceInfo.CpuUsage = util.ParseFloat(parsedList[3])
		deviceInfo.Uptime, _ = o.ConvertToSeconds(o.ExtractSysUpTime(parsedList[4]))
	}

	return deviceInfo
}
