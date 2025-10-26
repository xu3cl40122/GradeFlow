package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// 固定欄位索引（成績之前的欄位，用位置讀取不依賴表頭）
const (
	FieldSubjectCode = 0  // 科目代號
	FieldSubjectName = 1  // 科目名稱
	FieldStudentID   = 2  // 學號
	FieldYear        = 3  // 年級
	FieldClass       = 4  // 班級
	FieldSeatNumber  = 5  // 座號
	FieldStudentName = 6  // 學生姓名
	FieldGroup       = 7  // 組別
	FieldChoice      = 8  // 選擇
	FieldNonChoice   = 9  // 非選擇
	FieldMakeupDeduct = 10 // 補考扣
	FieldViolation   = 11 // 違規
	FieldMakeupDeductScore = 12 // 補考扣分
	FieldScore       = 13 // 成績
	// 索引 14 之後是動態欄位（答案1-N），長度依考試而定
)

// 學生成績資料（完整欄位）
type GradeRecord struct {
	Row []string // 儲存完整的 CSV 列資料
}

// 取得特定欄位的輔助方法（使用固定索引）
func (g *GradeRecord) SubjectCode() string { return g.getField(FieldSubjectCode) }
func (g *GradeRecord) SubjectName() string { return g.getField(FieldSubjectName) }
func (g *GradeRecord) StudentID() string   { return g.getField(FieldStudentID) }
func (g *GradeRecord) Year() string        { return g.getField(FieldYear) }
func (g *GradeRecord) Class() string       { return g.getField(FieldClass) }
func (g *GradeRecord) SeatNumber() string  { return g.getField(FieldSeatNumber) }
func (g *GradeRecord) StudentName() string { return g.getField(FieldStudentName) }
func (g *GradeRecord) Group() string       { return g.getField(FieldGroup) }
func (g *GradeRecord) Score() string       { return g.getField(FieldScore) }

func (g *GradeRecord) getField(index int) string {
	if index < len(g.Row) {
		return strings.TrimSpace(g.Row[index])
	}
	return ""
}

// 教師資料
type TeacherRecord struct {
	SubjectCode string // 科目代號
	SubjectName string // 科目名稱
	Year        string // 年級
	Class       string // 班級
	Group       string // 組別
	TeacherName string // 老師名稱
	Email       string // email
}

// 用於分組的鍵值
type ReportKey struct {
	TeacherName string
	Year        string
	SubjectCode string
	SubjectName string
}

// Email 設定
type EmailSettings struct {
	SenderEmail     string `json:"senderEmail"`
	ShouldSendEmail string `json:"shouldSendEmail"`
	EmailTitle      string `json:"emailTitle"`
}

// CSV 標題列
var csvHeaders []string

func main() {
	fmt.Println("成績報表整理工具 (CSV 版本)")
	fmt.Println("============================")

	// 讀取成績資料
	grades, err := readGradeCSV("data/grade.csv")
	if err != nil {
		log.Fatalf("讀取成績檔案失敗: %v", err)
	}
	fmt.Printf("成功讀取 %d 筆成績資料\n", len(grades))

	// 讀取教師資料
	teachers, err := readTeacherCSV("data/teacher.csv")
	if err != nil {
		log.Fatalf("讀取教師檔案失敗: %v", err)
	}
	fmt.Printf("成功讀取 %d 筆教師資料\n", len(teachers))

	// 建立教師對應表 (科目代號+年級+班級 -> 教師，忽略組別)
	teacherMap := buildTeacherMap(teachers)

	// 將學生成績與教師配對
	reportData, errorRecords := matchStudentsToTeachers(grades, teacherMap)
	fmt.Printf("成功配對 %d 個報表群組\n", len(reportData))

	if len(errorRecords) > 0 {
		fmt.Printf("警告: %d 筆資料找不到對應教師\n", len(errorRecords))
		// 產生錯誤記錄檔案
		if err := generateErrorCSV(errorRecords); err != nil {
			log.Printf("產生錯誤記錄檔案失敗: %v", err)
		}
	}

	// 產生 CSV 報表
	reportFiles := generateCSVReports(reportData)
	if len(reportFiles) == 0 {
		log.Fatal("沒有產生任何報表")
	}

	fmt.Println("報表產生完成！")

	// 讀取 email 設定
	settings, err := readEmailSettings("data/setting.json")
	if err != nil {
		log.Printf("讀取 Email 設定失敗: %v", err)
		return
	}

	// 檢查是否要發送 email
	if settings.ShouldSendEmail == "true" {
		fmt.Println("\n開始發送 Email...")
		// 按教師分組報表檔案
		teacherFiles := groupReportsByTeacher(reportFiles, reportData, teachers)

		// 發送 email 給每位教師
		for teacherEmail, files := range teacherFiles {
			teacherName := getTeacherNameByEmail(teachers, teacherEmail)
			err := sendEmail(settings, teacherEmail, teacherName, files)
			if err != nil {
				log.Printf("發送 Email 給 %s (%s) 失敗: %v", teacherName, teacherEmail, err)
			} else {
				fmt.Printf("✓ 已發送 Email 給 %s (%s)，包含 %d 個附件\n", teacherName, teacherEmail, len(files))
			}
		}
		fmt.Println("Email 發送完成！")
	} else {
		fmt.Println("\nEmail 發送已停用（setting.json 中 shouldSendEmail 為 false）")
	}
}

