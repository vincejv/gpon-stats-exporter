package device

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/vincejv/gpon-parser/model"
	"github.com/vincejv/gpon-parser/util"
)

// Structs for responses
type ZLTG202_PonSts struct {
	PonMode          string  `json:"pon_mode"`
	PonConnectStatus string  `json:"pon_connect_status"`
	PonLos           string  `json:"pon_los"`
	PonMacAddress    string  `json:"pon_mac_address"`
	PonEncryption    string  `json:"pon_encryption"`
	PonFecUsState    string  `json:"pon_fec_us_state"`
	PonFecDsState    string  `json:"pon_fec_ds_state"`
	BytesSent        int64   `json:"bytes_sent"`
	BytesReceived    int64   `json:"bytes_received"`
	PacketsSent      int64   `json:"packets_sent"`
	PacketsReceived  int64   `json:"packets_received"`
	TxPower          float64 `json:"tx_power"`
	RxPower          float64 `json:"rx_power"`
	Temperature      float64 `json:"temperature"`
	Voltage          float64 `json:"voltage"`
	BiasCurrent      float64 `json:"bias_current"`
}

type ZLTG202_DeviceInfo struct {
	DevModel string `json:"dev_model"`
	GponSN   string `json:"gpon_sn"`
	StVer    string `json:"st_ver"`
}

type ZLTG202_AllStatusInit struct {
	WlanSSID2         string `json:"wlan_ssid_2"`
	WlanSSID5         string `json:"wlan_ssid_5"`
	InternetConnected string `json:"internet_connected"`
	InternetIPv4      string `json:"internet_ipv4"`
	InternetIPv6      string `json:"internet_ipv6"`
	VoiceIPv4         string `json:"voice_ipv4"`
	InternetMode      string `json:"internet_mode"`
	WiredDev          int    `json:"wired_dev"`
	Wireless2Dev      int    `json:"wireless_2_dev"`
	Wireless5Dev      int    `json:"wireless_5_dev"`
	AllClients        int    `json:"all_cli"`
	LANIPAddress      string `json:"lan_ipAddr"`
	LANSubnetMask     string `json:"lan_subnetMask"`
	LANDNS            string `json:"lan_dns"`
	LANDHCPServer     string `json:"lan_dhcpServer"`
	MemoryUsage       string `json:"mem_uage"`
	CPUUsage          string `json:"cpu_uage"`
	SystemUptime      string `json:"system_uptime"`
}

// G202 Mutex payload
type ZLTG202_Payload struct {
	sync.RWMutex
	pon *ZLTG202_PonSts
	dev *ZLTG202_DeviceInfo
	ip  *ZLTG202_AllStatusInit
}

func (gponPayload *ZLTG202_Payload) GetGponData() (*ZLTG202_PonSts, *ZLTG202_DeviceInfo, *ZLTG202_AllStatusInit) {
	gponPayload.RLock()
	defer gponPayload.RUnlock()
	return gponPayload.pon, gponPayload.dev, gponPayload.ip
}

func (gponPayload *ZLTG202_Payload) SetGponData(pon *ZLTG202_PonSts, dev *ZLTG202_DeviceInfo, ip *ZLTG202_AllStatusInit) {
	gponPayload.Lock()
	gponPayload.pon = pon
	gponPayload.dev = dev
	gponPayload.ip = ip
	gponPayload.Unlock()
}

// cron job
func (o ZLTG202) UpdateCachedPage() {
	// clear previous cache
	cachedZltG202Data.SetGponData(nil, nil, nil)
	SvcHealth.SetFlag(false)

	// Define the URL and request body
	url := o.GetGponUrl()

	// Call the fetchData function
	headers := map[string]string{}

	// Fetch AllStatusInit
	allStatusData, err := o.FetchAndParse(url+"getASPdata/all_status_init", headers)
	if err != nil {
		log.Printf("Error fetching all status: %s", err.Error())
		return
	}
	allStatus := o.ParseAllStatusInit(allStatusData)
	// log.Printf("AllStatusInit: %+v\n", allStatus)

	// Fetch PonStatus
	ponStatusData, err := o.FetchAndParse(url+"getASPdata/new_ponGetStatus", headers)
	if err != nil {
		log.Printf("Error fetching PON status: %s", err.Error())
		return
	}
	ponStatus := o.ParsePonStatus(ponStatusData)
	// log.Printf("PonStatus: %+v\n", ponStatus)

	// Fetch DeviceInfo
	deviceInfoData, err := o.FetchAndParse(url+"getinfo/devModel&gpon_sn&stVer&", headers)
	if err != nil {
		log.Printf("Error fetching device info: %s", err.Error())
		return
	}
	deviceInfo := o.ParseDeviceInfo(deviceInfoData)
	// log.Printf("DeviceInfo: %+v\n", deviceInfo)

	if err == nil {
		cachedZltG202Data.SetGponData(&ponStatus, &deviceInfo, &allStatus)
		SvcHealth.SetFlag(true)
	}
}

