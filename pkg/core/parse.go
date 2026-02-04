package core

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// TraceLine 表示日志中的一条指令快照
type TraceLine struct {
	Step   uint32
	Addr   uint64
	Offset uint64
	Instr  string
	Regs   [31]uint64 // x0-x30
	SP     uint64
	PC     uint64
}

// TraceManager 管理指令跟踪
type TraceManager struct {
	Instructions []*TraceLine
	PrevLine     *TraceLine // 添加上一条指令的缓存
	CurrentIndex int
	totalLines   int    // 文件总行数（可能大于Instructions长度）
	LoadedRange  [2]int // 已加载的范围[start, end)

	FileName   string // 新增：记录文件名
	windowSize int    // 新增：窗口大小
	isLoading  bool   // 新增：防止重复加载
}

func NewTraceManager() *TraceManager {
	return &TraceManager{
		Instructions: make([]*TraceLine, 0),
		CurrentIndex: 0,
		totalLines:   0,
		LoadedRange:  [2]int{-1, -1},
		windowSize:   2000, // 默认窗口大小
		isLoading:    false,
	}
}

func (tm *TraceManager) LoadWindow(center int) error {
	if tm.FileName == "" || tm.isLoading {
		return nil
	}

	tm.isLoading = true
	defer func() { tm.isLoading = false }()

	// 计算窗口范围
	halfWindow := tm.windowSize / 2
	start := center - halfWindow
	if start < 0 {
		start = 0
	}
	end := start + tm.windowSize
	if end > tm.totalLines {
		end = tm.totalLines
		start = end - tm.windowSize
		if start < 0 {
			start = 0
		}
	}

	// 如果窗口已经加载，直接返回
	if start == tm.LoadedRange[0] && end == tm.LoadedRange[1] {
		return nil
	}

	return tm.loadFileWindow(start, end)
}

func (tm *TraceManager) loadFileWindow(start, end int) error {
	file, err := os.Open(tm.FileName)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// 清空现有指令
	tm.Instructions = make([]*TraceLine, 0)

	// 扫描并加载指定范围的行
	currentLine := 0
	for scanner.Scan() {
		if currentLine >= start && currentLine < end {
			line := scanner.Text()
			traceLine, err := ParseLine(line)
			if err != nil {
				fmt.Printf("解析错误 第%d行: %v\n", currentLine+1, err)
				// 即使解析错误，也添加一个占位符
				tm.Instructions = append(tm.Instructions, nil)
			} else {
				tm.Instructions = append(tm.Instructions, traceLine)
			}
		}
		currentLine++

		// 如果已经过了end，就停止
		if currentLine >= end {
			break
		}
	}

	// 记录加载范围
	tm.LoadedRange = [2]int{start, end}

	// 如果当前索引不在新窗口内，调整当前索引到窗口中间
	if tm.CurrentIndex < start || tm.CurrentIndex >= end {
		tm.CurrentIndex = start + len(tm.Instructions)/2
	}

	return scanner.Err()
}

func (tm *TraceManager) GetCurrent() *TraceLine {
	// 获取窗口内的索引
	windowIndex := tm.CurrentIndex - tm.LoadedRange[0]

	// 检查索引是否在有效范围内
	if tm.LoadedRange[0] <= tm.CurrentIndex &&
		tm.CurrentIndex < tm.LoadedRange[1] &&
		windowIndex >= 0 &&
		windowIndex < len(tm.Instructions) {
		return tm.Instructions[windowIndex]
	}

	return nil
}
func (tm *TraceManager) GetLine(index int) *TraceLine {
	// 检查索引是否在已加载范围内
	if index >= tm.LoadedRange[0] && index < tm.LoadedRange[1] {
		windowIndex := index - tm.LoadedRange[0]
		if windowIndex >= 0 && windowIndex < len(tm.Instructions) {
			return tm.Instructions[windowIndex]
		}
	}

	// 如果请求的行不在当前窗口，但还在文件范围内
	if index >= 0 && index < tm.totalLines {
		// 触发异步加载（但不阻塞返回）
		go tm.LoadWindow(index)
		return nil
	}

	return nil
}

func (tm *TraceManager) Total() int {
	return tm.totalLines
}

