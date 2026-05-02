package handler

import (
	"net/http"
	"strconv"

	"fraud-scorer/internal/cache"
	"fraud-scorer/internal/config"
	"fraud-scorer/internal/models"
	"fraud-scorer/internal/scorer"
	"fraud-scorer/internal/store"
	"fraud-scorer/internal/worker"

	"github.com/gin-gonic/gin"
)

// TransactionHandler holds dependencies for HTTP handlers.
type TransactionHandler struct {
	cfg     *config.Config
	store   store.TransactionStore
	scorer  *scorer.RiskScorer
	cache   *cache.VelocityCache
	workers *worker.IngestWorkerPool
}

// NewTransactionHandler creates a handler.
func NewTransactionHandler(cfg *config.Config, store store.TransactionStore, cache *cache.VelocityCache, workers *worker.IngestWorkerPool) *TransactionHandler {
	s := scorer.NewRiskScorer(cfg, store, cache)
	return &TransactionHandler{cfg: cfg, store: store, scorer: s, cache: cache, workers: workers}
}

// Score godoc
// @Summary      Score a transaction
// @Description  Real-time fraud risk scoring for a transaction during authorization.
// @Tags         scoring
// @Accept       json
// @Produce      json
// @Param        request  body      models.ScoreRequest  true  "Transaction to score"
// @Success      200      {object}  models.RiskScoreResponse
// @Failure      400      {object}  models.ErrorResponse
// @Router       /api/v1/score [post]
func (h *TransactionHandler) Score(c *gin.Context) {
	var req models.ScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	resp, tx, err := h.scorer.Score(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Async persist the scored transaction so future scoring sees it.
	h.workers.Queue(tx)

	c.JSON(http.StatusOK, resp)
}

// Ingest godoc
// @Summary      Ingest a historical transaction
// @Description  Store a transaction record for historical pattern analysis.
// @Tags         transactions
// @Accept       json
// @Produce      json
// @Param        request  body      models.ScoreRequest  true  "Transaction to ingest"
// @Success      201      {object}  models.IngestResponse
// @Failure      400      {object}  models.ErrorResponse
// @Router       /api/v1/transactions [post]
func (h *TransactionHandler) Ingest(c *gin.Context) {
	var req models.ScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	_, tx, err := h.scorer.Score(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Block until persisted for ingestion endpoint.
	h.workers.QueueBlocking(tx)

	c.JSON(http.StatusCreated, models.IngestResponse{Status: "ingested", TransactionID: tx.TransactionID})
}

// List godoc
// @Summary      List transactions
// @Description  Retrieve recent transaction records.
// @Tags         transactions
// @Produce      json
// @Param        limit   query  int  false  "Limit"   default(20)
// @Param        offset  query  int  false  "Offset"  default(0)
// @Success      200     {array}  models.Transaction
// @Router       /api/v1/transactions [get]
func (h *TransactionHandler) List(c *gin.Context) {
	limit := 20
	offset := 0
	if l, ok := c.GetQuery("limit"); ok {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o, ok := c.GetQuery("offset"); ok {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	txs, err := h.store.List(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, txs)
}

// Feedback godoc
// @Summary      Submit feedback
// @Description  Mark a transaction as confirmed fraud or legitimate.
// @Tags         feedback
// @Accept       json
// @Produce      json
// @Param        request  body      models.FeedbackRequest  true  "Feedback payload"
// @Success      200      {object}  models.FeedbackResponse
// @Failure      400      {object}  models.ErrorResponse
// @Router       /api/v1/feedback [post]
func (h *TransactionHandler) Feedback(c *gin.Context) {
	var req models.FeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.store.UpdateConfirmedStatus(c.Request.Context(), req.TransactionID, req.ConfirmedStatus); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.FeedbackResponse{
		Status:          "updated",
		TransactionID:   req.TransactionID,
		ConfirmedStatus: req.ConfirmedStatus,
	})
}

// Health godoc
// @Summary      Health check
// @Description  Check if the service is running.
// @Tags         health
// @Produce      json
// @Success      200  {object}  models.HealthResponse
// @Router       /health [get]
func (h *TransactionHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, models.HealthResponse{Status: "ok"})
}
