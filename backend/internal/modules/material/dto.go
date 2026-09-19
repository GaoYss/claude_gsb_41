package material

import "streetlight/pkg/pagination"

// CatalogCreateRequest 新增材料目录请求。
type CatalogCreateRequest struct {
	Name     string `json:"name" binding:"required,max=128"`
	Spec     string `json:"spec" binding:"max=128"`
	Unit     string `json:"unit" binding:"max=16"`
	Category string `json:"category" binding:"max=64"`
	Enabled  *bool  `json:"enabled"`
	Remark   string `json:"remark" binding:"max=255"`
}

// CatalogUpdateRequest 修改材料目录请求。
type CatalogUpdateRequest struct {
	Name     *string `json:"name" binding:"omitempty,max=128"`
	Spec     *string `json:"spec" binding:"omitempty,max=128"`
	Unit     *string `json:"unit" binding:"omitempty,max=16"`
	Category *string `json:"category" binding:"omitempty,max=64"`
	Enabled  *bool   `json:"enabled"`
	Remark   *string `json:"remark" binding:"omitempty,max=255"`
}

// CatalogListQuery 材料目录查询条件。
type CatalogListQuery struct {
	pagination.Params
	Keyword  string `form:"keyword"` // 材料名称 / 规格
	Category string `form:"category"`
	Enabled  *bool  `form:"enabled"`
}

// FaultMaterialCreateRequest 登记故障所需材料请求。
type FaultMaterialCreateRequest struct {
	CatalogID *uint  `json:"catalog_id"`
	Name      string `json:"name" binding:"required,max=128"`
	Spec      string `json:"spec" binding:"max=128"`
	Unit      string `json:"unit" binding:"max=16"`
	Quantity  *int   `json:"quantity" binding:"omitempty,min=1,max=9999"`
	Required  *bool  `json:"required"`
	Received  *bool  `json:"received"`
	Remark    string `json:"remark" binding:"max=255"`
}

// FaultMaterialUpdateRequest 修改故障材料项请求。
type FaultMaterialUpdateRequest struct {
	Name     *string `json:"name" binding:"omitempty,max=128"`
	Spec     *string `json:"spec" binding:"omitempty,max=128"`
	Unit     *string `json:"unit" binding:"omitempty,max=16"`
	Quantity *int    `json:"quantity" binding:"omitempty,min=1,max=9999"`
	Required *bool   `json:"required"`
	Remark   *string `json:"remark" binding:"omitempty,max=255"`
}

// FaultMaterialListQuery 故障材料清单查询条件。
type FaultMaterialListQuery struct {
	pagination.Params
	Keyword     string `form:"keyword"` // 故障单号 / 路灯编号 / 材料名称
	RoadName    string `form:"road_name"`
	Received    string `form:"received"` // "" 不过滤 / yes / no
	MissingOnly bool   `form:"missing_only"`
}

// ReceiveRequest 材料到位状态变更请求。
type ReceiveRequest struct {
	Received bool `json:"received"`
}

// MediaQuery 现场媒体查询条件。
type MediaQuery struct {
	FaultID   uint   `form:"fault_id"`
	Stage     string `form:"stage"`
	MediaType string `form:"media_type"`
}

// MediaMeta 媒体上传元数据。
type MediaMeta struct {
	Stages     []string `json:"stages"`
	MediaTypes []string `json:"media_types"`
}

// CatalogMeta 材料目录页字典。
type CatalogMeta struct {
	Categories []string `json:"categories"`
	Stages     []string `json:"stages"`
	MediaTypes []string `json:"media_types"`
}

// Completeness 单条故障的材料完整情况。
type Completeness struct {
	FaultID       uint            `json:"fault_id"`
	FaultNo       string          `json:"fault_no"`
	LampCode      string          `json:"lamp_code"`
	RoadName      string          `json:"road_name"`
	FaultStatus   string          `json:"fault_status"`
	TotalCount    int             `json:"total_count"`
	RequiredCount int             `json:"required_count"`
	ReceivedCount int             `json:"received_count"`
	MissingCount  int             `json:"missing_count"` // 必要且未到位的材料数量
	Complete      bool            `json:"complete"`
	MissingItems  []FaultMaterial `json:"missing_items"`
}

// CompletenessSummary 概览页使用的材料完整率汇总。
type CompletenessSummary struct {
	FaultCount      int64          `json:"fault_count"`      // 纳入统计的故障数(配置了材料清单的故障)
	CompleteCount   int64          `json:"complete_count"`   // 材料齐全的故障数
	IncompleteCount int64          `json:"incomplete_count"` // 仍有必要材料缺失的故障数
	CompleteRate    float64        `json:"complete_rate"`    // 完整率(百分比, 0-100)
	ItemTotal       int64          `json:"item_total"`       // 材料项总数
	RequiredTotal   int64          `json:"required_total"`   // 必要材料项总数
	MissingTotal    int64          `json:"missing_total"`    // 缺失的必要材料总数
	MissingFaults   []Completeness `json:"missing_faults"`   // 缺失材料的故障清单
}
