package material

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"path/filepath"
	"strings"
	"time"

	"streetlight/internal/apperr"
	"streetlight/internal/modules/fault"
	"streetlight/pkg/pagination"
)

// 上传文件大小限制。
const (
	MaxImageSize = 20 << 20  // 照片 20MB
	MaxVideoSize = 200 << 20 // 视频 200MB
)

// materialSortSpec 材料目录列表排序白名单(当前固定 id 倒序, 保留扩展位)。
var catalogSortSpec = pagination.SortSpec{
	Allowed: map[string]string{
		"id":         "id",
		"name":       "name",
		"category":   "category",
		"created_at": "created_at",
	},
	Default: "id",
}

var itemSortSpec = pagination.SortSpec{
	Allowed: map[string]string{
		"id":         "id",
		"fault_no":   "fault_no",
		"name":       "name",
		"created_at": "created_at",
	},
	Default: "id",
}

// 允许上传的照片/视频扩展名。
var (
	imageExts = map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
		".gif": "image/gif", ".webp": "image/webp", ".bmp": "image/bmp",
	}
	videoExts = map[string]string{
		".mp4": "video/mp4", ".mov": "video/quicktime", ".avi": "video/x-msvideo",
		".mkv": "video/x-matroska", ".webm": "video/webm", ".3gp": "video/3gpp",
	}
)

// FaultPort 由故障登记模块实现, 材料模块通过它读取故障信息。
type FaultPort interface {
	GetByID(ctx context.Context, id uint) (*fault.Fault, error)
}

// Service 承载现场材料管理的业务规则。
type Service struct {
	repo    *Repository
	faults  FaultPort
	storage *FileStorage
}

// NewService 构造现场材料服务。
func NewService(repo *Repository, faults FaultPort, storage *FileStorage) *Service {
	return &Service{repo: repo, faults: faults, storage: storage}
}

// ---------------- 材料目录 ----------------

// ListCatalogs 分页查询材料目录。
func (s *Service) ListCatalogs(ctx context.Context, query CatalogListQuery) ([]Catalog, int64, pagination.Query, error) {
	page := pagination.Parse(query.Params, catalogSortSpec)
	enabled := query.Enabled
	filter := CatalogFilter{Keyword: strings.TrimSpace(query.Keyword), Category: strings.TrimSpace(query.Category), Enabled: enabled}
	items, total, err := s.repo.ListCatalogs(ctx, filter, page)
	if err != nil {
		return nil, 0, page, err
	}
	return items, total, page, nil
}

// CreateCatalog 新增材料目录。
func (s *Service) CreateCatalog(ctx context.Context, req CatalogCreateRequest) (*Catalog, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperr.BadRequest("材料名称不能为空")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	entity := &Catalog{
		Name:     name,
		Spec:     strings.TrimSpace(req.Spec),
		Unit:     strings.TrimSpace(req.Unit),
		Category: strings.TrimSpace(req.Category),
		Enabled:  enabled,
		Remark:   strings.TrimSpace(req.Remark),
	}
	if err := s.repo.CreateCatalog(ctx, entity); err != nil {
		return nil, err
	}
	return entity, nil
}

// UpdateCatalog 修改材料目录。
func (s *Service) UpdateCatalog(ctx context.Context, id uint, req CatalogUpdateRequest) (*Catalog, error) {
	entity, err := s.repo.GetCatalogByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, apperr.BadRequest("材料名称不能为空")
		}
		entity.Name = name
	}
	if req.Spec != nil {
		entity.Spec = strings.TrimSpace(*req.Spec)
	}
	if req.Unit != nil {
		entity.Unit = strings.TrimSpace(*req.Unit)
	}
	if req.Category != nil {
		entity.Category = strings.TrimSpace(*req.Category)
	}
	if req.Enabled != nil {
		entity.Enabled = *req.Enabled
	}
	if req.Remark != nil {
		entity.Remark = strings.TrimSpace(*req.Remark)
	}
	if err := s.repo.UpdateCatalog(ctx, entity); err != nil {
		return nil, err
	}
	return entity, nil
}

