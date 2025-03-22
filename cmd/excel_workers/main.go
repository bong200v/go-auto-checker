package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"Go_auto_checker/internal/workers"
	"github.com/xuri/excelize/v2"
)

// Biến toàn cục để lưu kết quả
var (
	allResults   []*workers.WorkerResult
	startTime    time.Time
	saveResults  bool
	processedIds map[int]bool
)

// Account đại diện cho một tài khoản trong file Excel
type Account struct {
	Username string
	Password string
	RowNum   int
}

// Đọc danh sách tài khoản từ file Excel
func readExcelFile(filePath string, startRow int) ([]Account, error) {
	// Kiểm tra file tồn tại
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file không tồn tại: %s", filePath)
	}

	// Mở file Excel
	xlsx, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("không thể mở file: %v", err)
	}
	defer xlsx.Close()

	// Lấy tên sheet đầu tiên
	sheets := xlsx.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("file Excel không có sheet nào")
	}
	sheetName := sheets[0]

	// Đọc dữ liệu từ sheet
	rows, err := xlsx.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("không thể đọc sheet %s: %v", sheetName, err)
	}

	var accounts []Account

	// Bắt đầu từ dòng startRow (thông thường là 2 để bỏ qua header)
	for i := startRow - 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) < 3 {
			// Bỏ qua dòng không đủ cột
			continue
		}

		username := row[1] // Cột 2 (B)
		password := row[2] // Cột 3 (C)

		// Chỉ lấy những dòng có cả username và password
		if username != "" && password != "" {
			accounts = append(accounts, Account{
				Username: username,
				Password: password,
				RowNum:   i + 1, // Số dòng thực tế (bắt đầu từ 1)
			})
		}
	}

	return accounts, nil
}

// Cập nhật kết quả vào các file Excel riêng biệt
func updateExcelResults(filePath string, results []*workers.WorkerResult) {
	// Tạo thư mục kết quả nếu chưa tồn tại
	resultsDir := "results"
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return
	}
	
	// Phân loại kết quả
	var successResults, failedResults []*workers.WorkerResult
	for _, result := range results {
		if result.Success {
			successResults = append(successResults, result)
		} else {
			failedResults = append(failedResults, result)
		}
	}

	// Tạo file Excel cho tài khoản thành công
	if len(successResults) > 0 {
		successFile := filepath.Join(resultsDir, "success", "b1_success.xlsx")
		createAccountsExcel(successFile, successResults, true)
	}
	
	// Tạo file Excel cho tài khoản thất bại
	if len(failedResults) > 0 {
		failedFile := filepath.Join(resultsDir, "failed", "b1_failed.xlsx")
		createAccountsExcel(failedFile, failedResults, false)
	}
}

