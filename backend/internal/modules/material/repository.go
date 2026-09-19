package material

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"streetlight/internal/apperr"
	"streetlight/internal/modules/fault"
	"streetlight/pkg/pagination"
)

// CatalogFilter 材料目录查询条件。
type CatalogFilter struct {
	Keyword  string
	Category string
	Enabled  *bool
}

// FaultMaterialFilter 故障材料清单查询条件。
type FaultMaterialFilter struct {
	Keyword     string
	RoadName    string
	Received    string // "" / yes / no
	MissingOnly bool   // 仅查询必要且未到位
}

// Repository 负责现场材料相关的数据访问。
type Repository struct {
	db *gorm.DB
}

// NewRepository 构造现场材料仓储。
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) session(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

// ---------------- 材料目录 ----------------

// CreateCatalog 新增材料目录项。
func (r *Repository) CreateCatalog(ctx context.Context, entity *Catalog) error {
	if err := r.session(ctx).Create(entity).Error; err != nil {
		return fmt.Errorf("新增材料目录失败: %w", err)
	}
	return nil
}

// UpdateCatalog 保存材料目录项。
func (r *Repository) UpdateCatalog(ctx context.Context, entity *Catalog) error {
	if err := r.session(ctx).Save(entity).Error; err != nil {
		return fmt.Errorf("更新材料目录失败: %w", err)
	}
	return nil
}

// GetCatalogByID 按主键查询材料目录。
func (r *Repository) GetCatalogByID(ctx context.Context, id uint) (*Catalog, error) {
	var entity Catalog
	err := r.session(ctx).First(&entity, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("材料目录不存在: id=%d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询材料目录失败: %w", err)
	}
	return &entity, nil
}

// ListCatalogs 分页查询材料目录。
func (r *Repository) ListCatalogs(ctx context.Context, filter CatalogFilter, page pagination.Query) ([]Catalog, int64, error) {
	base := func() *gorm.DB {
		statement := r.session(ctx).Model(&Catalog{})
		if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			statement = statement.Where("name LIKE ? OR spec LIKE ?", like, like)
		}
		if value := strings.TrimSpace(filter.Category); value != "" {
			statement = statement.Where("category = ?", value)
		}
		if filter.Enabled != nil {
			statement = statement.Where("enabled = ?", *filter.Enabled)
		}
		return statement
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计材料目录失败: %w", err)
	}

	entities := make([]Catalog, 0)
	if err := base().Order("id DESC").Offset(page.Offset()).Limit(page.Limit()).Find(&entities).Error; err != nil {
		return nil, 0, fmt.Errorf("查询材料目录失败: %w", err)
	}
	return entities, total, nil
}

// ListEnabledCatalogs 查询全部启用的材料目录, 供登记材料时选择。
func (r *Repository) ListEnabledCatalogs(ctx context.Context) ([]Catalog, error) {
	entities := make([]Catalog, 0)
	err := r.session(ctx).Where("enabled = ?", true).Order("category ASC, id ASC").Find(&entities).Error
	if err != nil {
		return nil, fmt.Errorf("查询材料目录选项失败: %w", err)
	}
	return entities, nil
}

// DistinctCatalogCategories 查询材料分类列表。
func (r *Repository) DistinctCatalogCategories(ctx context.Context) ([]string, error) {
	values := make([]string, 0)
	err := r.session(ctx).Model(&Catalog{}).
		Where("category <> ''").
		Distinct().
		Order("category").
		Pluck("category", &values).Error
	if err != nil {
		return nil, fmt.Errorf("查询材料分类失败: %w", err)
	}
	return values, nil
}

// DeleteCatalog 删除材料目录, 已被故障材料引用的记录由数据库外键策略处理,
// 当前故障材料仅保存 catalog_id 快照, 删除目录不影响现场清单。
func (r *Repository) DeleteCatalog(ctx context.Context, id uint) error {
	result := r.session(ctx).Delete(&Catalog{}, id)
	if result.Error != nil {
		return fmt.Errorf("删除材料目录失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return apperr.NotFound("材料目录不存在: id=%d", id)
	}
	return nil
}

// ---------------- 故障材料清单 ----------------

// CreateFaultMaterial 新增故障材料项。
func (r *Repository) CreateFaultMaterial(ctx context.Context, entity *FaultMaterial) error {
	if err := r.session(ctx).Create(entity).Error; err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict("材料 %s 已在该故障清单中", entity.DisplayName())
		}
		return fmt.Errorf("登记故障材料失败: %w", err)
	}
	return nil
}

// UpdateFaultMaterial 保存故障材料项。
func (r *Repository) UpdateFaultMaterial(ctx context.Context, entity *FaultMaterial) error {
	if err := r.session(ctx).Save(entity).Error; err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict("材料 %s 与清单中其它项重复", entity.DisplayName())
		}
		return fmt.Errorf("更新故障材料失败: %w", err)
	}
	return nil
}

