package web

import (
	"io"
	"net/http"
	"waldi/internal/importblogir"
)

// maxBlogirUploadBytes caps the uploaded export file; blog.ir archives are
// plain XML text, so even a very large blog stays well under this.
const maxBlogirUploadBytes = 50 << 20

// handleImportBlogirForm serves the hidden /settings/import-blogir page. It
// is not linked from any nav — the URL is meant to be sent directly to a
// writer migrating from blog.ir so they can restore their own backup.
func (s *Server) handleImportBlogirForm(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	pd := s.newPageData(r, user)
	pd.Title = pd.T("import_blogir.title")
	pd.SEO = noindexSEO()
	pd.ImportBlogir = &ImportBlogirView{}
	s.renderer.Render(w, "import_blogir.html", pd)
}

func (s *Server) handleImportBlogir(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	view := &ImportBlogirView{}
	render := func(status int) {
		pd := s.newPageData(r, user)
		pd.Title = pd.T("import_blogir.title")
		pd.SEO = noindexSEO()
		pd.ImportBlogir = view
		s.renderer.RenderStatus(w, status, "import_blogir.html", pd)
	}

	if s.store == nil {
		view.Error = "import_blogir.error.unavailable"
		render(http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBlogirUploadBytes)
	if err := r.ParseMultipartForm(maxBlogirUploadBytes); err != nil {
		view.Error = "import_blogir.error.file_too_large"
		render(http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("export")
	if err != nil {
		view.Error = "import_blogir.error.file_missing"
		render(http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	raw, err := io.ReadAll(file)
	if err != nil {
		view.Error = "import_blogir.error.file_too_large"
		render(http.StatusBadRequest)
		return
	}

	exp, err := importblogir.ParseExport(raw)
	if err != nil {
		view.Error = "import_blogir.error.parse"
		render(http.StatusBadRequest)
		return
	}

	imp := importblogir.Importer{
		Store: s.store,
		User:  *user,
	}
	result, err := imp.Run(r.Context(), exp.Posts)
	if err != nil {
		s.logger.Error("importing blogir export", "err", err)
		view.Error = "import_blogir.error.failed"
		render(http.StatusInternalServerError)
		return
	}

	view.Done = true
	view.Imported = result.Imported
	view.Skipped = result.Skipped
	for _, f := range result.Failed {
		view.Failed = append(view.Failed, ImportBlogirFailureView{
			Title: f.Title,
			Slug:  f.Slug,
			Err:   f.Err.Error(),
		})
	}
	render(http.StatusOK)
}