// 讀取 grade.csv 檔案
func readGradeCSV(filename string) ([]GradeRecord, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // 允許欄位數量不同（成績後的欄位長度會變）
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var grades []GradeRecord
	// 第一列是標題
	if len(records) > 0 {
		csvHeaders = records[0]
		fmt.Printf("CSV 標題欄位數: %d\n", len(csvHeaders))
		fmt.Printf("固定欄位（索引 0-13）: %v\n", csvHeaders[0:min(14, len(csvHeaders))])
		if len(csvHeaders) > 14 {
			fmt.Printf("動態欄位（索引 14+）: %d 個欄位\n", len(csvHeaders)-14)
		}
	}

	// 跳過標題列，讀取資料
	for i := 1; i < len(records); i++ {
		row := records[i]
		if len(row) == 0 {
			continue
		}

		// 確保至少有基本識別欄位（到學生姓名為止，索引 0-6）
		// 這樣即使沒有組別、成績等欄位也能被包含
		if len(row) < FieldStudentName+1 {
			log.Printf("警告: 第 %d 列資料欄位不足 (需要至少 %d 個基本欄位，實際 %d 個)，已跳過",
				i+1, FieldStudentName+1, len(row))
			continue
		}

		// 如果欄位不足，補齊到標題列的長度
		if len(row) < len(csvHeaders) {
			// 補空白欄位
			paddedRow := make([]string, len(csvHeaders))
			copy(paddedRow, row)
			row = paddedRow
		}

		grade := GradeRecord{
			Row: row,
		}
		grades = append(grades, grade)
	}

	return grades, nil
}

// 讀取 teacher.csv 檔案
func readTeacherCSV(filename string) ([]TeacherRecord, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var teachers []TeacherRecord
	// 跳過標題列
	for i, record := range records {
		if i == 0 {
			continue
		}
		if len(record) < 7 {
			continue
		}

		teacher := TeacherRecord{
			SubjectCode: strings.TrimSpace(record[0]),
			SubjectName: strings.TrimSpace(record[1]),
			Year:        strings.TrimSpace(record[2]),
			Class:       strings.TrimSpace(record[3]),
			Group:       strings.TrimSpace(record[4]),
			TeacherName: strings.TrimSpace(record[5]),
			Email:       strings.TrimSpace(record[6]),
		}
		teachers = append(teachers, teacher)
	}

	return teachers, nil
}

// 建立教師對應表
func buildTeacherMap(teachers []TeacherRecord) map[string]TeacherRecord {
	teacherMap := make(map[string]TeacherRecord)
	for _, teacher := range teachers {
		// 組別 1, 2, 3 或空都視為沒有組別，配對時忽略組別
		// 使用 科目代號_年級_班級 作為 key
		key := fmt.Sprintf("%s_%s_%s",
			teacher.SubjectCode,
			teacher.Year,
			teacher.Class,
		)
		teacherMap[key] = teacher
	}
	return teacherMap
}

// 將學生成績與教師配對
func matchStudentsToTeachers(grades []GradeRecord, teacherMap map[string]TeacherRecord) (map[ReportKey][]GradeRecord, []GradeRecord) {
	reportData := make(map[ReportKey][]GradeRecord)
	var errorRecords []GradeRecord

	for _, grade := range grades {
		// 尋找對應的教師（忽略組別）
		key := fmt.Sprintf("%s_%s_%s",
			grade.SubjectCode(),
			grade.Year(),
			grade.Class(),
		)

		teacher, found := teacherMap[key]
		if !found {
			// 找不到對應教師，加入錯誤記錄
			errorRecords = append(errorRecords, grade)
			continue
		}

		// 建立報表鍵值 (教師+年級+科目)
		reportKey := ReportKey{
			TeacherName: teacher.TeacherName,
			Year:        grade.Year(),
			SubjectCode: grade.SubjectCode(),
			SubjectName: grade.SubjectName(),
		}

		reportData[reportKey] = append(reportData[reportKey], grade)
	}

	return reportData, errorRecords
}

// 產生錯誤記錄 CSV
func generateErrorCSV(errorRecords []GradeRecord) error {
	outputDir := "output"
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		return err
	}

	filename := filepath.Join(outputDir, "error.csv")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 寫入標題列
	if err := writer.Write(csvHeaders); err != nil {
		return err
	}

	// 寫入錯誤記錄
	for _, record := range errorRecords {
		if err := writer.Write(record.Row); err != nil {
			return err
		}
	}

	fmt.Printf("已產生錯誤記錄: %s (%d 筆資料)\n", filename, len(errorRecords))
	return nil
}