// DeleteCatalog 删除材料目录, 已被故障材料引用时解除关联但保留现场快照。
func (s *Service) DeleteCatalog(ctx context.Context, id uint) error {
	if _, err := s.repo.GetCatalogByID(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteCatalog(ctx, id)
}

// CatalogMeta 返回材料目录相关字典。
func (s *Service) CatalogMeta(ctx context.Context) (*CatalogMeta, error) {
	categories, err := s.repo.DistinctCatalogCategories(ctx)
	if err != nil {
		return nil, err
	}
	return &CatalogMeta{Categories: categories, Stages: Stages(), MediaTypes: []string{MediaImage, MediaVideo}}, nil
}

// ---------------- 故障材料清单 ----------------

// ListItems 查询某条故障的材料清单。
func (s *Service) ListItems(ctx context.Context, faultID uint) ([]FaultMaterial, error) {
	if _, err := s.faults.GetByID(ctx, faultID); err != nil {
		return nil, err
	}
	return s.repo.ListFaultMaterials(ctx, faultID)
}

// ListItemsPage 跨故障明细分页查询(材料管理总览页)。
func (s *Service) ListItemsPage(ctx context.Context, query FaultMaterialListQuery) ([]FaultMaterial, int64, pagination.Query, error) {
	page := pagination.Parse(query.Params, itemSortSpec)
	received := strings.TrimSpace(query.Received)
	if received != "" && received != "yes" && received != "no" {
		return nil, 0, page, apperr.BadRequest("到位状态取值只能是 yes 或 no")
	}
	filter := FaultMaterialFilter{
		Keyword:     strings.TrimSpace(query.Keyword),
		RoadName:    strings.TrimSpace(query.RoadName),
		Received:    received,
		MissingOnly: query.MissingOnly,
	}
	items, total, err := s.repo.ListFaultMaterialsPage(ctx, filter, page)
	if err != nil {
		return nil, 0, page, err
	}
	return items, total, page, nil
}

// CreateItem 为故障登记一项所需材料。
func (s *Service) CreateItem(ctx context.Context, faultID uint, req FaultMaterialCreateRequest) (*FaultMaterial, error) {
	target, err := s.faults.GetByID(ctx, faultID)
	if err != nil {
		return nil, err
	}
	if target.Status == fault.StatusClosed {
		return nil, apperr.Conflict("故障 %s 已关闭, 不允许再登记材料", target.FaultNo)
	}

	entity := &FaultMaterial{
		FaultID:  target.ID,
		FaultNo:  target.FaultNo,
		LampCode: target.LampCode,
		RoadName: target.RoadName,
		Remark:   strings.TrimSpace(req.Remark),
	}

	// 引用材料目录时以目录为主数据, 请求中显式给出的取值优先。
	if req.CatalogID != nil && *req.CatalogID > 0 {
		catalog, err := s.repo.GetCatalogByID(ctx, *req.CatalogID)
		if err != nil {
			return nil, err
		}
		entity.CatalogID = &catalog.ID
		entity.Name = catalog.Name
		entity.Spec = catalog.Spec
		entity.Unit = catalog.Unit
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		entity.Name = name
	}
	if entity.Name == "" {
		return nil, apperr.BadRequest("材料名称不能为空")
	}
	if req.Spec != "" {
		entity.Spec = strings.TrimSpace(req.Spec)
	}
	if req.Unit != "" {
		entity.Unit = strings.TrimSpace(req.Unit)
	}

	entity.Quantity = 1
	if req.Quantity != nil {
		entity.Quantity = *req.Quantity
	}
	entity.Required = true
	if req.Required != nil {
		entity.Required = *req.Required
	}
	if req.Received != nil && *req.Received {
		entity.Received = true
		now := time.Now()
		entity.ReceivedAt = &now
	}

	if err := s.repo.CreateFaultMaterial(ctx, entity); err != nil {
		return nil, err
	}
	return entity, nil
}

// UpdateItem 修改故障材料项。
func (s *Service) UpdateItem(ctx context.Context, id uint, req FaultMaterialUpdateRequest) (*FaultMaterial, error) {
	entity, err := s.repo.GetFaultMaterialByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureFaultOpen(ctx, entity.FaultID); err != nil {
		return nil, err
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, apperr.BadRequest("材料名称不能为空")
		}
		entity.Name = name
	}
	if req.Spec != nil {
		entity.Spec = strings.TrimSpace(*req.Spec)
	}
	if req.Unit != nil {
		entity.Unit = strings.TrimSpace(*req.Unit)
	}
	if req.Quantity != nil {
		entity.Quantity = *req.Quantity
	}
	if req.Required != nil {
		entity.Required = *req.Required
	}
	if req.Remark != nil {
		entity.Remark = strings.TrimSpace(*req.Remark)
	}
	if err := s.repo.UpdateFaultMaterial(ctx, entity); err != nil {
		return nil, err
	}
	return entity, nil
}

