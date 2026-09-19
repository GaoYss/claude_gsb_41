package material

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DefaultMediaRoot 是现场照片/视频的默认存储目录。
const DefaultMediaRoot = "data/materials"

// Module 现场材料管理模块: 材料目录、故障材料清单与分阶段现场照片/视频。
type Module struct {
	repository *Repository
	service    *Service
	handler    *Handler
}

// New 构造现场材料模块, faults 为故障模块提供的端口实现, mediaRoot 为媒体文件目录。
func New(db *gorm.DB, faults FaultPort, mediaRoot string) (*Module, error) {
	storage, err := NewFileStorage(mediaRoot)
	if err != nil {
		return nil, err
	}
	repository := NewRepository(db)
	service := NewService(repository, faults, storage)
	return &Module{
		repository: repository,
		service:    service,
		handler:    NewHandler(service),
	}, nil
}

// Service 暴露业务服务, 供故障模块装配材料校验端口。
func (m *Module) Service() *Service { return m.service }

// Name 实现 module.Module 接口。
func (m *Module) Name() string { return "现场材料管理" }

// Models 实现 module.Module 接口。
func (m *Module) Models() []any {
	return []any{&Catalog{}, &FaultMaterial{}, &Media{}}
}

// RegisterRoutes 实现 module.Module 接口。
func (m *Module) RegisterRoutes(api *gin.RouterGroup) {
	group := api.Group("/materials")

	// 材料目录
	catalog := group.Group("/catalog")
	{
		catalog.GET("", m.handler.ListCatalogs)
		catalog.POST("", m.handler.CreateCatalog)
		catalog.GET("/meta", m.handler.CatalogMeta)
		catalog.PUT("/:id", m.handler.UpdateCatalog)
		catalog.DELETE("/:id", m.handler.DeleteCatalog)
	}

	// 故障材料清单(跨故障明细 + 单项操作)
	group.GET("/items", m.handler.ListItemsPage)
	group.PUT("/items/:id", m.handler.UpdateItem)
	group.POST("/items/:id/receive", m.handler.ReceiveItem)
	group.DELETE("/items/:id", m.handler.DeleteItem)
	group.GET("/summary", m.handler.Summary)

	// 单条故障维度: 材料清单 / 完整情况 / 现场媒体
	faultGroup := group.Group("/faults/:faultId")
	{
		faultGroup.GET("/items", m.handler.ListItems)
		faultGroup.POST("/items", m.handler.CreateItem)
		faultGroup.GET("/completeness", m.handler.Completeness)
		faultGroup.GET("/media", m.handler.ListMedia)
		faultGroup.POST("/media", m.handler.UploadMedia)
	}

	// 媒体下载(支持 Range 在线播放)与删除
	group.GET("/media/:id", m.handler.DownloadMedia)
	group.DELETE("/media/:id", m.handler.DeleteMedia)
}
