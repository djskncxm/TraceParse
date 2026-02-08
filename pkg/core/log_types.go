package core

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LogEntry 表示一条日志记录
type LogEntry struct {
	Step    int
	Content string
}

// BL 跳转日志条目
type BLLogEntry struct {
	LogEntry
	Address   string
	Function  string
	MemoryHex []string
}

// RW 读写日志条目
type RWLogEntry struct {
	LogEntry
	Type      string // "r" 或 "w"
	Address   string
	Offset    string
	MemoryHex []string
}

// LogManager 管理 BL 和 RW 日志
type LogManager struct {
	BlLogs map[int][]*BLLogEntry // 按步数索引的 BL 日志
	RwLogs map[int][]*RWLogEntry // 按步数索引的 RW 日志
}

func NewLogManager() *LogManager {
	return &LogManager{
		BlLogs: make(map[int][]*BLLogEntry),
		RwLogs: make(map[int][]*RWLogEntry),
	}
}

// ParseBLLine 解析 BL 日志行
func ParseBLLine(line string) (*BLLogEntry, error) {
	// 解析 BL 日志格式: "19584: [0x7fda1a4240][0]: __memset_chk"
	parts := strings.SplitN(line, ":", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid BL log format")
	}

	// 解析步数
	stepStr := strings.TrimSpace(parts[0])
	step, err := strconv.Atoi(stepStr)
	if err != nil {
		return nil, fmt.Errorf("invalid step number: %v", err)
	}

	// 解析地址和函数名
	content := strings.TrimSpace(parts[1])

	// 尝试提取地址和函数名
	var address, function string
	if idx := strings.Index(content, "["); idx != -1 && strings.Contains(content, "]") {
		// 提取 [address] 部分
		endIdx := strings.Index(content, "]")
		address = content[idx+1 : endIdx]

		// 提取函数名（在最后一个 : 之后）
		if colonIdx := strings.LastIndex(content, ":"); colonIdx != -1 {
			function = strings.TrimSpace(content[colonIdx+1:])
		}
	}

	return &BLLogEntry{
		LogEntry: LogEntry{
			Step:    step,
			Content: line,
		},
		Address:  address,
		Function: function,
	}, nil
}

// ParseRWLine 解析 RW 日志行
func ParseRWLine(line string) (*RWLogEntry, error) {
	// 解析 RW 日志格式: "1: (w)(0x7fda1a4210+0x8)"
	parts := strings.SplitN(line, ":", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid RW log format")
	}

	// 解析步数
	stepStr := strings.TrimSpace(parts[0])
	step, err := strconv.Atoi(stepStr)
	if err != nil {
		// bug
		return nil, fmt.Errorf("invalid step number: %v", err)
	}

	// 解析类型、地址和偏移量
	content := strings.TrimSpace(parts[1])

	var logType, address, offset string
	if strings.HasPrefix(content, "(w)") {
		logType = "w"
	} else if strings.HasPrefix(content, "(r)") {
		logType = "r"
	}

	// 提取地址和偏移量
	startIdx := strings.Index(content, "(")
	endIdx := strings.LastIndex(content, ")")
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		addrPart := content[startIdx+1 : endIdx]
		if plusIdx := strings.Index(addrPart, "+"); plusIdx != -1 {
			address = strings.TrimSpace(addrPart[:plusIdx])
			offset = strings.TrimSpace(addrPart[plusIdx+1:])
		} else {
			address = addrPart
		}
	}

	return &RWLogEntry{
		LogEntry: LogEntry{
			Step:    step,
			Content: line,
		},
		Type:    logType,
		Address: address,
		Offset:  offset,
	}, nil
}

// LoadBLLog 加载 BL 日志文件
func (lm *LogManager) LoadBLLog(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentEntry *BLLogEntry
	var memoryLines []string

	for scanner.Scan() {
		line := scanner.Text()

		// 跳过空行
		if strings.TrimSpace(line) == "" {
			continue
		}

		// 检查是否是新的 BL 条目（包含地址格式）
		if strings.Contains(line, "[0x") && strings.Contains(line, "]:") {
			// 保存前一个条目（如果有）
			if currentEntry != nil {
				currentEntry.MemoryHex = memoryLines
				lm.BlLogs[currentEntry.Step] = append(lm.BlLogs[currentEntry.Step], currentEntry)
			}

			// 解析新条目
			entry, err := ParseBLLine(line)
			if err != nil {
				// 跳转也同样不一定每行都有跳转，不应该解析不到到就报错，继续下一行即可
				// fmt.Printf("Error parsing BL log line: %v\n", err)
				continue
			}

			currentEntry = entry
			memoryLines = []string{}
		} else if currentEntry != nil && strings.Contains(line, "|") {
			// 内存十六进制行
			memoryLines = append(memoryLines, line)
		}
	}

	// 保存最后一个条目
	if currentEntry != nil {
		currentEntry.MemoryHex = memoryLines
		lm.BlLogs[currentEntry.Step] = append(lm.BlLogs[currentEntry.Step], currentEntry)
	}

	return scanner.Err()
}

// LoadRWLog 加载 RW 日志文件
func (lm *LogManager) LoadRWLog(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentEntry *RWLogEntry
	var memoryLines []string

	for scanner.Scan() {
		line := scanner.Text()

		// 跳过空行
		if strings.TrimSpace(line) == "" {
			continue
		}

		// 检查是否是新的 RW 条目（包含 (r) 或 (w)）
		if (strings.Contains(line, "(r)") || strings.Contains(line, "(w)")) && strings.Contains(line, ":") {
			// 保存前一个条目（如果有）
			if currentEntry != nil {
				currentEntry.MemoryHex = memoryLines
				lm.RwLogs[currentEntry.Step] = append(lm.RwLogs[currentEntry.Step], currentEntry)
			}

			// 解析新条目
			entry, err := ParseRWLine(line)
			if err != nil {
				// 也不是每行都有对应的内存读写，暂时注释，日后寻找更合适的解决方法
				// fmt.Printf("Error parsing RW log line: %v\n", err)
				continue
			}

			currentEntry = entry
			memoryLines = []string{}
		} else if currentEntry != nil && strings.Contains(line, "|") {
			// 内存十六进制行
			memoryLines = append(memoryLines, line)
		}
	}

	// 保存最后一个条目
	if currentEntry != nil {
		currentEntry.MemoryHex = memoryLines
		lm.RwLogs[currentEntry.Step] = append(lm.RwLogs[currentEntry.Step], currentEntry)
	}

	return scanner.Err()
}

// GetBLLogsForStep 获取指定步数的 BL 日志
func (lm *LogManager) GetBLLogsForStep(step int) []*BLLogEntry {
	if logs, exists := lm.BlLogs[step]; exists {
		return logs
	}

	// 如果没有精确匹配，查找最接近的小于等于当前步数的日志
	var nearestLogs []*BLLogEntry
	nearestStep := -1

	for s := range lm.BlLogs {
		if s <= step && s > nearestStep {
			nearestStep = s
			nearestLogs = lm.BlLogs[s]
		}
	}

	return nearestLogs
}

// GetRWLogsForStep 获取指定步数的 RW 日志
func (lm *LogManager) GetRWLogsForStep(step int) []*RWLogEntry {
	if logs, exists := lm.RwLogs[step]; exists {
		return logs
	}

	// 如果没有精确匹配，查找最接近的小于等于当前步数的日志
	var nearestLogs []*RWLogEntry
	nearestStep := -1

	for s := range lm.RwLogs {
		if s <= step && s > nearestStep {
			nearestStep = s
			nearestLogs = lm.RwLogs[s]
		}
	}

	return nearestLogs
}