// ReceiveItem 标记材料到位 / 取消到位。
func (s *Service) ReceiveItem(ctx context.Context, id uint, received bool) (*FaultMaterial, error) {
	entity, err := s.repo.GetFaultMaterialByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureFaultOpen(ctx, entity.FaultID); err != nil {
		return nil, err
	}
	entity.Received = received
	if received {
		now := time.Now()
		entity.ReceivedAt = &now
	} else {
		entity.ReceivedAt = nil
	}
	if err := s.repo.UpdateFaultMaterial(ctx, entity); err != nil {
		return nil, err
	}
	return entity, nil
}

// DeleteItem 删除故障材料项。
func (s *Service) DeleteItem(ctx context.Context, id uint) error {
	entity, err := s.repo.GetFaultMaterialByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ensureFaultOpen(ctx, entity.FaultID); err != nil {
		return err
	}
	return s.repo.DeleteFaultMaterial(ctx, id)
}

// Completeness 查询单条故障的材料完整情况。
func (s *Service) Completeness(ctx context.Context, faultID uint) (*Completeness, error) {
	target, err := s.faults.GetByID(ctx, faultID)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.ListFaultMaterials(ctx, faultID)
	if err != nil {
		return nil, err
	}
	return buildCompleteness(target, items), nil
}

// BatchStatus 批量查询故障材料状态, 实现故障模块的 MaterialStatusPort 端口。
func (s *Service) BatchStatus(ctx context.Context, faultIDs []uint) (map[uint]fault.MaterialStatus, error) {
	result := make(map[uint]fault.MaterialStatus, len(faultIDs))
	if len(faultIDs) == 0 {
		return result, nil
	}
	stats, err := s.repo.StatsByFaultIDs(ctx, faultIDs)
	if err != nil {
		return nil, err
	}
	missing, err := s.repo.MissingByFaultIDs(ctx, faultIDs)
	if err != nil {
		return nil, err
	}
	for _, id := range faultIDs {
		stat, ok := stats[id]
		if !ok {
			result[id] = fault.MaterialStatus{}
			continue
		}
		missingCount := int64(len(missing[id]))
		result[id] = fault.MaterialStatus{
			HasChecklist:  stat.TotalCount > 0,
			Complete:      missingCount == 0,
			TotalCount:    int(stat.TotalCount),
			RequiredCount: int(stat.RequiredCount),
			MissingCount:  int(missingCount),
		}
	}
	return result, nil
}

// AssertClosable 必要材料缺失时拒绝故障闭环, 实现故障模块的 MaterialStatusPort 端口。
func (s *Service) AssertClosable(ctx context.Context, faultID uint) error {
	missing, err := s.repo.MissingByFaultIDs(ctx, []uint{faultID})
	if err != nil {
		return err
	}
	items := missing[faultID]
	if len(items) == 0 {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.DisplayName())
	}
	return apperr.Conflict("必要材料尚未齐全, 不允许闭环。缺失材料: %s", strings.Join(names, "、"))
}

