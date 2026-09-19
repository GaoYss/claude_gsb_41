package bootstrap

import (
	"fmt"

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
// 依赖关系: 路灯台账 <- 故障登记 <- 维修记录, 维修状态查询依赖三者的只读仓储。
// 现场材料管理读取故障信息, 并向故障模块反向注入"闭环前材料校验"端口,
// 与"删除路灯前校验未闭环故障"一样在构造完成后回填, 避免构造循环依赖。
func buildModules(db *gorm.DB, mediaRoot string) ([]module.Module, error) {
	lampModule := lamp.New(db)

	faultModule := fault.New(db, lampModule.Service())
	lampModule.Service().SetOpenFaultCounter(faultModule.Repository())

	repairModule := repair.New(db, faultModule.Service())

	materialModule, err := material.New(db, faultModule.Service(), mediaRoot)
	if err != nil {
		return nil, fmt.Errorf("初始化现场材料模块失败: %w", err)
	}
	faultModule.Service().SetMaterialStatusPort(materialModule.Service())

	statusModule := status.New(
		db,
		lampModule.Repository(),
		faultModule.Repository(),
		repairModule.Repository(),
		materialModule.Service(),
	)

	return []module.Module{
		lampModule,
		faultModule,
		repairModule,
		materialModule,
		statusModule,
	}, nil
}
