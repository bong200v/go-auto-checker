package workers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"Go_auto_checker/internal/models"
	"Go_auto_checker/internal/session"
)

// Màu sắc cho terminal
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// WorkerConfig cấu hình cho mỗi worker
type WorkerConfig struct {
	Username    string
	Password    string
	WorkerId    int
	IdyKeyTTL   time.Duration
	MaxRetries  int
	Verbose     bool
	SaveResults bool
	ResultsDir  string
	ExtraInfo   string // Thông tin bổ sung về worker, ví dụ: dòng trong Excel
}

// WorkerResult kết quả của mỗi worker
type WorkerResult struct {
	WorkerId     int
	Username     string
	Password     string       // Mật khẩu của tài khoản
	Success      bool
	AccountID    float64
	Nickname     string
	ErrorMessage string
	Duration     time.Duration
	StartTime    time.Time
	EndTime      time.Time
	ExtraInfo    string       // Thông tin bổ sung
}

// WorkerStats thống kê về worker
type WorkerStats struct {
	TotalWorkers  int
	ActiveWorkers int32
	Completed     int32
	Successful    int32
	Failed        int32
	StartTime     time.Time
	mutex         sync.Mutex
}

// Tạo danh sách các User Agent hiện đại cho năm 2025
var modernUserAgents = []string{
	// Chrome cho Windows (phiên bản 125-132)
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	
	// Chrome cho macOS (phiên bản 125-132)
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	
	// Chrome cho Linux
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	
	// Firefox cho Windows (phiên bản 127-132)
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:132.0) Gecko/20100101 Firefox/132.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:130.0) Gecko/20100101 Firefox/130.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:128.0) Gecko/20100101 Firefox/128.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:127.0) Gecko/20100101 Firefox/127.0",
	
	// Firefox cho macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:132.0) Gecko/20100101 Firefox/132.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:130.0) Gecko/20100101 Firefox/130.0",
	
	// Safari cho macOS 
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/615.1.50 (KHTML, like Gecko) Version/17.5 Safari/615.1.50",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/614.2.25 (KHTML, like Gecko) Version/17.4 Safari/614.2.25",
	
	// Edge cho Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36 Edg/132.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.0.0",
	
	// Chrome cho Android
	"Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Mobile Safari/537.36",
	
	// Safari cho iOS
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/615.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/615.1.15",
	"Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X) AppleWebKit/615.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/615.1.15",
}

// Lấy User Agent ngẫu nhiên hiện đại (2025)
func getRandomModernUserAgent() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return modernUserAgents[r.Intn(len(modernUserAgents))]
}

// Thông tin thiết bị giả lập
type DeviceInfo struct {
	UserAgent     string
	Platform      string
	ScreenWidth   int
	ScreenHeight  int
	FingerIDX     string
}

// Tạo thông tin thiết bị ngẫu nhiên
func generateRandomDeviceInfo() DeviceInfo {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	
	// Sử dụng User Agent hiện đại (2025)
	userAgent := getRandomModernUserAgent()
	
	// Tạo thông tin màn hình ngẫu nhiên
	var screenWidth, screenHeight int
	if r.Intn(2) == 0 {
		// Desktop resolution
		screenWidths := []int{1366, 1440, 1536, 1920, 2560}
		screenHeights := []int{768, 900, 864, 1080, 1440}
		idx := r.Intn(len(screenWidths))
		screenWidth = screenWidths[idx]
		screenHeight = screenHeights[idx]
	} else {
		// Mobile resolution
		screenWidths := []int{375, 390, 412, 414, 428}
		screenHeights := []int{667, 844, 915, 896, 926}
		idx := r.Intn(len(screenWidths))
		screenWidth = screenWidths[idx]
		screenHeight = screenHeights[idx]
	}
	
	// Xác định platform từ UserAgent
	var platform string
	if strings.Contains(userAgent, "Windows") {
		platform = "Windows"
	} else if strings.Contains(userAgent, "Mac OS") {
		platform = "MacOS"
	} else if strings.Contains(userAgent, "Linux") {
		platform = "Linux"
	} else {
		platform = "Unknown"
	}
	
	// Tạo FingerIDX ngẫu nhiên - chuỗi hex 32 ký tự
	fingerIDX := generateRandomHex(32)
	
	return DeviceInfo{
		UserAgent:    userAgent,
		Platform:     platform,
		ScreenWidth:  screenWidth,
		ScreenHeight: screenHeight,
		FingerIDX:    fingerIDX,
	}
}