// DeleteFaultMaterial 删除故障材料项。
func (r *Repository) DeleteFaultMaterial(ctx context.Context, id uint) error {
	result := r.session(ctx).Delete(&FaultMaterial{}, id)
	if result.Error != nil {
		return fmt.Errorf("删除故障材料失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return apperr.NotFound("故障材料项不存在: id=%d", id)
	}
	return nil
}

// GetFaultMaterialByID 按主键查询故障材料项。
func (r *Repository) GetFaultMaterialByID(ctx context.Context, id uint) (*FaultMaterial, error) {
	var entity FaultMaterial
	err := r.session(ctx).First(&entity, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("故障材料项不存在: id=%d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询故障材料失败: %w", err)
	}
	return &entity, nil
}

// ListFaultMaterials 查询指定故障的材料清单, 必要项优先、再按登记顺序排列。
func (r *Repository) ListFaultMaterials(ctx context.Context, faultID uint) ([]FaultMaterial, error) {
	entities := make([]FaultMaterial, 0)
	err := r.session(ctx).
		Where("fault_id = ?", faultID).
		Order("required DESC, id ASC").
		Find(&entities).Error
	if err != nil {
		return nil, fmt.Errorf("查询故障材料清单失败: %w", err)
	}
	return entities, nil
}

// ListFaultMaterialsPage 跨故障分页查询材料项, 用于材料管理总览。
func (r *Repository) ListFaultMaterialsPage(ctx context.Context, filter FaultMaterialFilter, page pagination.Query) ([]FaultMaterial, int64, error) {
	base := func() *gorm.DB {
		statement := r.session(ctx).Model(&FaultMaterial{})
		if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			statement = statement.Where(
				"fault_no LIKE ? OR lamp_code LIKE ? OR name LIKE ?",
				like, like, like,
			)
		}
		if value := strings.TrimSpace(filter.RoadName); value != "" {
			statement = statement.Where("road_name = ?", value)
		}
		switch filter.Received {
		case "yes":
			statement = statement.Where("received = ?", true)
		case "no":
			statement = statement.Where("received = ?", false)
		}
		if filter.MissingOnly {
			statement = statement.Where("required = ? AND received = ?", true, false)
		}
		return statement
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计故障材料失败: %w", err)
	}

	entities := make([]FaultMaterial, 0)
	if err := base().Order("id DESC").Offset(page.Offset()).Limit(page.Limit()).Find(&entities).Error; err != nil {
		return nil, 0, fmt.Errorf("查询故障材料失败: %w", err)
	}
	return entities, total, nil
}

// CountFaultMaterials 统计某条故障的材料项数量。
func (r *Repository) CountFaultMaterials(ctx context.Context, faultID uint) (int64, error) {
	var total int64
	err := r.session(ctx).Model(&FaultMaterial{}).Where("fault_id = ?", faultID).Count(&total).Error
	if err != nil {
		return 0, fmt.Errorf("统计故障材料数量失败: %w", err)
	}
	return total, nil
}

// FaultIDsWithMaterials 返回配置了材料清单的全部故障 ID。
func (r *Repository) FaultIDsWithMaterials(ctx context.Context) ([]uint, error) {
	ids := make([]uint, 0)
	err := r.session(ctx).Model(&FaultMaterial{}).
		Distinct().
		Order("fault_id").
		Pluck("fault_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("查询材料故障清单失败: %w", err)
	}
	return ids, nil
}

// FaultsByIDs 按主键批量查询故障, 用于材料完整率汇总(状态模块同款读模型做法)。
func (r *Repository) FaultsByIDs(ctx context.Context, ids []uint) ([]fault.Fault, error) {
	entities := make([]fault.Fault, 0)
	if len(ids) == 0 {
		return entities, nil
	}
	err := r.session(ctx).Model(&fault.Fault{}).Where("id IN ?", ids).Find(&entities).Error
	if err != nil {
		return nil, fmt.Errorf("查询故障信息失败: %w", err)
	}
	return entities, nil
}

// MissingByFaultIDs 批量查询给定故障下"必要且未到位"的材料项, 按故障 ID 分组。
func (r *Repository) MissingByFaultIDs(ctx context.Context, faultIDs []uint) (map[uint][]FaultMaterial, error) {
	result := make(map[uint][]FaultMaterial)
	if len(faultIDs) == 0 {
		return result, nil
	}
	entities := make([]FaultMaterial, 0)
	err := r.session(ctx).
		Where("fault_id IN ? AND required = ? AND received = ?", faultIDs, true, false).
		Order("fault_id ASC, id ASC").
		Find(&entities).Error
	if err != nil {
		return nil, fmt.Errorf("查询缺失材料失败: %w", err)
	}
	for index := range entities {
		item := entities[index]
		result[item.FaultID] = append(result[item.FaultID], item)
	}
	return result, nil
}

