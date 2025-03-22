package session

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"sync"
	"time"

	"Go_auto_checker/internal/models"
)

const (
	defaultTimeout = 10 * time.Second
	defaultKeepAlive = 30 * time.Second
	maxIdleConns = 100
	idleConnTimeout = 90 * time.Second
	tlsHandshakeTimeout = 10 * time.Second
	expectContinueTimeout = 1 * time.Second
)

var (
	// Biên dịch regexp trước để tối ưu hiệu suất
	verificationTokenRegex = regexp.MustCompile(`<meta name="__RequestVerificationToken" content="([^"]+)"`)
	sessionIdRegex = regexp.MustCompile(`var uniqueSessionId = "([^"]+)"`)
)

// Session struct chứa thông tin session
type Session struct {
	client      *http.Client
	BaseURL     string
	LoginInfo   models.SessionInfo
	Verbose     bool      // Thêm flag verbose để kiểm soát log
	mutex       sync.Mutex // Mutex để đồng bộ hóa việc cập nhật thông tin session
	lastVerifyTime time.Time // Thời gian xác minh captcha gần nhất
	idyKeyTTL   time.Duration // Thời gian sống của IdyKey (mặc định 10 phút)
	maxConcurrent int         // Số lượng request đồng thời tối đa
	semaphore   chan struct{} // Semaphore để giới hạn số lượng request đồng thời
	userAgent   string        // UserAgent tùy chỉnh cho session
}

// New tạo một session mới với client HTTP tối ưu
func New() *Session {
	jar, _ := cookiejar.New(nil)
	
	// Tối ưu HTTP transport
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   defaultTimeout,
			KeepAlive: defaultKeepAlive,
		}).DialContext,
		MaxIdleConns:          maxIdleConns,
		IdleConnTimeout:       idleConnTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
		ForceAttemptHTTP2:     true,
	}
	
	client := &http.Client{
		Jar:       jar,
		Transport: transport,
		Timeout:   defaultTimeout,
	}
	
	return &Session{
		client:    client,
		BaseURL:   "https://www.ku2552.net", // Địa chỉ URL của trang web mục tiêu
		LoginInfo: models.SessionInfo{},
		Verbose:   false, // Mặc định không hiển thị log chi tiết
		idyKeyTTL: 10 * time.Minute, // IdyKey thường có thời gian sống 10 phút
		userAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36", // UserAgent mặc định
	}
}

// GetLoginInfo trả về thông tin đăng nhập hiện tại
func (s *Session) GetLoginInfo() models.SessionInfo {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.LoginInfo
}

// SetLoginInfo thiết lập thông tin đăng nhập từ bên ngoài
func (s *Session) SetLoginInfo(info models.SessionInfo) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.LoginInfo = info
	
	// Nếu IdyKey được cập nhật, cập nhật thời gian xác minh gần nhất
	if info.IdyKey != "" {
		s.lastVerifyTime = time.Now()
	}
}

// SetVerbose thiết lập chế độ hiển thị log chi tiết
func (s *Session) SetVerbose(verbose bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.Verbose = verbose
}

// IsIdyKeyValid kiểm tra xem IdyKey hiện tại có hợp lệ không
func (s *Session) IsIdyKeyValid() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	// Nếu không có IdyKey hoặc thời gian xác minh gần nhất chưa được thiết lập
	if s.LoginInfo.IdyKey == "" || s.lastVerifyTime.IsZero() {
		return false
	}
	
	// Kiểm tra xem IdyKey có hết hạn chưa
	return time.Since(s.lastVerifyTime) < s.idyKeyTTL
}

// SetUserAgent thiết lập UserAgent tùy chỉnh cho session
func (s *Session) SetUserAgent(ua string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.userAgent = ua
}

// GetUserAgent trả về UserAgent hiện tại của session
func (s *Session) GetUserAgent() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.userAgent
}

