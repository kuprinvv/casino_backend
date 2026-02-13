package line

import (
	dto "casino_backend/internal/api/dto/line"
	"casino_backend/internal/converter"
	"casino_backend/internal/service"
	"casino_backend/pkg/req"
	"casino_backend/pkg/resp"
	"net/http"
)

type HandlerDeps struct {
	Serv service.LineService
}

type Handler struct {
	serv service.LineService
}

func NewHandler(deps HandlerDeps) *Handler {
	return &Handler{serv: deps.Serv}
}

func (h *Handler) Spin(w http.ResponseWriter, r *http.Request) {
	payload, err := req.Decode[dto.LineSpinRequest](r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.serv.Spin(r.Context(), converter.ToLineSpin(payload))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := converter.ToLineSpinResponse(*result)

	resp.WriteJSONResponse(w, http.StatusOK, response)
}

func (h *Handler) BuyBonus(w http.ResponseWriter, r *http.Request) {
	payload, err := req.Decode[dto.BonusSpinRequest](r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.serv.BuyBonus(r.Context(), converter.ToBonusSpin(payload))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := converter.ToBonusSpinResponse(*result)

	resp.WriteJSONResponse(w, http.StatusOK, response)
}
