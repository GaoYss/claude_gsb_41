package material

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Module 现场材料模块, 负责故障各阶段照片 / 视频的上传、分组查看与下载。
type Module struct {
	repository *Repository
	service    *Service
	handler    *Handler
}

// New 构造现场材料模块, faults 为故障登记模块提供的端口实现, uploadDir 为文件存储目录。
func New(db *gorm.DB, faults FaultPort, uploadDir string) *Module {
	repository := NewRepository(db)
	service := NewService(repository, faults, uploadDir)
	return &Module{
		repository: repository,
		service:    service,
		handler:    NewHandler(service),
	}
}

// Service 暴露业务服务, 供故障模块装配闭环校验端口。
func (m *Module) Service() *Service { return m.service }

// Repository 暴露仓储, 供状态查询模块装配只读统计。
func (m *Module) Repository() *Repository { return m.repository }

// Name 实现 module.Module 接口。
func (m *Module) Name() string { return "现场材料" }

// Models 实现 module.Module 接口。
func (m *Module) Models() []any { return []any{&Material{}} }

// RegisterRoutes 实现 module.Module 接口。
func (m *Module) RegisterRoutes(api *gin.RouterGroup) {
	api.POST("/faults/:id/materials", m.handler.Upload)
	api.GET("/faults/:id/materials", m.handler.ListByFault)
	api.GET("/materials/:id/file", m.handler.File)
	api.DELETE("/materials/:id", m.handler.Delete)
}