// FetchHomepage lấy thông tin session từ trang chủ
func (s *Session) FetchHomepage() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	
	// Sử dụng semaphore để giới hạn số lượng request đồng thời
	s.acquireSemaphore()
	defer s.releaseSemaphore()
	
	// Cập nhật timestamp
	s.mutex.Lock()
	s.LoginInfo.Timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	s.mutex.Unlock()

	// Tạo request với context
	req, err := http.NewRequestWithContext(ctx, "GET", s.BaseURL + "/Home/Index", nil)
	if err != nil {
		return err
	}

	// Thêm headers
	req.Header.Set("User-Agent", s.GetUserAgent())
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,fr-FR;q=0.8,fr;q=0.7,en-US;q=0.6,en;q=0.5")
	
	// Gửi request
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Chỉ hiển thị thông tin chi tiết trong chế độ verbose
	if s.Verbose {
		fmt.Println("Response status:", resp.Status)
	}

	// Đọc nội dung HTML để kiểm tra
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	bodyText := string(bodyBytes)
	
	// Chỉ hiển thị HTML preview nếu ở chế độ verbose
	if s.Verbose && len(bodyText) > 0 {
		previewLen := min(1000, len(bodyText))
		fmt.Printf("HTML Preview (first %d chars):\n%s\n", previewLen, bodyText[:previewLen])
	}

	// Lấy cookie PHPSESSID
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	for _, cookie := range resp.Cookies() {
		if s.Verbose {
			fmt.Printf("Cookie found: %s = %s\n", cookie.Name, cookie.Value)
		}
		if cookie.Name == "PHPSESSID" {
			s.LoginInfo.PHPSESSID = cookie.Value
			if s.Verbose {
				fmt.Println("Found PHPSESSID in cookies:", s.LoginInfo.PHPSESSID)
			}
		}
	}

	// Nếu không tìm thấy thông tin cần thiết trong phản hồi, sử dụng thông tin cố định
	if s.LoginInfo.PHPSESSID == "" {
		s.LoginInfo.PHPSESSID = "hauociktjsqv61hljlvvc48vo9"
		if s.Verbose {
			fmt.Println("Using fixed PHPSESSID:", s.LoginInfo.PHPSESSID)
		}
	}
	
	// Tìm RequestVerificationToken trong HTML sử dụng regexp
	if matches := verificationTokenRegex.FindStringSubmatch(bodyText); len(matches) > 1 {
		s.LoginInfo.RequestVerificationToken = matches[1]
		if s.Verbose {
			fmt.Println("Found RequestVerificationToken in HTML:", s.LoginInfo.RequestVerificationToken)
		}
	} else {
		s.LoginInfo.RequestVerificationToken = "yJ0xP1urbp_RxUNPGYpImmM4AnQztLk-_qVAitlH1rbYQ4GjYuq3kLCg1WHbURXVNeTcML0N9FmY2pNZQg0C0-QPJRA1:BARMFJW_zyYrgEgCC-vOTjMk1j4Q8pGo1WCYieiq6GyST1F2dps0B611m7wgXqlLcAvJKaryeJGr5bUqwnCAECq3b101"
		if s.Verbose {
			fmt.Println("Using fixed RequestVerificationToken:", s.LoginInfo.RequestVerificationToken)
		}
	}
	
	// Tìm UniqueSessionId trong HTML sử dụng regexp
	if matches := sessionIdRegex.FindStringSubmatch(bodyText); len(matches) > 1 {
		s.LoginInfo.UniqueSessionId = matches[1]
		if s.Verbose {
			fmt.Println("Found UniqueSessionId in HTML:", s.LoginInfo.UniqueSessionId)
		}
	} else {
		s.LoginInfo.UniqueSessionId = "TM1502723360721420288"
		if s.Verbose {
			fmt.Println("Using fixed UniqueSessionId:", s.LoginInfo.UniqueSessionId)
		}
	}
	
	// Sử dụng hoặc thiết lập FingerIDX
	if s.LoginInfo.FingerIDX == "" {
		s.LoginInfo.FingerIDX = "dce9c067016eb9a336274cc43e44fd70"
	}
	if s.Verbose {
		fmt.Println("Using FingerIDX:", s.LoginInfo.FingerIDX)
	}
	
	// Hiển thị IdyKey nếu có và ở chế độ verbose
	if s.LoginInfo.IdyKey != "" && s.Verbose {
		fmt.Println("Using IdyKey:", s.LoginInfo.IdyKey)
	}

	return nil
}

// GetSliderCaptcha gửi request để lấy thông tin captcha
func (s *Session) GetSliderCaptcha() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	
	// Sử dụng semaphore để giới hạn số lượng request đồng thời
	s.acquireSemaphore()
	defer s.releaseSemaphore()
	
	// Cập nhật timestamp
	s.mutex.Lock()
	s.LoginInfo.Timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	s.mutex.Unlock()

	// Tạo URL cho request captcha
	captchaURL := s.BaseURL + "/api/Verify/GetSliderCaptcha"

	// Tạo request với context
	req, err := http.NewRequestWithContext(ctx, "GET", captchaURL, nil)
	if err != nil {
		return "", err
	}

	// Thêm headers
	req.Header.Set("User-Agent", s.GetUserAgent())
	req.Header.Set("Content-Type", "application/json")
	
	s.mutex.Lock()
	req.Header.Set("RequestVerificationToken", s.LoginInfo.RequestVerificationToken)
	s.mutex.Unlock()

	// Gửi request
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Đọc body response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Trả về chuỗi JSON
	return string(bodyBytes), nil
}

