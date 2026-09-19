package bootstrap

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"

	"streetlight/internal/modules/fault"
	"streetlight/internal/modules/material"
)

// seedMaterialItem 描述一条故障材料演示数据。
type seedMaterialItem struct {
	name     string
	spec     string
	unit     string
	quantity int
	required bool
	received bool
}

// seedMediaStage 描述一条现场照片演示数据所处的阶段。

// seedMaterialCatalog 构造材料目录演示数据。
func seedMaterialCatalog() []material.Catalog {
	catalogs := []material.Catalog{
		{Name: "LED 驱动电源", Spec: "150W 恒流", Unit: "个", Category: "电气元件", Enabled: true, Remark: "灯不亮/频闪常用备件"},
		{Name: "LED 灯头总成", Spec: "60W IP65", Unit: "套", Category: "照明灯具", Enabled: true},
		{Name: "防水接头", Spec: "二芯 1.5mm²", Unit: "套", Category: "线缆辅材", Enabled: true},
		{Name: "通讯模块", Spec: "NB-IoT", Unit: "个", Category: "控制设备", Enabled: true},
		{Name: "交流接触器", Spec: "25A 220V", Unit: "只", Category: "控制设备", Enabled: true},
		{Name: "熔断器", Spec: "10A", Unit: "只", Category: "电气元件", Enabled: true},
		{Name: "电缆", Spec: "YJV 2x2.5", Unit: "米", Category: "线缆辅材", Enabled: true},
		{Name: "热缩管", Spec: "φ10", Unit: "套", Category: "线缆辅材", Enabled: true},
		{Name: "基础法兰", Spec: "M24 镀锌", Unit: "套", Category: "结构件", Enabled: true},
		{Name: "绝缘胶带", Spec: "PVC 黑色", Unit: "卷", Category: "线缆辅材", Enabled: true},
	}
	return catalogs
}

// seedMaterialPlan 返回各演示故障的材料清单(按 cases 下标对应)。
// 已关闭故障(下标 7-10)的必要材料必须全部到位, 与闭环规则保持一致。
func seedMaterialPlan() map[int][]seedMaterialItem {
	return map[int][]seedMaterialItem{
		0: {
			{"LED 驱动电源", "150W 恒流", "个", 1, true, false},
		},
		1: {
			{"LED 驱动电源", "150W 恒流", "个", 1, true, false},
		},
		2: {
			{"交流接触器", "25A 220V", "只", 1, true, false},
		},
		3: {
			{"防水接头", "二芯 1.5mm²", "套", 2, true, true},
			{"电缆", "YJV 2x2.5", "米", 5, true, false},
		},
		4: {
			{"通讯模块", "NB-IoT", "个", 1, true, false},
		},
		5: {
			{"LED 驱动电源", "150W 恒流", "个", 1, true, true},
		},
		6: {
			{"LED 灯头总成", "60W IP65", "套", 1, true, true},
			{"密封胶条", "L 型", "米", 2, false, false},
		},
		7: {
			{"基础法兰", "M24 镀锌", "套", 1, true, true},
			{"混凝土", "C30", "方", 1, false, true},
		},
		8: {
			{"熔断器", "10A", "只", 1, true, true},
		},
		9: {
			{"LED 驱动电源", "150W 恒流", "个", 1, true, true},
		},
		10: {
			{"电缆", "YJV 2x2.5", "米", 40, true, true},
			{"热缩管", "φ10", "套", 4, true, true},
		},
		11: {
			{"交流接触器", "25A 220V", "只", 1, true, true},
		},
		13: {
			{"通讯模块", "NB-IoT", "个", 1, true, false},
			{"绝缘胶带", "PVC 黑色", "卷", 1, false, true},
		},
	}
}

// seedMediaPlan 返回各演示故障上传现场照片的阶段(按 cases 下标对应)。
func seedMediaPlan() map[int][]string {
	return map[int][]string{
		0:  {material.StageRegister},
		3:  {material.StageRegister, material.StageProcess},
		4:  {material.StageProcess},
		5:  {material.StageRegister, material.StageProcess, material.StageAcceptance},
		6:  {material.StageProcess, material.StageAcceptance},
		7:  {material.StageRegister, material.StageProcess, material.StageAcceptance},
		8:  {material.StageRegister, material.StageAcceptance},
		9:  {material.StageProcess, material.StageAcceptance},
		10: {material.StageRegister, material.StageProcess, material.StageAcceptance},
		11: {material.StageProcess, material.StageAcceptance},
		13: {material.StageProcess},
	}
}

