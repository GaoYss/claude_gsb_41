package material

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

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

// ---------------- 材料目录 ----------------

// ListCatalogs 查询材料目录列表。
func (h *Handler) ListCatalogs(c *gin.Context) {
	var query CatalogListQuery
	if err := httpx.BindQuery(c, &query); err != nil {
		response.Fail(c, err)
		return
	}
	items, total, page, err := h.service.ListCatalogs(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.NewPageData(items, total, page.Page, page.PageSize))
}

// CreateCatalog 新增材料目录。
func (h *Handler) CreateCatalog(c *gin.Context) {
	var req CatalogCreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	entity, err := h.service.CreateCatalog(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, entity)
}

// CatalogMeta 材料目录字典。
func (h *Handler) CatalogMeta(c *gin.Context) {
	meta, err := h.service.CatalogMeta(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, meta)
}

// UpdateCatalog 修改材料目录。
func (h *Handler) UpdateCatalog(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var req CatalogUpdateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	entity, err := h.service.UpdateCatalog(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, entity)
}

// DeleteCatalog 删除材料目录。
func (h *Handler) DeleteCatalog(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.service.DeleteCatalog(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// ---------------- 故障材料清单 ----------------

// ListItems 查询指定故障的材料清单与完整情况。
func (h *Handler) ListItems(c *gin.Context) {
	faultID, err := httpx.ParseID(c, "faultId")
	if err != nil {
		response.Fail(c, err)
		return
	}
	items, err := h.service.ListItems(c.Request.Context(), faultID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, items)
}

// ListItemsPage 跨故障材料明细分页查询。
func (h *Handler) ListItemsPage(c *gin.Context) {
	var query FaultMaterialListQuery
	if err := httpx.BindQuery(c, &query); err != nil {
		response.Fail(c, err)
		return
	}
	items, total, page, err := h.service.ListItemsPage(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.NewPageData(items, total, page.Page, page.PageSize))
}

// CreateItem 为故障登记一项所需材料。
func (h *Handler) CreateItem(c *gin.Context) {
	faultID, err := httpx.ParseID(c, "faultId")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var req FaultMaterialCreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	entity, err := h.service.CreateItem(c.Request.Context(), faultID, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, entity)
}

// UpdateItem 修改故障材料项。
func (h *Handler) UpdateItem(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var req FaultMaterialUpdateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	entity, err := h.service.UpdateItem(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, entity)
}

// ReceiveItem 标记材料到位 / 取消到位。
func (h *Handler) ReceiveItem(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var req ReceiveRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	entity, err := h.service.ReceiveItem(c.Request.Context(), id, req.Received)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, entity)
}

// DeleteItem 删除故障材料项。
func (h *Handler) DeleteItem(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.service.DeleteItem(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// Completeness 查询单条故障的材料完整情况。
func (h *Handler) Completeness(c *gin.Context) {
	faultID, err := httpx.ParseID(c, "faultId")
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.Completeness(c.Request.Context(), faultID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// Summary 材料完整率与缺失材料故障清单(运行看板使用)。
func (h *Handler) Summary(c *gin.Context) {
	result, err := h.service.Summary(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// ---------------- 现场照片/视频 ----------------

// ListMedia 查询故障现场媒体, 按登记/维修过程/完工验收三个阶段分组返回。
func (h *Handler) ListMedia(c *gin.Context) {
	faultID, err := httpx.ParseID(c, "faultId")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var query MediaQuery
	if err := httpx.BindQuery(c, &query); err != nil {
		response.Fail(c, err)
		return
	}
	query.FaultID = faultID
	items, err := h.service.ListMedia(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, GroupMedia(items))
}

// UploadMedia 上传现场照片或视频(multipart/form-data, 字段 file)。
func (h *Handler) UploadMedia(c *gin.Context) {
	faultID, err := httpx.ParseID(c, "faultId")
	if err != nil {
		response.Fail(c, err)
		return
	}

	header, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, err)
		return
	}
	file, err := header.Open()
	if err != nil {
		response.Fail(c, err)
		return
	}
	defer file.Close()

	entity, err := h.service.UploadMedia(
		c.Request.Context(),
		faultID,
		c.PostForm("stage"),
		c.PostForm("uploader"),
		c.PostForm("remark"),
		file,
		header.Filename,
		header.Size,
	)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, entity)
}

// DownloadMedia 下载(或在线播放)现场媒体文件, 支持 Range 请求。
func (h *Handler) DownloadMedia(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	entity, reader, err := h.service.OpenMedia(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	defer reader.Close()

	seekReader, ok := reader.(io.ReadSeeker)
	if !ok {
		response.Fail(c, fmt.Errorf("媒体文件不支持断点读取"))
		return
	}

	// 缺省附件下载, download=0 时内联展示(图片预览 / 视频播放由浏览器处理)。
	disposition := "attachment"
	if c.Query("download") == "0" {
		disposition = "inline"
	}
	c.Header("Content-Disposition", buildDisposition(disposition, entity.FileName))
	if entity.MimeType != "" {
		c.Header("Content-Type", entity.MimeType)
	}
	c.Header("Content-Length", strconv.FormatInt(entity.FileSize, 10))
	c.Header("Accept-Ranges", "bytes")

	// http.ServeContent 原生支持 Range / 206, 满足视频拖动播放。
	http.ServeContent(c.Writer, c.Request, entity.FileName, entity.CreatedAt, seekReader)
}

// DeleteMedia 删除现场媒体。
func (h *Handler) DeleteMedia(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.service.DeleteMedia(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// buildDisposition 组装同时兼容 ASCII 与 UTF-8 文件名的 Content-Disposition。
func buildDisposition(kind, fileName string) string {
	fallback := url.PathEscape(fileName)
	return fmt.Sprintf("%s; filename=\"%s\"; filename*=UTF-8''%s", kind, fallback, url.PathEscape(fileName))
}
