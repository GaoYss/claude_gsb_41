package material

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"streetlight/internal/apperr"
	"streetlight/internal/modules/fault"
)

// 上传大小限制: 照片 10MB, 视频 200MB。
const (
	maxPhotoSize = 10 << 20
	maxVideoSize = 200 << 20
)

// 允许的照片 / 视频 MIME 类型及其标准扩展名。
var photoTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

var videoTypes = map[string]string{
	"video/mp4":       ".mp4",
	"video/quicktime": ".mov",
	"video/webm":      ".webm",
	"video/x-m4v":     ".m4v",
}

// FaultPort 由故障登记模块实现, 材料模块通过它确认故障存在且未关闭。
type FaultPort interface {
	GetByID(ctx context.Context, id uint) (*fault.Fault, error)
}

// Service 承载现场材料的业务规则: 分阶段上传、完整性评估与文件存取。
type Service struct {
	repo      *Repository
	faults    FaultPort
	uploadDir string
}

// NewService 构造现场材料服务, uploadDir 为文件落盘目录。
func NewService(repo *Repository, faults FaultPort, uploadDir string) *Service {
	return &Service{repo: repo, faults: faults, uploadDir: uploadDir}
}

// Repository 暴露仓储, 供状态查询模块装配只读统计。
func (s *Service) Repository() *Repository { return s.repo }

// Upload 为指定故障的某个阶段上传一张照片或一段视频。
func (s *Service) Upload(ctx context.Context, faultID uint, stage string, title string, header *multipart.FileHeader) (*Material, error) {
	target, err := s.faults.GetByID(ctx, faultID)
	if err != nil {
		return nil, err
	}
	if target.Status == fault.StatusClosed {
		return nil, apperr.Conflict("故障 %s 已关闭, 不允许再上传现场材料", target.FaultNo)
	}

	stage = strings.TrimSpace(stage)
	if !IsValidStage(stage) {
		return nil, apperr.BadRequest("非法的材料阶段: %s (可选: registration / repair / acceptance)", stage)
	}
	if header == nil || header.Size == 0 {
		return nil, apperr.BadRequest("请选择要上传的照片或视频文件")
	}

	kind, contentType, err := detectKind(header)
	if err != nil {
		return nil, err
	}
	if err = checkSize(kind, header.Size); err != nil {
		return nil, err
	}

	src, err := header.Open()
	if err != nil {
		return nil, apperr.BadRequest("读取上传文件失败, 请重试")
	}
	defer src.Close()

	relative, err := s.storeFile(target.FaultNo, stage, contentType, src)
	if err != nil {
		return nil, err
	}

	entity := &Material{
		FaultID:      target.ID,
		FaultNo:      target.FaultNo,
		Stage:        stage,
		Kind:         kind,
		Title:        strings.TrimSpace(title),
		FileName:     relative,
		OriginalName: filepath.Base(header.Filename),
		ContentType:  contentType,
		Size:         header.Size,
	}
	if entity.Title == "" {
		entity.Title = entity.OriginalName
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		_ = os.Remove(s.absolutePath(relative))
		return nil, err
	}
	return entity, nil
}

