package core

import (
	"fmt"
	"strconv"
	"strings"
)

type CommandType int

const (
	CmdNext CommandType = iota
	CmdPrev
	CmdGoTo
	CmdReg
	CmdClear
	CmdHelp
	CmdQuit
	CmdRun  // 添加运行命令
	CmdStop // 添加停止命令
	CmdStep // 添加步进命令
)

type Command struct {
	Type CommandType
	Args []string
	Raw  string
}

type User struct {
	CurrentLine     int
	TraceManager    *TraceManager
	IsRunning       bool
	AutoStep        bool
	StepDelay       int      // 毫秒
	LastCommand     *Command // 添加上一个命令
	RepeatCount     int      // 重复次数计数
	RegDetector     *RegisterChangeDetector
}

func NewUser(tm *TraceManager) *User {
	return &User{
		CurrentLine:  0,
		TraceManager: tm,
		IsRunning:    false,
		AutoStep:     false,
		StepDelay:    100,
		LastCommand:  nil,
		RepeatCount:  0,
		RegDetector:  NewRegisterChangeDetector(),
	}
}

func (u *User) ParseCommand(cmd string) *Command {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		// 如果输入为空，返回上一个命令（如果有）
		if u.LastCommand != nil && u.RepeatCount > 0 {
			u.RepeatCount++
			// 为跳转命令特殊处理：不重复带参数的跳转
			if u.LastCommand.Type == CmdGoTo {
				return nil
			}
			return u.LastCommand
		}
		return nil
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}

	command := &Command{
		Raw:  cmd,
		Args: parts[1:],
	}

	// 检查是否是带次数的命令（例如：n5, p3）
	if len(parts) == 1 {
		// 尝试解析 n5 这种格式
		baseCmd := parts[0]
		if len(baseCmd) > 1 {
			lastChar := baseCmd[len(baseCmd)-1]
			if lastChar >= '0' && lastChar <= '9' {
				// 可能是带数字的命令
				cmdPart := baseCmd[:len(baseCmd)-1]
				numPart := baseCmd[len(baseCmd)-1:]
				if count, err := strconv.Atoi(numPart); err == nil && count > 0 {
					// 检查命令类型
					switch cmdPart {
					case "n", "next":
						command.Type = CmdNext
						command.Args = []string{strconv.Itoa(count)}
					case "p", "prev", "previous":
						command.Type = CmdPrev
						command.Args = []string{strconv.Itoa(count)}
					}
					if command.Type == CmdNext || command.Type == CmdPrev {
						u.LastCommand = command
						u.RepeatCount = 1
						return command
					}
				}
			}
		}
	}

	switch parts[0] {
	case "n", "next":
		command.Type = CmdNext
		if len(parts) > 1 {
			command.Args = []string{parts[1]}
		}
	case "p", "prev", "previous":
		command.Type = CmdPrev
		if len(parts) > 1 {
			command.Args = []string{parts[1]}
		}
	case "g", "goto":
		command.Type = CmdGoTo
	case "r", "reg", "registers":
		command.Type = CmdReg
	case "c", "clear":
		command.Type = CmdClear
	case "h", "help", "?":
		command.Type = CmdHelp
	case "q", "quit", "exit":
		command.Type = CmdQuit
	case "run":
		command.Type = CmdRun
		u.AutoStep = true
	case "stop":
		command.Type = CmdStop
		u.AutoStep = false
	case "step":
		command.Type = CmdStep
		if len(parts) > 1 {
			if delay, err := strconv.Atoi(parts[1]); err == nil && delay > 0 {
				u.StepDelay = delay
			}
		}
	default:
		// 尝试解析为数字，直接跳转到该行
		if _, err := strconv.Atoi(parts[0]); err == nil {
			command.Type = CmdGoTo
			command.Args = []string{parts[0]}
		}
	}

	// 保存当前命令（除了help、quit等）
	if command.Type == CmdNext || command.Type == CmdPrev || command.Type == CmdRun {
		u.LastCommand = command
		u.RepeatCount = 1
	} else {
		u.LastCommand = nil
		u.RepeatCount = 0
	}

	return command
}

func (u *User) GetCurrentInstruction() *TraceLine {
	return u.TraceManager.GetCurrent()
}