// seedMaterials 写入材料目录、故障材料清单与分阶段现场照片演示数据。
func seedMaterials(db *gorm.DB, mediaRoot string, faults []fault.Fault, cases []seedFaultCase) error {
	catalogs := seedMaterialCatalog()
	if err := db.Create(&catalogs).Error; err != nil {
		return fmt.Errorf("写入材料目录演示数据失败: %w", err)
	}
	catalogByName := make(map[string]*material.Catalog, len(catalogs))
	for index := range catalogs {
		catalogByName[catalogs[index].Name] = &catalogs[index]
	}

	now := time.Now()
	plan := seedMaterialPlan()
	items := make([]material.FaultMaterial, 0)
	for caseIndex := range cases {
		expects, ok := plan[caseIndex]
		if !ok {
			continue
		}
		target := faults[caseIndex]
		for _, expect := range expects {
			row := material.FaultMaterial{
				FaultID:  target.ID,
				FaultNo:  target.FaultNo,
				LampCode: target.LampCode,
				RoadName: target.RoadName,
				Name:     expect.name,
				Spec:     expect.spec,
				Unit:     expect.unit,
				Quantity: expect.quantity,
				Required: expect.required,
				Received: expect.received,
				Remark:   "演示数据",
			}
			if catalog, exists := catalogByName[expect.name]; exists {
				catalogID := catalog.ID
				row.CatalogID = &catalogID
			}
			if expect.received {
				receivedAt := now.Add(-2 * hour)
				row.ReceivedAt = &receivedAt
			}
			items = append(items, row)
		}
	}
	if len(items) > 0 {
		if err := db.Create(&items).Error; err != nil {
			return fmt.Errorf("写入故障材料演示数据失败: %w", err)
		}
	}

	if err := seedMediaFiles(db, mediaRoot, faults, cases); err != nil {
		return err
	}

	slog.Info("现场材料演示数据初始化完成",
		"材料目录", len(catalogs),
		"故障材料项", len(items),
	)
	return nil
}

// placeholderPNG 是 1x1 像素 PNG 占位照片, 用于现场媒体演示数据。
const placeholderPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

// seedMediaFiles 在媒体目录写入占位照片并登记媒体记录。
func seedMediaFiles(db *gorm.DB, mediaRoot string, faults []fault.Fault, cases []seedFaultCase) error {
	pngBytes, err := base64.StdEncoding.DecodeString(placeholderPNG)
	if err != nil {
		return fmt.Errorf("解析演示照片失败: %w", err)
	}

	dir := filepath.Join(mediaRoot, "seed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建演示照片目录失败: %w", err)
	}

	plan := seedMediaPlan()
	medias := make([]material.Media, 0)
	for caseIndex := range cases {
		stages, ok := plan[caseIndex]
		if !ok {
			continue
		}
		target := faults[caseIndex]
		for _, stage := range stages {
			key := filepath.ToSlash(filepath.Join("seed", fmt.Sprintf("%d_%s.png", target.ID, stage)))
			path := filepath.Join(mediaRoot, key)
			if err := os.WriteFile(path, pngBytes, 0o644); err != nil {
				return fmt.Errorf("写入演示照片失败: %w", err)
			}
			medias = append(medias, material.Media{
				FaultID:    target.ID,
				FaultNo:    target.FaultNo,
				Stage:      stage,
				MediaType:  material.MediaImage,
				FileName:   material.StageLabel(stage) + "现场照片.png",
				StorageKey: key,
				MimeType:   "image/png",
				FileSize:   int64(len(pngBytes)),
				Uploader:   "演示数据",
				Remark:     "现场材料管理演示照片",
			})
		}
	}
	if len(medias) > 0 {
		if err := db.Create(&medias).Error; err != nil {
			return fmt.Errorf("写入现场媒体演示数据失败: %w", err)
		}
	}
	return nil
}