// ListByFault 按阶段分组返回故障的现场材料与完整性结论。
func (s *Service) ListByFault(ctx context.Context, faultID uint) (*FaultMaterials, error) {
	if _, err := s.faults.GetByID(ctx, faultID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListByFault(ctx, faultID)
	if err != nil {
		return nil, err
	}
	counts, err := s.repo.CountByFaults(ctx, []uint{faultID})
	if err != nil {
		return nil, err
	}

	byStage := make(map[string][]Material, len(Stages()))
	for _, item := range items {
		byStage[item.Stage] = append(byStage[item.Stage], item)
	}

	groups := make([]StageGroup, 0, len(Stages()))
	for _, stage := range Stages() {
		stageItems := byStage[stage]
		if stageItems == nil {
			stageItems = make([]Material, 0)
		}
		groups = append(groups, StageGroup{
			Stage:       stage,
			Label:       StageLabel(stage),
			Requirement: requirementText(stage),
			Items:       stageItems,
		})
	}

	missing := MissingFromCounts(counts, faultID)
	return &FaultMaterials{
		FaultID:  faultID,
		Complete: len(missing) == 0,
		Missing:  missing,
		Stages:   groups,
	}, nil
}

// Delete 删除材料记录并清理磁盘文件, 已关闭故障的材料不允许删除。
func (s *Service) Delete(ctx context.Context, id uint) error {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	target, err := s.faults.GetByID(ctx, entity.FaultID)
	if err != nil {
		return err
	}
	if target.Status == fault.StatusClosed {
		return apperr.Conflict("故障 %s 已关闭, 现场材料不允许删除", target.FaultNo)
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if err := os.Remove(s.absolutePath(entity.FileName)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("清理材料文件失败: %w", err)
	}
	return nil
}

// FileForRead 返回材料文件的可读句柄与元信息, 调用方负责关闭。
func (s *Service) FileForRead(ctx context.Context, id uint) (*os.File, *Material, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	file, err := os.Open(s.absolutePath(entity.FileName))
	if os.IsNotExist(err) {
		return nil, nil, apperr.NotFound("材料文件已丢失: id=%d", id)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("读取材料文件失败: %w", err)
	}
	return file, entity, nil
}

// MissingRequired 评估故障缺失的必要材料, 实现故障模块的闭环校验端口。
func (s *Service) MissingRequired(ctx context.Context, faultID uint) ([]string, error) {
	counts, err := s.repo.CountByFaults(ctx, []uint{faultID})
	if err != nil {
		return nil, err
	}
	return MissingFromCounts(counts, faultID), nil
}

// CompletenessByFaults 批量评估多条故障缺失的必要材料, 实现故障列表的材料齐全标记。
func (s *Service) CompletenessByFaults(ctx context.Context, faultIDs []uint) (map[uint][]string, error) {
	result := make(map[uint][]string, len(faultIDs))
	if len(faultIDs) == 0 {
		return result, nil
	}
	counts, err := s.repo.CountByFaults(ctx, faultIDs)
	if err != nil {
		return nil, err
	}
	for _, id := range faultIDs {
		result[id] = MissingFromCounts(counts, id)
	}
	return result, nil
}

// storeFile 将上传内容写入 uploadDir/<故障单号>/<阶段>/ 下的随机文件名, 返回相对路径。
func (s *Service) storeFile(faultNo, stage, contentType string, src io.Reader) (string, error) {
	ext := extensionOf(contentType)
	name := fmt.Sprintf("%s-%s%s", time.Now().Format("20060102-150405"), randomToken(), ext)
	relative := filepath.Join(sanitizeSegment(faultNo), stage, name)

	absolute := s.absolutePath(relative)
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return "", fmt.Errorf("创建材料目录失败: %w", err)
	}
	dst, err := os.OpenFile(absolute, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("保存材料文件失败: %w", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(absolute)
		return "", fmt.Errorf("写入材料文件失败: %w", err)
	}
	return relative, nil
}

// absolutePath 将库中存储的相对路径还原为磁盘绝对路径, 并防止路径逃逸。
func (s *Service) absolutePath(relative string) string {
	cleaned := filepath.Clean(relative)
	base := filepath.Clean(s.uploadDir)
	absolute := filepath.Join(base, cleaned)
	if absolute != base && !strings.HasPrefix(absolute, base+string(os.PathSeparator)) {
		return filepath.Join(base, filepath.Base(cleaned))
	}
	return absolute
}

// detectKind 依据声明类型与文件嗅探判定材料类型, 返回规范化后的 MIME。
func detectKind(header *multipart.FileHeader) (string, string, error) {
	declared := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	candidates := []string{declared}

	// 浏览器对个别格式会上报 application/octet-stream, 此时 sniff 前 512 字节兜底。
	if declared == "" || declared == "application/octet-stream" {
		if sniffed := sniffContentType(header); sniffed != "" {
			candidates = []string{sniffed, declared}
		}
	}

	for _, candidate := range candidates {
		if _, ok := photoTypes[candidate]; ok {
			return KindPhoto, candidate, nil
		}
		if _, ok := videoTypes[candidate]; ok {
			return KindVideo, candidate, nil
		}
	}
	return "", "", apperr.BadRequest("仅支持上传照片(jpg/png/webp/gif)或视频(mp4/mov/webm), 当前类型: %s", declared)
}

// sniffContentType 读取文件头 512 字节嗅探真实类型。
func sniffContentType(header *multipart.FileHeader) string {
	src, err := header.Open()
	if err != nil {
		return ""
	}
	defer src.Close()
	buffer := make([]byte, 512)
	n, err := src.Read(buffer)
	if err != nil && n == 0 {
		return ""
	}
	return http.DetectContentType(buffer[:n])
}

// checkSize 按材料类型校验文件大小。
func checkSize(kind string, size int64) error {
	if kind == KindPhoto && size > maxPhotoSize {
		return apperr.BadRequest("照片大小不能超过 %d MB, 当前 %.1f MB", maxPhotoSize>>20, float64(size)/float64(1<<20))
	}
	if kind == KindVideo && size > maxVideoSize {
		return apperr.BadRequest("视频大小不能超过 %d MB, 当前 %.1f MB", maxVideoSize>>20, float64(size)/float64(1<<20))
	}
	return nil
}

// extensionOf 返回 MIME 类型对应的标准扩展名。
func extensionOf(contentType string) string {
	if ext, ok := photoTypes[contentType]; ok {
		return ext
	}
	if ext, ok := videoTypes[contentType]; ok {
		return ext
	}
	return ".bin"
}

// randomToken 生成短随机串, 避免同秒上传重名。
func randomToken() string {
	buffer := make([]byte, 4)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
	}
	return hex.EncodeToString(buffer)
}

// sanitizeSegment 清洗路径片段, 仅保留字母数字与横线, 防止目录穿越。
func sanitizeSegment(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			builder.WriteRune(r)
		}
	}
	if builder.Len() == 0 {
		return "misc"
	}
	return builder.String()
}
