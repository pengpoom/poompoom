package api

import (
	"errors"
	"strings"

	"imagestudio/internal/businessjobs"
	"imagestudio/internal/businesssettings"
)

func businessJobCapacityLimits(settings businesssettings.Settings) businessjobs.CapacityLimits {
	settings = businesssettings.Normalize(settings)
	return businessjobs.CapacityLimits{
		MaxUserActiveJobs:      settings.Runtime.MaxUserActiveJobs,
		MaxQueuedJobs:          settings.Runtime.MaxQueuedJobs,
		MaxProviderRunningJobs: settings.Runtime.MaxProviderRunningJobs,
	}
}

func businessJobCapacityErrorCode(err error) string {
	var capacityErr *businessjobs.CapacityError
	if errors.As(err, &capacityErr) {
		return strings.TrimSpace(capacityErr.Code)
	}
	return ""
}

func businessJobCapacityMessage(code string) string {
	switch strings.TrimSpace(code) {
	case businessjobs.CapacityUserActiveLimit:
		return "你已有任务正在排队或生成，请稍后再试。"
	default:
		return imageBusyMessage
	}
}