// Tạo chuỗi hex ngẫu nhiên với độ dài n
func generateRandomHex(n int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	const hexChars = "0123456789abcdef"
	result := make([]byte, n)
	for i := range result {
		result[i] = hexChars[r.Intn(len(hexChars))]
	}
	return string(result)
}

// Tạo worker stats mới
func NewWorkerStats(totalWorkers int) *WorkerStats {
	return &WorkerStats{
		TotalWorkers: totalWorkers,
		StartTime:    time.Now(),
	}
}

// Đánh dấu một worker đã bắt đầu
func (s *WorkerStats) WorkerStarted() {
	atomic.AddInt32(&s.ActiveWorkers, 1)
}

// Đánh dấu một worker đã hoàn thành
func (s *WorkerStats) WorkerCompleted(success bool) {
	atomic.AddInt32(&s.Completed, 1)
	atomic.AddInt32(&s.ActiveWorkers, -1)
	
	if success {
		atomic.AddInt32(&s.Successful, 1)
	} else {
		atomic.AddInt32(&s.Failed, 1)
	}
}

// In ra thông báo với màu sắc
func printSuccess(message string, workerId int) {
	fmt.Printf("[%d] %s%s✓ %s%s\n", workerId, colorBold, colorGreen, message, colorReset)
}

func printError(message string, workerId int) {
	fmt.Printf("[%d] %s%s✗ %s%s\n", workerId, colorBold, colorRed, message, colorReset)
}

func printInfo(message string, workerId int) {
	fmt.Printf("[%d] %s%s➜ %s%s\n", workerId, colorBold, colorCyan, message, colorReset)
}

func printHeader(message string, workerId int) {
	fmt.Printf("\n[%d] %s%s=== %s ===%s\n", workerId, colorBold, colorYellow, message, colorReset)
}

