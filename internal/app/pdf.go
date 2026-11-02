package app

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"
	"time"

	"designhub/internal/model"
)

// PDF export (DATA-MODEL.md 7.28, SCREENS.md 25). The browser sends the
// record as it is shown on screen, with styles and images inlined. The
// server adds the header and footer from the record itself and renders the
// PDF with headless Chromium in the pdf container, which has no network
// access beyond DesignHub.

var scriptRe = regexp.MustCompile(`(?is)<script\b.*?</script\s*>|<script\b[^>]*/?>|\son[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)

type pdfInput struct {
	ID    string `json:"id"`
	HTML  string `json:"html"`
	Paper string `json:"paper"` // letter or a4
	Scope string `json:"scope"` // what was exported, for the audit log
	Pages int    `json:"pages"` // pages in a page-tree export
}

// pdfHeader returns the header facts of a record.
func (a *App) pdfHeader(ctx context.Context, u *User, id string) (title, rev, status string, notReleased bool, err error) {
	if e := cache.get(id); e != nil {
		it := e.Item
		if u.IsVendor() {
			it = vendorItem(it)
		}
		title, rev, status = it.Title, it.Revision, it.Status
		if it.Type == model.TypePage {
			notReleased = a.isControlled(ctx, it) && it.Status != model.StatusReleased
			if !a.isControlled(ctx, it) {
				status = ""
			}
		} else {
			notReleased = it.Status != model.StatusReleased
		}
		return
	}
	ri, e := a.recordInfo(ctx, id)
	if e != nil {
		return "", "", "", false, errNotFound
	}
	return ri.Title, "", ri.Status, false, nil
}

func (a *App) apiPDF(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	var in pdfInput
	r.Body = http.MaxBytesReader(w, r.Body, 60<<20)
	if err := decode(r, &in); err != nil {
		return err
	}
	if !a.canSee(ctx, u, in.ID) {
		return errNotFound
	}
	title, rev, status, notReleased, err := a.pdfHeader(ctx, u, in.ID)
	if err != nil {
		return err
	}
	body := scriptRe.ReplaceAllString(in.HTML, "")
	if strings.TrimSpace(body) == "" {
		return badRequest("Nothing to export.")
	}
	stamp := ""
	if notReleased {
		stamp = `<span class="nr">Not released</span>`
	}
	idLine := in.ID
	if rev != "" {
		idLine += " rev " + rev
	}
	if status != "" {
		idLine += " &middot; " + html.EscapeString(status)
	}
	head := `<html><head><style>body{font:9px "Segoe UI",Arial,sans-serif;color:#1d232c;margin:0 12mm;width:100%;display:flex;justify-content:space-between;}
		.nr{color:#b3261e;font-weight:700;margin-left:8px}b{font-weight:600}</style></head><body><span><b>DesignHub</b> &nbsp; ` +
		html.EscapeString(idLine) + ` &nbsp; ` + html.EscapeString(truncate(title, 90)) + `</span><span>` + stamp + `</span></body></html>`
	foot := `<html><head><style>body{font:8.5px "Segoe UI",Arial,sans-serif;color:#5d6775;margin:0 12mm;width:100%;display:flex;justify-content:space-between;}
		.nr{color:#b3261e;font-weight:700}</style></head><body><span>Exported by ` + html.EscapeString(u.DisplayName) + ` on ` +
		time.Now().Format("2006-01-02 15:04 MST") + `</span>` + stamp +
		`<span>Page <span class="pageNumber"></span> of <span class="totalPages"></span></span></body></html>`
	pdf, err := a.renderPDF(ctx, body, head, foot, in.Paper)
	if err != nil {
		return err
	}
	a.Audit(ctx, u, clientIP(r), "export.pdf", in.ID, map[string]any{"scope": truncate(in.Scope, 60), "pages": in.Pages, "bytes": len(pdf)})
	name := in.ID + ".pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	w.Header().Set("Cache-Control", "no-store")
	_, err = w.Write(pdf)
	return err
}

// renderPDF converts HTML to PDF with the pdf service.
func (a *App) renderPDF(ctx context.Context, body, header, footer, paper string) ([]byte, error) {
	if a.Cfg.PDFURL == "" {
		return nil, &Error{Status: http.StatusServiceUnavailable, Code: "pdf_off", Message: "PDF export is not set up on this server."}
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	file := func(name, content string) {
		fw, _ := mw.CreateFormFile("files", name)
		fw.Write([]byte(content))
	}
	file("index.html", body)
	file("header.html", header)
	file("footer.html", footer)
	width, height := "8.5", "11"
	if paper == "a4" {
		width, height = "8.27", "11.7"
	}
	for k, v := range map[string]string{"paperWidth": width, "paperHeight": height, "marginTop": "0.75", "marginBottom": "0.7",
		"marginLeft": "0.6", "marginRight": "0.6", "printBackground": "true", "preferCssPageSize": "false"} {
		mw.WriteField(k, v)
	}
	mw.Close()
	cctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodPost, strings.TrimRight(a.Cfg.PDFURL, "/")+"/forms/chromium/convert/html", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, &Error{Status: http.StatusServiceUnavailable, Code: "pdf_unavailable", Message: "The PDF service is not responding. Check the pdf container."}
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, 200<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &Error{Status: http.StatusBadGateway, Code: "pdf_failed", Message: fmt.Sprintf("The PDF could not be created (%s).", truncate(strings.TrimSpace(string(out)), 200))}
	}
	return out, nil
}

// pdfStatus reports whether the pdf service answers, for Admin, System.
func (a *App) pdfStatus(ctx context.Context) string {
	if a.Cfg.PDFURL == "" {
		return "Not set up"
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(cctx, http.MethodGet, strings.TrimRight(a.Cfg.PDFURL, "/")+"/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "Not responding"
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return "Connected"
	}
	return fmt.Sprintf("Error %d", resp.StatusCode)
}
