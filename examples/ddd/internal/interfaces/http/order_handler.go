package httpapi

import "net/http"

func (h *handler) createOrder(w http.ResponseWriter, r *http.Request) {
	userID, err := pathID(r)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	var req createOrderRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, h.log, err)
		return
	}
	o, err := h.orders.Place(r.Context(), userID, req.toCommandItems())
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	writeData(w, http.StatusCreated, toOrderResponse(o))
}

func (h *handler) getOrder(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	o, err := h.orders.Get(r.Context(), id)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	writeData(w, http.StatusOK, toOrderResponse(o))
}

func (h *handler) listUserOrders(w http.ResponseWriter, r *http.Request) {
	userID, err := pathID(r)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	offset, limit, err := pagination(r)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	orders, total, err := h.orders.ListByUser(r.Context(), userID, offset, limit)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	writeData(w, http.StatusOK, pageResponse[orderResponse]{
		Items:  toOrderResponses(orders),
		Total:  total,
		Offset: offset,
		Limit:  limit,
	})
}

func (h *handler) payOrder(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	o, err := h.orders.Pay(r.Context(), id)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	writeData(w, http.StatusOK, toOrderResponse(o))
}

func (h *handler) cancelOrder(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	o, err := h.orders.Cancel(r.Context(), id)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	writeData(w, http.StatusOK, toOrderResponse(o))
}
