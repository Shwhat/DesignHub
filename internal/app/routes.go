package app

import "net/http"

func (a *App) routes(mux *http.ServeMux) {
	pub := func(pattern string, fn handlerFunc) { mux.HandleFunc(pattern, a.handle(fn)) }
	api := func(pattern string, fn handlerFunc) { mux.HandleFunc(pattern, a.handle(a.withUser(fn))) }

	// Sign-in and session
	pub("GET /api/signin", a.apiSignInInfo)
	pub("POST /api/signin", a.apiSignIn)
	pub("GET /api/auth/google/start", a.apiGoogleStart)
	pub("GET /api/auth/google/callback", a.apiGoogleCallback)
	api("POST /api/signout", a.apiSignOut)
	api("GET /api/me", a.apiMe)
	api("PATCH /api/me", a.apiUpdateMe)
	api("POST /api/me/password", a.apiChangePassword)
	api("PUT /api/me/signing-pin", a.apiSetSigningPin)
	api("GET /api/me/sessions", a.apiSessions)
	api("POST /api/me/sessions/signout-others", a.apiSignOutOthers)
	api("GET /api/prefs/{key}", a.apiGetPref)
	api("PUT /api/prefs/{key}", a.apiPutPref)
	api("GET /api/events", a.apiEvents)

	// Administration
	api("GET /api/users", a.apiUsers)
	api("POST /api/users", a.apiCreateUser)
	api("GET /api/users/{id}", a.apiUser)
	api("PATCH /api/users/{id}", a.apiUpdateUser)
	api("GET /api/roles", a.apiRoles)
	api("PATCH /api/roles/{id}", a.apiUpdateRole)
	api("GET /api/admin/system", a.apiGetSystemSettings)
	api("PUT /api/admin/system", a.apiPutSystemSettings)
	api("GET /api/admin/google", a.apiGetGoogle)
	api("PUT /api/admin/google", a.apiPutGoogle)
	api("GET /api/admin/status", a.apiSystemStatus)
	api("GET /api/admin/audit", a.apiAuditLog)
	api("POST /api/admin/reindex", a.apiReindex)
	api("POST /api/admin/backup", a.apiBackupNow)
	api("GET /api/lists", a.apiLists)
	api("PUT /api/lists/{list}", a.apiPutList)
	api("GET /api/manufacturers", a.apiManufacturers)
	api("POST /api/manufacturers", a.apiCreateManufacturer)
	api("PATCH /api/manufacturers/{code}", a.apiUpdateManufacturer)

	a.routesContent(api)
}