// Summary 汇总材料完整率与缺失材料的故障清单, 供运行看板展示。
func (s *Service) Summary(ctx context.Context) (*CompletenessSummary, error) {
	ids, err := s.repo.FaultIDsWithMaterials(ctx)
	if err != nil {
		return nil, err
	}
	targets, err := s.repo.FaultsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	targetMap := make(map[uint]*fault.Fault, len(targets))
	for index := range targets {
		targetMap[targets[index].ID] = &targets[index]
	}

	missingMap, err := s.repo.MissingByFaultIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	stats, err := s.repo.StatsByFaultIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	itemTotal, requiredTotal, missingTotal, err := s.repo.MaterialTotals(ctx)
	if err != nil {
		return nil, err
	}

	summary := &CompletenessSummary{
		FaultCount:    int64(len(ids)),
		ItemTotal:     itemTotal,
		RequiredTotal: requiredTotal,
		MissingTotal:  missingTotal,
		MissingFaults: make([]Completeness, 0),
	}

	incomplete := make([]Completeness, 0)
	for _, id := range ids {
		target := targetMap[id]
		if target == nil {
			continue
		}
		missingItems := missingMap[id]
		stat := stats[id]
		record := Completeness{
			FaultID:       target.ID,
			FaultNo:       target.FaultNo,
			LampCode:      target.LampCode,
			RoadName:      target.RoadName,
			FaultStatus:   target.Status,
			TotalCount:    int(stat.TotalCount),
			RequiredCount: int(stat.RequiredCount),
			ReceivedCount: int(stat.ReceivedCount),
			MissingCount:  len(missingItems),
			Complete:      len(missingItems) == 0,
			MissingItems:  missingItems,
		}
		if record.Complete {
			summary.CompleteCount++
		} else {
			summary.IncompleteCount++
			incomplete = append(incomplete, record)
		}
	}

	if summary.FaultCount > 0 {
		rate := float64(summary.CompleteCount) * 100 / float64(summary.FaultCount)
		summary.CompleteRate = math.Round(rate*100) / 100
	}

	// 缺失材料的故障按缺失数量倒序, 取前 20 条进入看板清单。
	sortByMissingDesc(incomplete)
	if len(incomplete) > 20 {
		incomplete = incomplete[:20]
	}
	summary.MissingFaults = incomplete
	return summary, nil
}

// ---------------- 现场照片/视频 ----------------

// ListMedia 查询现场媒体, 可按故障 / 阶段 / 类型过滤。
func (s *Service) ListMedia(ctx context.Context, query MediaQuery) ([]Media, error) {
	filter := MediaFilter{FaultID: query.FaultID}
	if stage := strings.TrimSpace(query.Stage); stage != "" {
		if !IsValidStage(stage) {
			return nil, apperr.BadRequest("非法的材料阶段: %s", stage)
		}
		filter.Stage = stage
	}
	if mediaType := strings.TrimSpace(query.MediaType); mediaType != "" {
		if mediaType != MediaImage && mediaType != MediaVideo {
			return nil, apperr.BadRequest("非法的媒体类型: %s", mediaType)
		}
		filter.MediaType = mediaType
	}
	return s.repo.ListMedia(ctx, filter)
}

// UploadMedia 保存上传的现场照片/视频并登记媒体记录。
func (s *Service) UploadMedia(ctx context.Context, faultID uint, stage, uploader, remark string, reader io.Reader, fileName string, fileSize int64) (*Media, error) {
	target, err := s.faults.GetByID(ctx, faultID)
	if err != nil {
		return nil, err
	}
	if target.Status == fault.StatusClosed {
		return nil, apperr.Conflict("故障 %s 已关闭, 不允许再上传现场材料", target.FaultNo)
	}
	stage = strings.TrimSpace(stage)
	if !IsValidStage(stage) {
		return nil, apperr.BadRequest("非法的材料阶段: %s", stage)
	}

	mediaType, ext, err := classifyUpload(fileName, fileSize)
	if err != nil {
		return nil, err
	}

	storageKey, err := s.storage.Save(reader, ext)
	if err != nil {
		return nil, err
	}

	entity := &Media{
		FaultID:    target.ID,
		FaultNo:    target.FaultNo,
		Stage:      stage,
		MediaType:  mediaType,
		FileName:   sanitizeFileName(fileName),
		StorageKey: storageKey,
		MimeType:   mimeTypeByExt(ext, mediaType),
		FileSize:   fileSize,
		Uploader:   strings.TrimSpace(uploader),
		Remark:     strings.TrimSpace(remark),
	}
	if err := s.repo.CreateMedia(ctx, entity); err != nil {
		if removeErr := s.storage.Remove(storageKey); removeErr != nil {
			slog.Warn("清理媒体文件失败", "storage_key", storageKey, "error", removeErr)
		}
		return nil, err
	}
	return entity, nil
}