// Tạo file Excel cho danh sách tài khoản
func createAccountsExcel(filePath string, results []*workers.WorkerResult, isSuccess bool) {
	// Tạo file Excel mới
	xlsx := excelize.NewFile()
	defer xlsx.Close()
	
	// Lấy tên sheet mặc định
	sheetName := xlsx.GetSheetName(0)
	
	// Tạo tiêu đề
	title := "Danh sách tài khoản đăng nhập thành công"
	if !isSuccess {
		title = "Danh sách tài khoản đăng nhập thất bại"
	}
	
	// Thiết lập style cho tiêu đề
	titleStyle, _ := xlsx.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   14,
			Color:  "000000",
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	
	// Thiết lập style cho header
	headerStyle, _ := xlsx.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"4472C4"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "CCCCCC", Style: 1},
			{Type: "top", Color: "CCCCCC", Style: 1},
			{Type: "right", Color: "CCCCCC", Style: 1},
			{Type: "bottom", Color: "CCCCCC", Style: 1},
		},
	})
	
	// Thiết lập style cho dữ liệu
	dataStyle, _ := xlsx.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "CCCCCC", Style: 1},
			{Type: "top", Color: "CCCCCC", Style: 1},
			{Type: "right", Color: "CCCCCC", Style: 1},
			{Type: "bottom", Color: "CCCCCC", Style: 1},
		},
	})
	
	// Thiết lập tiêu đề
	xlsx.MergeCell(sheetName, "A1", "F1")
	xlsx.SetCellValue(sheetName, "A1", title)
	xlsx.SetCellStyle(sheetName, "A1", "F1", titleStyle)
	xlsx.SetRowHeight(sheetName, 1, 30)
	
	// Thiết lập header
	headers := []string{"STT", "Tài khoản", "Mật khẩu"}
	if isSuccess {
		headers = append(headers, "AccountID", "Thời gian", "Ghi chú")
	} else {
		headers = append(headers, "Lỗi", "Thời gian", "Ghi chú")
	}
	
	// Ghi header
	for i, header := range headers {
		cell := string(rune('A'+i)) + "3"
		xlsx.SetCellValue(sheetName, cell, header)
		xlsx.SetCellStyle(sheetName, cell, cell, headerStyle)
	}
	
	// Thiết lập độ rộng cột
	xlsx.SetColWidth(sheetName, "A", "A", 10)
	xlsx.SetColWidth(sheetName, "B", "B", 20)
	xlsx.SetColWidth(sheetName, "C", "C", 20)
	xlsx.SetColWidth(sheetName, "D", "D", 20)
	xlsx.SetColWidth(sheetName, "E", "E", 20)
	xlsx.SetColWidth(sheetName, "F", "F", 30)
	
	// Ghi dữ liệu
	for i, result := range results {
		rowNum := i + 4 // Bắt đầu từ dòng 4
		
		// STT
		xlsx.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), i+1)
		
		// Tài khoản
		xlsx.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), result.Username)
		
		// Mật khẩu
		xlsx.SetCellValue(sheetName, fmt.Sprintf("C%d", rowNum), result.Password)
		
		if isSuccess {
			// AccountID
			xlsx.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), result.AccountID)
		} else {
			// Lỗi
			xlsx.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), result.ErrorMessage)
		}
		
		// Thời gian
		xlsx.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), 
			result.Duration.Round(time.Millisecond).String())
		
		// Ghi chú
		xlsx.SetCellValue(sheetName, fmt.Sprintf("F%d", rowNum), result.ExtraInfo)
		
		// Thiết lập style cho dòng dữ liệu
		xlsx.SetCellStyle(sheetName, fmt.Sprintf("A%d", rowNum), fmt.Sprintf("F%d", rowNum), dataStyle)
	}
	
	// Thêm timestamp ở cuối
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	timestamp := time.Now().In(loc).Format("02/01/2006 15:04:05")
	
	lastRow := len(results) + 5
	xlsx.MergeCell(sheetName, fmt.Sprintf("A%d", lastRow), fmt.Sprintf("F%d", lastRow))
	xlsx.SetCellValue(sheetName, fmt.Sprintf("A%d", lastRow), 
		fmt.Sprintf("Tổng số: %d | Thời gian tạo: %s", len(results), timestamp))
	
	// Đảm bảo thư mục tồn tại
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	
	// Lưu file
	if err := xlsx.SaveAs(filePath); err != nil {
		return
	}
	
	// Thực hiện flush để đảm bảo dữ liệu được ghi
	forceFlush(filePath)
	
	// Lưu thêm bản sao dạng text để đảm bảo luôn có kết quả
	textFilePath := filePath + ".txt"
	var content string
	
	// Header
	if isSuccess {
		content = "STT\tTài khoản\tMật khẩu\tAccountID\tThời gian\tGhi chú\n"
	} else {
		content = "STT\tTài khoản\tMật khẩu\tLỗi\tThời gian\tGhi chú\n"
	}
	
	// Dữ liệu
	for i, result := range results {
		if isSuccess {
			content += fmt.Sprintf("%d\t%s\t%s\t%.0f\t%s\t%s\n", 
				i+1, result.Username, result.Password, result.AccountID, 
				result.Duration.Round(time.Millisecond).String(), result.ExtraInfo)
		} else {
			content += fmt.Sprintf("%d\t%s\t%s\t%s\t%s\t%s\n", 
				i+1, result.Username, result.Password, result.ErrorMessage, 
				result.Duration.Round(time.Millisecond).String(), result.ExtraInfo)
		}
	}
	
	// Lưu file text
	os.WriteFile(textFilePath, []byte(content), 0644)
}

