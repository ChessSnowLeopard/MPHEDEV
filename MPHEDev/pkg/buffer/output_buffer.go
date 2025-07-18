package buffer

import (
	"fmt"
	"sync"
	"time"
)

// OutputBuffer 输出缓冲区
type OutputBuffer struct {
	mutex   sync.RWMutex
	outputs []OutputEntry
	maxSize int
	role    string
}

// OutputEntry 输出条目
type OutputEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // info, warning, error, debug
	Message   string    `json:"message"`
	Role      string    `json:"role"`
}

// NewOutputBuffer 创建新的输出缓冲区
func NewOutputBuffer(role string, maxSize int) *OutputBuffer {
	if maxSize <= 0 {
		maxSize = 1000 // 默认最大1000条记录
	}

	return &OutputBuffer{
		outputs: make([]OutputEntry, 0, maxSize),
		maxSize: maxSize,
		role:    role,
	}
}

// AddOutput 添加输出
func (ob *OutputBuffer) AddOutput(level, message string) {
	ob.mutex.Lock()
	defer ob.mutex.Unlock()

	entry := OutputEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Role:      ob.role,
	}

	// 添加到开头（最新的在前面）
	ob.outputs = append([]OutputEntry{entry}, ob.outputs...)

	// 如果超出最大大小，删除最旧的
	if len(ob.outputs) > ob.maxSize {
		ob.outputs = ob.outputs[:ob.maxSize]
	}
}

// AddInfo 添加信息级别输出
func (ob *OutputBuffer) AddInfo(message string) {
	ob.AddOutput("info", message)
}

// AddWarning 添加警告级别输出
func (ob *OutputBuffer) AddWarning(message string) {
	ob.AddOutput("warning", message)
}

// AddError 添加错误级别输出
func (ob *OutputBuffer) AddError(message string) {
	ob.AddOutput("error", message)
}

// AddDebug 添加调试级别输出
func (ob *OutputBuffer) AddDebug(message string) {
	ob.AddOutput("debug", message)
}

// GetOutputs 获取所有输出
func (ob *OutputBuffer) GetOutputs() []OutputEntry {
	ob.mutex.RLock()
	defer ob.mutex.RUnlock()

	// 返回副本
	outputs := make([]OutputEntry, len(ob.outputs))
	copy(outputs, ob.outputs)
	return outputs
}

// GetOutputsAsString 获取所有输出作为字符串
func (ob *OutputBuffer) GetOutputsAsString() string {
	ob.mutex.RLock()
	defer ob.mutex.RUnlock()

	if len(ob.outputs) == 0 {
		return "暂无输出"
	}

	var result string
	for i := len(ob.outputs) - 1; i >= 0; i-- { // 按时间顺序显示
		entry := ob.outputs[i]
		timestamp := entry.Timestamp.Format("15:04:05")
		result += fmt.Sprintf("[%s] [%s] %s\n", timestamp, entry.Level, entry.Message)
	}

	return result
}

// GetOutputsAsStringFiltered 获取过滤后的输出作为字符串
func (ob *OutputBuffer) GetOutputsAsStringFiltered(levels []string, maxLines int) string {
	ob.mutex.RLock()
	defer ob.mutex.RUnlock()

	if len(ob.outputs) == 0 {
		return "暂无输出"
	}

	// 创建级别过滤映射
	levelFilter := make(map[string]bool)
	for _, level := range levels {
		levelFilter[level] = true
	}

	var result string
	lineCount := 0

	// 按时间顺序显示（最新的在前面）
	for i := 0; i < len(ob.outputs) && (maxLines <= 0 || lineCount < maxLines); i++ {
		entry := ob.outputs[i]

		// 如果指定了过滤级别，则只显示指定级别的输出
		if len(levelFilter) > 0 && !levelFilter[entry.Level] {
			continue
		}

		timestamp := entry.Timestamp.Format("15:04:05")
		result += fmt.Sprintf("[%s] [%s] %s\n", timestamp, entry.Level, entry.Message)
		lineCount++
	}

	if result == "" {
		return "暂无符合条件的输出"
	}

	return result
}

// GetKeyOutputs 获取关键输出（只包含error、warning和重要的info）
func (ob *OutputBuffer) GetKeyOutputs(maxLines int) string {
	keyLevels := []string{"error", "warning", "info"}
	return ob.GetOutputsAsStringFiltered(keyLevels, maxLines)
}

// GetRecentOutputs 获取最近的输出
func (ob *OutputBuffer) GetRecentOutputs(maxLines int) string {
	return ob.GetOutputsAsStringFiltered([]string{}, maxLines)
}

// Clear 清空输出缓冲区
func (ob *OutputBuffer) Clear() {
	ob.mutex.Lock()
	defer ob.mutex.Unlock()
	ob.outputs = make([]OutputEntry, 0, ob.maxSize)
}

// GetRole 获取角色
func (ob *OutputBuffer) GetRole() string {
	return ob.role
}

// SetRole 设置角色
func (ob *OutputBuffer) SetRole(role string) {
	ob.mutex.Lock()
	defer ob.mutex.Unlock()
	ob.role = role
}

// GetStats 获取统计信息
func (ob *OutputBuffer) GetStats() map[string]interface{} {
	ob.mutex.RLock()
	defer ob.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_entries": len(ob.outputs),
		"max_size":      ob.maxSize,
		"role":          ob.role,
	}

	if len(ob.outputs) > 0 {
		stats["latest_timestamp"] = ob.outputs[0].Timestamp
		stats["oldest_timestamp"] = ob.outputs[len(ob.outputs)-1].Timestamp
	}

	// 统计各级别数量
	levelCounts := make(map[string]int)
	for _, entry := range ob.outputs {
		levelCounts[entry.Level]++
	}
	stats["level_counts"] = levelCounts

	return stats
}
