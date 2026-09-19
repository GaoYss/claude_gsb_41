package material

var stageLabels = map[string]string{
	StageRegister:   "登记环节",
	StageProcess:    "维修过程",
	StageAcceptance: "完工验收",
}

var mediaTypeLabels = map[string]string{
	MediaImage: "照片",
	MediaVideo: "视频",
}

// StageLabel 返回材料阶段的中文名称, 未知取值原样返回。
func StageLabel(stage string) string {
	if label, ok := stageLabels[stage]; ok {
		return label
	}
	return stage
}

// MediaTypeLabel 返回媒体类型的中文名称。
func MediaTypeLabel(mediaType string) string {
	if label, ok := mediaTypeLabels[mediaType]; ok {
		return label
	}
	return mediaType
}
