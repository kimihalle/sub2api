package service

import (
	"context"
	"strings"
)

// IsSanbaoVideoModel reports whether a model name belongs to Sanbao's OpenAPI video set.
func IsSanbaoVideoModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	switch model {
	case "sd2_9img_full",
		"sd2_full_no_real_multi_res",
		"sd2_9img_seconds_720p_real",
		"sd2_9img_special_full",
		"sd2_fast_9img_line4",
		"sd2_fast_9img_line2",
		"sd2_fast_9img_seconds_10_15_720p",
		"sd2_value",
		"sd2_4img_real_720p",
		"sd2_9img_limited_special",
		"grok_video_1_5_7ref":
		return true
	default:
		return strings.HasPrefix(model, "sd2_")
	}
}

func IsSanbaoVideoPerGenerationModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	switch model {
	case "sd2_fast_9img_line4",
		"sd2_fast_9img_line2",
		"sd2_fast_9img_seconds_10_15_720p",
		"sd2_value",
		"sd2_4img_real_720p",
		"sd2_9img_limited_special",
		"grok_video_1_5_7ref":
		return true
	default:
		return false
	}
}

func DefaultSanbaoVideoCost(model, resolution string, durationSeconds, videoCount int, multiplier float64) (*CostBreakdown, bool) {
	if videoCount <= 0 {
		videoCount = 1
	}
	if durationSeconds <= 0 {
		durationSeconds = 5
	}
	model = strings.ToLower(strings.TrimSpace(model))
	resolution = NormalizeVideoBillingResolutionOrDefault(resolution)
	var unit float64
	var ok bool
	if IsSanbaoVideoPerGenerationModel(model) {
		unit, ok = defaultSanbaoVideoPerGenerationPrice(model)
	} else {
		unit, ok = defaultSanbaoVideoPerSecondPrice(model, resolution)
		if ok {
			unit *= float64(durationSeconds)
		}
	}
	if !ok {
		return nil, false
	}
	if multiplier < 0 {
		multiplier = 0
	}
	total := unit * float64(videoCount)
	return &CostBreakdown{
		TotalCost:   total,
		ActualCost:  total * multiplier,
		BillingMode: string(BillingModeVideo),
	}, true
}

func defaultSanbaoVideoPerSecondPrice(model, resolution string) (float64, bool) {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "sd2_9img_full":
		if resolution == VideoBillingResolution1080P {
			return 0.70, true
		}
		return 0.64, true
	case "sd2_full_no_real_multi_res":
		switch resolution {
		case VideoBillingResolution480P:
			return 0.30, true
		case VideoBillingResolution1080P:
			return 0.84, true
		default:
			return 0.48, true
		}
	case "sd2_9img_seconds_720p_real":
		return 0.55, true
	case "sd2_9img_special_full":
		if resolution == VideoBillingResolution1080P {
			return 0.77, true
		}
		return 0.61, true
	default:
		return 0, false
	}
}

func defaultSanbaoVideoPerGenerationPrice(model string) (float64, bool) {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "sd2_fast_9img_line4":
		return 2.00, true
	case "sd2_fast_9img_line2":
		return 2.90, true
	case "sd2_fast_9img_seconds_10_15_720p":
		return 3.30, true
	case "sd2_value":
		return 4.00, true
	case "sd2_4img_real_720p":
		return 4.80, true
	case "sd2_9img_limited_special":
		return 5.80, true
	case "grok_video_1_5_7ref":
		return 0.60, true
	default:
		return 0, false
	}
}

// GetAccountByID exposes the gateway account repository for asynchronous media
// status lookups that must keep using the original upstream account.
func (s *OpenAIGatewayService) GetAccountByID(ctx context.Context, id int64) (*Account, error) {
	if s == nil || s.accountRepo == nil {
		return nil, ErrNoAvailableAccounts
	}
	return s.accountRepo.GetByID(ctx, id)
}
