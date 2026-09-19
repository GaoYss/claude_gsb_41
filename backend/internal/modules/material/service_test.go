package material_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/textproto"
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

// 1x1 像素 PNG, 用于构造合法的照片上传内容。
var tinyPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE, 0x00, 0x00, 0x00,
	0x0C, 0x49, 0x44, 0x41, 0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
	0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D, 0xB0, 0x00, 0x00, 0x00,
	0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

// harness 使用内存数据库与临时上传目录装配真实模块, 验证材料与故障的联动规则。
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

	require.NoError(t, db.AutoMigrate(&lamp.Lamp{}, &fault.Fault{}, &material.Material{}))

	lampRepository := lamp.NewRepository(db)
	lampService := lamp.NewService(lampRepository)

	faultRepository := fault.NewRepository(db)
	faultService := fault.NewService(faultRepository, lampService)
	lampService.SetOpenFaultCounter(faultRepository)

	materialRepository := material.NewRepository(db)
	materialService := material.NewService(materialRepository, faultService, t.TempDir())
	faultService.SetMaterialPort(materialService)

	return &harness{lamps: lampService, faults: faultService, materials: materialService}
}

func (h *harness) createFault(t *testing.T) *fault.Fault {
	t.Helper()
	ctx := context.Background()
	device, err := h.lamps.Create(ctx, lamp.CreateRequest{
		Code:     "LD-M-001",
		RoadName: "测试路",
		LampType: lamp.LampTypeLED,
	})
	require.NoError(t, err)

	entity, err := h.faults.Create(ctx, fault.CreateRequest{
		LampID:      device.ID,
		FaultType:   "灯不亮",
		FaultLevel:  fault.LevelHigh,
		Description: "材料模块测试故障",
		Reporter:    "巡检员",
	})
	require.NoError(t, err)
	return entity
}

