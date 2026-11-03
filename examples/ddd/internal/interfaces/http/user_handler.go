package httpapi

import "net/http"

func (h *handler) createUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, h.log, err)
		return
	}
	u, err := h.users.Create(r.Context(), req.Name, req.Email)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	writeData(w, http.StatusCreated, toUserResponse(u))
}

func (h *handler) getUser(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	u, err := h.users.Get(r.Context(), id)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	writeData(w, http.StatusOK, toUserResponse(u))
}

func (h *handler) listUsers(w http.ResponseWriter, r *http.Request) {
	offset, limit, err := pagination(r)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	users, total, err := h.users.List(r.Context(), offset, limit)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	writeData(w, http.StatusOK, pageResponse[userResponse]{
		Items:  toUserResponses(users),
		Total:  total,
		Offset: offset,
		Limit:  limit,
	})
}