// CountMissingByFaultID 查询单条故障缺失的必要材料数量。
func (r *Repository) CountMissingByFaultID(ctx context.Context, faultID uint) (int64, error) {
	var total int64
	err := r.session(ctx).Model(&FaultMaterial{}).
		Where("fault_id = ? AND required = ? AND received = ?", faultID, true, false).
		Count(&total).Error
	if err != nil {
		return 0, fmt.Errorf("统计缺失材料失败: %w", err)
	}
	return total, nil
}

// StatsByFaultIDs 批量统计每个故障的材料项数量 / 必要项数 / 已到位数量。
func (r *Repository) StatsByFaultIDs(ctx context.Context, faultIDs []uint) (map[uint]FaultMaterialStat, error) {
	type row struct {
		FaultID       uint
		TotalCount    int64
		RequiredCount int64
		ReceivedCount int64
	}
	rows := make([]row, 0)
	result := make(map[uint]FaultMaterialStat, len(faultIDs))
	if len(faultIDs) == 0 {
		return result, nil
	}
	err := r.session(ctx).Model(&FaultMaterial{}).
		Select("fault_id, COUNT(*) AS total_count, "+
			"SUM(CASE WHEN required THEN 1 ELSE 0 END) AS required_count, "+
			"SUM(CASE WHEN received THEN 1 ELSE 0 END) AS received_count").
		Where("fault_id IN ?", faultIDs).
		Group("fault_id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("统计材料到位情况失败: %w", err)
	}
	for _, item := range rows {
		result[item.FaultID] = FaultMaterialStat{
			TotalCount:    item.TotalCount,
			RequiredCount: item.RequiredCount,
			ReceivedCount: item.ReceivedCount,
		}
	}
	return result, nil
}

// FaultMaterialStat 是单条故障的材料数量统计(持久层内部使用)。
type FaultMaterialStat struct {
	TotalCount    int64
	RequiredCount int64
	ReceivedCount int64
}

// MaterialTotals 汇总全部材料项 / 必要项 / 缺失项数量。
func (r *Repository) MaterialTotals(ctx context.Context) (itemTotal, requiredTotal, missingTotal int64, err error) {
	type row struct {
		ItemTotal     int64
		RequiredTotal int64
		MissingTotal  int64
	}
	var data row
	err = r.session(ctx).Model(&FaultMaterial{}).
		Select("COUNT(*) AS item_total, " +
			"SUM(CASE WHEN required THEN 1 ELSE 0 END) AS required_total, " +
			"SUM(CASE WHEN required AND NOT received THEN 1 ELSE 0 END) AS missing_total").
		Scan(&data).Error
	if err != nil {
		return 0, 0, 0, fmt.Errorf("汇总材料数量失败: %w", err)
	}
	return data.ItemTotal, data.RequiredTotal, data.MissingTotal, nil
}

// ---------------- 现场媒体 ----------------

// CreateMedia 新增媒体记录。
func (r *Repository) CreateMedia(ctx context.Context, entity *Media) error {
	if err := r.session(ctx).Create(entity).Error; err != nil {
		return fmt.Errorf("保存现场媒体记录失败: %w", err)
	}
	return nil
}

// GetMediaByID 按主键查询媒体记录。
func (r *Repository) GetMediaByID(ctx context.Context, id uint) (*Media, error) {
	var entity Media
	err := r.session(ctx).First(&entity, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("现场媒体不存在: id=%d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询现场媒体失败: %w", err)
	}
	return &entity, nil
}

// ListMedia 按故障 / 阶段 / 类型查询媒体记录。
func (r *Repository) ListMedia(ctx context.Context, filter MediaFilter) ([]Media, error) {
	entities := make([]Media, 0)
	statement := r.session(ctx).Model(&Media{})
	if filter.FaultID > 0 {
		statement = statement.Where("fault_id = ?", filter.FaultID)
	}
	if filter.Stage != "" {
		statement = statement.Where("stage = ?", filter.Stage)
	}
	if filter.MediaType != "" {
		statement = statement.Where("media_type = ?", filter.MediaType)
	}
	if err := statement.Order("stage ASC, id DESC").Find(&entities).Error; err != nil {
		return nil, fmt.Errorf("查询现场媒体失败: %w", err)
	}
	return entities, nil
}

// MediaFilter 媒体查询条件。
type MediaFilter struct {
	FaultID   uint
	Stage     string
	MediaType string
}

// DeleteMedia 删除媒体记录。
func (r *Repository) DeleteMedia(ctx context.Context, id uint) error {
	result := r.session(ctx).Delete(&Media{}, id)
	if result.Error != nil {
		return fmt.Errorf("删除现场媒体失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return apperr.NotFound("现场媒体不存在: id=%d", id)
	}
	return nil
}

// isUniqueViolation 兼容 sqlite 与 postgres 的唯一约束冲突判断。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint failed") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "unique violation")
}