// CheckSliderCaptcha gửi dữ liệu trail để xác minh captcha
func (s *Session) CheckSliderCaptcha(trailData []int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	
	// Sử dụng semaphore để giới hạn số lượng request đồng thời
	s.acquireSemaphore()
	defer s.releaseSemaphore()
	
	// Cập nhật timestamp
	s.mutex.Lock()
	s.LoginInfo.Timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	s.mutex.Unlock()

	// Tạo URL cho request xác minh
	verifyURL := s.BaseURL + "/api/Verify/CheckSliderCaptcha"

	// Tạo dữ liệu cho request
	requestData := struct {
		TrailData []int `json:"TrailData"`
	}{
		TrailData: trailData,
	}
	
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return "", err
	}

	// Tạo request với context
	req, err := http.NewRequestWithContext(ctx, "POST", verifyURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	// Thêm headers
	req.Header.Set("User-Agent", s.GetUserAgent())
	req.Header.Set("Content-Type", "application/json")
	
	s.mutex.Lock()
	req.Header.Set("RequestVerificationToken", s.LoginInfo.RequestVerificationToken)
	s.mutex.Unlock()

	// Gửi request
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Đọc body response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Cập nhật thời gian xác minh gần nhất
	s.mutex.Lock()
	s.lastVerifyTime = time.Now()
	s.mutex.Unlock()

	responseStr := string(bodyBytes)
	return responseStr, nil
}

// VerifyCaptchaIfNeeded xác minh captcha nếu cần
func (s *Session) VerifyCaptchaIfNeeded() (bool, error) {
	// Kiểm tra xem IdyKey hiện tại có hợp lệ không
	if s.IsIdyKeyValid() {
		if s.Verbose {
			fmt.Println("IdyKey is still valid, skipping captcha verification")
		}
		return false, nil // Không cần xác minh captcha
	}
	
	// Lấy và xác minh captcha
	if _, err := s.GetSliderCaptcha(); err != nil {
		return false, fmt.Errorf("error getting slider captcha: %w", err)
	}
	
	// Predefined trail data that works for verification
	trailData := []int{4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3, 3, 3, 5, 7, 7, 8, 8, 9, 9, 9, 9, 11, 11, 11, 11, 11, 11, 11}
	
	verifyResponse, err := s.CheckSliderCaptcha(trailData)
	if err != nil {
		return false, fmt.Errorf("error verifying slider captcha: %w", err)
	}
	
	var respData struct {
		Data struct {
			Message string `json:"Message"`
		} `json:"Data"`
	}
	
	if err := json.Unmarshal([]byte(verifyResponse), &respData); err != nil {
		return false, fmt.Errorf("error parsing verification response: %w", err)
	}
	
	if respData.Data.Message == "" {
		return false, fmt.Errorf("IdyKey not found in response")
	}
	
	// Cập nhật IdyKey trong LoginInfo
	s.mutex.Lock()
	s.LoginInfo.IdyKey = respData.Data.Message
	s.lastVerifyTime = time.Now()
	s.mutex.Unlock()
	
	return true, nil // Đã xác minh captcha thành công
}