// RunWorker chạy một worker riêng biệt
func RunWorker(config WorkerConfig, wg *sync.WaitGroup, stats *WorkerStats) *WorkerResult {
	defer wg.Done()
	if stats != nil {
		stats.WorkerStarted()
		defer func() {
			stats.WorkerCompleted(false) // mặc định là thất bại, sẽ cập nhật nếu thành công
		}()
	}
	
	// Khởi tạo kết quả
	result := &WorkerResult{
		WorkerId:  config.WorkerId,
		Username:  config.Username,
		Password:  config.Password,
		Success:   false,
		StartTime: time.Now(),
		ExtraInfo: config.ExtraInfo,
	}
	
	// Tạo thông tin thiết bị ngẫu nhiên
	deviceInfo := generateRandomDeviceInfo()
	
	// Tạo session riêng cho mỗi worker
	s := session.New()
	s.SetVerbose(config.Verbose)
	s.SetIdyKeyTTL(config.IdyKeyTTL)
	s.SetMaxConcurrent(1) // Mỗi worker chỉ dùng tối đa 1 connection
	
	// Thiết lập UserAgent và thông tin phiên
	s.SetUserAgent(deviceInfo.UserAgent)
	sessionInfo := s.GetLoginInfo()
	sessionInfo.FingerIDX = deviceInfo.FingerIDX
	s.SetLoginInfo(sessionInfo)
	
	// Hiển thị thông tin worker
	printHeader("WORKER STARTED", config.WorkerId)
	printInfo(fmt.Sprintf("Starting worker for username: %s", config.Username), config.WorkerId)
	if config.ExtraInfo != "" {
		printInfo(fmt.Sprintf("Extra info: %s", config.ExtraInfo), config.WorkerId)
	}
	printInfo(fmt.Sprintf("Device: %s [%dx%d]", deviceInfo.Platform, deviceInfo.ScreenWidth, deviceInfo.ScreenHeight), config.WorkerId)
	
	// Fetch session information
	printInfo("Fetching session information...", config.WorkerId)
	if err := s.FetchHomepage(); err != nil {
		errMsg := fmt.Sprintf("Error fetching homepage: %v", err)
		printError(errMsg, config.WorkerId)
		
		result.Success = false
		result.ErrorMessage = errMsg
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		
		if stats != nil {
			stats.WorkerCompleted(false)
		}
		return result
	}
	printSuccess("Session information retrieved", config.WorkerId)
	
	// Verify captcha if needed
	if !s.IsIdyKeyValid() {
		printHeader("CAPTCHA VERIFICATION", config.WorkerId)
		verifyCaptchaForWorker(s, config.WorkerId)
	} else {
		printSuccess("Using existing valid IdyKey", config.WorkerId)
	}
	
	// Login
	printHeader("LOGIN", config.WorkerId)
	printInfo(fmt.Sprintf("Attempting to login with username: %s", config.Username), config.WorkerId)
	
	// Try to log in
	loginStartTime := time.Now()
	response, err := s.LoginRequest(config.Username, config.Password)
	if err != nil {
		errMsg := fmt.Sprintf("Login error: %v", err)
		printError(errMsg, config.WorkerId)
		
		result.Success = false
		result.ErrorMessage = errMsg
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		
		if stats != nil {
			stats.WorkerCompleted(false)
		}
		return result
	}
	
	// Parse login response
	var loginResponse map[string]interface{}
	if err := json.Unmarshal([]byte(response), &loginResponse); err != nil {
		errMsg := fmt.Sprintf("Error parsing login response: %v", err)
		printError(errMsg, config.WorkerId)
		
		result.Success = false
		result.ErrorMessage = errMsg
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		
		if stats != nil {
			stats.WorkerCompleted(false)
		}
		return result
	}
	
	// Kiểm tra kết quả đăng nhập
	if data, ok := loginResponse["Data"].(map[string]interface{}); ok && data != nil {
		// Nếu có AccountID và NickName trong Data, coi như đăng nhập thành công
		if accountID, ok := data["AccountID"].(float64); ok && accountID > 0 {
			printSuccess("Login successful!", config.WorkerId)
			fmt.Printf("[%d] %sAccount ID:%s %.0f\n", config.WorkerId, colorBlue, colorReset, accountID)
			
			var nickname string
			if nickVal, ok := data["NickName"].(string); ok {
				nickname = nickVal
				fmt.Printf("[%d] %sNickname:%s %s\n", config.WorkerId, colorBlue, colorReset, nickname)
			}
			
			if cookieID, ok := data["CookieID"].(string); ok {
				fmt.Printf("[%d] %sCookie ID received%s (length: %d)\n", config.WorkerId, colorBlue, colorReset, len(cookieID))
			}
			
			// Update result
			result.Success = true
			result.AccountID = accountID
			result.Nickname = nickname
			
			// Save results if needed
			if config.SaveResults {
				saveWorkerResults(s.GetLoginInfo(), result, config)
			}
		} else {
			errMsg := "Login failed! No valid AccountID found in response."
			printError(errMsg, config.WorkerId)
			
			result.Success = false
			result.ErrorMessage = errMsg
		}
	} else if errInfo, ok := loginResponse["Error"].(map[string]interface{}); ok && errInfo != nil {
		// Xử lý lỗi
		errMsg := "Login failed!"
		printError(errMsg, config.WorkerId)
		
		if errVal, ok := errInfo["Message"].(string); ok {
			fmt.Printf("[%d] %sError message:%s %s\n", config.WorkerId, colorRed, colorReset, errVal)
			errMsg = errVal
		}
		
		result.Success = false
		result.ErrorMessage = errMsg
	} else {
		errMsg := "Login failed! Unexpected response format."
		printError(errMsg, config.WorkerId)
		
		result.Success = false
		result.ErrorMessage = errMsg
	}
	
	// Kết thúc worker
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	
	loginDuration := time.Since(loginStartTime)
	fmt.Printf("[%d] %sTotal duration:%s %v (Login: %v)\n", config.WorkerId, colorPurple, colorReset, result.Duration, loginDuration)
	
	if stats != nil {
		stats.WorkerCompleted(result.Success)
	}
	
	return result
}

