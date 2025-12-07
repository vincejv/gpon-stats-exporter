package device

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/reiver/go-telnet"
	"github.com/vincejv/gpon-parser/model"
	"github.com/vincejv/gpon-parser/util"
	"golang.org/x/net/publicsuffix"
)

var client http.Client
var TelnetInit util.GlobalFlag
var TelnetScripts util.GlobalFlag
var telnetCreds util.LoginCreds
var deviceStatCached util.CachedStat

func (o ZTEF670L) GetGponUrl() string {
	protocol := util.Getenv("ONT_WEB_PROTOCOL", "http")
	return fmt.Sprintf("%s://%s:%s", protocol, o.GetModemIp(), o.GetWebUiPort())
}

func (o ZTEF670L) GetTelnetUrl() string {
	return fmt.Sprintf("%s:%s", o.GetModemIp(), o.GetTelnetPort())
}

func (o ZTEF670L) GetWebUsern() string {
	return util.Getenv("ONT_WEB_USER", "admin")
}

func (o ZTEF670L) GetWebPassw() string {
	return util.Getenv("ONT_WEB_PASS", "admin")
}

func (o ZTEF670L) GetModemIp() string {
	return util.Getenv("ONT_WEB_HOST", "192.168.1.1")
}

func (o ZTEF670L) GetTelnetPort() string {
	return util.Getenv("ONT_TELNET_PORT", "23")
}

func (o ZTEF670L) GetWebUiPort() string {
	return util.Getenv("ONT_WEB_PORT", "80")
}

func (o ZTEF670L) GrabLoginTokens() (loginTok string, csrfTok string) {
	// Parse tokens from login page
	req, err := http.NewRequest("GET", o.GetGponUrl()+"/", nil)
	if err != nil {
		log.Println(err)
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
		}
		bodyString := string(bodyBytes)
		loginTok := `Frm_Logintoken", "`
		csrfTok := `"Frm_Loginchecktoken", "`
		loginTokNdx := strings.Index(bodyString, loginTok) + len(loginTok)
		csrfTokNdx := strings.Index(bodyString, csrfTok) + len(csrfTok)
		loginTokVal := strings.Split(bodyString[loginTokNdx:], `"),`)[0]
		csrfTokVal := strings.Split(bodyString[csrfTokNdx:], `");`)[0]
		return loginTokVal, csrfTokVal
	}
	return "", ""
}

// cron job
func (o ZTEF670L) UpdateCachedPage() {
	// clear previous cache
	cachedPage.SetPage(nil)
	cachedPage2.SetPage(nil)
	SvcHealth.SetFlag(false)

	// Cookie jar setup
	options := cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	}
	jar, err := cookiejar.New(&options)
	if err != nil {
		log.Println(err)
		return
	}
	client = http.Client{Jar: jar}

	randNo := fmt.Sprintf("%d", util.RandInt(10000000, 89999999))

	// 0. Get login tokens from login page, CSRF and token verification to ONT
	loginTok, csrfTok := o.GrabLoginTokens()

	// 1a. Prepare login query
	hasher := sha256.New()
	bv := []byte(o.GetWebPassw() + randNo)
	hasher.Write(bv)

	form := url.Values{}
	form.Add("action", "login")
	form.Add("Username", o.GetWebUsern())                     // webgui username
	form.Add("Password", hex.EncodeToString(hasher.Sum(nil))) // sha256 (original pwd + pwdRand )
	form.Add("Frm_Logintoken", loginTok)                      // parse from login page
	form.Add("UserRandomNum", randNo)                         // random number
	form.Add("Frm_Loginchecktoken", csrfTok)                  // parse from login lage

	// 2. Perform login to UI
	req, err := http.NewRequest("POST", o.GetGponUrl()+"/", strings.NewReader(form.Encode()))
	if err != nil {
		log.Println(err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()

	// 2. Get optical power
	o.parsePage(client, GponSvc.GetGponUrl()+"/getpage.gch?pid=1002&nextpage=pon_status_link_info_t.gch", cachedPage)

	// 3. Get device information
	o.parsePage(client, GponSvc.GetGponUrl()+"/getpage.gch?pid=1002&nextpage=status_dev_info_t.gch", cachedPage2)

	// 4. Logout from UI
	form = url.Values{}
	form.Add("logout", "1")
	form.Add("_SESSION_TOKEN", csrfTok)
	req, err = http.NewRequest("POST", o.GetGponUrl()+"/", strings.NewReader(form.Encode()))
	if err != nil {
		log.Println(err)
		return
	}

	// 5. Retrieve remaining stats from telnet
	deviceStatCached.SetStat(o.GetStatsFromTelnet())

	resp, err = client.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
	SvcHealth.SetFlag(true)
}

func (o ZTEF670L) parsePage(client http.Client, url string, docPage *util.DocPage) {
	resp, err := client.Get(url)
	if err != nil {
		log.Println(err)
	}
	if err != nil {
		log.Println(err)
	}

	respRaw, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		docPage.SetPage(nil)
		docPage.SetStrPage("")
	}
	respData := string(respRaw)

	doc, err := htmlquery.Parse(strings.NewReader(string(respData)))
	if err == nil {
		docPage.SetPage(doc)
		docPage.SetStrPage(respData)
	} else {
		log.Println(err)
		docPage.SetPage(nil)
		docPage.SetStrPage("")
	}

	defer resp.Body.Close()
}