// LoginRequest thực hiện đăng nhập với tên đăng nhập và mật khẩu
func (s *Session) LoginRequest(username, password string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	
	// Sử dụng semaphore để giới hạn số lượng request đồng thời
	s.acquireSemaphore()
	defer s.releaseSemaphore()
	
	// Xác minh captcha nếu cần
	if _, err := s.VerifyCaptchaIfNeeded(); err != nil {
		return "", fmt.Errorf("captcha verification failed: %w", err)
	}
	
	// Cập nhật timestamp
	s.mutex.Lock()
	s.LoginInfo.Timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	loginInfo := s.LoginInfo // Tạo bản sao để tránh deadlock
	s.mutex.Unlock()

	// Tạo URL đăng nhập
	loginURL := s.BaseURL + "/api/Authorize/SignIn"

	// Mã hóa mật khẩu base64
	encodedPassword := base64.StdEncoding.EncodeToString([]byte(password))

	// Ghi log các thông tin đăng nhập để debug - chỉ khi ở chế độ verbose
	if s.Verbose {
		fmt.Println("Login Details:")
		fmt.Printf("  AccountID: %s\n", username)
		fmt.Printf("  RequestVerificationToken: %s\n", loginInfo.RequestVerificationToken)
		fmt.Printf("  UniqueSessionId: %s\n", loginInfo.UniqueSessionId)
		fmt.Printf("  FingerIDX: %s\n", loginInfo.FingerIDX)
		fmt.Printf("  IdyKey: %s\n", loginInfo.IdyKey)
		fmt.Printf("  Timestamp: %d\n", loginInfo.Timestamp)
	}

	// Tạo dữ liệu đăng nhập sử dụng struct thay vì map
	type ElementInfo struct {
		Id         string                 `json:"Id"`
		Enabled    bool                   `json:"Enabled"`
		Visibility bool                   `json:"Visibility"`
		DisableMap map[string]interface{} `json:"DisableMap"`
	}
	
	type ElementManager struct {
		ElementMap map[string]ElementInfo `json:"ElementMap"`
	}
	
	type ProtectCodeModel struct {
		CellPhone                                    string `json:"CellPhone"`
		CaptchaCode                                  string `json:"CaptchaCode"`
		PWD                                          string `json:"PWD"`
		CountDownSecond                              int    `json:"CountDownSecond"`
		DefaultCountDownSecond                       int    `json:"DefaultCountDownSecond"`
		IsCaptchaSent                                bool   `json:"IsCaptchaSent"`
		IsCaptchaCodeVerified                        bool   `json:"IsCaptchaCodeVerified"`
		VerifiedEffectiveTime                        int    `json:"VerifiedEffectiveTime"`
		DefaultVerifiedEffectiveTime                 int    `json:"DefaultVerifiedEffectiveTime"`
		SendCaptchaCodeMsg                           string `json:"SendCaptchaCodeMsg"`
		SendCaptchaButtonName                        string `json:"SendCaptchaButtonName"`
		SendVerifyCodeCount                          int    `json:"SendVerifyCodeCount"`
		CallCustomerServiceCounts                    int    `json:"CallCustomerServiceCounts"`
		IsCallCustomerService                        bool   `json:"IsCallCustomerService"`
		IsServiceCallBackValid                       bool   `json:"IsServiceCallBackValid"`
		IsCanNotUseSMSProvider                       bool   `json:"IsCanNotUseSMSProvider"`
		CheckCellPhoneIsVerifiedOrOverLimitReturnMessage string `json:"CheckCellPhoneIsVerifiedOrOverLimitReturnMessage"`
	}
	
	type LoginRequest struct {
		AccountID                    string           `json:"AccountID"`
		AccountPWD                   string           `json:"AccountPWD"`
		ProtectCode                  string           `json:"ProtectCode"`
		LocalStorgeCookie           string           `json:"LocalStorgeCookie"`
		FingerIDX                    string           `json:"FingerIDX"`
		ScreenResolution            string           `json:"ScreenResolution"`
		ShowSliderCaptcha           bool             `json:"ShowSliderCaptcha"`
		ShowPhoneVerify             bool             `json:"ShowPhoneVerify"`
		VerifySliderCaptcha         bool             `json:"VerifySliderCaptcha"`
		CellPhone                    string           `json:"CellPhone"`
		ProtectCodeCellPhone        string           `json:"ProtectCodeCellPhone"`
		IsCellPhoneValid            bool             `json:"IsCellPhoneValid"`
		IdyKey                       string           `json:"IdyKey"`
		CaptchaCode                  string           `json:"CaptchaCode"`
		LoginVerification           int              `json:"LoginVerification"`
		IsLobbyProtect              bool             `json:"IsLobbyProtect"`
		SignInOverLimitIsRefreshPage bool             `json:"SignInOverLimitIsRefreshPage"`
		UniqueSessionId             string           `json:"UniqueSessionId"`
		ElementManager              ElementManager   `json:"ElementManager"`
		ProtectCodeModel            ProtectCodeModel `json:"ProtectCodeModel"`
		DepositNewsModel            []string         `json:"DepositNewsModel"`
	}
	
	// Tạo element map với các thành phần cần thiết
	elementMap := map[string]ElementInfo{
		"ProtectCodeSendCaptchaButton": {
			Id:         "ProtectCodeSendCaptchaButton",
			Enabled:    true,
			Visibility: true,
			DisableMap: map[string]interface{}{},
		},
		"ProtectCodeLoginButton": {
			Id:         "ProtectCodeLoginButton",
			Enabled:    false,
			Visibility: true,
			DisableMap: map[string]interface{}{},
		},
		"VerifiedEffectiveTime": {
			Id:         "VerifiedEffectiveTime",
			Enabled:    true,
			Visibility: true,
			DisableMap: map[string]interface{}{
				"VerifyCaptchaChange": true,
			},
		},
		"signin": {
			Id:         "signin",
			Enabled:    true,
			Visibility: true,
			DisableMap: map[string]interface{}{
				"DoSignIn": true,
			},
		},
		"ProtectCodeCaptchaCode": {
			Id:         "ProtectCodeCaptchaCode",
			Enabled:    true,
			Visibility: true,
			DisableMap: map[string]interface{}{},
		},
	}
	
	// Tạo struct request đầy đủ
	requestData := LoginRequest{
		AccountID:                    username,
		AccountPWD:                   encodedPassword,
		ProtectCode:                  "",
		LocalStorgeCookie:           "",
		FingerIDX:                    loginInfo.FingerIDX,
		ScreenResolution:            "1920*1080",
		ShowSliderCaptcha:           true,
		ShowPhoneVerify:             false,
		VerifySliderCaptcha:         true,
		CellPhone:                    "",
		ProtectCodeCellPhone:        "",
		IsCellPhoneValid:            false,
		IdyKey:                       loginInfo.IdyKey,
		CaptchaCode:                  "",
		LoginVerification:           1,
		IsLobbyProtect:              false,
		SignInOverLimitIsRefreshPage: false,
		UniqueSessionId:             loginInfo.UniqueSessionId,
		ElementManager: ElementManager{
			ElementMap: elementMap,
		},
		ProtectCodeModel: ProtectCodeModel{
			CellPhone:                    "",
			CaptchaCode:                  "",
			PWD:                          "",
			CountDownSecond:              -1,
			DefaultCountDownSecond:       30,
			IsCaptchaSent:                false,
			IsCaptchaCodeVerified:        false,
			VerifiedEffectiveTime:        10,
			DefaultVerifiedEffectiveTime: 10,
			SendCaptchaCodeMsg:           "",
			SendCaptchaButtonName:        "loading",
			SendVerifyCodeCount:          0,
			CallCustomerServiceCounts:    0,
			IsCallCustomerService:        false,
			IsServiceCallBackValid:       false,
			IsCanNotUseSMSProvider:       false,
			CheckCellPhoneIsVerifiedOrOverLimitReturnMessage: "",
		},
		DepositNewsModel:            []string{},
	}
	
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return "", err
	}

	// Chỉ log request data trong chế độ verbose
	if s.Verbose {
		fmt.Println("Login request data:", string(jsonData))
	}

	// Tạo request với context
	req, err := http.NewRequestWithContext(ctx, "POST", loginURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	// Thêm headers
	req.Header.Set("User-Agent", s.GetUserAgent())
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,fr-FR;q=0.8,fr;q=0.7,en-US;q=0.6,en;q=0.5")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("RequestVerificationToken", loginInfo.RequestVerificationToken)
	req.Header.Set("UniqueTick", fmt.Sprintf("%d", loginInfo.Timestamp))

	// Gửi request
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Đọc body response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	responseStr := string(bodyBytes)
	if s.Verbose {
		fmt.Println("Login response:", responseStr)
	}

	return responseStr, nil
}

