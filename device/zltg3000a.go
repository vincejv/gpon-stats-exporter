package device

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"net/http"

	"github.com/vincejv/gpon-parser/model"
	"github.com/vincejv/gpon-parser/util"
)

// Define a struct to match the JSON response
type ZLTG3000A_Payload struct {
	Success            bool   `json:"success"`
	Cmd                int    `json:"cmd"`
	PonSN              string `json:"pon_sn"`
	XponMode           string `json:"xponMode"`
	TrafficStatus      string `json:"trafficStatus"`
	TxPower            string `json:"TxPower"`
	RxPower            string `json:"RxPower"`
	WorkTemperature    string `json:"WorkTemperature"`
	IP                 string `json:"IP"`
	Netmask            string `json:"netmask"`
	Type               string `json:"type"`
	FirstDNS           string `json:"firstDNS"`
	WifiName24         string `json:"wifiName24"`
	WifiOpen24         string `json:"wifiOpen24"`
	WifiName5          string `json:"wifiName5"`
	WifiOpen5          string `json:"wifiOpen5"`
	LineEnabled        string `json:"lineEnabled"`
	SipName            string `json:"sipName"`
	Status             string `json:"Status"`
	RegFailReason      string `json:"RegFailReason"`
	CycleNum           string `json:"CycleNum"`
	VEntryName         string `json:"vEntryName"`
	VEntryID           string `json:"vEntryID"`
	VActive            string `json:"vActive"`
	VWANModem          string `json:"vWANModem"`
	VWanStatus4        string `json:"vWanStatus4"`
	VIP                string `json:"vIP"`
	VWanStatus6        string `json:"vWanStatus6"`
	VIP6               string `json:"vIP6"`
	Tr069ConnectStatus string `json:"tr069_connect_status"`
	ParamType          string `json:"param_type"`
	LosType            string `json:"los_type"`
	BandsteerEnable    string `json:"bandsteerEnable"`
	Mac                string `json:"mac"`
	BoardType          string `json:"board_type"`
	DeviceSN           string `json:"device_sn"`
	Memory             string `json:"memory"`
	HwVersion          string `json:"hwversion"`
	Uptime             string `json:"uptime"`
	CPUUsage           string `json:"cpu_usage"`
	FakeVersion        string `json:"fake_version"`
}

// Mutex payload
type GponPayload struct {
	sync.RWMutex
	doc *ZLTG3000A_Payload
}

func (gponPayload *GponPayload) GetGponData() *ZLTG3000A_Payload {
	gponPayload.RLock()
	defer gponPayload.RUnlock()
	return gponPayload.doc
}

func (gponPayload *GponPayload) SetGponData(payload *ZLTG3000A_Payload) {
	gponPayload.Lock()
	gponPayload.doc = payload
	gponPayload.Unlock()
}

func (o ZLTG3000A) GetGponUrl() string {
	host := util.Getenv("ONT_WEB_HOST", "192.168.254.254")
	port := util.Getenv("ONT_WEB_PORT", "80")
	return fmt.Sprintf("http://%s:%s/cgi-bin/http.cgi", host, port)
}

// memoryUsage calculates memory usage percentage and returns it as a float
func (o ZLTG3000A) ParseMemoryUsage(memory string) (float64, error) {
	// Split the memory string by commas
	parts := strings.Split(memory, ",")

	// Convert the first and second parts to numbers (assuming the first two parts are total and available memory)
	totalMemoryStr := util.RemoveLastNChars((strings.TrimSpace(parts[0])), 3)
	usedMemoryStr := util.RemoveLastNChars((strings.TrimSpace(parts[1])), 3)

	// Convert strings to integers
	totalMemory, err := strconv.Atoi(totalMemoryStr)
	if err != nil {
		return 0, fmt.Errorf("error converting total memory: %v", err)
	}

	usedMemory, err := strconv.Atoi(usedMemoryStr)
	if err != nil {
		return 0, fmt.Errorf("error converting used memory: %v", err)
	}

	// Calculate memory usage percentage
	memoryUsage := 100 - (float64(usedMemory) / float64(totalMemory) * 100)

	// Return the raw float value
	return memoryUsage, nil
}

// fetchData makes the HTTP request and returns the response or an error
func (o ZLTG3000A) fetchData(url string, jsonData []byte) (*ZLTG3000A_Payload, error) {
	// Create a new request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Add headers to the request
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://globebroadband.net")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", "http://globebroadband.net/")
	req.Header.Set("Sec-GPC", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")

	// Send the request using an HTTP client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	// Unmarshal JSON response into the Response struct
	var result ZLTG3000A_Payload
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}

	// Return the result as a pointer to the Response struct
	return &result, nil
}

// cron job
func (o ZLTG3000A) UpdateCachedPage() {
	// clear previous cache
	cachedGponData.SetGponData(nil)
	SvcHealth.SetFlag(false)

	// Define the URL and request body
	url := o.GetGponUrl()
	jsonData := []byte(`{"cmd":481,"method":"GET","sessionId":""}`)

	// Call the fetchData function
	response, err := o.fetchData(url, jsonData)

	if err == nil {
		cachedGponData.SetGponData(response)
		SvcHealth.SetFlag(true)
	}
}

func (o ZLTG3000A) GetOpticalInfo() *model.OpticalStats {
	var opticalInfo *model.OpticalStats

	if cachedGponData.GetGponData() != nil {
		opticalInfo = new(model.OpticalStats)
		gp := cachedGponData.GetGponData()

		opticalInfo.TxPower, _ = util.ConvertPowerToDBm(gp.TxPower)
		opticalInfo.RxPower, _ = util.ConvertPowerToDBm(gp.RxPower)
		opticalInfo.Temperature, _ = util.ConvertWorkTemperature(gp.WorkTemperature)
		// opticalInfo.SupplyVoltage  info not available
		// opticalInfo.BiasCurrent    info not available
	}

	return opticalInfo
}

func (o ZLTG3000A) GetDeviceInfo() *model.DeviceStats {
	var deviceInfo *model.DeviceStats

	if cachedGponData.GetGponData() != nil {
		deviceInfo = new(model.DeviceStats)
		gp := cachedGponData.GetGponData()

		deviceInfo.DeviceModel = gp.BoardType
		deviceInfo.ModelSerial = gp.PonSN
		deviceInfo.SoftwareVersion = gp.FakeVersion
		deviceInfo.MemoryUsage, _ = o.ParseMemoryUsage(gp.Memory)
		deviceInfo.CpuUsage = util.ParseFloat(gp.CPUUsage)
		deviceInfo.Uptime = util.ParseInt64(gp.Uptime)
	}

	return deviceInfo
}