func (o ZTEF670L) GetStatsFromTelnet() (deviceInfo *model.DeviceStats) {
	var telnetUsern string
	var telnetPassw string

	if !TelnetInit.GetFlag() {
		TelnetInit.SetFlag(true)
		telnetUsern, telnetPassw = o.FactoryMode(o.GetWebUsern(), o.GetWebPassw())
		telnetCreds.SetCreds(telnetUsern, telnetPassw)
		log.Println("New telnet creds: " + telnetUsern + " | " + telnetPassw)
	} else {
		telnetUsern, telnetPassw = telnetCreds.GetCreds()
	}

	conn, err := telnet.DialTo(o.GetTelnetUrl())
	if err != nil {
		TelnetInit.SetFlag(false)
		log.Println("Unable to dial telnet on " + o.GetTelnetUrl() + " will regenerate credentials after next stat pull")
		return deviceInfo
	}

	err = util.PerformTelnetLogin(conn, telnetUsern, telnetPassw)
	if err != nil {
		if err.Error() == "access denied" {
			TelnetInit.SetFlag(false)
			log.Println("Unable to login to telnet with the last known credentials, will retry to regenerate credentials after 3 seconds")
			time.Sleep(3 * time.Second)
			return o.GetStatsFromTelnet()
		} else {
			log.Println(err.Error())
		}
		return deviceInfo
	}

	// custom commands to run in telnet once
	if !TelnetScripts.GetFlag() {
		TelnetScripts.SetFlag(true)
		log.Println("Running telnet custom scripts")
		telnetResp := strings.Split(util.ExecTelnet(conn, `ip link show nbif0 | awk '{for(i=1;i<=NF;i++) if ($i=="mtu") print $(i+1)}'`), "\n")
		mtuStr := strings.TrimSpace(telnetResp[1])
		mtu := util.ParseInt(mtuStr)
		if mtu < 1520 {
			log.Println("nbif0 mtu is too low at " + mtuStr + " bytes")
			util.ExecTelnet(conn, `ifconfig nbif0 mtu 1600 up`)
			log.Println("nbif0 mtu increased to 1600 bytes")
		} else {
			log.Println("nbif0 mtu is sufficient, currently set at " + mtuStr + " bytes")
		}
	}

	telnetResp := strings.Split(util.ExecTelnet(conn, `cat /proc/cpuusage && cat /proc/meminfo | grep "MemFree\|MemTotal" && setmac show | grep "2176\|2177" && cat /proc/uptime`), "\n")
	util.ExecTelnet(conn, `exit`)

	deviceInfo = new(model.DeviceStats)
	var cpuAvg float64 = 0.0
	var cpuCores int = 2
	cpuResp := telnetResp[1 : cpuCores+1]
	for i := 0; i < cpuCores; i++ {
		buff, err := strconv.ParseFloat(regexp.MustCompile(`[\:\%\s]+`).Split(cpuResp[i], -1)[1], 64)
		if err != nil {
			log.Println("Error parsing CPU Usage")
			log.Println(err)
			return deviceInfo
		}
		deviceInfo.CpuDtlUsage = append(deviceInfo.CpuDtlUsage, buff)
		cpuAvg += buff
	}

	cpuAvg /= float64(cpuCores)
	deviceInfo.CpuUsage = cpuAvg

	memResp := telnetResp[cpuCores+1 : cpuCores+3]
	totalMem := util.ParseInt64(regexp.MustCompile(`[\:\\kB\s]+`).Split(memResp[0], -1)[1])
	freeMem := util.ParseInt64(regexp.MustCompile(`[\:\\kB\s]+`).Split(memResp[1], -1)[1])
	deviceInfo.MemoryUsage = (1 - (float64(freeMem) / float64(totalMem))) * 100

	ponResp := telnetResp[cpuCores+3 : cpuCores+5]
	ponSerial, _ := hex.DecodeString(strings.ReplaceAll(strings.TrimSpace(strings.Split(ponResp[0], "is set to")[1])+strings.TrimSpace(strings.Split(ponResp[1], "is set to")[1]), " ", ""))
	deviceInfo.ModelSerial = string(ponSerial)

	uptimeResp := telnetResp[cpuCores+5 : cpuCores+6]
	deviceInfo.Uptime = util.ParseInt64(strings.Split(uptimeResp[0], ".")[0])

	return deviceInfo
}

