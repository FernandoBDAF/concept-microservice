package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/fernandobarroso/microservices/api-service/internal/domain/document"
)

type DocumentHandler struct {
	service *document.Service
	logger  *zap.Logger
}

func NewDocumentHandler(service *document.Service, logger *zap.Logger) *DocumentHandler {
	return &DocumentHandler{
		service: service,
		logger:  logger.Named("document_handler"),
	}
}

func (h *DocumentHandler) Upload(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	userID, err := uuid.Parse(toString(userIDVal))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	profileIDStr := c.PostForm("profile_id")
	if profileIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "profile_id is required"})
		return
	}
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile_id"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	mtype, err := mimetype.DetectReader(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to detect file type"})
		return
	}

	if _, err := file.Seek(0, 0); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process file"})
		return
	}

	doc, taskID, err := h.service.Upload(
		c.Request.Context(),
		userID,
		profileID,
		header.Filename,
		file,
		header.Size,
		mtype.String(),
	)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"document_id": doc.ID,
		"task_id":     taskID,
		"filename":    doc.OriginalFilename,
		"status":      doc.Status,
		"message":     "Document uploaded successfully, processing queued",
	})
}

func (h *DocumentHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID"})
		return
	}

	doc, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, doc.ToResponse())
}

func (h *DocumentHandler) GetStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID"})
		return
	}

	doc, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response := gin.H{
		"id":     doc.ID,
		"status": doc.Status,
	}

	if doc.ProcessingStartedAt != nil {
		response["processing_started_at"] = doc.ProcessingStartedAt
	}
	if doc.ProcessingCompletedAt != nil {
		response["processing_completed_at"] = doc.ProcessingCompletedAt
	}
	if doc.ErrorMessage != nil {
		response["error_message"] = doc.ErrorMessage
	}

	c.JSON(http.StatusOK, response)
}

func (h *DocumentHandler) Download(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID"})
		return
	}

	url, err := h.service.GetDownloadURL(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"download_url": url,
		"expires_in":   "15 minutes",
	})
}

func (h *DocumentHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Document deleted successfully",
	})
}

func (h *DocumentHandler) ListByProfile(c *gin.Context) {
	profileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile ID"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	docs, total, err := h.service.GetByProfileID(c.Request.Context(), profileID, page, pageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}

	responses := make([]*document.DocumentResponse, len(docs))
	for i, doc := range docs {
		responses[i] = doc.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{
		"documents":   responses,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + pageSize - 1) / pageSize,
	})
}

func (h *DocumentHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, document.ErrDocumentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
	case errors.Is(err, document.ErrInvalidFileType):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, document.ErrInvalidMimeType):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, document.ErrFileTooLarge):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, document.ErrEmptyFile):
		c.JSON(http.StatusBadRequest, gin.H{"error": "file cannot be empty"})
	default:
		h.logger.Error("Unexpected error", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