// Tạo file Excel tổng hợp
func createSummaryExcel(filePath string, successResults []*workers.WorkerResult, failedResults []*workers.WorkerResult) {
	// Tạo file Excel mới
	xlsx := excelize.NewFile()
	defer xlsx.Close()
	
	// Lấy tên sheet mặc định
	sheetName := xlsx.GetSheetName(0)
	
	// Thiết lập style cho tiêu đề
	titleStyle, _ := xlsx.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   14,
			Color:  "000000",
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	
	// Thiết lập style cho header
	headerStyle, _ := xlsx.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"4472C4"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "CCCCCC", Style: 1},
			{Type: "top", Color: "CCCCCC", Style: 1},
			{Type: "right", Color: "CCCCCC", Style: 1},
			{Type: "bottom", Color: "CCCCCC", Style: 1},
		},
	})
	
	// Thiết lập style cho dòng thành công
	successStyle, _ := xlsx.NewStyle(&excelize.Style{
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"E2EFDA"},
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "CCCCCC", Style: 1},
			{Type: "top", Color: "CCCCCC", Style: 1},
			{Type: "right", Color: "CCCCCC", Style: 1},
			{Type: "bottom", Color: "CCCCCC", Style: 1},
		},
	})
	
	// Thiết lập style cho dòng thất bại
	failedStyle, _ := xlsx.NewStyle(&excelize.Style{
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"FFC7CE"},
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "CCCCCC", Style: 1},
			{Type: "top", Color: "CCCCCC", Style: 1},
			{Type: "right", Color: "CCCCCC", Style: 1},
			{Type: "bottom", Color: "CCCCCC", Style: 1},
		},
	})
	
	// Thiết lập style cho dữ liệu thông thường
	dataStyle, _ := xlsx.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "CCCCCC", Style: 1},
			{Type: "top", Color: "CCCCCC", Style: 1},
			{Type: "right", Color: "CCCCCC", Style: 1},
			{Type: "bottom", Color: "CCCCCC", Style: 1},
		},
	})
	
	// Thiết lập tiêu đề
	xlsx.MergeCell(sheetName, "A1", "G1")
	xlsx.SetCellValue(sheetName, "A1", "BÁO CÁO TỔNG HỢP KẾT QUẢ ĐĂNG NHẬP")
	xlsx.SetCellStyle(sheetName, "A1", "G1", titleStyle)
	xlsx.SetRowHeight(sheetName, 1, 30)
	
	// Thiết lập header
	headers := []string{"STT", "Tài khoản", "Mật khẩu", "AccountID", "Trạng thái", "Thời gian", "Ghi chú"}
	
	// Ghi header
	for i, header := range headers {
		cell := string(rune('A'+i)) + "3"
		xlsx.SetCellValue(sheetName, cell, header)
		xlsx.SetCellStyle(sheetName, cell, cell, headerStyle)
	}
	
	// Thiết lập độ rộng cột
	xlsx.SetColWidth(sheetName, "A", "A", 10)
	xlsx.SetColWidth(sheetName, "B", "B", 20)
	xlsx.SetColWidth(sheetName, "C", "C", 20)
	xlsx.SetColWidth(sheetName, "D", "D", 20)
	xlsx.SetColWidth(sheetName, "E", "E", 15)
	xlsx.SetColWidth(sheetName, "F", "F", 20)
	xlsx.SetColWidth(sheetName, "G", "G", 30)
	
	// Ghi dữ liệu thành công
	rowCount := 0
	for i, result := range successResults {
		rowNum := i + 4 // Bắt đầu từ dòng 4
		rowCount++
		
		// STT
		xlsx.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), rowCount)
		
		// Tài khoản
		xlsx.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), result.Username)
		
		// Mật khẩu
		xlsx.SetCellValue(sheetName, fmt.Sprintf("C%d", rowNum), result.Password)
		
		// AccountID
		xlsx.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), result.AccountID)
		
		// Trạng thái
		xlsx.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), "Thành công")
		
		// Thời gian
		xlsx.SetCellValue(sheetName, fmt.Sprintf("F%d", rowNum), 
			result.Duration.Round(time.Millisecond).String())
		
		// Ghi chú
		xlsx.SetCellValue(sheetName, fmt.Sprintf("G%d", rowNum), result.ExtraInfo)
		
		// Thiết lập style cho dòng dữ liệu
		xlsx.SetCellStyle(sheetName, fmt.Sprintf("A%d", rowNum), fmt.Sprintf("G%d", rowNum), successStyle)
	}
	
	// Ghi dữ liệu thất bại
	startRow := len(successResults) + 4
	for i, result := range failedResults {
		rowNum := startRow + i
		rowCount++
		
		// STT
		xlsx.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), rowCount)
		
		// Tài khoản
		xlsx.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), result.Username)
		
		// Mật khẩu
		xlsx.SetCellValue(sheetName, fmt.Sprintf("C%d", rowNum), result.Password)
		
		// AccountID (rỗng hoặc N/A với tài khoản thất bại)
		xlsx.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), "N/A")
		
		// Trạng thái
		xlsx.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), "Thất bại")
		
		// Thời gian
		xlsx.SetCellValue(sheetName, fmt.Sprintf("F%d", rowNum), 
			result.Duration.Round(time.Millisecond).String())
		
		// Ghi chú (hiển thị lỗi)
		xlsx.SetCellValue(sheetName, fmt.Sprintf("G%d", rowNum), result.ErrorMessage)
		
		// Thiết lập style cho dòng dữ liệu
		xlsx.SetCellStyle(sheetName, fmt.Sprintf("A%d", rowNum), fmt.Sprintf("G%d", rowNum), failedStyle)
	}
	
	// Tổng kết ở cuối
	summaryRow := rowCount + 5
	
	// Tóm tắt
	xlsx.MergeCell(sheetName, fmt.Sprintf("A%d", summaryRow), fmt.Sprintf("G%d", summaryRow))
	xlsx.SetCellValue(sheetName, fmt.Sprintf("A%d", summaryRow), 
		fmt.Sprintf("TỔNG KẾT: %d tài khoản | %d thành công | %d thất bại", 
			len(successResults) + len(failedResults), len(successResults), len(failedResults)))
	xlsx.SetCellStyle(sheetName, fmt.Sprintf("A%d", summaryRow), fmt.Sprintf("G%d", summaryRow), dataStyle)
	
	// Thêm timestamp
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	timestamp := time.Now().In(loc).Format("02/01/2006 15:04:05")
	
	// Thêm dòng timestamp
	timestampRow := summaryRow + 1
	xlsx.MergeCell(sheetName, fmt.Sprintf("A%d", timestampRow), fmt.Sprintf("G%d", timestampRow))
	xlsx.SetCellValue(sheetName, fmt.Sprintf("A%d", timestampRow), fmt.Sprintf("Thời gian tạo: %s", timestamp))
	xlsx.SetCellStyle(sheetName, fmt.Sprintf("A%d", timestampRow), fmt.Sprintf("G%d", timestampRow), dataStyle)
	
	// Đảm bảo thư mục tồn tại
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Lỗi khi tạo thư mục: %v\n", err)
		return
	}
	
	// Lưu file
	if err := xlsx.SaveAs(filePath); err != nil {
		fmt.Printf("Lỗi khi lưu file Excel tổng hợp: %v\n", err)
	} else {
		fmt.Printf("Đã lưu báo cáo tổng hợp vào file: %s\n", filePath)
		
		// Thực hiện flush để đảm bảo dữ liệu được ghi
		err = forceFlush(filePath)
		if err != nil {
			fmt.Printf("Cảnh báo: Không thể flush dữ liệu: %v\n", err)
		}
	}
	
	// Lưu thêm bản sao dạng text
	textFilePath := filePath + ".txt"
	var content string
	
	// Header
	content = "STT\tTài khoản\tMật khẩu\tAccountID\tTrạng thái\tThời gian\tGhi chú\n"
	
	// Dữ liệu thành công
	for i, result := range successResults {
		content += fmt.Sprintf("%d\t%s\t%s\t%.0f\t%s\t%s\t%s\n", 
			i+1, result.Username, result.Password, result.AccountID, 
			"Thành công", result.Duration.Round(time.Millisecond).String(), result.ExtraInfo)
	}
	
	// Dữ liệu thất bại
	startIdx := len(successResults) + 1
	for i, result := range failedResults {
		content += fmt.Sprintf("%d\t%s\t%s\t%s\t%s\t%s\t%s\n", 
			startIdx+i, result.Username, result.Password, "N/A", 
			"Thất bại", result.Duration.Round(time.Millisecond).String(), result.ErrorMessage)
	}
	
	// Tổng kết
	content += fmt.Sprintf("\nTỔNG KẾT: %d tài khoản | %d thành công | %d thất bại\n", 
		len(successResults) + len(failedResults), len(successResults), len(failedResults))
	content += fmt.Sprintf("Thời gian tạo: %s\n", timestamp)
	
	// Lưu file text
	if err := os.WriteFile(textFilePath, []byte(content), 0644); err != nil {
		fmt.Printf("Lỗi khi lưu file text tổng hợp: %v\n", err)
	} else {
		fmt.Printf("Đã lưu bản sao dạng text tổng hợp vào: %s\n", textFilePath)
	}
}

