package material_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"streetlight/internal/apperr"
	"streetlight/internal/modules/fault"
	"streetlight/internal/modules/lamp"
	"streetlight/internal/modules/material"
)

type harness struct {
	lamps     *lamp.Service
	faults    *fault.Service
	materials *material.Service
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(
		&lamp.Lamp{}, &fault.Fault{},
		&material.Catalog{}, &material.FaultMaterial{}, &material.Media{},
	))

	lampRepository := lamp.NewRepository(db)
	lampService := lamp.NewService(lampRepository)

	faultRepository := fault.NewRepository(db)
	faultService := fault.NewService(faultRepository, lampService)
	lampService.SetOpenFaultCounter(faultRepository)

	storage, err := material.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	materialRepository := material.NewRepository(db)
	materialService := material.NewService(materialRepository, faultService, storage)
	faultService.SetMaterialStatusPort(materialService)

	return &harness{lamps: lampService, faults: faultService, materials: materialService}
}

func (h *harness) createLamp(t *testing.T, code string) *lamp.Lamp {
	t.Helper()
	entity, err := h.lamps.Create(context.Background(), lamp.CreateRequest{
		Code: code, Name: "测试灯杆", RoadName: "测试路", LampType: lamp.LampTypeLED,
	})
	require.NoError(t, err)
	return entity
}

func (h *harness) createFault(t *testing.T, lampID uint) *fault.Fault {
	t.Helper()
	entity, err := h.faults.Create(context.Background(), fault.CreateRequest{
		LampID: lampID, FaultType: "灯不亮", FaultLevel: fault.LevelHigh,
		Description: "现场材料模块测试", Reporter: "巡检员",
	})
	require.NoError(t, err)
	return entity
}

func boolPointer(value bool) *bool { return &value }

