package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// apiFieldMeta is a resource field's shape, sent alongside data so a
// consumer can render/validate without a second call.
type apiFieldMeta struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

type apiPaging struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

type apiListResponse struct {
	Resource string         `json:"resource"`
	Fields   []apiFieldMeta `json:"fields"`
	Data     []Record       `json:"data"`
	Paging   apiPaging      `json:"paging"`
}

type apiItemResponse struct {
	Resource string         `json:"resource"`
	Fields   []apiFieldMeta `json:"fields"`
	Data     Record         `json:"data"`
}

type apiDeleteResponse struct {
	Resource string `json:"resource"`
	Deleted  bool   `json:"deleted"`
	ID       string `json:"id"`
}

type apiErrorResponse struct {
	Error       string            `json:"error"`
	FieldErrors map[string]string `json:"fieldErrors,omitempty"`
}

// registerAPI adds the generic, token-authenticated JSON REST surface:
// every resource's data is reachable at /api/{resource}..., subject to the
// exact same permission checks (via hasPermission) as the htmx UI — just
// authenticated by a bearer token instead of a session cookie.
func (a *App) registerAPI() {
	a.mux.HandleFunc("GET /api/{resource}", a.requireAPIToken(a.requireAPIPermission(ActionView, a.handleAPIList)))
	a.mux.HandleFunc("GET /api/{resource}/{id}", a.requireAPIToken(a.requireAPIPermission(ActionView, a.handleAPIGet)))
	a.mux.HandleFunc("POST /api/{resource}", a.requireAPIToken(a.requireAPIPermission(ActionCreate, a.handleAPICreate)))
	a.mux.HandleFunc("PUT /api/{resource}/{id}", a.requireAPIToken(a.requireAPIPermission(ActionEdit, a.handleAPIUpdate)))
	a.mux.HandleFunc("DELETE /api/{resource}/{id}", a.requireAPIToken(a.requireAPIPermission(ActionDelete, a.handleAPIDelete)))
}

// requireAPIToken resolves the Authorization: Bearer <token> header to a
// User via the rbac store, attaching it to the request context under the
// same key the session middleware uses — so CurrentUser(r) (and therefore
// hasPermission) works identically regardless of which auth path reached
// the handler.
func (a *App) requireAPIToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		raw, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || raw == "" {
			writeAPIError(w, http.StatusUnauthorized, "missing or invalid Authorization header; expected \"Bearer <token>\"")
			return
		}

		user, err := a.rbac.lookupToken(raw)
		if err != nil {
			writeAPIError(w, http.StatusUnauthorized, err.Error())
			return
		}

		ctx := context.WithValue(r.Context(), userCtxKey{}, user)
		next(w, r.WithContext(ctx))
	}
}

// requireAPIPermission is requirePermission's JSON counterpart: same
// hasPermission check, a JSON 403 instead of a rendered forbidden page.
func (a *App) requireAPIPermission(action Action, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("resource")
		if !a.hasPermission(CurrentUser(r), key, action) {
			writeAPIError(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r)
	}
}

func (a *App) apiResourceFromPath(w http.ResponseWriter, r *http.Request) (*Resource, bool) {
	res, ok := a.getResource(r.PathValue("resource"))
	if !ok {
		writeAPIError(w, http.StatusNotFound, "resource not found")
		return nil, false
	}
	return res, true
}

func apiFieldsMeta(fields []Field) []apiFieldMeta {
	out := make([]apiFieldMeta, 0, len(fields))
	for _, f := range fields {
		out = append(out, apiFieldMeta{Name: f.Name, Label: f.Label, Type: string(f.Type), Required: f.Required})
	}
	return out
}

func (a *App) handleAPIList(w http.ResponseWriter, r *http.Request) {
	res, ok := a.apiResourceFromPath(w, r)
	if !ok {
		return
	}

	page := parsePage(r)
	pageSize := res.pageSize()
	if ps, err := strconv.Atoi(r.URL.Query().Get("pageSize")); err == nil && ps > 0 {
		pageSize = ps
	}

	result, err := res.Provider.List(page, pageSize)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = (result.Total + pageSize - 1) / pageSize
	}

	writeAPIJSON(w, http.StatusOK, apiListResponse{
		Resource: res.Key,
		Fields:   apiFieldsMeta(res.AllFields()),
		Data:     result.Records,
		Paging:   apiPaging{Page: page, PageSize: pageSize, Total: result.Total, TotalPages: totalPages},
	})
}

func (a *App) handleAPIGet(w http.ResponseWriter, r *http.Request) {
	res, ok := a.apiResourceFromPath(w, r)
	if !ok {
		return
	}
	rec, err := res.Provider.Get(r.PathValue("id"))
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "record not found")
		return
	}
	writeAPIJSON(w, http.StatusOK, apiItemResponse{Resource: res.Key, Fields: apiFieldsMeta(res.AllFields()), Data: rec})
}

func (a *App) handleAPICreate(w http.ResponseWriter, r *http.Request) {
	res, ok := a.apiResourceFromPath(w, r)
	if !ok {
		return
	}

	data, err := decodeAPIRecord(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errs := validateRecord(res.FormFields(), data); len(errs) > 0 {
		writeAPIErrorFields(w, http.StatusBadRequest, "validation failed", errs)
		return
	}

	created, err := res.Provider.Create(data)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeAPIJSON(w, http.StatusCreated, apiItemResponse{Resource: res.Key, Fields: apiFieldsMeta(res.AllFields()), Data: created})
}

func (a *App) handleAPIUpdate(w http.ResponseWriter, r *http.Request) {
	res, ok := a.apiResourceFromPath(w, r)
	if !ok {
		return
	}

	data, err := decodeAPIRecord(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errs := validateRecord(res.FormFields(), data); len(errs) > 0 {
		writeAPIErrorFields(w, http.StatusBadRequest, "validation failed", errs)
		return
	}

	updated, err := res.Provider.Update(r.PathValue("id"), data)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeAPIJSON(w, http.StatusOK, apiItemResponse{Resource: res.Key, Fields: apiFieldsMeta(res.AllFields()), Data: updated})
}

func (a *App) handleAPIDelete(w http.ResponseWriter, r *http.Request) {
	res, ok := a.apiResourceFromPath(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if err := res.Provider.Delete(id); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeAPIJSON(w, http.StatusOK, apiDeleteResponse{Resource: res.Key, Deleted: true, ID: id})
}

func decodeAPIRecord(r *http.Request) (Record, error) {
	var data Record
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		return nil, err
	}
	if data == nil {
		data = Record{}
	}
	return data, nil
}

func writeAPIError(w http.ResponseWriter, status int, msg string) {
	writeAPIJSON(w, status, apiErrorResponse{Error: msg})
}

func writeAPIErrorFields(w http.ResponseWriter, status int, msg string, fieldErrs map[string]string) {
	writeAPIJSON(w, status, apiErrorResponse{Error: msg, FieldErrors: fieldErrs})
}

func writeAPIJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
