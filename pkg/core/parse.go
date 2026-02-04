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
}

func NewTraceManager() *TraceManager {
	return &TraceManager{
		Instructions: make([]*TraceLine, 0),
		CurrentIndex: 0,
	}
}

func (tm *TraceManager) GetCurrent() *TraceLine {
	if tm.CurrentIndex < 0 || tm.CurrentIndex >= len(tm.Instructions) {
		return nil
	}
	return tm.Instructions[tm.CurrentIndex]
}

func (tm *TraceManager) GetLine(index int) *TraceLine {
	if index >= 0 && index < len(tm.Instructions) {
		return tm.Instructions[index]
	}
	return nil
}

func (tm *TraceManager) Total() int {
	return len(tm.Instructions)
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

// 流式读取日志文件
func ReadTraceFile(filename string, callback func(*TraceLine)) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		traceLine, err := ParseLine(line)
		if err != nil {
			fmt.Println("解析错误:", err)
			continue
		}
		callback(traceLine)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func (tm *TraceManager) GetPrevLine() *TraceLine {
	if tm.CurrentIndex <= 0 || tm.CurrentIndex >= len(tm.Instructions) {
		return nil
	}
	return tm.Instructions[tm.CurrentIndex-1]
}

// 在 Next 和 Prev 方法中更新 PrevLine 缓存
func (tm *TraceManager) Next() bool {
	if tm.CurrentIndex < len(tm.Instructions)-1 {
		tm.PrevLine = tm.GetCurrent() // 缓存当前指令作为下一次的上一条
		tm.CurrentIndex++
		return true
	}
	return false
}

func (tm *TraceManager) Prev() bool {
	if tm.CurrentIndex > 0 {
		tm.CurrentIndex--
		// 更新 PrevLine，现在上一条是索引-2
		if tm.CurrentIndex-1 >= 0 {
			tm.PrevLine = tm.Instructions[tm.CurrentIndex-1]
		} else {
			tm.PrevLine = nil
		}
		return true
	}
	return false
}

func (tm *TraceManager) GoTo(index int) bool {
	if index >= 0 && index < len(tm.Instructions) {
		// 更新 PrevLine
		if index-1 >= 0 {
			tm.PrevLine = tm.Instructions[index-1]
		} else {
			tm.PrevLine = nil
		}
		tm.CurrentIndex = index
		return true
	}
	return false
}

// 修改 AddInstruction 方法
func (tm *TraceManager) AddInstruction(t *TraceLine) {
	tm.Instructions = append(tm.Instructions, t)
}