// ParseLine 解析日志中的一行
func ParseLine(line string) (*TraceLine, error) {
	fields := strings.Split(line, "|")
	if len(fields) != 37 {
		return nil, fmt.Errorf("字段数量不对: %d", len(fields))
	}

	t := &TraceLine{}

	// step
	step, err := strconv.ParseUint(strings.TrimSpace(fields[0]), 16, 32)
	if err != nil {
		return nil, fmt.Errorf("解析 step 失败: %v", err)
	}
	t.Step = uint32(step)

	// addr
	addr, err := strconv.ParseUint(strings.TrimSpace(fields[1]), 0, 64)
	if err != nil {
		return nil, fmt.Errorf("解析 addr 失败: %v", err)
	}
	t.Addr = addr

	// offset
	offset, err := strconv.ParseUint(strings.TrimSpace(fields[2]), 0, 64)
	if err != nil {
		return nil, fmt.Errorf("解析 offset 失败: %v", err)
	}
	t.Offset = offset

	// instr
	t.Instr = strings.TrimSpace(fields[3])
	if strings.HasPrefix(t.Instr, "\"") && strings.HasSuffix(t.Instr, "\"") {
		t.Instr = t.Instr[1 : len(t.Instr)-1]
	}

	// x0-x28
	for i := 0; i <= 28; i++ {
		val, err := strconv.ParseUint(strings.TrimSpace(fields[4+i]), 0, 64)
		if err != nil {
			return nil, fmt.Errorf("解析 x%d 失败: %v", i, err)
		}
		t.Regs[i] = val
	}

	// x29
	val, err := strconv.ParseUint(strings.TrimSpace(fields[33]), 0, 64)
	if err != nil {
		return nil, fmt.Errorf("解析 x29 失败: %v", err)
	}
	t.Regs[29] = val

	// x30
	val, err = strconv.ParseUint(strings.TrimSpace(fields[34]), 0, 64)
	if err != nil {
		return nil, fmt.Errorf("解析 x30 失败: %v", err)
	}
	t.Regs[30] = val

	// sp
	val, err = strconv.ParseUint(strings.TrimSpace(fields[35]), 0, 64)
	if err != nil {
		return nil, fmt.Errorf("解析 sp 失败: %v", err)
	}
	t.SP = val

	// pc
	val, err = strconv.ParseUint(strings.TrimSpace(fields[36]), 0, 64)
	if err != nil {
		return nil, fmt.Errorf("解析 pc 失败: %v", err)
	}
	t.PC = val

	return t, nil
}

// 流式读取日志文件，但只加载一部分
func ReadTraceFile(filename string, tm *TraceManager) error {
	// 首先统计总行数
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// 快速统计行数
	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}
	tm.totalLines = lineCount

	// 保存文件名
	tm.FileName = filename

	// 加载初始窗口（以第0行为中心）
	return tm.LoadWindow(0)
}

func (tm *TraceManager) GetPrevLine() *TraceLine {
	if tm.CurrentIndex <= 0 || tm.CurrentIndex >= len(tm.Instructions) {
		return nil
	}
	return tm.Instructions[tm.CurrentIndex-1]
}

// 在 Next 和 Prev 方法中更新 PrevLine 缓存
// 修改 Next、Prev、GoTo 方法，确保窗口跟随
func (tm *TraceManager) Next() bool {
	if tm.CurrentIndex < tm.totalLines-1 {
		tm.PrevLine = tm.GetCurrent()
		tm.CurrentIndex++

		// 检查是否需要滑动窗口
		windowEnd := tm.LoadedRange[1]
		if tm.CurrentIndex >= windowEnd-100 { // 接近窗口末尾时滑动
			go tm.LoadWindow(tm.CurrentIndex)
		}
		return true
	}
	return false
}

func (tm *TraceManager) Prev() bool {
	if tm.CurrentIndex > 0 {
		tm.CurrentIndex--
		if tm.CurrentIndex-1 >= 0 {
			tm.PrevLine = tm.GetLine(tm.CurrentIndex - 1)
		} else {
			tm.PrevLine = nil
		}

		// 检查是否需要滑动窗口
		windowStart := tm.LoadedRange[0]
		if tm.CurrentIndex <= windowStart+100 { // 接近窗口开头时滑动
			go tm.LoadWindow(tm.CurrentIndex)
		}
		return true
	}
	return false
}

func (tm *TraceManager) GoTo(index int) bool {
	if index >= 0 && index < tm.totalLines {
		// 更新 PrevLine
		if index-1 >= 0 {
			tm.PrevLine = tm.GetLine(index - 1)
		} else {
			tm.PrevLine = nil
		}
		tm.CurrentIndex = index

		// 加载以目标行为中心的窗口
		go tm.LoadWindow(index)

		return true
	}
	return false
}

// 修改 AddInstruction 方法
func (tm *TraceManager) AddInstruction(t *TraceLine) {
	tm.Instructions = append(tm.Instructions, t)
	tm.totalLines = len(tm.Instructions)
}
