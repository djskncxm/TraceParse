package core

import "fmt"

type RegisterChangeDetector struct {
	lastValues [33]uint64 // 31个通用寄存器 + SP + PC
	hasPrev    bool
}

func NewRegisterChangeDetector() *RegisterChangeDetector {
	return &RegisterChangeDetector{}
}

func (r *RegisterChangeDetector) Update(current *TraceLine) map[int]bool {
	changes := make(map[int]bool)

	if r.hasPrev {
		// 检查通用寄存器
		for i := 0; i < 31; i++ {
			if current.Regs[i] != r.lastValues[i] {
				changes[i] = true
			}
		}
		// 检查 SP
		if current.SP != r.lastValues[31] {
			changes[31] = true // SP 的索引为 31
		}
		// 检查 PC
		if current.PC != r.lastValues[32] {
			changes[32] = true // PC 的索引为 32
		}
	}

	// 更新缓存值
	if current != nil {
		for i := 0; i < 31; i++ {
			r.lastValues[i] = current.Regs[i]
		}
		r.lastValues[31] = current.SP
		r.lastValues[32] = current.PC
		r.hasPrev = true
	}

	return changes
}

// 获取寄存器名称
func (r *RegisterChangeDetector) GetRegisterName(index int) string {
	if index >= 0 && index < 31 {
		return fmt.Sprintf("x%d", index)
	} else if index == 31 {
		return "SP"
	} else if index == 32 {
		return "PC"
	}
	return ""
}