func (o ZTEF670L) GetOpticalInfo() *model.OpticalStats {
	var opticalInfo *model.OpticalStats

	if cachedPage.GetPage() != nil {
		opticalInfo = new(model.OpticalStats)

		parsedList := make([]string, 0, 5)
		for i := 1; i < 6; i++ {
			if util.ParseHtmlPage(&parsedList, cachedPage, fmt.Sprintf("/html/body/div[3]/div[1]/div[3]/div[1]/table[2]/tbody/tr[%d]/td[2]", i)) != nil {
				return opticalInfo
			}
		}

		rxPower := strings.Split(strings.Split(cachedPage.GetStrPage(), `var RxPower = "`)[1], `";`)[0]
		txPower := strings.Split(strings.Split(cachedPage.GetStrPage(), `var TxPower = "`)[1], `";`)[0]
		rxPowerParsed := util.ParseFloat(rxPower)
		txPowerParsed := util.ParseFloat(txPower)

		opticalInfo.RxPower = rxPowerParsed / 10000
		opticalInfo.TxPower = txPowerParsed / 10000
		opticalInfo.Temperature = util.ParseFloat(parsedList[4])
		opticalInfo.SupplyVoltage = util.ParseFloat(parsedList[2]) / 1000000
		opticalInfo.BiasCurrent = util.ParseFloat(parsedList[3]) / 1000
	}

	return opticalInfo
}

func (o ZTEF670L) GetDeviceInfo() *model.DeviceStats {
	var deviceInfo *model.DeviceStats

	if cachedPage2.GetPage() != nil {
		// -- stats parsed from TELNET shell
		if deviceStatCached.GetStat() != nil {
			deviceInfo = deviceStatCached.GetStat()
		} else {
			deviceInfo = new(model.DeviceStats)
		}

		// -- stats parsed from WEB UI
		parsedList := make([]string, 0, 6)
		for i := 1; i < 6; i++ {
			if util.ParseHtmlPage(&parsedList, cachedPage2, fmt.Sprintf("/html/body/div[3]/div[1]/div[3]/table[2]/tbody/tr[%d]/td[2]", i)) != nil {
				return deviceInfo
			}
		}

		deviceInfo.DeviceModel = fmt.Sprintf("%s %s", parsedList[0], parsedList[2])
		deviceInfo.SoftwareVersion = parsedList[3]
	}

	return deviceInfo
}

