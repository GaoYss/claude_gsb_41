package material

// MediaGroup 现场媒体按阶段分组的结果。
type MediaGroup struct {
	Stage string  `json:"stage"`
	Label string  `json:"label"`
	Items []Media `json:"items"`
}

// GroupMedia 将扁平媒体列表按业务阶段分组, 保证三个阶段固定顺序输出。
func GroupMedia(items []Media) []MediaGroup {
	groups := make([]MediaGroup, 0, len(Stages()))
	index := make(map[string]int, len(Stages()))
	for _, stage := range Stages() {
		index[stage] = len(groups)
		groups = append(groups, MediaGroup{Stage: stage, Label: StageLabel(stage), Items: make([]Media, 0)})
	}
	for _, item := range items {
		pos, ok := index[item.Stage]
		if !ok {
			pos = len(groups)
			index[item.Stage] = pos
			groups = append(groups, MediaGroup{Stage: item.Stage, Label: StageLabel(item.Stage), Items: make([]Media, 0)})
		}
		groups[pos].Items = append(groups[pos].Items, item)
	}
	return groups
}
