package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

// AppState manages the application's state
type AppState struct {
	mutex         sync.Mutex
	Running       bool
	StartTime     time.Time
	TotalAccounts int
	ProcessedCount int
	SuccessCount  int
	FailedCount   int
	CurrentBatch  int
	TotalBatches  int
	CurrentFile   string
	Duration      string
	SessionTimestamp string
}

// NewAppState initializes a new application state
func NewAppState() *AppState {
	return &AppState{
		Running:       false,
		TotalAccounts: 0,
		ProcessedCount: 0,
		SuccessCount:  0,
		FailedCount:   0,
		CurrentBatch:  0,
		TotalBatches:  0,
		Duration:      "0s",
	}
}

// Account represents an account with username and password
type Account struct {
	Username string
	Password string
	Row      int
	ExtraFields []string
}

// BatchResult represents the result of processing an account
type BatchResult struct {
	Account    Account
	Success    bool
	LogMessage string
}

func main() {
	// Create necessary directories
	os.MkdirAll("uploads", os.ModePerm)
	os.MkdirAll("results/success", os.ModePerm)
	os.MkdirAll("results/failed", os.ModePerm)

	// Cleanup old files with timestamps in success and failed directories
	cleanupOldFiles()

	// Initialize app state
	appState := NewAppState()

	// Set up Gin router
	r := gin.Default()
	r.LoadHTMLGlob("templates/*.html")
	
	// Phục vụ các file tĩnh
	r.Static("/css", "templates/css")
	r.Static("/js", "templates/js")
	
	// Set maximum multipart memory
	r.MaxMultipartMemory = 8 << 20 // 8 MiB

	// Routes
	r.GET("/", func(c *gin.Context) {
		appState.mutex.Lock()
		defer appState.mutex.Unlock()
		
		c.HTML(http.StatusOK, "index.html", gin.H{
			"Title":         "B1 Account Checker",
			"Running":       appState.Running,
			"TotalAccounts": appState.TotalAccounts,
			"SuccessCount":  appState.SuccessCount,
			"FailedCount":   appState.FailedCount,
			"Duration":      appState.Duration,
		})
	})

	// Status endpoint
	r.GET("/status", func(c *gin.Context) {
		appState.mutex.Lock()
		defer appState.mutex.Unlock()
		
		if appState.Running {
			appState.Duration = time.Since(appState.StartTime).Round(time.Second).String()
		}
		
		c.JSON(http.StatusOK, appState)
	})

	// Upload endpoint
	r.POST("/upload", func(c *gin.Context) {
		appState.mutex.Lock()
		if appState.Running {
			appState.mutex.Unlock()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot upload while processing is running"})
			return
		}
		appState.mutex.Unlock()

		file, err := c.FormFile("excel_file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
			return
		}

		// Check file extension
		if !strings.HasSuffix(file.Filename, ".xlsx") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Only .xlsx files are supported"})
			return
		}

		// Save the file
		dst := filepath.Join("uploads", file.Filename)
		if err := c.SaveUploadedFile(file, dst); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"file": file.Filename})
	})

	// Notify completed endpoint - để thông báo khi xử lý hoàn tất
	r.GET("/notify-completed", func(c *gin.Context) {
		timestamp := c.Query("timestamp")
		success := c.Query("success")
		failed := c.Query("failed")
		
		log.Printf("Nhận thông báo đã hoàn thành xử lý: timestamp=%s, success=%s, failed=%s", 
			timestamp, success, failed)
		
		// Trả về thông báo thành công
		c.JSON(http.StatusOK, gin.H{
			"status": "completed",
			"timestamp": timestamp,
			"success": success,
			"failed": failed,
		})
	})

	// Start processing endpoint
	r.POST("/start", func(c *gin.Context) {
		appState.mutex.Lock()
		if appState.Running {
			appState.mutex.Unlock()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Processing is already running"})
			return
		}
		appState.mutex.Unlock()

		excelFile := c.PostForm("excel_file")
		if excelFile == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No Excel file selected"})
			return
		}

		maxWorkers, err := strconv.Atoi(c.PostForm("max_workers"))
		if err != nil || maxWorkers < 1 {
			maxWorkers = 10 // Default value
		}

		startRow, err := strconv.Atoi(c.PostForm("start_row"))
		if err != nil || startRow < 2 {
			startRow = 2 // Default value
		}

		saveResults := c.PostForm("save_results") == "on"

		filePath := filepath.Join("uploads", excelFile)
		
		// Start processing in a goroutine
		go startProcessing(appState, filePath, maxWorkers, startRow, saveResults)

		c.JSON(http.StatusOK, gin.H{"message": "Processing started"})
	})

	// Stop processing endpoint
	r.POST("/stop", func(c *gin.Context) {
		appState.mutex.Lock()
		defer appState.mutex.Unlock()
		
		if !appState.Running {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No processing is running"})
			return
		}
		
		appState.Running = false
		c.JSON(http.StatusOK, gin.H{"message": "Processing stopped"})
	})

	// List uploaded files
	r.GET("/files", func(c *gin.Context) {
		files, err := os.ReadDir("uploads")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read uploads directory"})
			return
		}

		var fileList []string
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".xlsx") {
				fileList = append(fileList, file.Name())
			}
		}

		c.JSON(http.StatusOK, fileList)
	})

	// List result files
	r.GET("/results", func(c *gin.Context) {
		successFiles, _ := os.ReadDir("results/success")
		failedFiles, _ := os.ReadDir("results/failed")

		var successList, failedList []string
		for _, file := range successFiles {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".xlsx") {
				successList = append(successList, file.Name())
			}
		}

		for _, file := range failedFiles {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".xlsx") {
				failedList = append(failedList, file.Name())
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": successList,
			"failed":  failedList,
		})
	})

	// Download endpoint
	r.GET("/download/:type/:file", func(c *gin.Context) {
		fileType := c.Param("type")
		file := c.Param("file")
		
		// Tham số để đổi tên file khi tải xuống
		downloadName := c.Query("download")
		
		var filePath string
		if fileType == "success" {
			filePath = filepath.Join("results", "success", file)
		} else if fileType == "failed" {
			filePath = filepath.Join("results", "failed", file)
		} else {
			filePath = filepath.Join("results", file)
		}
		
		// Kiểm tra nếu file tồn tại
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
			return
		}
		
		// Tên file khi tải xuống
		filename := file
		if downloadName != "" {
			filename = downloadName
		}
		
		// Thiết lập header để trình duyệt hiển thị hộp thoại tải xuống
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Transfer-Encoding", "binary")
		c.Header("Content-Disposition", "attachment; filename="+filename)
		
		// Trả về file
		c.File(filePath)
	})

	// Start the server
	fmt.Println("B1 Account Checker Web Interface starting at http://localhost:8080")
	r.Run(":8080")
}