var AES_KEY_POOL = [...]byte{
	0x7B, 0x56, 0xB0, 0xF7, 0xDA, 0x0E, 0x68, 0x52, 0xC8, 0x19,
	0xF3, 0x2B, 0x84, 0x90, 0x79, 0xE5, 0x62, 0xF8, 0xEA, 0xD2,
	0x64, 0x93, 0x87, 0xDF, 0x73, 0xD7, 0xFB, 0xCC, 0xAA, 0xFE,
	0x75, 0x43, 0x1C, 0x29, 0xDF, 0x4C, 0x52, 0x2C, 0x6E, 0x7B,
	0x45, 0x3D, 0x1F, 0xF1, 0xDE, 0xBC, 0x27, 0x85, 0x8A, 0x45,
	0x91, 0xBE, 0x38, 0x13, 0xDE, 0x67, 0x32, 0x08, 0x54, 0x11,
	0x75, 0xF4, 0xD3, 0xB4, 0xA4, 0xB3, 0x12, 0x86, 0x67, 0x23,
	0x99, 0x4C, 0x61, 0x7F, 0xB1, 0xD2, 0x30, 0xDF, 0x47, 0xF1,
	0x76, 0x93, 0xA3, 0x8C, 0x95, 0xD3, 0x59, 0xBF, 0x87, 0x8E,
	0xF3, 0xB3, 0xE4, 0x76, 0x49, 0x88,
}

var AES_KEY_POOL_NEW = [...]byte{
	0x8C, 0x23, 0x65, 0xD1, 0xFC, 0x32, 0x45, 0x37, 0x11, 0x28,
	0x71, 0x63, 0x07, 0x20, 0x69, 0x14, 0x73, 0xE7, 0xD4, 0x53,
	0x13, 0x24, 0x36, 0xC2, 0xB5, 0xE1, 0xFC, 0xCF, 0x8A, 0x9A,
	0x41, 0x89, 0x3C, 0x49, 0xCF, 0x5C, 0x72, 0x8C, 0x9E, 0xEB,
	0x75, 0x0D, 0x3F, 0xD1, 0xFE, 0xCC, 0x57, 0x65, 0x7A, 0x35,
	0x21, 0x3E, 0x68, 0x53, 0x7E, 0x97, 0x02, 0x48, 0x74, 0x71,
	0x95, 0x34, 0x53, 0x84, 0xB4, 0xC3, 0xE2, 0xD6, 0x27, 0x3D,
	0xE6, 0x5D, 0x72, 0x9C, 0xBC, 0x3D, 0x03, 0xFD, 0x76, 0xC1,
	0x9C, 0x25, 0xA8, 0x92, 0x47, 0xE4, 0x18, 0x0F, 0x24, 0x3F,
	0x4F, 0x67, 0xEC, 0x97, 0xF4, 0x99,
}

func (o ZTEF670L) Reset() bool {
	var reqBody = []byte(`SendSq.gch`)
	var url = fmt.Sprintf("%s/webFac", o.GetGponUrl())

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Println("Factory mode reset: request creation fail")
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 400
}

func (o ZTEF670L) RequestFactoryMode() {
	var reqBody = []byte(`RequestFactoryMode.gch`)
	var url = fmt.Sprintf("%s/webFac", o.GetGponUrl())

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Println("Factory mode request: request creation fail")
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		log.Println("Factory mode request: request execution fail")
	}
	if resp != nil {
		defer resp.Body.Close()
	}
}

