package bootstrap

import (
	"gorm.io/gorm"

	"streetlight/internal/module"
	"streetlight/internal/modules/fault"
	"streetlight/internal/modules/lamp"
	"streetlight/internal/modules/material"
	"streetlight/internal/modules/repair"
	"streetlight/internal/modules/status"
)

// buildModules 按依赖方向装配业务模块。
//
// 依赖关系: 路灯台账 <- 故障登记 <- 维修记录, 故障登记 <- 现场材料, 维修状态查询依赖四者的只读仓储。
// 其中 "删除路灯前校验未闭环故障" 需要路灯模块反向调用故障模块,
// "故障闭环前校验必要材料" 需要故障模块反向调用材料模块,
// 均通过构造完成后的回填(SetXxx)注入, 避免循环构造依赖。
func buildModules(db *gorm.DB, uploadDir string) []module.Module {
	lampModule := lamp.New(db)

	faultModule := fault.New(db, lampModule.Service())
	lampModule.Service().SetOpenFaultCounter(faultModule.Repository())

	repairModule := repair.New(db, faultModule.Service())

	materialModule := material.New(db, faultModule.Service(), uploadDir)
	faultModule.Service().SetMaterialPort(materialModule.Service())

	statusModule := status.New(
		db,
		lampModule.Repository(),
		faultModule.Repository(),
		repairModule.Repository(),
		materialModule.Repository(),
	)

	return []module.Module{
		lampModule,
		faultModule,
		repairModule,
		materialModule,
		statusModule,
	}
}
