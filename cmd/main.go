package main

import (
	"flag"
	"github.com/djskncxm/TraceParse/pkg/core"
	"github.com/djskncxm/TraceParse/pkg/tui"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {
	// 添加命令行参数解析
	var traceFile string
	flag.StringVar(&traceFile, "f", "", "Trace file to load")
	flag.StringVar(&traceFile, "file", "", "Trace file to load")
	flag.Parse()

	app := tview.NewApplication()

	// 创建 TraceManager 和 User
	tm := core.NewTraceManager()
	user := core.NewUser(tm)

	// 创建视图
	asmView := tui.NewAsmView()
	regView := tui.NewRegView()
	statusView := tui.NewStatusView()
	memoryView := tui.NewMemoryView()

	// 创建 BL 和 RW 视图
	blView := tui.NewLogView("BL Log")
	rwView := tui.NewLogView("RW Log")

	// 创建应用状态
	state := &tui.AppState{
		TraceManager: tm,
		User:         user,
		App:          app,
		AsmView:      asmView,
		RegView:      regView,
		StatusView:   statusView,
		MemoryView:   memoryView,
		BlView:       blView,
		RwView:       rwView,
		AutoStepChan: make(chan bool, 1),
	}

	// 启动自动步进管理器
	go tui.StartAutoStep(state)

	// 创建输入框
	inputField := tui.NewInputView(state)
	state.InputField = inputField

	// 添加全局键盘快捷键
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRight:
			// 右箭头：下一个指令
			cmd := user.ParseCommand("n")
			tui.UpdateDisplay(state, cmd)
			return nil
		case tcell.KeyLeft:
			// 左箭头：上一个指令
			cmd := user.ParseCommand("p")
			tui.UpdateDisplay(state, cmd)
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q', 'Q':
				app.Stop()
				return nil
			case ']':
				// 空格键：重复上一个命令或执行next
				cmd := user.ParseCommand("")
				if cmd != nil {
					tui.UpdateDisplay(state, cmd)
				} else {
					// 否则执行 return
					return nil
				}
				return nil
			}
		}
		return event
	})

	rightPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(statusView, 5, 1, false). // 状态视图：固定5行
		AddItem(regView, 0, 2, false).    // 寄存器视图：动态高度，比例2
		AddItem(blView, 0, 2, false).     // BL日志视图：动态高度，比例2
		AddItem(rwView, 0, 2, false).     // RW日志视图：动态高度，比例2
		AddItem(inputField, 3, 0, true)   // 输入框：固定3行

	root := tview.NewFlex().
		AddItem(asmView, 0, 2, false).    // 汇编视图占3/5
		AddItem(rightPanel, 0, 3, false). // 右侧面板占2/5
		AddItem(memoryView, 0, 2, false)  // 内存视图：动态高度，比例2

	// 加载指令文件（与之前相同）
	go func() {
		var filename string

		// 优先使用命令行参数指定的文件
		if traceFile != "" {
			filename = traceFile
		}

		// 初始状态信息
		// app.QueueUpdateDraw(func() { statusView.SetText("Loading...") })

		if filename != "" {
			err := tui.LoadInstructionsFromFile(filename, state)
			if err != nil {
				// 如果没有文件，创建一些示例指令
				app.QueueUpdateDraw(func() {
					statusView.SetText("Error loading file: " + err.Error() +
						"\nUsing example instructions...")
				})
			}
		}
	}()

	// 设置初始帮助信息
	helpText := ` Welcome to TraceParse!
	Commands: 
	n, next       - 下一条指令 (n5 for 5 steps) 
	p, prev       - 上一条指令 (p3 for 3 steps) 
	g <line>      - 跳转某行 
	run           - 自动向下执行 
	s/stop        - 停止向下执 
	step <ms>     - 设置自动执行间隔 
	"]"           - 重复上一个命令 
	q, quit       - 退出
	
  面板现在有一个bug，必须按一个键位才会显示文件，我也不知道为啥，还在排查
	`

	memoryView.SetText(helpText)

	// 运行应用
	if err := app.SetRoot(root, true).
		SetFocus(inputField).
		Run(); err != nil {
	}
}