// Hàm này đảm bảo dữ liệu được ghi vào đĩa thông qua fsync
func forceFlush(filePath string) error {
	// Mở file với quyền đọc/ghi
	file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("lỗi khi mở file để flush: %v", err)
	}
	defer file.Close()
	
	// Gọi Sync để đảm bảo dữ liệu được ghi xuống ổ đĩa
	err = file.Sync()
	if err != nil {
		return fmt.Errorf("lỗi khi gọi fsync: %v", err)
	}
	
	return nil
}

// Lưu trạng thái hiện tại để có thể khôi phục nếu chương trình bị tắt đột ngột
func saveCheckpoint(accounts []Account, results []*workers.WorkerResult) (map[int]bool, int, int) {
	// Khởi tạo processedIds từ results
	processedIds := make(map[int]bool)
	var successCount, failedCount int
	
	// Lưu kết quả hiện tại nếu có
	if len(results) > 0 {
		// Cập nhật kết quả
		updateExcelResults("", results)
		
		// Phân loại tài khoản
		for _, result := range results {
			// Lấy index của account
			accountIndex := -1
			for i, acc := range accounts {
				if acc.Username == result.Username && acc.Password == result.Password {
					accountIndex = i
					break
				}
			}
			
			if accountIndex != -1 {
				processedIds[accountIndex] = true
			}
			
			if result.Success {
				successCount++
			} else {
				failedCount++
			}
		}
		
		// Lưu các tài khoản đã xử lý
		os.WriteFile(filepath.Join("results", "processed_ids.txt"), 
			[]byte(fmt.Sprintf("%d\n", len(processedIds))), 0644)
	}
	
	return processedIds, successCount, failedCount
}