// startProcessing handles the account processing logic
func startProcessing(appState *AppState, filePath string, maxWorkers, startRow int, saveResults bool) {
	// Tạo timestamp cho phiên theo giờ Việt Nam (UTC+7)
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		log.Printf("Không thể load location Asia/Ho_Chi_Minh, sử dụng UTC+7 cố định: %v", err)
		loc = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60) // UTC+7
	}
	sessionTime := time.Now().In(loc)
	timestamp := sessionTime.Format("20060102_150405")
	
	log.Printf("Bắt đầu xử lý với timestamp: %s (giờ Việt Nam)", timestamp)

	appState.mutex.Lock()
	appState.Running = true
	appState.StartTime = sessionTime
	appState.TotalAccounts = 0
	appState.ProcessedCount = 0
	appState.SuccessCount = 0
	appState.FailedCount = 0
	appState.CurrentBatch = 0
	appState.TotalBatches = 0
	appState.CurrentFile = filepath.Base(filePath)
	appState.Duration = "0s"
	// Lưu timestamp phiên vào AppState
	appState.SessionTimestamp = timestamp
	appState.mutex.Unlock()

	// Open Excel file
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		log.Printf("Error opening Excel file: %v", err)
		appState.mutex.Lock()
		appState.Running = false
		appState.mutex.Unlock()
		return
	}
	defer f.Close()

	// Get the first sheet
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		log.Println("No sheets found in the Excel file")
		appState.mutex.Lock()
		appState.Running = false
		appState.mutex.Unlock()
		return
	}
	
	sheetName := sheets[0]
	rows, err := f.GetRows(sheetName)
	if err != nil {
		log.Printf("Error reading rows: %v", err)
		appState.mutex.Lock()
		appState.Running = false
		appState.mutex.Unlock()
		return
	}

	// Extract accounts from Excel
	var accounts []Account
	for i := startRow - 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) >= 3 { // Cần ít nhất 3 cột (1: STT, 2: Tài khoản, 3: Mật khẩu)
			// Lấy username từ cột 2 và password từ cột 3
			username := strings.TrimSpace(row[1]) // Cột 2 (index 1)
			password := strings.TrimSpace(row[2]) // Cột 3 (index 2)
			
			// Tạo đối tượng Account
			acc := Account{
				Username: username,
				Password: password,
				Row:      i + 1, // 1-indexed row number
			}
			
			// Lưu các trường thông tin bổ sung từ các cột khác (bao gồm cả cột 1 - STT)
			extraFields := make([]string, 0, len(row)-2)
			
			// Thêm cột 1 (STT) vào extraFields
			extraFields = append(extraFields, strings.TrimSpace(row[0]))
			
			// Thêm các cột từ cột 4 trở đi (nếu có)
			if len(row) > 3 {
				for j := 3; j < len(row); j++ {
					extraFields = append(extraFields, strings.TrimSpace(row[j]))
				}
			}
			
			acc.ExtraFields = extraFields
			accounts = append(accounts, acc)
		}
	}

	appState.mutex.Lock()
	appState.TotalAccounts = len(accounts)
	appState.TotalBatches = (len(accounts) + maxWorkers - 1) / maxWorkers
	appState.mutex.Unlock()

	// Lưu tất cả kết quả cho cuối cùng
	var allResults []BatchResult

	// Process accounts in batches
	for i := 0; i < len(accounts); i += maxWorkers {
		// Check if processing was stopped
		appState.mutex.Lock()
		if !appState.Running {
			appState.mutex.Unlock()
			break
		}
		
		appState.CurrentBatch++
		appState.mutex.Unlock()

		end := i + maxWorkers
		if end > len(accounts) {
			end = len(accounts)
		}
		
		batchAccounts := accounts[i:end]
		var wg sync.WaitGroup
		var batchSuccessCount, batchFailedCount int
		var batchMutex sync.Mutex

		// Store results from this batch
		batchResults := make([]BatchResult, len(batchAccounts))

		// Process each account in the batch concurrently
		for j, account := range batchAccounts {
			wg.Add(1)
			go func(j int, acc Account) {
				defer wg.Done()

				// Simulate login process
				time.Sleep(time.Second * time.Duration(1+j%3)) // Simulate variable processing time
				
				// Simulate success/failure (80% success rate for demo)
				success := j%5 != 0
				logMessage := ""
				
				if success {
					logMessage = fmt.Sprintf("Successfully logged in with account %s", acc.Username)
					batchMutex.Lock()
					batchSuccessCount++
					batchMutex.Unlock()
				} else {
					logMessage = fmt.Sprintf("Failed to login with account %s: Wrong username or password", acc.Username)
					batchMutex.Lock()
					batchFailedCount++
					batchMutex.Unlock()
				}
				
				batchResults[j] = BatchResult{
					Account:    acc,
					Success:    success,
					LogMessage: logMessage,
				}
			}(j, account)
		}

		wg.Wait()

		// Update app state with batch results
		appState.mutex.Lock()
		appState.ProcessedCount += len(batchAccounts)
		appState.SuccessCount += batchSuccessCount
		appState.FailedCount += batchFailedCount
		appState.Duration = time.Since(appState.StartTime).Round(time.Second).String()
		
		// Check if processing was stopped
		if !appState.Running {
			appState.mutex.Unlock()
			break
		}
		appState.mutex.Unlock()

		// Lưu kết quả của batch hiện tại vào danh sách tổng hợp
		allResults = append(allResults, batchResults...)

		// Update processed_success.txt and processed_failed.txt
		updateTextResults(batchResults)
	}

	// Lưu tất cả kết quả vào file Excel khi hoàn thành
	if saveResults && len(allResults) > 0 {
		updateExcelResults(allResults, saveResults)
	}

	// Cập nhật trạng thái khi hoàn thành xử lý
	appState.mutex.Lock()
	appState.Running = false
	appState.Duration = time.Since(appState.StartTime).Round(time.Second).String()
	log.Printf("Đã hoàn thành xử lý với timestamp %s, thời gian: %s, thành công: %d, thất bại: %d", 
		appState.SessionTimestamp, appState.Duration, appState.SuccessCount, appState.FailedCount)
	appState.mutex.Unlock()

	// Gửi thông báo hoàn thành xử lý đến trang web
	go func() {
		// Đợi 1 giây để đảm bảo dữ liệu đã được cập nhật đầy đủ
		time.Sleep(time.Second)
		
		// Gửi request thông báo hoàn thành
		completedURL := fmt.Sprintf("http://localhost:8080/notify-completed?timestamp=%s&success=%d&failed=%d", 
			appState.SessionTimestamp, appState.SuccessCount, appState.FailedCount)
		_, err := http.Get(completedURL)
		if err != nil {
			log.Printf("Lỗi khi gửi thông báo hoàn thành: %v", err)
		} else {
			log.Printf("Đã gửi thông báo hoàn thành xử lý đến trang web")
		}
	}()
}

