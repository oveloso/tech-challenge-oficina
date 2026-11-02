package workorders

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"oficina/internal/domain"
	"oficina/internal/platform"
)

type createOrderRequest struct {
	CustomerID         string `json:"customerId"`
	VehicleID          string `json:"vehicleId"`
	ProblemDescription string `json:"problemDescription"`
	Selection
}

func (r *createOrderRequest) validate() error {
	if err := platform.UUID(r.CustomerID); err != nil {
		return err
	}
	if err := platform.UUID(r.VehicleID); err != nil {
		return err
	}
	r.CustomerID = strings.ToLower(r.CustomerID)
	r.VehicleID = strings.ToLower(r.VehicleID)
	r.ProblemDescription = strings.TrimSpace(r.ProblemDescription)
	if length := utf8.RuneCountInString(r.ProblemDescription); length < 1 || length > 4000 {
		return domain.Invalid("Tamanho inválido para problemDescription.")
	}
	return r.Selection.Validate()
}

type versionRequest struct {
	ExpectedVersion int64 `json:"expectedVersion"`
}

func (r versionRequest) validate() error {
	if r.ExpectedVersion < 1 {
		return domain.Invalid("expectedVersion deve ser um inteiro positivo.")
	}
	return nil
}

type replaceItemsRequest struct {
	versionRequest
	Selection
}

func (r *replaceItemsRequest) validate() error {
	if err := r.versionRequest.validate(); err != nil {
		return err
	}
	return r.Selection.Validate()
}

type quoteDecisionRequest struct {
	QuoteID  string  `json:"quoteId"`
	Decision string  `json:"decision"`
	Reason   *string `json:"reason"`
}

func (r *quoteDecisionRequest) validate() error {
	if err := platform.UUID(r.QuoteID); err != nil {
		return err
	}
	r.QuoteID = strings.ToLower(r.QuoteID)
	if r.Decision != "APPROVED" && r.Decision != "REJECTED" {
		return domain.Invalid("Decisão inválida.")
	}
	if r.Reason != nil && r.Decision == "APPROVED" && strings.TrimSpace(*r.Reason) != "" {
		return domain.Invalid("O motivo só se aplica à rejeição.")
	}
	return nil
}

func (r *quoteDecisionRequest) reasonText() string {
	if r.Reason == nil {
		return ""
	}
	return *r.Reason
}

func decodeOrderRequest(c *gin.Context, request any) error {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return domain.Invalid("Envie o corpo como application/json.")
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(request); err != nil {
		return domain.Invalid("JSON inválido, campo não permitido ou corpo acima de 1 MiB.")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return domain.Invalid("Envie apenas um objeto JSON.")
	}
	return nil
}
