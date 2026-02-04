package tui

import (
	"fmt"
	"github.com/djskncxm/TraceParse/pkg/core"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"strings"
	"time"
)

// AppState 管理应用状态
type AppState struct {
	TraceManager *core.TraceManager
	User         *core.User
	App          *tview.Application
	AsmView      *tview.TextView
	RegView      *tview.TextView
	StatusView   *tview.TextView
	InputField   *tview.InputField
	AutoStepChan chan bool // 用于控制自动步进
	MemoryView   *tview.TextView
}

func NewAsmView() *tview.TextView {
	asmView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false)
	asmView.SetBorder(true).SetTitle("Assembly Instructions")
	asmView.SetBackgroundColor(tcell.ColorDefault)
	return asmView
}

func NewRegView() *tview.TextView {
	regView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	regView.SetBorder(true).SetTitle("Registers")
	regView.SetBackgroundColor(tcell.ColorDefault)
	return regView
}

func NewStatusView() *tview.TextView {
	statusView := tview.NewTextView().
		SetDynamicColors(true)
	statusView.SetBorder(true).SetTitle("Status")
	statusView.SetBackgroundColor(tcell.ColorDefault)
	return statusView
}

func NewMemoryView() *tview.TextView {
	memoryView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	memoryView.SetBorder(true).SetTitle("Memory")
	memoryView.SetBackgroundColor(tcell.ColorDefault)
	memoryView.SetText("Memory view - Not implemented yet")
	return memoryView
}

func NewInputView(state *AppState) *tview.InputField {
	inputField := tview.NewInputField().
		SetLabel("Command: ").
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorDarkSlateGray)

	inputField.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		cmd := strings.TrimSpace(inputField.GetText())
		inputField.SetText("")

		if cmd == "" {
			return
		}

		// 解析命令
		command := state.User.ParseCommand(cmd)

		// 更新显示
		UpdateDisplay(state, command)

		// 处理特殊命令
		if command != nil {
			switch command.Type {
			case core.CmdRun:
				state.AutoStepChan <- true
			case core.CmdStop:
				state.AutoStepChan <- false
			case core.CmdQuit:
				state.App.Stop()
			}
		}
	})

	return inputField
}

func UpdateAsmView(state *AppState) {
	total := state.TraceManager.Total()
	currentIdx := state.TraceManager.CurrentIndex

	// 计算显示范围
	windowSize := 51 // 显示的行数
	start := currentIdx - windowSize/2
	if start < 0 {
		start = 0
	}
	end := start + windowSize
	if end > total {
		end = total
		start = end - windowSize
		if start < 0 {
			start = 0
		}
	}

	var sb strings.Builder
	for i := start; i < end; i++ {
		inst := state.TraceManager.GetLine(i)
		if inst == nil {
			continue
		}

		// 格式化指令行
		line := fmt.Sprintf("%4d | 0x%016x | %s", inst.Step, inst.Addr, inst.Instr)

		// 检查下一条指令（如果有）来判断哪些寄存器会被修改
		nextInst := state.TraceManager.GetLine(i + 1)
		if nextInst != nil {
			// 比较两个指令的寄存器值
			var changedRegs []string
			for reg := 0; reg < 31; reg++ {
				if inst.Regs[reg] != nextInst.Regs[reg] {
					changedRegs = append(changedRegs, fmt.Sprintf("x%d", reg))
				}
			}
			if inst.SP != nextInst.SP {
				changedRegs = append(changedRegs, "SP")
			}
			if inst.PC != nextInst.PC {
				changedRegs = append(changedRegs, "PC")
			}

			if len(changedRegs) > 0 {
				line += fmt.Sprintf(" [gray]→ %s[-]", strings.Join(changedRegs, ", "))
			}
		}

		// 高亮当前行
		if i == currentIdx {
			sb.WriteString(fmt.Sprintf("[yellow]▶ %s[white]\n", line))
		} else {
			sb.WriteString(fmt.Sprintf("  %s\n", line))
		}
	}

	// 添加页眉信息
	header := fmt.Sprintf("[cyan]Instructions: %d/%d[white]\n", currentIdx+1, total)
	state.AsmView.SetText(header + sb.String())

	// 滚动到合适位置
	state.AsmView.ScrollToBeginning()
}

func UpdateRegView(state *AppState) {
	regInfo := state.User.GetRegisterInfo()
	state.RegView.SetText(regInfo)
}

// 添加自动步进函数
func StartAutoStep(state *AppState) {
	go func() {
		for {
			select {
			case run := <-state.AutoStepChan:
				if run {
					// 开始自动步进
					for state.User.AutoStep {
						// 执行下一个命令
						cmd := &core.Command{Type: core.CmdNext}
						message, updated := state.User.ExecuteCommand(cmd)

						// 在主线程中更新显示
						state.App.QueueUpdateDraw(func() {
							if updated {
								UpdateAsmView(state)
								UpdateRegView(state)
							}
							if message != "" {
								state.StatusView.SetText(message)
							}
						})

						// 延迟
						time.Sleep(time.Duration(state.User.StepDelay) * time.Millisecond)

						// 检查是否到达末尾
						if state.TraceManager.CurrentIndex >= state.TraceManager.Total()-1 {
							state.User.AutoStep = false
							state.App.QueueUpdateDraw(func() {
								state.StatusView.SetText("Reached end of trace")
							})
							break
						}
					}
				} else {
					// 停止自动步进
					state.User.AutoStep = false
				}
			}
		}
	}()
}

func UpdateDisplay(state *AppState, command *core.Command) {
	if state.TraceManager.Total() == 0 {
		state.AsmView.SetText("No instructions loaded")
		state.RegView.SetText("")
		state.StatusView.SetText("Load instructions using: load <filename>")
		return
	}

	// 执行命令
	if command != nil {
		message, updated := state.User.ExecuteCommand(command)
		if message != "" {
			state.StatusView.SetText(message)
		}
		if !updated && command.Type != core.CmdRun && command.Type != core.CmdStop && command.Type != core.CmdStep {
			// 命令没有更新指令位置
			return
		}
	}

	// 更新汇编视图（高亮当前行）
	UpdateAsmView(state)

	// 更新寄存器视图
	UpdateRegView(state)

	// 更新状态视图
	UpdateStatusView(state)
}

func UpdateStatusView(state *AppState) {
	statusInfo := state.User.GetStatusInfo()

	// 添加帮助提示
	helpText := `[gray]Commands: n/p, space=repeat, ←/→=prev/next, q=quit[-]`

	state.StatusView.SetText(statusInfo + "\n" + helpText)
}

// LoadInstructionsFromFile 加载指令文件
func LoadInstructionsFromFile(filename string, state *AppState) error {
	tm := state.TraceManager

	// 清空现有指令
	tm.Instructions = make([]*core.TraceLine, 0)
	tm.CurrentIndex = 0
	tm.PrevLine = nil

	// 重置寄存器检测器
	state.User.RegDetector = core.NewRegisterChangeDetector()

	// 读取文件
	err := core.ReadTraceFile(filename, func(t *core.TraceLine) {
		tm.AddInstruction(t)
	})

	if err != nil {
		return err
	}

	// 重置用户状态
	state.User.LastCommand = nil
	state.User.RepeatCount = 0
	state.User.AutoStep = false

	// 更新显示
	UpdateDisplay(state, nil)

	return nil
}