// Đọc IDs đã xử lý từ file
func loadProcessedIds() map[int]bool {
	result := make(map[int]bool)
	
	idsFile := filepath.Join("results", "processed_ids.txt")
	content, err := os.ReadFile(idsFile)
	if err != nil {
		return result
	}
	
	var id int
	for _, line := range []byte(content) {
		if line == '\n' {
			if id > 0 {
				result[id] = true
				id = 0
			}
		} else if line >= '0' && line <= '9' {
			id = id*10 + int(line-'0')
		}
	}
	
	if id > 0 {
		result[id] = true
	}
	
	return result
}

// Xử lý tín hiệu dừng
func setupSignalHandler(signalChan chan bool, done *sync.WaitGroup, accounts []Account, results []*workers.WorkerResult, saveResults bool, verbose bool, startTime time.Time) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-signals
		
		// Lưu checkpoint trước khi thoát
		processedIDs, successCount, failedCount := saveCheckpoint(accounts, results)
		
		// Hiển thị thống kê
		duration := time.Since(startTime)
		fmt.Printf("\nĐã xử lý: %d/%d tài khoản (%d thành công, %d thất bại) trong %s\n", 
			len(processedIDs), len(accounts), successCount, failedCount, 
			duration.Round(time.Millisecond))
		
		// Đóng channel để dừng workers
		close(signalChan)
		
		// Đợi tất cả worker hoàn thành
		done.Wait()
		
		// Đợi I/O hoàn tất
		time.Sleep(5 * time.Second)
		
		os.Exit(0)
	}()
}

