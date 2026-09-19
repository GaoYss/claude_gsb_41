package material

import "time"

// 现场材料记录所处的业务阶段。
const (
	StageRegister   = "register"   // 登记环节
	StageProcess    = "process"    // 维修过程
	StageAcceptance = "acceptance" // 完工验收
)

// 媒体类型。
const (
	MediaImage = "image" // 照片
	MediaVideo = "video" // 视频
)

// Stages 返回全部现场材料阶段, 顺序即前端分组展示顺序。
func Stages() []string {
	return []string{StageRegister, StageProcess, StageAcceptance}
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

// Catalog 材料目录, 维护常见耗材/备件的主数据, 登记故障材料时可直接引用。
type Catalog struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `gorm:"size:128;index;not null" json:"name"`
	Spec     string `gorm:"size:128" json:"spec"`
	Unit     string `gorm:"size:16" json:"unit"`
	Category string `gorm:"size:64;index" json:"category"`
	// Enabled 不设数据库 default: GORM 对 default 布尔列会省略零值, 显式停用会被错误写成 true。
	Enabled bool   `gorm:"index;not null" json:"enabled"`
	Remark  string `gorm:"size:255" json:"remark"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (Catalog) TableName() string { return "material_catalog" }

// FaultMaterial 故障材料清单中的一项, 记录某条故障所需材料及其到位情况。
type FaultMaterial struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	FaultID   uint   `gorm:"uniqueIndex:idx_fault_material_item,priority:1;not null" json:"fault_id"`
	FaultNo   string `gorm:"size:64;index" json:"fault_no"`
	LampCode  string `gorm:"size:64;index" json:"lamp_code"`
	RoadName  string `gorm:"size:128;index" json:"road_name"`
	CatalogID *uint  `gorm:"index" json:"catalog_id"`
	Name      string `gorm:"size:128;not null;uniqueIndex:idx_fault_material_item,priority:2" json:"name"`
	Spec      string `gorm:"size:128;uniqueIndex:idx_fault_material_item,priority:3" json:"spec"`
	Unit      string `gorm:"size:16" json:"unit"`
	Quantity  int    `gorm:"not null;default:1" json:"quantity"`
	// Required / Received 不设数据库 default: GORM 对 default 布尔列会省略零值,
	// 显式的"非必要/未到位"(false)会被错误写成默认值, 必要材料标记会因此失真。
	Required   bool       `gorm:"index;not null" json:"required"` // 必要材料: 缺失时不允许故障闭环
	Received   bool       `gorm:"index;not null" json:"received"`
	ReceivedAt *time.Time `json:"received_at"`
	Remark     string     `gorm:"size:255" json:"remark"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (FaultMaterial) TableName() string { return "fault_material" }

// DisplayName 返回带规格的材料名称, 用于清单与提示信息。
func (m *FaultMaterial) DisplayName() string {
	if spec := m.Spec; spec != "" {
		return m.Name + "(" + spec + ")"
	}
	return m.Name
}

// Media 现场照片/视频, 按登记、维修过程、完工验收三个阶段归类。
type Media struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	FaultID    uint   `gorm:"index;not null" json:"fault_id"`
	FaultNo    string `gorm:"size:64;index" json:"fault_no"`
	Stage      string `gorm:"size:32;index;not null" json:"stage"`
	MediaType  string `gorm:"size:16;index;not null" json:"media_type"`
	FileName   string `gorm:"size:255;not null" json:"file_name"`
	StorageKey string `gorm:"size:255;not null" json:"-"`
	MimeType   string `gorm:"size:64" json:"mime_type"`
	FileSize   int64  `json:"file_size"`
	Uploader   string `gorm:"size:64" json:"uploader"`
	Remark     string `gorm:"size:255" json:"remark"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (Media) TableName() string { return "material_media" }