// OpenMedia 取媒体记录并打开其文件, 调用方负责关闭文件。
func (s *Service) OpenMedia(ctx context.Context, id uint) (*Media, io.ReadCloser, error) {
	entity, err := s.repo.GetMediaByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	file, err := s.storage.Open(entity.StorageKey)
	if err != nil {
		return nil, nil, err
	}
	return entity, file, nil
}

// DeleteMedia 删除媒体记录与文件。
func (s *Service) DeleteMedia(ctx context.Context, id uint) error {
	entity, err := s.repo.GetMediaByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ensureFaultOpen(ctx, entity.FaultID); err != nil {
		return err
	}
	if err := s.repo.DeleteMedia(ctx, id); err != nil {
		return err
	}
	if err := s.storage.Remove(entity.StorageKey); err != nil {
		slog.Warn("删除媒体文件失败", "storage_key", entity.StorageKey, "error", err)
	}
	return nil
}

// ensureFaultOpen 校验故障未关闭, 已关闭时拒绝材料变更。
func (s *Service) ensureFaultOpen(ctx context.Context, faultID uint) error {
	target, err := s.faults.GetByID(ctx, faultID)
	if err != nil {
		return err
	}
	if target.Status == fault.StatusClosed {
		return apperr.Conflict("故障 %s 已关闭, 材料清单不允许变更", target.FaultNo)
	}
	return nil
}

// buildCompleteness 依据故障与其材料清单计算完整情况。
func buildCompleteness(target *fault.Fault, items []FaultMaterial) *Completeness {
	result := &Completeness{
		FaultID:      target.ID,
		FaultNo:      target.FaultNo,
		LampCode:     target.LampCode,
		RoadName:     target.RoadName,
		FaultStatus:  target.Status,
		TotalCount:   len(items),
		MissingItems: make([]FaultMaterial, 0),
	}
	for _, item := range items {
		if item.Required {
			result.RequiredCount++
			if !item.Received {
				result.MissingCount++
				result.MissingItems = append(result.MissingItems, item)
			}
		}
		if item.Received {
			result.ReceivedCount++
		}
	}
	result.Complete = result.MissingCount == 0
	return result
}

// sortByMissingDesc 缺失清单按缺失数量倒序、故障 ID 正序排列。
func sortByMissingDesc(items []Completeness) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].MissingCount > items[i].MissingCount ||
				(items[j].MissingCount == items[i].MissingCount && items[j].FaultID < items[i].FaultID) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// classifyUpload 依据扩展名与大小判定媒体类型, 拒绝不支持的文件。
func classifyUpload(fileName string, fileSize int64) (mediaType, ext string, err error) {
	ext = strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	if ext == "" {
		return "", "", apperr.BadRequest("无法识别文件类型, 请上传带扩展名的照片或视频")
	}
	if _, ok := imageExts[ext]; ok {
		if fileSize > MaxImageSize {
			return "", "", apperr.BadRequest("照片大小不能超过 %dMB", MaxImageSize>>20)
		}
		return MediaImage, ext, nil
	}
	if _, ok := videoExts[ext]; ok {
		if fileSize > MaxVideoSize {
			return "", "", apperr.BadRequest("视频大小不能超过 %dMB", MaxVideoSize>>20)
		}
		return MediaVideo, ext, nil
	}
	return "", "", apperr.BadRequest("不支持的文件格式 %s, 仅允许上传照片(jpg/png/gif/webp 等)或视频(mp4/mov/avi 等)", ext)
}

// mimeTypeByExt 返回扩展名对应的 MIME 类型, 未知时按媒体类型给兜底值。
func mimeTypeByExt(ext, mediaType string) string {
	if mime, ok := imageExts[ext]; ok {
		return mime
	}
	if mime, ok := videoExts[ext]; ok {
		return mime
	}
	if mediaType == MediaVideo {
		return "video/mp4"
	}
	return "image/jpeg"
}

// sanitizeFileName 去掉文件名中的路径分隔符, 仅保留基名。
func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "\x00", "")
	if len(name) > 255 {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		maxBase := 255 - len(ext)
		if maxBase < 1 {
			maxBase = 1
		}
		name = base[:maxBase] + ext
	}
	if name == "" || name == "." || name == string(filepath.Separator) {
		return fmt.Sprintf("media%s", filepath.Ext(strings.TrimSpace(name)))
	}
	return name
}