// 必要材料缺失时不允许故障闭环, 齐全后可以关闭。
func TestCloseBlockedUntilRequiredMaterialsReceived(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	device := h.createLamp(t, "LD-M-001")
	target := h.createFault(t, device.ID)

	// 登记一项尚未到位的必要材料
	_, err := h.materials.CreateItem(ctx, target.ID, material.FaultMaterialCreateRequest{
		Name: "LED 驱动电源", Spec: "150W", Unit: "个", Required: boolPointer(true),
	})
	require.NoError(t, err)

	// 缺失必要材料: 关闭被拦截
	_, err = h.faults.Close(ctx, target.ID, fault.CloseRequest{Remark: "尝试闭环"})
	require.Error(t, err)
	bizErr, ok := apperr.As(err)
	require.True(t, ok)
	require.Equal(t, apperr.CodeConflict, bizErr.Code)

	// 故障列表应标记材料未齐全
	list, _, _, err := h.faults.List(ctx, fault.ListQuery{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.True(t, list[0].MaterialConfigured)
	require.False(t, list[0].MaterialComplete)
	require.Equal(t, 1, list[0].MaterialMissingCount)

	// 标记材料到位
	items, err := h.materials.ListItems(ctx, target.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	_, err = h.materials.ReceiveItem(ctx, items[0].ID, true)
	require.NoError(t, err)

	// 材料齐全: 关闭成功
	updated, err := h.faults.GetByID(ctx, target.ID)
	require.NoError(t, err)
	require.True(t, updated.MaterialComplete)
	_, err = h.faults.Close(ctx, target.ID, fault.CloseRequest{Remark: "材料齐全, 闭环"})
	require.NoError(t, err)
}

// 非必要材料缺失不拦截闭环; 完整率汇总按故障维度统计。
func TestOptionalMaterialAndSummary(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	// 故障一: 必要材料齐全, 另有一项可选材料未到位 => 齐全
	device1 := h.createLamp(t, "LD-M-101")
	fault1 := h.createFault(t, device1.ID)
	required, err := h.materials.CreateItem(ctx, fault1.ID, material.FaultMaterialCreateRequest{
		Name: "熔断器", Required: boolPointer(true), Received: boolPointer(true),
	})
	require.NoError(t, err)
	require.True(t, required.Received)
	_, err = h.materials.CreateItem(ctx, fault1.ID, material.FaultMaterialCreateRequest{
		Name: "扎带", Required: boolPointer(false),
	})
	require.NoError(t, err)

	// 故障二: 必要材料缺失 => 不齐全
	device2 := h.createLamp(t, "LD-M-102")
	fault2 := h.createFault(t, device2.ID)
	_, err = h.materials.CreateItem(ctx, fault2.ID, material.FaultMaterialCreateRequest{
		Name: "通讯模块", Required: boolPointer(true),
	})
	require.NoError(t, err)

	summary, err := h.materials.Summary(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), summary.FaultCount)
	require.Equal(t, int64(1), summary.CompleteCount)
	require.Equal(t, int64(1), summary.IncompleteCount)
	require.Equal(t, float64(50), summary.CompleteRate)
	require.Len(t, summary.MissingFaults, 1)
	require.Equal(t, fault2.ID, summary.MissingFaults[0].FaultID)
	require.Equal(t, 1, summary.MissingFaults[0].MissingCount)
	require.Equal(t, "通讯模块", summary.MissingFaults[0].MissingItems[0].Name)

	// 单故障完整情况
	completeness, err := h.materials.Completeness(ctx, fault1.ID)
	require.NoError(t, err)
	require.True(t, completeness.Complete)
	require.Equal(t, 2, completeness.TotalCount)
	require.Equal(t, 1, completeness.RequiredCount)
}

// 照片上传后按阶段分组, 视频与图片类型识别正确, 不支持的类型被拒绝。
func TestUploadAndGroupMedia(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	device := h.createLamp(t, "LD-M-201")
	target := h.createFault(t, device.ID)

	png := []byte("fake-png-content")
	image, err := h.materials.UploadMedia(
		ctx, target.ID, material.StageRegister, "巡检员", "故障现场",
		bytes.NewReader(png), "现场.jpg", int64(len(png)),
	)
	require.NoError(t, err)
	require.Equal(t, material.MediaImage, image.MediaType)
	require.Equal(t, material.StageRegister, image.Stage)
	require.NotEmpty(t, image.StorageKey)

	video := []byte("fake-video-content")
	mov, err := h.materials.UploadMedia(
		ctx, target.ID, material.StageAcceptance, "验收员", "完工视频",
		bytes.NewReader(video), "验收.mp4", int64(len(video)),
	)
	require.NoError(t, err)
	require.Equal(t, material.MediaVideo, mov.MediaType)

	// 非法阶段
	_, err = h.materials.UploadMedia(ctx, target.ID, "unknown", "", "",
		bytes.NewReader(png), "x.jpg", 1)
	require.Error(t, err)

	// 非法文件类型
	_, err = h.materials.UploadMedia(ctx, target.ID, material.StageProcess, "", "",
		bytes.NewReader(png), "virus.exe", 1)
	require.Error(t, err)

	groups, err := h.materials.ListMedia(ctx, material.MediaQuery{FaultID: target.ID})
	require.NoError(t, err)
	grouped := material.GroupMedia(groups)
	require.Len(t, grouped, 3, "三个阶段都应输出")
	require.Equal(t, material.StageRegister, grouped[0].Stage)
	require.Len(t, grouped[0].Items, 1)
	require.Len(t, grouped[1].Items, 0)
	require.Len(t, grouped[2].Items, 1)

	// 文件可以读回
	_, reader, err := h.materials.OpenMedia(ctx, image.ID)
	require.NoError(t, err)
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	require.Equal(t, png, content)
}

// 已关闭故障不允许再变更材料或上传媒体。
func TestMutationRejectedAfterClose(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	device := h.createLamp(t, "LD-M-301")
	target := h.createFault(t, device.ID)

	item, err := h.materials.CreateItem(ctx, target.ID, material.FaultMaterialCreateRequest{
		Name: "驱动电源", Required: boolPointer(true), Received: boolPointer(true),
	})
	require.NoError(t, err)
	_, err = h.faults.Close(ctx, target.ID, fault.CloseRequest{})
	require.NoError(t, err)

	_, err = h.materials.CreateItem(ctx, target.ID, material.FaultMaterialCreateRequest{Name: "其它"})
	require.Error(t, err)
	_, err = h.materials.ReceiveItem(ctx, item.ID, false)
	require.Error(t, err)
	_, err = h.materials.UploadMedia(ctx, target.ID, material.StageRegister, "", "",
		bytes.NewReader([]byte("x")), "a.jpg", 1)
	require.Error(t, err)
}
