package utils

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const csvCountryHeader = "国家/地区"

// alignCSVCountryColumn 在追加前把缺少「国家/地区」列的旧版 CSV 补齐：表头插入新列，
// 历史行按各自的「地区码」补上国家名，其余列保持原顺序，这样新行和旧行始终对齐。
// 只读表头判断，已对齐的文件不会被重读；空文件或无法解析的旧文件一律原样保留。
func alignCSVCountryColumn(path string) error {
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		return nil
	}
	header, err := readCSVHeaderLine(path)
	if err != nil || len(header) == 0 {
		return nil
	}
	if _, aligned := csvCountryInsertColumn(header); aligned {
		return nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	hasBOM := bytes.HasPrefix(raw, []byte(utf8BOM))
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte(utf8BOM))))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil || len(records) == 0 {
		// 读不懂的旧文件不重写：列不齐也比重写一份用户看不懂的内容安全。
		return nil
	}

	insertAt, _ := csvCountryInsertColumn(records[0])
	// 新列插在「地区码」之后，所以先取旧表头的地区码下标，插入后它仍然有效。
	coloIndex := coloIndexOf(records[0])
	records[0] = insertCSVCell(records[0], insertAt, csvCountryHeader)
	for index := 1; index < len(records); index++ {
		country := "N/A"
		if coloIndex >= 0 && coloIndex < len(records[index]) {
			country = ColoCountryName(records[index][coloIndex])
		}
		records[index] = insertCSVCell(records[index], insertAt, country)
	}

	var buffer bytes.Buffer
	if hasBOM {
		buffer.WriteString(utf8BOM)
	}
	if err := csv.NewWriter(&buffer).WriteAll(records); err != nil {
		return err
	}
	return replaceFileAtomically(path, buffer.Bytes(), info.Mode())
}

func readCSVHeaderLine(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	// 表头没有内嵌换行，读一行就够；整份读入留到确实需要重写时再做。
	line, err := bufio.NewReader(file).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	records, err := csv.NewReader(strings.NewReader(line)).Read()
	if err != nil {
		return nil, err
	}
	return records, nil
}

// csvCountryInsertColumn 返回「国家/地区」应写入的列下标；aligned 为 true 表示表头已有该列。
func csvCountryInsertColumn(header []string) (index int, aligned bool) {
	coloIndex := coloIndexOf(header)
	for _, name := range header {
		if strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")) == csvCountryHeader {
			return 0, true
		}
	}
	if coloIndex >= 0 {
		return coloIndex + 1, false
	}
	return len(header), false
}

func coloIndexOf(header []string) int {
	for index, name := range header {
		if strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")) == "地区码" {
			return index
		}
	}
	return -1
}

// insertCSVCell 在 index 处插入 value；行比 index 短时先补空列，避免旧文件残缺行错位。
func insertCSVCell(record []string, index int, value string) []string {
	if index > len(record) {
		record = append(record, make([]string, index-len(record))...)
	}
	result := make([]string, 0, len(record)+1)
	result = append(result, record[:index]...)
	result = append(result, value)
	return append(result, record[index:]...)
}

func replaceFileAtomically(path string, raw []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if _, err := temp.Write(raw); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Chmod(mode.Perm()); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
