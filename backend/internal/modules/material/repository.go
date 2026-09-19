package material

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"streetlight/internal/apperr"
)

// Repository 负责现场材料的数据访问。
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

// Create 新增材料记录。
func (r *Repository) Create(ctx context.Context, entity *Material) error {
	if err := r.session(ctx).Create(entity).Error; err != nil {
		return fmt.Errorf("登记现场材料失败: %w", err)
	}
	return nil
}

// GetByID 按主键查询材料。
func (r *Repository) GetByID(ctx context.Context, id uint) (*Material, error) {
	var entity Material
	err := r.session(ctx).First(&entity, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("现场材料不存在: id=%d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询现场材料失败: %w", err)
	}
	return &entity, nil
}

// Delete 按主键删除材料记录。
func (r *Repository) Delete(ctx context.Context, id uint) error {
	if err := r.session(ctx).Delete(&Material{}, id).Error; err != nil {
		return fmt.Errorf("删除现场材料失败: %w", err)
	}
	return nil
}

// ListByFault 查询某条故障的全部材料, 按阶段与上传时间排序。
func (r *Repository) ListByFault(ctx context.Context, faultID uint) ([]Material, error) {
	entities := make([]Material, 0)
	err := r.session(ctx).
		Where("fault_id = ?", faultID).
		Order("stage ASC, id ASC").
		Find(&entities).Error
	if err != nil {
		return nil, fmt.Errorf("查询故障现场材料失败: %w", err)
	}
	return entities, nil
}

// CountByFaults 按故障分组统计各阶段各类型的材料数量, faultIDs 为空时统计全部故障。
func (r *Repository) CountByFaults(ctx context.Context, faultIDs []uint) ([]CountRow, error) {
	rows := make([]CountRow, 0)
	statement := r.session(ctx).Model(&Material{}).
		Select("fault_id, stage, kind, COUNT(*) AS total").
		Group("fault_id, stage, kind")
	if len(faultIDs) > 0 {
		statement = statement.Where("fault_id IN ?", faultIDs)
	}
	if err := statement.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("统计现场材料失败: %w", err)
	}
	return rows, nil
}

// CountFiles 统计材料文件总数。
func (r *Repository) CountFiles(ctx context.Context) (int64, error) {
	var total int64
	if err := r.session(ctx).Model(&Material{}).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("统计现场材料总数失败: %w", err)
	}
	return total, nil
}
