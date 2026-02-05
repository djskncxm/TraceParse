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
	LoadedFile   string // 记录加载的文件名
}

func NewAsmView() *tview.TextView {
	asmView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false)
	asmView.SetBorder(true).SetTitle("|Assembly Instructions|")
	asmView.SetBackgroundColor(tcell.ColorDefault)
	return asmView
}

func NewRegView() *tview.TextView {
	regView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	regView.SetBorder(true).SetTitle("|Registers|")
	regView.SetBackgroundColor(tcell.ColorDefault)
	return regView
}

func NewStatusView() *tview.TextView {
	statusView := tview.NewTextView().
		SetDynamicColors(true)
	statusView.SetBorder(true).SetTitle("|Status|")
	statusView.SetBackgroundColor(tcell.ColorDefault)
	return statusView
}

func NewMemoryView() *tview.TextView {
	memoryView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	memoryView.SetBorder(true).SetTitle("|Memory|")
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

	if total == 0 {
		state.AsmView.SetText("No instructions loaded")
		return
	}

	// 计算显示范围
	windowSize := 51
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
			// 显示加载状态
			line := fmt.Sprintf("%4d | [gray]Loading...[-]", i+1)
			if i == currentIdx {
				sb.WriteString(fmt.Sprintf("[yellow]▶ %s[white]\n", line))
			} else {
				sb.WriteString(fmt.Sprintf("  %s\n", line))
			}
			continue
		}

		// 格式化指令行
		line := fmt.Sprintf("%4d | 0x%x | %s", inst.Step, inst.Offset, inst.Instr)

		// 检查寄存器变化（只检查下一条指令是否已加载）
		nextIdx := i + 1
		if nextIdx < total {
			nextInst := state.TraceManager.GetLine(nextIdx)
			if nextInst != nil {
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
		}

		// 高亮当前行
		if i == currentIdx {
			sb.WriteString(fmt.Sprintf("[red]▶ %s[white]\n", line))
		} else {
			sb.WriteString(fmt.Sprintf("  %s\n", line))
		}
	}

	// 添加页眉信息和加载范围
	loadedStart := state.TraceManager.LoadedRange[0]
	loadedEnd := state.TraceManager.LoadedRange[1]
	header := fmt.Sprintf("[green]Instructions: %d/%d | Loaded: [%d, %d) (%d lines)[white]\n",
		currentIdx+1, total, loadedStart, loadedEnd, loadedEnd-loadedStart)

	state.AsmView.SetText(header + sb.String())
	state.AsmView.ScrollToBeginning()
}

// 添加一个刷新函数，用于窗口加载完成后更新UI
func RefreshDisplay(state *AppState) {
	state.App.QueueUpdateDraw(func() {
		UpdateAsmView(state)
		UpdateRegView(state)
		UpdateStatusView(state)
	})
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
	state.LoadedFile = filename

	// 使用简化的加载方法
	err := core.ReadTraceFile(filename, state.TraceManager)
	if err != nil {
		return err
	}

	// 重置用户状态
	state.User.LastCommand = nil
	state.User.RepeatCount = 0
	state.User.AutoStep = false
	state.User.RegDetector = core.NewRegisterChangeDetector()

	// 更新显示
	UpdateDisplay(state, nil)

	return nil
}