// Verify captcha for worker
func verifyCaptchaForWorker(s *session.Session, workerId int) {
	printInfo("Getting slider captcha...", workerId)
	_, err := s.GetSliderCaptcha()
	if err != nil {
		printError(fmt.Sprintf("Error getting slider captcha: %v", err), workerId)
		return
	}
	
	// Predefined trail data that works for verification
	trailData := []int{4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3, 3, 3, 5, 7, 7, 8, 8, 9, 9, 9, 9, 11, 11, 11, 11, 11, 11, 11}
	printInfo("Verifying slider captcha...", workerId)
	
	verifyResponse, err := s.CheckSliderCaptcha(trailData)
	if err != nil {
		printError(fmt.Sprintf("Error verifying slider captcha: %v", err), workerId)
		return
	}
	
	var respData map[string]interface{}
	if err := json.Unmarshal([]byte(verifyResponse), &respData); err != nil {
		printError(fmt.Sprintf("Error parsing verification response: %v", err), workerId)
		return
	}
	
	// Xử lý response và cập nhật IdyKey
	if data, ok := respData["Data"].(map[string]interface{}); ok {
		if idyKey, ok := data["Message"].(string); ok {
			printSuccess(fmt.Sprintf("Captcha verified successfully! IdyKey: %s...", idyKey[:5]), workerId)
			// Cập nhật sessionInfo
			sessionInfo := s.GetLoginInfo()
			sessionInfo.IdyKey = idyKey
			s.SetLoginInfo(sessionInfo)
		} else {
			printError("IdyKey not found in response", workerId)
		}
	} else {
		printError("Unexpected response format", workerId)
	}
}

// Save worker results to file
func saveWorkerResults(sessionInfo models.SessionInfo, result *WorkerResult, config WorkerConfig) {
	// Create base results directory if not exists
	resultsDir := config.ResultsDir
	if resultsDir == "" {
		resultsDir = "results"
	}
	
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		printError(fmt.Sprintf("Error creating results directory: %v", err), config.WorkerId)
		return
	}
	
	// Thông báo kết quả xử lý tài khoản
	if result.Success {
		printSuccess(fmt.Sprintf("Đăng nhập thành công với tài khoản: %s (ID: %d)", result.Username, int(result.AccountID)), config.WorkerId)
	} else {
		printError(fmt.Sprintf("Đăng nhập thất bại với tài khoản: %s (Lỗi: %s)", result.Username, result.ErrorMessage), config.WorkerId)
	}
}

// RunMultipleWorkers chạy nhiều workers cùng lúc
func RunMultipleWorkers(configs []WorkerConfig) []*WorkerResult {
	workerCount := len(configs)
	results := make([]*WorkerResult, workerCount)
	var wg sync.WaitGroup
	wg.Add(workerCount)
	
	// Tạo stats để theo dõi
	stats := NewWorkerStats(workerCount)
	
	fmt.Printf("\n%s%s=== Starting %d workers ===%s\n\n", colorBold, colorPurple, workerCount, colorReset)
	
	// Start all workers concurrently
	for i, config := range configs {
		go func(idx int, cfg WorkerConfig) {
			results[idx] = RunWorker(cfg, &wg, stats)
		}(i, config)
	}
	
	// Wait for all workers to complete
	wg.Wait()
	
	// Hiển thị kết quả tổng hợp
	displaySummary(results, stats)
	
	return results
}

// Hiển thị tổng kết sau khi tất cả workers hoàn thành
func displaySummary(results []*WorkerResult, stats *WorkerStats) {
	fmt.Printf("\n%s%s=== Summary ===%s\n", colorBold, colorPurple, colorReset)
	fmt.Printf("Total workers: %d\n", stats.TotalWorkers)
	fmt.Printf("Successful: %d\n", stats.Successful)
	fmt.Printf("Failed: %d\n", stats.Failed)
	
	totalDuration := time.Since(stats.StartTime)
	fmt.Printf("Total duration: %v\n", totalDuration)
	
	// Hiển thị chi tiết kết quả của từng worker
	fmt.Printf("\n%s%s=== Worker Details ===%s\n", colorBold, colorPurple, colorReset)
	for _, result := range results {
		if result.Success {
			fmt.Printf("[%d] %s%s✓%s Username: %s, AccountID: %.0f, Duration: %v\n", 
				result.WorkerId, colorBold, colorGreen, colorReset,
				result.Username, result.AccountID, result.Duration)
		} else {
			fmt.Printf("[%d] %s%s✗%s Username: %s, Error: %s, Duration: %v\n", 
				result.WorkerId, colorBold, colorRed, colorReset,
				result.Username, result.ErrorMessage, result.Duration)
		}
	}
	
	fmt.Printf("\n%s%s=== End of Summary ===%s\n", colorBold, colorPurple, colorReset)
} 