// updateExcelResults lưu kết quả vào file Excel
func updateExcelResults(results []BatchResult, saveResults bool) {
	if !saveResults {
		return
	}

	// Tạo thư mục results nếu chưa tồn tại
	os.MkdirAll("results/success", 0755)
	os.MkdirAll("results/failed", 0755)

	// Tạo danh sách tài khoản thành công và thất bại
	var successResults []BatchResult
	var failedResults []BatchResult

	for _, result := range results {
		if result.Success {
			successResults = append(successResults, result)
		} else {
			failedResults = append(failedResults, result)
		}
	}

	// Đường dẫn file cố định
	successFilePath := "results/success/b1_success.xlsx"
	failedFilePath := "results/failed/b1_failed.xlsx"

	// Xử lý tài khoản thành công
	if len(successResults) > 0 {
		// Luôn tạo file mới thay vì append
		f := excelize.NewFile()
		
		// Đếm số lượng cột bổ sung từ file gốc
		numExtraFields := 0
		if len(successResults) > 0 && len(successResults[0].Account.ExtraFields) > 0 {
			numExtraFields = len(successResults[0].Account.ExtraFields)
		}
		
		// Thêm header cơ bản
		f.SetCellValue("Sheet1", "A1", "STT")
		f.SetCellValue("Sheet1", "B1", "Tài khoản")
		f.SetCellValue("Sheet1", "C1", "Mật khẩu")
		f.SetCellValue("Sheet1", "D1", "Dòng trong Excel")
		
		// Thêm header cho các cột bổ sung (nếu có)
		extraHeaders := []string{"STT Gốc", "Thông tin 1", "Thông tin 2", "Thông tin 3", "Thông tin 4"}
		if numExtraFields > 0 {
			for i := 0; i < numExtraFields; i++ {
				colLetter := string(rune('E' + i))
				headerName := fmt.Sprintf("Thông tin %d", i)
				if i < len(extraHeaders) {
					headerName = extraHeaders[i]
				}
				f.SetCellValue("Sheet1", fmt.Sprintf("%s1", colLetter), headerName)
			}
		}
		
		// Tạo style cho header
		headerStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 11, Color: "#FFFFFF"},
			Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
			Border: []excelize.Border{
				{Type: "bottom", Color: "#000000", Style: 1},
			},
			Alignment: &excelize.Alignment{
				Horizontal: "center",
				Vertical:   "center",
			},
		})
		
		// Tính chữ cái cuối cùng của cột có header
		lastHeaderCol := "D"
		if numExtraFields > 0 {
			lastHeaderCol = string(rune('D' + numExtraFields))
		}
		
		f.SetCellStyle("Sheet1", "A1", lastHeaderCol + "1", headerStyle)
		
		// Thiết lập độ rộng cột
		f.SetColWidth("Sheet1", "A", "A", 10)
		f.SetColWidth("Sheet1", "B", "B", 30)
		f.SetColWidth("Sheet1", "C", "C", 20)
		f.SetColWidth("Sheet1", "D", "D", 15)
		
		// Thiết lập độ rộng cho các cột bổ sung
		if numExtraFields > 0 {
			lastCol := string(rune('D' + numExtraFields))
			f.SetColWidth("Sheet1", "E", lastCol, 20)
		}

		// Thêm dữ liệu mới, bắt đầu từ dòng 2 (sau header)
		startRow := 2
		
		// Thêm dữ liệu mới
		loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
		if err != nil {
			loc = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60) // UTC+7
		}
		vietnamTime := time.Now().In(loc)
		timestamp := vietnamTime.Format("02-01-2006 15:04:05")
		for i, result := range successResults {
			row := startRow + i
			f.SetCellValue("Sheet1", fmt.Sprintf("A%d", row), i+1)
			f.SetCellValue("Sheet1", fmt.Sprintf("B%d", row), result.Account.Username)
			f.SetCellValue("Sheet1", fmt.Sprintf("C%d", row), result.Account.Password)
			f.SetCellValue("Sheet1", fmt.Sprintf("D%d", row), result.Account.Row)
			
			// Thêm các thông tin bổ sung từ file gốc (nếu có)
			if len(result.Account.ExtraFields) > 0 {
				for j, extraValue := range result.Account.ExtraFields {
					colLetter := string(rune('E' + j))
					f.SetCellValue("Sheet1", fmt.Sprintf("%s%d", colLetter, row), extraValue)
				}
			}
		}

		// Style cho các dòng chẵn
		evenRowStyle, _ := f.NewStyle(&excelize.Style{
			Fill: excelize.Fill{Type: "pattern", Color: []string{"#E9EFF7"}, Pattern: 1},
		})

		// Áp dụng style cho các dòng chẵn
		for i := startRow; i < startRow+len(successResults); i++ {
			if i%2 == 0 {
				lastCol := "D"
				if numExtraFields > 0 {
					lastCol = string(rune('D' + numExtraFields))
				}
				f.SetCellStyle("Sheet1", fmt.Sprintf("A%d", i), fmt.Sprintf("%s%d", lastCol, i), evenRowStyle)
			}
		}

		// Thêm dòng thông tin tổng kết
		summaryRow := startRow + len(successResults) + 1
		lastCol := "D"
		if numExtraFields > 0 {
			lastCol = string(rune('D' + numExtraFields))
		}
		f.MergeCell("Sheet1", fmt.Sprintf("A%d", summaryRow), fmt.Sprintf("%s%d", lastCol, summaryRow))
		f.SetCellValue("Sheet1", fmt.Sprintf("A%d", summaryRow), fmt.Sprintf("Tổng số: %d tài khoản | Thời gian: %s", len(successResults), timestamp))
		
		// Lưu file
		if err := f.SaveAs(successFilePath); err != nil {
			log.Printf("Error saving success file: %v", err)
		}
	}

	// Xử lý tài khoản thất bại
	if len(failedResults) > 0 {
		// Luôn tạo file mới thay vì append
		f := excelize.NewFile()
		
		// Đếm số lượng cột bổ sung từ file gốc
		numExtraFields := 0
		if len(failedResults) > 0 && len(failedResults[0].Account.ExtraFields) > 0 {
			numExtraFields = len(failedResults[0].Account.ExtraFields)
		}
		
		// Thêm header cơ bản
		f.SetCellValue("Sheet1", "A1", "STT")
		f.SetCellValue("Sheet1", "B1", "Tài khoản")
		f.SetCellValue("Sheet1", "C1", "Mật khẩu")
		f.SetCellValue("Sheet1", "D1", "Dòng trong Excel")
		
		// Thêm header cho các cột bổ sung (nếu có)
		extraHeaders := []string{"Thông tin 1", "Thông tin 2", "Thông tin 3", "Thông tin 4", "Thông tin 5"}
		if numExtraFields > 0 {
			for i := 0; i < numExtraFields; i++ {
				colLetter := string(rune('E' + i))
				headerName := fmt.Sprintf("Thông tin %d", i+1)
				if i < len(extraHeaders) {
					headerName = extraHeaders[i]
				}
				f.SetCellValue("Sheet1", fmt.Sprintf("%s1", colLetter), headerName)
			}
		}
		
		// Tạo style cho header
		headerStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 11, Color: "#FFFFFF"},
			Fill: excelize.Fill{Type: "pattern", Color: []string{"#FF6666"}, Pattern: 1},
			Border: []excelize.Border{
				{Type: "bottom", Color: "#000000", Style: 1},
			},
			Alignment: &excelize.Alignment{
				Horizontal: "center",
				Vertical:   "center",
			},
		})
		
		// Tính chữ cái cuối cùng của cột có header
		lastHeaderCol := "D"
		if numExtraFields > 0 {
			lastHeaderCol = string(rune('D' + numExtraFields))
		}
		
		f.SetCellStyle("Sheet1", "A1", lastHeaderCol + "1", headerStyle)
		
		// Thiết lập độ rộng cột
		f.SetColWidth("Sheet1", "A", "A", 10)
		f.SetColWidth("Sheet1", "B", "B", 30)
		f.SetColWidth("Sheet1", "C", "C", 20)
		f.SetColWidth("Sheet1", "D", "D", 15)
		
		// Thiết lập độ rộng cho các cột bổ sung
		if numExtraFields > 0 {
			lastCol := string(rune('D' + numExtraFields))
			f.SetColWidth("Sheet1", "E", lastCol, 20)
		}

		// Thêm dữ liệu mới, bắt đầu từ dòng 2 (sau header)
		startRow := 2

		// Thêm dữ liệu mới
		loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
		if err != nil {
			loc = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60) // UTC+7
		}
		vietnamTime := time.Now().In(loc)
		timestamp := vietnamTime.Format("02-01-2006 15:04:05")
		for i, result := range failedResults {
			row := startRow + i
			f.SetCellValue("Sheet1", fmt.Sprintf("A%d", row), i+1)
			f.SetCellValue("Sheet1", fmt.Sprintf("B%d", row), result.Account.Username)
			f.SetCellValue("Sheet1", fmt.Sprintf("C%d", row), result.Account.Password)
			f.SetCellValue("Sheet1", fmt.Sprintf("D%d", row), result.Account.Row)
			
			// Thêm các thông tin bổ sung từ file gốc (nếu có)
			if len(result.Account.ExtraFields) > 0 {
				for j, extraValue := range result.Account.ExtraFields {
					colLetter := string(rune('E' + j))
					f.SetCellValue("Sheet1", fmt.Sprintf("%s%d", colLetter, row), extraValue)
				}
			}
		}

		// Style cho các dòng chẵn
		evenRowStyle, _ := f.NewStyle(&excelize.Style{
			Fill: excelize.Fill{Type: "pattern", Color: []string{"#FFEBEE"}, Pattern: 1},
		})

		// Áp dụng style cho các dòng chẵn
		for i := startRow; i < startRow+len(failedResults); i++ {
			if i%2 == 0 {
				lastCol := "D"
				if numExtraFields > 0 {
					lastCol = string(rune('D' + numExtraFields))
				}
				f.SetCellStyle("Sheet1", fmt.Sprintf("A%d", i), fmt.Sprintf("%s%d", lastCol, i), evenRowStyle)
			}
		}

		// Thêm dòng thông tin tổng kết
		summaryRow := startRow + len(failedResults) + 1
		lastCol := "D"
		if numExtraFields > 0 {
			lastCol = string(rune('D' + numExtraFields))
		}
		f.MergeCell("Sheet1", fmt.Sprintf("A%d", summaryRow), fmt.Sprintf("%s%d", lastCol, summaryRow))
		f.SetCellValue("Sheet1", fmt.Sprintf("A%d", summaryRow), fmt.Sprintf("Tổng số: %d tài khoản | Thời gian: %s", len(failedResults), timestamp))
		
		// Lưu file
		if err := f.SaveAs(failedFilePath); err != nil {
			log.Printf("Error saving failed file: %v", err)
		}
	}
}

