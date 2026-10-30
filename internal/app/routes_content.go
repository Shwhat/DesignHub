package app

import (
	"net/http"

	"designhub/internal/model"
)

func (a *App) routesContent(api func(string, handlerFunc)) {
	// Home, search and notifications
	api("GET /api/home", a.apiHome)
	api("GET /api/search", a.apiSearch)
	api("GET /api/resolve", a.apiResolve)
	api("GET /api/filters", a.apiFilters)
	api("POST /api/filters", a.apiSaveFilter)
	api("DELETE /api/filters/{id}", a.apiDeleteFilter)
	api("GET /api/notifications", a.apiNotifications)
	api("GET /api/notifications/count", a.apiNotificationCount)
	api("POST /api/notifications/mark", a.apiMarkNotifications)

	// Spaces and pages
	api("GET /api/spaces", a.apiSpaces)
	api("POST /api/spaces", a.apiCreateSpace)
	api("GET /api/spaces/{id}", a.apiSpace)
	api("PATCH /api/spaces/{id}", a.apiUpdateSpace)
	api("GET /api/spaces/{id}/members", a.apiSpaceMembers)
	api("PUT /api/spaces/{id}/members", a.apiPutSpaceMember)
	api("GET /api/spaces/{id}/tree", a.apiPageTree)
	api("GET /api/spaces/{id}/work", a.apiSpaceWork)
	api("POST /api/pages", a.apiCreatePage)
	api("GET /api/pages/{id}", a.apiPage)
	api("PUT /api/pages/{id}", a.apiSavePage)
	api("DELETE /api/pages/{id}", a.apiDeletePage)
	api("POST /api/pages/{id}/move", a.apiMovePage)
	api("POST /api/pages/{id}/restore", a.apiRestorePage)
	api("POST /api/pages/{id}/edit-under-co", a.apiPageEditUnderCO)

	// Engineering library
	api("GET /api/library", a.apiLibrary)
	api("GET /api/library/validation", a.apiLibraryValidation)
	api("GET /api/items/new", a.apiNewItemOffer)
	api("POST /api/items", a.apiCreateItem)
	api("GET /api/items/{id}", a.apiItem)
	api("PUT /api/items/{id}", a.apiSaveItem)
	api("DELETE /api/items/{id}", a.apiDeleteItem)
	api("POST /api/items/{id}/revision", a.apiStartRevision)
	api("POST /api/items/{id}/convert", a.apiConvertReference)
	api("GET /api/items/{id}/bom", a.apiBOM)
	api("GET /api/items/{id}/where-used", a.apiWhereUsed)
	api("GET /api/items/{id}/validation", a.apiItemValidation)
	api("GET /api/traceability", a.apiTraceability)

	// Any record
	api("GET /api/records/{id}/history", a.apiHistory)
	api("GET /api/records/{id}/diff", a.apiDiff)
	api("GET /api/records/{id}/versions/{blob}", a.apiVersion)
	api("POST /api/records/{id}/restore", a.apiRestorePage)
	api("GET /api/records/{id}/backlinks", a.apiBacklinks)
	api("POST /api/records/{id}/watch", a.apiWatch)
	api("PUT /api/records/{id}/lock", a.apiLock)
	api("GET /api/records/{id}/comments", a.apiComments)
	api("POST /api/records/{id}/comments", a.apiAddComment)
	api("PATCH /api/records/{id}/comments/{cid}", a.apiUpdateComment)
	api("GET /api/records/{id}/files/{sha}", a.apiDownload)
	api("GET /api/records/{id}/draft", a.apiGetDraft)
	api("PUT /api/records/{id}/draft", a.apiPutDraft)
	api("DELETE /api/records/{id}/draft", a.apiDeleteDraft)
	api("POST /api/files", a.apiUpload)

	// Change control
	api("GET /api/change-orders", a.apiChangeOrders)
	api("POST /api/change-orders", a.apiCreateCO)
	api("GET /api/change-orders/{id}", a.apiCO)
	api("PATCH /api/change-orders/{id}", a.apiUpdateCO)
	api("POST /api/change-orders/{id}/items", a.apiAddCOItem)
	api("DELETE /api/change-orders/{id}/items/{item}", a.apiRemoveCOItem)
	api("PUT /api/change-orders/{id}/approvers", a.apiSetApprovers)
	api("GET /api/change-orders/{id}/changes", a.apiCOChanges)
	api("GET /api/change-orders/{id}/validation", a.apiCOValidation)
	api("POST /api/change-orders/{id}/submit", a.apiSubmitCO)
	api("POST /api/change-orders/{id}/withdraw", a.apiWithdrawCO)
	api("POST /api/change-orders/{id}/decision", a.apiDecideCO)
	api("POST /api/change-orders/{id}/release", a.apiReleaseCO)
	api("POST /api/change-orders/{id}/cancel", a.apiCancelCO)
	api("GET /api/change-orders/{id}/history", a.apiCOHistory)
	api("GET /api/baselines", a.apiBaselines)
	api("GET /api/baselines/{co}", a.apiBaseline)
	api("GET /api/admin/signing-key", a.apiSigningKey)

	// Work
	api("GET /api/my-work", a.apiMyWork)
	api("GET /api/work", a.apiWorkQuery)
	api("POST /api/work", a.apiCreateWork)
	api("GET /api/work/{id}", a.apiWork)
	api("PATCH /api/work/{id}", a.apiUpdateWork)
	api("POST /api/work/{id}/status", a.apiWorkStatus)
	api("POST /api/work/{id}/links", a.apiWorkLinks)
	api("POST /api/work/{id}/sign", a.apiSignNCR)
	api("POST /api/work/{id}/create-co", a.apiNCRCreateCO)
}

// apiPageEditUnderCO puts a released controlled page on a change order so it
// can be edited: an existing Draft change order, or a new one.
func (a *App) apiPageEditUnderCO(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || e.Item.Type != model.TypePage || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	if co := a.openCOFor(ctx, a.DB, id); co != nil {
		return ok(w, map[string]string{"co": co.ID})
	}
	if !a.pagePerms(ctx, u, e, nil).AddToCO {
		return errForbidden
	}
	var in struct {
		CO    string `json:"co"`
		Title string `json:"title"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	coID := in.CO
	if coID == "" {
		if !u.Can(PermCOCreate) {
			return &Error{Status: http.StatusForbidden, Code: "forbidden", Message: "Your library role cannot open change orders. Ask an Editor to open one."}
		}
		title := in.Title
		if title == "" {
			title = truncate("Update "+e.Item.Title, 100)
		}
		ct := "Documentation only"
		var err error
		coID, err = a.CreateCO(ctx, u, clientIP(r), coInput{Title: &title, ChangeType: &ct, HomeSpace: &e.Item.Space})
		if err != nil {
			return err
		}
	} else {
		c, err := a.coForUser(ctx, u, coID)
		if err != nil {
			return err
		}
		if !a.coEditable(u, c) {
			return badRequest("%s is not in Draft.", coID)
		}
	}
	action := "Revise"
	if e.Item.Status == model.StatusDraft {
		action = "Release"
	}
	if err := a.AddCOItem(ctx, u, clientIP(r), coID, id, action); err != nil {
		return err
	}
	return ok(w, map[string]string{"co": coID})
}