func (o ZTEF670L) SendSq() (cipher.Block, int) {
	var keyPool []byte
	aesVer := -1
	keyPoolNdx := -1
	// rand takes from time seconds, range 0-59
	randNo := util.RandInt(0, 59)

	// the byte after last digital can not be null
	var reqBody = []byte(`SendSq.gch?rand=` + fmt.Sprintf("%d", randNo) + `\r\n`)
	var url = fmt.Sprintf("%s/webFac", o.GetGponUrl())
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Println("Send sq: request creation fail")
		return nil, -1
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("Send sq: request execution fail")
		return nil, aesVer
	}
	defer resp.Body.Close()

	if resp.ContentLength == 0 {
		log.Println("Old aes key pool")
		keyPool = AES_KEY_POOL[:]
		keyPoolNdx = randNo
		aesVer = 1
	} else {
		log.Println("New aes key pool")
		log.Println("Does not support this model yet")
		return nil, -1
	}

	key := make([]byte, 24)
	for i := 0; i < 24; i++ {
		key[i] = (keyPool[keyPoolNdx+i] ^ 0xA5) & 0xFF
	}

	cipher, err := aes.NewCipher([]byte(key))
	if err != nil {
		log.Println("Send sq: error generating key")
		return nil, aesVer
	}
	return cipher, aesVer
}

func (o ZTEF670L) CheckLoginAuth(cb cipher.Block, user string, pass string) []byte {
	var rawBody = util.Pad([]byte(`CheckLoginAuth.gch?version50&user=`+user+`&pass=`+pass), 16)
	var reqBody = util.EncryptAesEcb(rawBody, cb)
	var url = fmt.Sprintf("%s/webFacEntry", o.GetGponUrl())

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Println("checkLoginAuth: request creation fail")
		return nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("checkLoginAuth: request execution fail")
		log.Println(err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		cipherText, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println("checkLoginAuth: body read fail")
		}
		if len(cipherText)%16 != 0 {
			cipherText = util.Pad(cipherText, 16)
		}
		var cipherTextDecrypted = util.DecryptAesEcb(cipherText, cb)
		return util.Unpad(cipherTextDecrypted, 16)
	} else {
		log.Println("checkLoginAuth: decryption fail")
	}
	return nil
}

func (o ZTEF670L) OpenTelnet(cb cipher.Block) []byte {
	// # mode 1:ops 2:dev 3:production 4:user
	var rawBody = util.Pad([]byte(`FactoryMode.gch?mode=2&user=notused`), 16)
	var reqBody = util.EncryptAesEcb(rawBody, cb)
	var url = fmt.Sprintf("%s/webFacEntry", o.GetGponUrl())

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Println("openTelnet: request creation fail")
		return nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("openTelnet: request execution fail")
		log.Println(err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		cipherText, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println("openTelnet: body read fail")
		}
		if len(cipherText)%16 != 0 {
			cipherText = util.Pad(cipherText, 16)
		}
		var cipherTextDecrypted = util.DecryptAesEcb(cipherText, cb)
		return util.Unpad(cipherTextDecrypted, 16)
	} else {
		log.Println("openTelnet: decryption fail")
	}
	return nil
}

func (o ZTEF670L) GetTelnetLogin(resp string) (username string, password string) {
	resp = strings.ReplaceAll(resp, "FactoryModeAuth.gch?user=", "")
	creds := strings.Split(resp, "&pass=")
	return creds[0], creds[1]
}

func (o ZTEF670L) FactoryMode(webLoginUsern string, webLoginPassw string) (username string, password string) {
	// Step 0. Reset factory telnet
	if o.Reset() {
		log.Println("Factory mode reset: request execution success")
	}

	// Step 1. Request factory mode
	log.Println("facStep 1:")
	o.RequestFactoryMode()
	log.Println("Factory mode request: request execution success")

	log.Println("facStep 2:")
	cb, cipherVer := o.SendSq()
	log.Println("Send SQ: Ok!")

	if cipherVer == 1 {
		log.Println("facStep 3:")
		if o.CheckLoginAuth(cb, webLoginUsern, webLoginPassw) != nil {
			log.Println("checkLoginAuth: Ok!")
			log.Println("facStep 4:")
			return o.GetTelnetLogin(string(o.OpenTelnet(cb)[:]))
		} else {
			log.Println("Incorrect login or unsupported device")
		}
	} else {
		log.Println("Unsupported device, cipherVer is not v1")
	}

	return "", ""
}