// ForceUpdateSession cập nhật thông tin session mới
func (s *Session) ForceUpdateSession() error {
	return s.FetchHomepage()
}

// min trả về giá trị nhỏ hơn trong hai giá trị
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// calculateFingerIDX tính toán fingerIDX từ userAgent và url
func calculateFingerIDX(userAgent, url string) string {
	// Đây chỉ là một ví dụ đơn giản, không phải cách tính thực tế
	// combined := userAgent + url (bỏ dòng này vì không sử dụng)
	result := "dce9c067016eb9a336274cc43e44fd70" // Giá trị mặc định
	
	return result
}

// SetIdyKeyTTL thiết lập thời gian sống của IdyKey
func (s *Session) SetIdyKeyTTL(ttl time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.idyKeyTTL = ttl
}

// SetMaxConcurrent thiết lập số lượng request đồng thời tối đa
func (s *Session) SetMaxConcurrent(max int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.maxConcurrent = max
	s.semaphore = make(chan struct{}, max)
}

// acquireSemaphore lấy một slot trong semaphore
func (s *Session) acquireSemaphore() {
	if s.semaphore != nil {
		s.semaphore <- struct{}{}
	}
}

// releaseSemaphore giải phóng một slot trong semaphore
func (s *Session) releaseSemaphore() {
	if s.semaphore != nil {
		<-s.semaphore
	}
} 