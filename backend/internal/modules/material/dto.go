package material

// StageGroup 是按阶段分组的材料视图, 前端按此结构渲染与下载。
type StageGroup struct {
	Stage       string     `json:"stage"`
	Label       string     `json:"label"`
	Requirement string     `json:"requirement"` // 必要材料要求说明, 便于前端提示
	Items       []Material `json:"items"`
}

// FaultMaterials 是单条故障的现场材料总览: 按阶段分组的明细 + 完整性结论。
type FaultMaterials struct {
	FaultID  uint         `json:"fault_id"`
	Complete bool         `json:"complete"`
	Missing  []string     `json:"missing"`
	Stages   []StageGroup `json:"stages"`
}

// requirementText 生成阶段必要材料要求说明, 无要求时返回空串。
func requirementText(stage string) string {
	for _, rule := range RequiredRules() {
		if rule.Stage == stage {
			return rule.Label
		}
	}
	return ""
}