// makeFile 构造一个 multipart 文件头, 模拟浏览器上传。
func makeFile(t *testing.T, filename, contentType string, content []byte) *multipart.FileHeader {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="file"; filename="` + filename + `"`},
		"Content-Type":        {contentType},
	})
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	reader := multipart.NewReader(body, writer.Boundary())
	form, err := reader.ReadForm(10 << 20)
	require.NoError(t, err)
	files := form.File["file"]
	require.Len(t, files, 1)
	return files[0]
}

// uploadAllStages 为故障上传三个阶段的必要材料, 使其满足闭环条件。
func uploadAllStages(t *testing.T, h *harness, faultID uint) {
	t.Helper()
	ctx := context.Background()
	for _, stage := range []string{material.StageRegistration, material.StageRepair, material.StageAcceptance} {
		_, err := h.materials.Upload(ctx, faultID, stage, "", makeFile(t, stage+".png", "image/png", tinyPNG))
		require.NoError(t, err, "阶段 %s 上传失败", stage)
	}
}

func TestUploadAndCompletenessFlow(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	entity := h.createFault(t)

	// 初始状态: 三项必要材料全部缺失
	missing, err := h.materials.MissingRequired(ctx, entity.ID)
	require.NoError(t, err)
	require.Len(t, missing, 3)

	// 上传登记照片后仍缺两项
	photo, err := h.materials.Upload(ctx, entity.ID, material.StageRegistration, "现场照片", makeFile(t, "scene.png", "image/png", tinyPNG))
	require.NoError(t, err)
	require.Equal(t, material.KindPhoto, photo.Kind)
	require.Equal(t, "现场照片", photo.Title)
	require.Equal(t, entity.FaultNo, photo.FaultNo)

	missing, err = h.materials.MissingRequired(ctx, entity.ID)
	require.NoError(t, err)
	require.Len(t, missing, 2)

	// 必要材料缺失时故障不允许闭环
	_, err = h.faults.Close(ctx, entity.ID, fault.CloseRequest{Remark: "尝试闭环"})
	requireConflict(t, err)

	// 补齐维修过程与完工验收材料后材料齐全
	_, err = h.materials.Upload(ctx, entity.ID, material.StageRepair, "", makeFile(t, "repair.png", "image/png", tinyPNG))
	require.NoError(t, err)
	_, err = h.materials.Upload(ctx, entity.ID, material.StageAcceptance, "", makeFile(t, "accept.png", "image/png", tinyPNG))
	require.NoError(t, err)

	missing, err = h.materials.MissingRequired(ctx, entity.ID)
	require.NoError(t, err)
	require.Empty(t, missing)

	// 分组视图: 三个阶段齐全, complete 为 true
	view, err := h.materials.ListByFault(ctx, entity.ID)
	require.NoError(t, err)
	require.True(t, view.Complete)
	require.Empty(t, view.Missing)
	require.Len(t, view.Stages, 3)
	for _, group := range view.Stages {
		require.Len(t, group.Items, 1, "阶段 %s 应有一条材料", group.Stage)
		require.NotEmpty(t, group.Requirement)
	}

	// 材料文件可读取
	file, meta, err := h.materials.FileForRead(ctx, photo.ID)
	require.NoError(t, err)
	require.Equal(t, "image/png", meta.ContentType)
	buffer := make([]byte, len(tinyPNG))
	_, err = file.Read(buffer)
	require.NoError(t, file.Close())
	require.NoError(t, err)
	require.Equal(t, tinyPNG, buffer)

	// 材料齐全后允许闭环
	closed, err := h.faults.Close(ctx, entity.ID, fault.CloseRequest{Remark: "复核通过"})
	require.NoError(t, err)
	require.Equal(t, fault.StatusClosed, closed.Status)

	// 已关闭故障不允许再上传或删除材料
	_, err = h.materials.Upload(ctx, entity.ID, material.StageRepair, "", makeFile(t, "late.png", "image/png", tinyPNG))
	requireConflict(t, err)
	requireConflict(t, h.materials.Delete(ctx, photo.ID))
}

func TestUploadValidation(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	entity := h.createFault(t)

	// 非法阶段
	_, err := h.materials.Upload(ctx, entity.ID, "unknown", "", makeFile(t, "a.png", "image/png", tinyPNG))
	requireBadRequest(t, err)

	// 不支持的文件类型
	_, err = h.materials.Upload(ctx, entity.ID, material.StageRegistration, "", makeFile(t, "a.txt", "text/plain", []byte("hello")))
	requireBadRequest(t, err)

	// 空文件
	_, err = h.materials.Upload(ctx, entity.ID, material.StageRegistration, "", makeFile(t, "a.png", "image/png", nil))
	requireBadRequest(t, err)

	// 故障不存在
	_, err = h.materials.Upload(ctx, 99999, material.StageRegistration, "", makeFile(t, "a.png", "image/png", tinyPNG))
	businessErr, ok := apperr.As(err)
	require.True(t, ok)
	require.Equal(t, http.StatusNotFound, businessErr.Status)
}

func TestCompletenessByFaultsAndDelete(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	entity := h.createFault(t)

	// 批量评估: 未上传时三项全缺
	result, err := h.materials.CompletenessByFaults(ctx, []uint{entity.ID})
	require.NoError(t, err)
	require.Len(t, result[entity.ID], 3)

	// 上传一项后删除, 缺失恢复
	photo, err := h.materials.Upload(ctx, entity.ID, material.StageRegistration, "", makeFile(t, "a.png", "image/png", tinyPNG))
	require.NoError(t, err)
	result, err = h.materials.CompletenessByFaults(ctx, []uint{entity.ID})
	require.NoError(t, err)
	require.Len(t, result[entity.ID], 2)

	require.NoError(t, h.materials.Delete(ctx, photo.ID))
	result, err = h.materials.CompletenessByFaults(ctx, []uint{entity.ID})
	require.NoError(t, err)
	require.Len(t, result[entity.ID], 3)

	// 删除后文件与记录都不应存在
	_, _, err = h.materials.FileForRead(ctx, photo.ID)
	businessErr, ok := apperr.As(err)
	require.True(t, ok)
	require.Equal(t, http.StatusNotFound, businessErr.Status)
}

func TestFaultListCarriesMaterialStatus(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	entity := h.createFault(t)
	uploadAllStages(t, h, entity.ID)

	items, total, _, err := h.faults.List(ctx, fault.ListQuery{})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.NotNil(t, items[0].MaterialComplete)
	require.True(t, *items[0].MaterialComplete)
	require.Empty(t, items[0].MaterialMissing)
}

// requireConflict 断言错误是 409 业务冲突。
func requireConflict(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	businessErr, ok := apperr.As(err)
	require.True(t, ok, "期望业务错误, 实际: %v", err)
	require.Equal(t, http.StatusConflict, businessErr.Status, "错误信息: %s", businessErr.Message)
}

// requireBadRequest 断言错误是 400 参数错误。
func requireBadRequest(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	businessErr, ok := apperr.As(err)
	require.True(t, ok, "期望业务错误, 实际: %v", err)
	require.Equal(t, http.StatusBadRequest, businessErr.Status, "错误信息: %s", businessErr.Message)
}
