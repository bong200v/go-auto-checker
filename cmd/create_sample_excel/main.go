package main

import (
	"fmt"
	"os"
	"path/filepath"
	
	"github.com/xuri/excelize/v2"
)

func main() {
	// Tạo file Excel mới
	xlsx := excelize.NewFile()
	
	// Thiết lập các cột tiêu đề
	xlsx.SetCellValue("Sheet1", "A1", "STT")
	xlsx.SetCellValue("Sheet1", "B1", "Tài khoản")
	xlsx.SetCellValue("Sheet1", "C1", "Mật khẩu")
	
	// Thêm dữ liệu mẫu
	xlsx.SetCellValue("Sheet1", "A2", 1)
	xlsx.SetCellValue("Sheet1", "B2", "100031580")  // Tài khoản thật
	xlsx.SetCellValue("Sheet1", "C2", "9006560")   // Mật khẩu thật
	
	// Thêm một số dòng mẫu nữa
	xlsx.SetCellValue("Sheet1", "A3", 2)
	xlsx.SetCellValue("Sheet1", "B3", "100031580")
	xlsx.SetCellValue("Sheet1", "C3", "9006560")
	
	xlsx.SetCellValue("Sheet1", "A4", 3)
	xlsx.SetCellValue("Sheet1", "B4", "100031580")
	xlsx.SetCellValue("Sheet1", "C4", "9006560")
	
	// Tạo các dòng thêm để test
	for i := 5; i <= 12; i++ {
		xlsx.SetCellValue("Sheet1", fmt.Sprintf("A%d", i), i-1)
		xlsx.SetCellValue("Sheet1", fmt.Sprintf("B%d", i), "100031580")
		xlsx.SetCellValue("Sheet1", fmt.Sprintf("C%d", i), "9006560")
	}
	
	// Thêm một số dòng không hợp lệ để test
	xlsx.SetCellValue("Sheet1", "A13", 12)
	xlsx.SetCellValue("Sheet1", "B13", "100031580") 
	// Cột C13 cố tình để trống
	
	xlsx.SetCellValue("Sheet1", "A14", 13)
	// Cột B14 cố tình để trống
	xlsx.SetCellValue("Sheet1", "C14", "9006560")
	
	// Định dạng bảng để dễ nhìn
	styleHeader, _ := xlsx.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#DDEBF7"}, Pattern: 1},
	})
	xlsx.SetCellStyle("Sheet1", "A1", "C1", styleHeader)
	
	// Định dạng ô dữ liệu
	styleData, _ := xlsx.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#FFFFFF"}, Pattern: 1},
	})
	xlsx.SetCellStyle("Sheet1", "A2", "C14", styleData)
	
	// Đặt độ rộng cột
	xlsx.SetColWidth("Sheet1", "A", "A", 8)
	xlsx.SetColWidth("Sheet1", "B", "B", 20)
	xlsx.SetColWidth("Sheet1", "C", "C", 20)
	
	// Tạo thư mục chứa file Excel nếu chưa tồn tại
	outputDir := "sample_data"
	os.MkdirAll(outputDir, 0755)
	
	// Lưu file Excel
	filePath := filepath.Join(outputDir, "accounts.xlsx")
	if err := xlsx.SaveAs(filePath); err != nil {
		fmt.Printf("Lỗi khi lưu file Excel: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Đã tạo file Excel mẫu: %s\n", filePath)
	fmt.Println("Chứa 10 tài khoản hợp lệ và 2 tài khoản không hợp lệ để test")
} 