func main() {
	// Phân tích tham số dòng lệnh
	excelFile := flag.String("excel", "", "Đường dẫn đến file Excel chứa danh sách tài khoản")
	maxWorkers := flag.Int("max", 10, "Số lượng worker tối đa")
	startRow := flag.Int("start", 2, "Dòng bắt đầu đọc trong file Excel")
	verbose := flag.Bool("v", false, "Hiển thị thông tin chi tiết")
	saveResults := flag.Bool("save", false, "Lưu kết quả vào file Excel")
	resumeFlag := flag.Bool("resume", false, "Tiếp tục từ lần chạy trước")
	
	flag.Parse()
	
	// Khởi tạo biến toàn cục
	var processedIds = make(map[int]bool)
	
	// Kiểm tra tham số bắt buộc
	if *excelFile == "" {
		fmt.Println("Vui lòng cung cấp đường dẫn đến file Excel (-excel)")
		flag.Usage()
		os.Exit(1)
	}
	
	// Kiểm tra file Excel tồn tại
	if _, err := os.Stat(*excelFile); os.IsNotExist(err) {
		fmt.Printf("File Excel không tồn tại: %s\n", *excelFile)
		os.Exit(1)
	}
	
	// Khởi tạo biến kết quả
	var allResults []*workers.WorkerResult
	
	// Thời gian bắt đầu
	startTime := time.Now()
	
	// Thiết lập xử lý tín hiệu dừng
	signalChan := make(chan bool)
	done := &sync.WaitGroup{}
	
	// Đọc danh sách tài khoản từ file Excel
	accounts, err := readExcelFile(*excelFile, *startRow)
	if err != nil {
		fmt.Printf("Lỗi khi đọc file Excel: %v\n", err)
		os.Exit(1)
	}
	
	// Hiển thị thông tin số lượng tài khoản
	fmt.Printf("Đã tìm thấy %d tài khoản trong file Excel\n", len(accounts))
	
	// Thiết lập xử lý tín hiệu dừng
	setupSignalHandler(signalChan, done, accounts, allResults, *saveResults, *verbose, startTime)
	
	// Đọc các IDs đã xử lý nếu cần tiếp tục từ lần chạy trước
	if *resumeFlag {
		processedIds = loadProcessedIds()
		if len(processedIds) > 0 {
			fmt.Printf("📝 Tiếp tục từ lần chạy trước, đã tìm thấy %d tài khoản đã xử lý\n", len(processedIds))
		}
	}
	
	// Tạo và dọn dẹp thư mục kết quả
	resultsDir := "results"
	if *saveResults {
		// Đảm bảo thư mục results tồn tại
		if err := os.MkdirAll(resultsDir, 0755); err != nil {
			fmt.Printf("Lỗi khi tạo thư mục kết quả: %v\n", err)
		}
		
		// Tạo thư mục phân loại
		successDir := filepath.Join(resultsDir, "success")
		failedDir := filepath.Join(resultsDir, "failed")
		
		if err := os.MkdirAll(successDir, 0755); err != nil {
			fmt.Printf("Lỗi khi tạo thư mục success: %v\n", err)
		}
		
		if err := os.MkdirAll(failedDir, 0755); err != nil {
			fmt.Printf("Lỗi khi tạo thư mục failed: %v\n", err)
		}
		
		// Xóa tất cả các file trong thư mục results gốc nếu không phải resume
		if !*resumeFlag {
			entries, err := os.ReadDir(resultsDir)
			if err == nil {
				for _, entry := range entries {
					// Bỏ qua các thư mục success và failed
					if entry.IsDir() && (entry.Name() == "success" || entry.Name() == "failed") {
						continue
					}
					
					// Xóa các file và thư mục khác
					path := filepath.Join(resultsDir, entry.Name())
					if err := os.RemoveAll(path); err != nil {
						fmt.Printf("Không thể xóa %s: %v\n", path, err)
					}
				}
			}
		}
	}
	
	// Đọc danh sách tài khoản từ file Excel
	fmt.Printf("Đang đọc file Excel: %s\n", *excelFile)
	accounts, err = readExcelFile(*excelFile, *startRow)
	if err != nil {
		fmt.Printf("Lỗi đọc file Excel: %v\n", err)
		return
	}
	
	totalAccounts := len(accounts)
	fmt.Printf("Tìm thấy %d tài khoản trong file Excel\n", totalAccounts)
	
	if totalAccounts == 0 {
		fmt.Println("Không có tài khoản nào được tìm thấy trong file Excel")
		return
	}
	
	// Giới hạn số lượng worker chạy cùng lúc
	concurrentWorkers := *maxWorkers
	if concurrentWorkers > totalAccounts {
		concurrentWorkers = totalAccounts
	}
	
	if concurrentWorkers < totalAccounts {
		fmt.Printf("Tổng số tài khoản: %d, chạy theo batch với mỗi batch %d worker\n", totalAccounts, concurrentWorkers)
	}
	
	// Xử lý tài khoản theo batch
	for i := 0; i < totalAccounts; i += concurrentWorkers {
		endIdx := i + concurrentWorkers
		if endIdx > totalAccounts {
			endIdx = totalAccounts
		}
		
		batchAccounts := accounts[i:endIdx]
		batchSize := len(batchAccounts)
		
		// Kiểm tra xem batch này có tài khoản chưa xử lý không
		hasUnprocessed := false
		for j := range batchAccounts {
			workerId := i + j + 1
			if !processedIds[workerId] {
				hasUnprocessed = true
				break
			}
		}
		
		if !hasUnprocessed {
			continue
		}
		
		// Tạo danh sách worker configs, chỉ cho những tài khoản chưa xử lý
		configs := make([]workers.WorkerConfig, 0, batchSize)
		for j, account := range batchAccounts {
			workerId := i + j + 1
			if !processedIds[workerId] {
				configs = append(configs, workers.WorkerConfig{
					Username:    account.Username,
					Password:    account.Password,
					WorkerId:    workerId,
					IdyKeyTTL:   10 * time.Minute,
					MaxRetries:  3,
					Verbose:     *verbose,
					SaveResults: *saveResults,
					ResultsDir:  "results",
					ExtraInfo:   fmt.Sprintf("Dòng %d", account.RowNum),
				})
			}
		}
		
		// Nếu không có config nào (tất cả tài khoản đã xử lý), bỏ qua batch này
		if len(configs) == 0 {
			continue
		}
		
		// Chạy workers
		batchResults := workers.RunMultipleWorkers(configs)
		
		// Đánh dấu các tài khoản đã xử lý
		for _, result := range batchResults {
			processedIds[result.WorkerId] = true
		}
		
		// Thêm kết quả vào danh sách tổng thể
		allResults = append(allResults, batchResults...)
		
		// Lưu kết quả ngay sau mỗi batch
		tempSaveResults := *saveResults
		*saveResults = true
		updateExcelResults("", allResults) // Lưu tất cả kết quả đã có
		*saveResults = tempSaveResults
		
		// Lưu trạng thái sau mỗi batch
		saveCheckpoint(accounts, allResults)
		
		// Thêm khoảng dừng giữa các batch
		if endIdx < totalAccounts {
			time.Sleep(1 * time.Second)
		}
	}
	
	// Hiển thị thống kê tổng hợp
	totalDuration := time.Since(startTime)
	fmt.Printf("\nTổng thời gian chạy: %.3fs\n", totalDuration.Seconds())
	
	// Đếm số lượng tài khoản thành công
	successCount := 0
	for _, result := range allResults {
		if result.Success {
			successCount++
		}
	}
	
	// Hiển thị kết quả cuối cùng
	if successCount == len(allResults) {
		fmt.Printf("Tất cả %d tài khoản đăng nhập thành công\n", len(allResults))
	} else {
		fmt.Printf("%d/%d tài khoản đăng nhập thành công\n", successCount, len(allResults))
	}
	
	// Lưu kết quả cuối cùng
	if *saveResults {
		updateExcelResults("", allResults)
	}
} 