func (u *User) GetRegisterInfo() string {
	if u.TraceManager.GetCurrent() == nil {
		return "No instruction loaded"
	}

	t := u.TraceManager.GetCurrent()
	// prev := u.TraceManager.GetPrevLine()
	
	// 更新寄存器变化检测
	changes := u.RegDetector.Update(t)
	
	var sb strings.Builder

	sb.WriteString("Registers:\n")
	for i := 0; i < 31; i++ {
		if i%4 == 0 && i > 0 {
			sb.WriteString("\n")
		}

		// 检查寄存器值是否变化
		regChanged := false
		if changes[i] {
			regChanged = true
		}

		// 寄存器 x0 可能永远为 0，所以特殊处理
		if i == 0 && t.Regs[i] == 0 {
			sb.WriteString(fmt.Sprintf("[gray]x%2d = 0x%016x[-]  ", i, t.Regs[i]))
		} else if regChanged {
			// 变化的寄存器用黄色高亮
			sb.WriteString(fmt.Sprintf("[yellow]x%2d = 0x%016x[-]  ", i, t.Regs[i]))
		} else {
			sb.WriteString(fmt.Sprintf("x%2d = 0x%016x  ", i, t.Regs[i]))
		}
	}

	sb.WriteString("\n\nSpecial Registers:\n")

	// 检查 SP 是否变化
	spChanged := changes[31]

	// 检查 PC 是否变化
	pcChanged := changes[32]

	if spChanged {
		sb.WriteString(fmt.Sprintf("[yellow]SP  = 0x%016x[-]\n", t.SP))
	} else {
		sb.WriteString(fmt.Sprintf("SP  = 0x%016x\n", t.SP))
	}

	if pcChanged {
		sb.WriteString(fmt.Sprintf("[yellow]PC  = 0x%016x[-]", t.PC))
	} else {
		sb.WriteString(fmt.Sprintf("PC  = 0x%016x", t.PC))
	}

	// 添加命令重复信息
	if u.LastCommand != nil && u.RepeatCount > 1 {
		sb.WriteString(fmt.Sprintf("\n\n[gray]Repeating: %s (x%d)[-]", u.LastCommand.Raw, u.RepeatCount))
	}

	return sb.String()
}

func (u *User) GetStatusInfo() string {
	current := u.TraceManager.GetCurrent()
	if current == nil {
		return "No instruction loaded"
	}

	info := fmt.Sprintf("Step: %d | Addr: 0x%x | Total: %d | Current: %d",
		current.Step, current.Addr, u.TraceManager.Total(), u.TraceManager.CurrentIndex+1)

	// 添加命令信息
	if u.LastCommand != nil && u.RepeatCount > 1 {
		info += fmt.Sprintf(" | Repeating: %s (x%d)", u.LastCommand.Raw, u.RepeatCount)
	} else if u.LastCommand != nil {
		info += fmt.Sprintf(" | Last: %s", u.LastCommand.Raw)
	}

	if u.AutoStep {
		info += " | [yellow]Auto-step ON[-]"
	}

	return info
}

func (u *User) ExecuteCommand(cmd *Command) (string, bool) {
	if cmd == nil {
		return "", false
	}

	var message string
	var updated bool

	switch cmd.Type {
	case CmdNext:
		count := 1
		if len(cmd.Args) > 0 {
			if n, err := strconv.Atoi(cmd.Args[0]); err == nil && n > 1 {
				count = n
			}
		}

		for i := 0; i < count; i++ {
			if u.TraceManager.Next() {
				updated = true
				if count > 1 && i == count-1 {
					message = fmt.Sprintf("Stepped %d instructions forward", count)
				}
			} else {
				if i == 0 {
					message = "Already at last instruction"
				} else {
					message = fmt.Sprintf("Stepped %d instructions (reached end)", i)
				}
				break
			}
		}

		if count == 1 && updated && message == "" {
			message = "Stepped to next instruction"
		}

	case CmdPrev:
		count := 1
		if len(cmd.Args) > 0 {
			if n, err := strconv.Atoi(cmd.Args[0]); err == nil && n > 1 {
				count = n
			}
		}

		for i := 0; i < count; i++ {
			if u.TraceManager.Prev() {
				updated = true
				if count > 1 && i == count-1 {
					message = fmt.Sprintf("Stepped %d instructions backward", count)
				}
			} else {
				if i == 0 {
					message = "Already at first instruction"
				} else {
					message = fmt.Sprintf("Stepped %d instructions (reached beginning)", i)
				}
				break
			}
		}

		if count == 1 && updated && message == "" {
			message = "Stepped to previous instruction"
		}

	case CmdGoTo:
		if len(cmd.Args) > 0 {
			if line, err := strconv.Atoi(cmd.Args[0]); err == nil {
				if u.TraceManager.GoTo(line) {
					message = fmt.Sprintf("Jumped to line %d", line)
					updated = true
				} else {
					message = fmt.Sprintf("Invalid line number: %d", line)
				}
			}
		} else {
			message = "Please specify a line number"
		}

	case CmdRun:
		u.AutoStep = true
		message = "Auto-step started. Press 'stop' to stop."
		updated = true

	case CmdStop:
		u.AutoStep = false
		message = "Auto-step stopped"
		updated = true

	case CmdStep:
		message = fmt.Sprintf("Step delay set to %d ms", u.StepDelay)

	case CmdQuit:
		message = "Quitting..."

	default:
		message = ""
	}

	return message, updated
}
