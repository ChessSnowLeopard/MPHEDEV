package sync

import (
	"MPHEDev/pkg/core/participant/types"
	"time"
)

// SyncService 同步服务接口
type SyncService interface {
	// 发送Done消息
	SendDoneMessage(phase types.SyncPhase, batchID int, targetID int) error

	// 等待Done确认
	WaitForDoneConfirmation(phase types.SyncPhase, batchID int, sourceID int, timeout time.Duration) error

	// 处理接收到的Done消息
	HandleDoneMessage(doneMsg *types.DoneMessage) error

	// 检查同步状态
	CheckSyncStatus(phase types.SyncPhase, batchID int) types.SyncStatus

	// 重置同步状态
	ResetSyncStatus(phase types.SyncPhase, batchID int) error

	// 设置依赖关系
	SetDependencies(phase types.SyncPhase, batchID int, dependencies map[int]bool) error

	// 检查是否所有依赖都已完成
	CheckDependenciesComplete(phase types.SyncPhase, batchID int) bool

	// 获取确认状态
	GetConfirmations(phase types.SyncPhase, batchID int) map[int]time.Time

	// 设置超时配置
	SetTimeout(timeout time.Duration)

	// 设置最大重试次数
	SetMaxRetries(maxRetries int)
}