// updateTextResults saves the results to text files
func updateTextResults(results []BatchResult) {
	// Tạo thư mục results nếu chưa tồn tại
	os.MkdirAll("results", 0755)
	
	// Đường dẫn file cố định
	successFilePath := "results/b1_success.txt"
	failedFilePath := "results/b1_failed.txt"
	
	// Xử lý tài khoản thành công
	if len(results) > 0 {
		var successBuffer bytes.Buffer
		var failedBuffer bytes.Buffer
		
		loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
		if err != nil {
			loc = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60) // UTC+7
		}
		vietnamTime := time.Now().In(loc)
		timestamp := vietnamTime.Format("02-01-2006 15:04:05")
		
		for _, result := range results {
			if result.Success {
				// Ghi vào buffer thành công
				fmt.Fprintf(&successBuffer, "Username: %s | Password: %s | Row: %d | Time: %s\n",
					result.Account.Username, result.Account.Password, result.Account.Row, timestamp)
			} else {
				// Ghi vào buffer thất bại
				fmt.Fprintf(&failedBuffer, "Username: %s | Password: %s | Row: %d | Error: %s | Time: %s\n",
					result.Account.Username, result.Account.Password, result.Account.Row, result.LogMessage, timestamp)
			}
		}
		
		// Ghi vào file thành công
		if successBuffer.Len() > 0 {
			// Mở file để append
			successFile, err := os.OpenFile(successFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				successFile.Write(successBuffer.Bytes())
				successFile.Close()
			}
		}
		
		// Ghi vào file thất bại
		if failedBuffer.Len() > 0 {
			// Mở file để append
			failedFile, err := os.OpenFile(failedFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				failedFile.Write(failedBuffer.Bytes())
				failedFile.Close()
			}
		}
	}
}

// cleanupOldFiles xóa các file kết quả cũ
func cleanupOldFiles() {
	// Xóa các file Excel cũ có timestamp
	files, err := filepath.Glob("results/*_*.xlsx")
	if err == nil {
		for _, file := range files {
			// Chỉ xóa các file có timestamp, giữ lại file cố định
			if !strings.Contains(file, "b1_success.xlsx") && !strings.Contains(file, "b1_failed.xlsx") {
				os.Remove(file)
			}
		}
	}
	
	// Xóa các file Excel trong thư mục con nếu không phải file cố định
	successFiles, _ := filepath.Glob("results/success/*.xlsx")
	for _, file := range successFiles {
		if !strings.Contains(file, "b1_success.xlsx") {
			os.Remove(file)
		}
	}
	
	failedFiles, _ := filepath.Glob("results/failed/*.xlsx")
	for _, file := range failedFiles {
		if !strings.Contains(file, "b1_failed.xlsx") {
			os.Remove(file)
		}
	}
	
	// Xóa các file text đã xử lý cũ
	textFiles, _ := filepath.Glob("results/processed_*.txt")
	for _, file := range textFiles {
		os.Remove(file)
	}
} 