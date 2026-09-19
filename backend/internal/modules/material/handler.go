package material

import (
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

	"streetlight/internal/apperr"
	"streetlight/internal/httpx"
	"streetlight/internal/response"
)

// Handler 处理现场材料相关的 HTTP 请求。
type Handler struct {
	service *Service
}

// NewHandler 构造现场材料处理器。
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Upload 上传故障某个阶段的现场照片或视频(multipart 表单: file + stage + title)。
func (h *Handler) Upload(c *gin.Context) {
	faultID, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	header, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, apperr.BadRequest("请选择要上传的照片或视频文件"))
		return
	}
	entity, err := h.service.Upload(c.Request.Context(), faultID, c.PostForm("stage"), c.PostForm("title"), header)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, entity)
}

// ListByFault 按阶段分组查询故障的现场材料。
func (h *Handler) ListByFault(c *gin.Context) {
	faultID, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.ListByFault(c.Request.Context(), faultID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// File 读取材料文件: 默认内联展示(供 <img>/<video> 引用), download=1 时作为附件下载。
func (h *Handler) File(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	file, entity, err := h.service.FileForRead(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	defer file.Close()

	c.Header("Content-Type", entity.ContentType)
	c.Header("Content-Length", strconv.FormatInt(entity.Size, 10))
	c.Header("X-Content-Type-Options", "nosniff")
	if c.Query("download") == "1" {
		name := entity.OriginalName
		if name == "" {
			name = entity.Title
		}
		c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, file)
}

// Delete 删除现场材料。
func (h *Handler) Delete(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}