// 產生 CSV 報表，返回檔案名稱列表
func generateCSVReports(reportData map[ReportKey][]GradeRecord) map[ReportKey]string {
	// 建立輸出資料夾
	outputDir := "output"
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		log.Printf("建立輸出資料夾失敗: %v", err)
		return nil
	}

	reportFiles := make(map[ReportKey]string)

	for key, grades := range reportData {
		// 檔案名稱: 教師名稱_年級_科目.csv
		filename := filepath.Join(outputDir,
			fmt.Sprintf("%s_年級%s_%s.csv", key.TeacherName, key.Year, key.SubjectName))

		err := createCSVReport(filename, key, grades)
		if err != nil {
			log.Printf("產生報表失敗 (%s): %v", filename, err)
			continue
		}
		fmt.Printf("已產生: %s (%d 筆資料)\n", filename, len(grades))
		reportFiles[key] = filename
	}

	return reportFiles
}

// 建立單一 CSV 報表
func createCSVReport(filename string, key ReportKey, grades []GradeRecord) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 寫入標題列（使用原始 CSV 的標題）
	if err := writer.Write(csvHeaders); err != nil {
		return err
	}

	// 依班級和座號排序
	sort.Slice(grades, func(i, j int) bool {
		if grades[i].Class() != grades[j].Class() {
			return grades[i].Class() < grades[j].Class()
		}
		// 嘗試將座號轉為數字比較
		seatI, errI := strconv.Atoi(grades[i].SeatNumber())
		seatJ, errJ := strconv.Atoi(grades[j].SeatNumber())
		if errI == nil && errJ == nil {
			return seatI < seatJ
		}
		return grades[i].SeatNumber() < grades[j].SeatNumber()
	})

	// 寫入資料（完整列）
	for _, grade := range grades {
		if err := writer.Write(grade.Row); err != nil {
			return err
		}
	}

	return nil
}

// 輔助函數：取最小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 讀取 Email 設定
func readEmailSettings(filename string) (*EmailSettings, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var settings EmailSettings
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// 按教師分組報表檔案
func groupReportsByTeacher(reportFiles map[ReportKey]string, reportData map[ReportKey][]GradeRecord, teachers []TeacherRecord) map[string][]string {
	teacherFiles := make(map[string][]string)

	for key, filename := range reportFiles {
		// 從 reportData 中取得任一筆資料來找教師
		if len(reportData[key]) > 0 {
			grade := reportData[key][0]
			// 找對應的教師
			matchKey := fmt.Sprintf("%s_%s_%s",
				grade.SubjectCode(),
				grade.Year(),
				grade.Class(),
			)

			for _, teacher := range teachers {
				teacherKey := fmt.Sprintf("%s_%s_%s",
					teacher.SubjectCode,
					teacher.Year,
					teacher.Class,
				)
				if teacherKey == matchKey {
					teacherFiles[teacher.Email] = append(teacherFiles[teacher.Email], filename)
					break
				}
			}
		}
	}

	return teacherFiles
}

// 根據 Email 取得教師名稱
func getTeacherNameByEmail(teachers []TeacherRecord, email string) string {
	for _, teacher := range teachers {
		if teacher.Email == email {
			return teacher.TeacherName
		}
	}
	return email
}

// 發送 Email
func sendEmail(settings *EmailSettings, recipientEmail string, recipientName string, attachments []string) error {
	// 建立郵件內容
	subject := settings.EmailTitle
	body := fmt.Sprintf("您好 %s 老師，\n\n附件為您的成績報表，請查收。\n\n此為系統自動發送的郵件，請勿回覆。", recipientName)

	// 組合郵件標頭和內容
	message := fmt.Sprintf("From: %s\r\n", settings.SenderEmail)
	message += fmt.Sprintf("To: %s\r\n", recipientEmail)
	message += fmt.Sprintf("Subject: %s\r\n", subject)
	message += "\r\n"
	message += body

	// TODO: 實際的 SMTP 發送需要配置 SMTP 伺服器資訊
	// 這裡先模擬發送成功
	fmt.Printf("模擬發送 Email:\n")
	fmt.Printf("  寄件者: %s\n", settings.SenderEmail)
	fmt.Printf("  收件者: %s (%s)\n", recipientName, recipientEmail)
	fmt.Printf("  主旨: %s\n", subject)
	fmt.Printf("  附件: %v\n", attachments)

	// 實際發送時需要 SMTP 設定（host, port, username, password）
	// err := smtp.SendMail(smtpHost+":"+smtpPort, auth, settings.SenderEmail, []string{recipientEmail}, []byte(message))
	// return err

	return nil
}