func (o ZLTG202) GetOpticalInfo() *model.OpticalStats {
	var opticalInfo *model.OpticalStats
	pon, _, _ := cachedZltG202Data.GetGponData()

	if pon != nil {
		opticalInfo = new(model.OpticalStats)

		opticalInfo.TxPower = pon.TxPower
		opticalInfo.RxPower = pon.RxPower
		opticalInfo.Temperature = pon.Temperature
		opticalInfo.SupplyVoltage = pon.Voltage
		opticalInfo.BiasCurrent = pon.BiasCurrent
	}

	return opticalInfo
}

func (o ZLTG202) GetDeviceInfo() *model.DeviceStats {
	var deviceInfo *model.DeviceStats
	_, dev, ip := cachedZltG202Data.GetGponData()

	if dev != nil && ip != nil {
		deviceInfo = new(model.DeviceStats)

		deviceInfo.DeviceModel = dev.DevModel
		deviceInfo.ModelSerial = dev.GponSN
		deviceInfo.SoftwareVersion = dev.StVer
		deviceInfo.MemoryUsage = util.ParseFloat(ip.MemoryUsage)
		deviceInfo.CpuUsage = util.ParseFloat(ip.CPUUsage)
		deviceInfo.Uptime = util.ParseDuration(ip.SystemUptime)
	}

	return deviceInfo
}

func (o ZLTG202) GetGponUrl() string {
	host := util.Getenv("ONT_WEB_HOST", "192.168.254.254")
	port := util.Getenv("ONT_WEB_PORT", "80")
	return fmt.Sprintf("http://%s:%s/boaform/", host, port)
}

func (o ZLTG202) GetHeaders() map[string]string {
	headers := map[string]string{
		"Accept":           "*/*",
		"Accept-Language":  "en-US,en;q=0.9",
		"Cache-Control":    "no-cache",
		"Connection":       "keep-alive",
		"Pragma":           "no-cache",
		"Referer":          "http://globehpw.net/",
		"Sec-GPC":          "1",
		"User-Agent":       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		"X-Requested-With": "XMLHttpRequest",
	}
	return headers
}

func (o ZLTG202) FetchAndParse(url string, headers map[string]string) (map[string]string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}

	// Parse the response
	data := make(map[string]string)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// Split by '&' for multiple key-value pairs in the same line
		pairs := strings.Split(line, "&")
		for _, pair := range pairs {
			// Split each pair by '=' into key and value
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Remove units (dBm, V, mA) from specific keys
			switch key {
			case "tx-power", "rx-power":
				value = strings.TrimSuffix(value, " dBm")
			case "voltage":
				value = strings.TrimSuffix(value, " V")
			case "bias-current":
				value = strings.TrimSuffix(value, " mA")
			}

			data[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return data, nil
}

func (o ZLTG202) ParseAllStatusInit(data map[string]string) ZLTG202_AllStatusInit {
	return ZLTG202_AllStatusInit{
		WlanSSID2:         data["wlan_ssid_2"],
		WlanSSID5:         data["wlan_ssid_5"],
		InternetConnected: data["internet_connected"],
		InternetIPv4:      data["internet_ipv4"],
		InternetIPv6:      data["internet_ipv6"],
		VoiceIPv4:         data["voice_ipv4"],
		InternetMode:      data["internet_mode"],
		WiredDev:          util.ParseInt(data["wired_dev"]),
		Wireless2Dev:      util.ParseInt(data["wireless_2_dev"]),
		Wireless5Dev:      util.ParseInt(data["wireless_5_dev"]),
		AllClients:        util.ParseInt(data["all_cli"]),
		LANIPAddress:      data["lan_ipAddr"],
		LANSubnetMask:     data["lan_subnetMask"],
		LANDNS:            data["lan_dns"],
		LANDHCPServer:     data["lan_dhcpServer"],
		MemoryUsage:       data["mem_uage"],
		CPUUsage:          data["cpu_uage"],
		SystemUptime:      data["system_uptime"],
	}
}

func (o ZLTG202) ParsePonStatus(data map[string]string) ZLTG202_PonSts {
	return ZLTG202_PonSts{
		PonMode:          data["pon_mode"],
		PonConnectStatus: data["pon_connect_status"],
		PonLos:           data["pon-los"],
		PonMacAddress:    data["pon-mac-address"],
		PonEncryption:    data["pon-encryption"],
		PonFecUsState:    data["pon-fec-us-state"],
		PonFecDsState:    data["pon-fec-ds-state"],
		BytesSent:        util.ParseInt64(data["bytes-sent"]),
		BytesReceived:    util.ParseInt64(data["bytes-received"]),
		PacketsSent:      util.ParseInt64(data["packets-sent"]),
		PacketsReceived:  util.ParseInt64(data["packets-received"]),
		TxPower:          util.ParseFloat(data["tx-power"]),
		RxPower:          util.ParseFloat(data["rx-power"]),
		Temperature:      util.ParseFloat(data["temperature"]),
		Voltage:          util.ParseFloat(data["voltage"]),
		BiasCurrent:      util.ParseFloat(data["bias-current"]),
	}
}

func (o ZLTG202) ParseDeviceInfo(data map[string]string) ZLTG202_DeviceInfo {
	return ZLTG202_DeviceInfo{
		DevModel: data["devModel"],
		GponSN:   data["gpon_sn"],
		StVer:    data["stVer"],
	}
}
