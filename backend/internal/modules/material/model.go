package material

import "time"

// 现场材料所属阶段。
const (
	StageRegistration = "registration" // 故障登记
	StageRepair       = "repair"       // 维修过程
	StageAcceptance   = "acceptance"   // 完工验收
)

// 现场材料类型。
const (
	KindPhoto = "photo" // 照片
	KindVideo = "video" // 视频
)

// Stages 返回全部阶段取值, 顺序即前端展示顺序。
func Stages() []string {
	return []string{StageRegistration, StageRepair, StageAcceptance}
}

// StageLabel 返回阶段中文名称。
func StageLabel(stage string) string {
	switch stage {
	case StageRegistration:
		return "故障登记"
	case StageRepair:
		return "维修过程"
	case StageAcceptance:
		return "完工验收"
	default:
		return stage
	}
}

// IsValidStage 校验阶段取值。
func IsValidStage(stage string) bool {
	for _, item := range Stages() {
		if item == stage {
			return true
		}
	}
	return false
}

// RequiredRule 描述某阶段必须提交的必要材料, Kinds 中任一种类满足数量即视为达标。
type RequiredRule struct {
	Stage    string
	Kinds    []string
	MinCount int64
	Label    string
}

// RequiredRules 返回闭环前必须备齐的现场材料清单。
func RequiredRules() []RequiredRule {
	return []RequiredRule{
		{Stage: StageRegistration, Kinds: []string{KindPhoto}, MinCount: 1, Label: "登记现场照片"},
		{Stage: StageRepair, Kinds: []string{KindPhoto, KindVideo}, MinCount: 1, Label: "维修过程影像"},
		{Stage: StageAcceptance, Kinds: []string{KindPhoto}, MinCount: 1, Label: "完工验收照片"},
	}
}

// CountRow 是单条故障在某个阶段下某种材料的数量统计。
type CountRow struct {
	FaultID uint
	Stage   string
	Kind    string
	Total   int64
}

// MissingFromCounts 依据必要材料规则评估缺失项, counts 为全部故障的汇总统计。
func MissingFromCounts(counts []CountRow, faultID uint) []string {
	totals := make(map[string]map[string]int64)
	for _, row := range counts {
		if row.FaultID != faultID {
			continue
		}
		if totals[row.Stage] == nil {
			totals[row.Stage] = make(map[string]int64)
		}
		totals[row.Stage][row.Kind] += row.Total
	}

	missing := make([]string, 0)
	for _, rule := range RequiredRules() {
		var sum int64
		for _, kind := range rule.Kinds {
			sum += totals[rule.Stage][kind]
		}
		if sum < rule.MinCount {
			missing = append(missing, rule.Label)
		}
	}
	return missing
}

// Material 现场材料记录, 一条记录对应故障某个阶段上传的一张照片或一段视频。
type Material struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	FaultID      uint      `gorm:"index;not null" json:"fault_id"`
	FaultNo      string    `gorm:"size:64;index" json:"fault_no"`
	Stage        string    `gorm:"size:32;index;not null" json:"stage"`
	Kind         string    `gorm:"size:16;not null" json:"kind"`
	Title        string    `gorm:"size:128" json:"title"`
	FileName     string    `gorm:"size:255;not null" json:"-"` // 相对上传目录的存储路径, 不对外暴露
	OriginalName string    `gorm:"size:255" json:"original_name"`
	ContentType  string    `gorm:"size:64" json:"content_type"`
	Size         int64     `json:"size"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (Material) TableName() string { return "fault_material" }
