package main

import (
	"database/sql"
	"embed"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"nastymatt/tierlister/db"
	"nastymatt/tierlister/db/sqlc"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed db/schema.sql
var schema string

//go:embed web/assets/*
var staticAssets embed.FS

var templates map[string]*template.Template
var baseTemplates = []string{
	"web/templates/base.gohtml",
	"web/templates/partials/nav.gohtml",
	"web/templates/partials/tierlist_edit_fieldset.gohtml",
	"web/templates/partials/image.gohtml",
	"web/templates/partials/image_upload.gohtml",
	"web/templates/partials/pagination.gohtml",
}
var pages = []string{"index", "tierlist_create", "tierlist_edit", "images"}
var funcMap = template.FuncMap{
	"dict": func(pairs ...any) (map[string]any, error) {
		if len(pairs)%2 != 0 {
			return nil, fmt.Errorf("dict: odd number of args")
		}
		m := make(map[string]any, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			key, ok := pairs[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict: keys must be strings")
			}
			m[key] = pairs[i+1]
		}
		return m, nil
	},
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"pageRange": func(from, to int) []int {
		pages := make([]int, to-from+1)
		for i := range pages {
			pages[i] = from + i
		}
		return pages
	},
}

func initTemplates() {
	templates = make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		files := append(baseTemplates, "web/templates/"+page+".gohtml")
		templates[page] = template.Must(
			template.New("").Funcs(funcMap).ParseFiles(files...),
		)
	}
}

type TierlistEditForm struct {
	Title    string
	ErrTitle string
}

type TierlistEditPage struct {
	TierlistEditForm
	Tierlist sqlc.Tierlist
	Tiers    []db.TierWithImages
	Images   db.Page[db.ImageView]
}

func (f *TierlistEditForm) Validate() bool {
	if strings.TrimSpace(f.Title) == "" {
		f.ErrTitle = "Title is required"
		return false
	}
	return true
}

func tmpl(w http.ResponseWriter, page string, data any) {
	tem, ok := templates[page]
	if !ok {
		slog.Error("unknown page requested", "page", page)
		http.Error(w, "unknowm page", http.StatusInternalServerError)
		return
	}
	if err := tem.ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("template render error", "err", err)
	}
}

func main() {
	initTemplates()

	dbc, err := sql.Open("sqlite", "data/app.db")
	if err != nil {
		log.Fatal("unable to open database:", err)
	}
	defer dbc.Close()
	if _, err := dbc.Exec(schema); err != nil {
		log.Fatal("unable to apply schema:", err)
	}

	queries := sqlc.New(dbc)

	mux := http.NewServeMux()
	staticSub, _ := fs.Sub(staticAssets, "web/assets")

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		tierlists, err := queries.ListTierlists(r.Context())
		if err != nil {
			slog.ErrorContext(r.Context(), "unable to load tierlits", "err", err)
			http.Error(w, "unable to load tierlists", http.StatusInternalServerError)
			return
		}
		tmpl(w, "index", map[string]any{"Tierlists": tierlists})
	})

	mux.HandleFunc("GET /tierlists/create", func(w http.ResponseWriter, r *http.Request) {
		tmpl(w, "tierlist_create", TierlistEditForm{})
	})

	mux.HandleFunc("POST /tierlists/create", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		form := TierlistEditForm{
			Title: r.FormValue("title"),
		}
		if !form.Validate() {
			w.WriteHeader(http.StatusUnprocessableEntity)
			tmpl(w, "tierlist_create", form)
			return
		}
		tierlist, err := queries.CreateTierlist(r.Context(), form.Title)
		if err != nil {
			slog.ErrorContext(r.Context(), "unable to create tierlist", "err", err)
			http.Error(w, "unable to create tierlist", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/tierlists/%d", tierlist.ID), http.StatusSeeOther)
	})

	mux.HandleFunc("GET /tierlists/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "tierlist id must be int", http.StatusBadRequest)
			return
		}

		tierlist, err := queries.GetTierlist(r.Context(), id)
		if err != nil {
			slog.InfoContext(r.Context(), "unable to find tierlist", "err", err)
			http.Error(w, "tierlist not found", http.StatusNotFound)
			return
		}

		tiers, err := db.GetTiersWithImages(r.Context(), *queries, id)
		if err != nil {
			slog.InfoContext(r.Context(), "unable to get tiers", "err", err)
			http.Error(w, "unable to get tiers", http.StatusInternalServerError)
			return
		}

		page, perPage, offset := db.PaginationParams(r)
		images, total, err := db.GetAvailableTierlistImages(r.Context(), *queries, id, perPage, offset)
		tmpl(w, "tierlist_edit", TierlistEditPage{
			TierlistEditForm: TierlistEditForm{Title: tierlist.Title},
			Tierlist:         tierlist,
			Tiers:            tiers,
			Images:           db.Paginate(images, page, perPage, total),
		})
	})

	mux.HandleFunc("PUT /tierlists/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "tierlist id must be int", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		form := TierlistEditForm{
			Title: r.FormValue("title"),
		}
		if !form.Validate() {
			http.Error(w, "Title is required", http.StatusUnprocessableEntity)
			return
		}
		if err := queries.UpdateTierlist(r.Context(), sqlc.UpdateTierlistParams{ID: id, Title: form.Title}); err != nil {
			slog.ErrorContext(r.Context(), "unable to update tierlist", "id", id, "err", err)
			http.Error(w, "unable to update tierlist", http.StatusInternalServerError)
			return
		}
		w.Header().Add("HX-Refresh", "true")
	})

	mux.HandleFunc("DELETE /tierlists/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "tierlist id must be int", http.StatusBadRequest)
			return
		}
		if err := queries.DeleteTierlist(r.Context(), id); err != nil {
			slog.ErrorContext(r.Context(), "unable to delete tierlist", "id", id, "err", err)
			http.Error(w, "unable to delete tierlist", http.StatusInternalServerError)
			return
		}
		if r.Header.Get("HX-Request") != "" {
			w.Header().Add("HX-Redirect", "/")
		}
	})

	mux.HandleFunc("PUT /tierlists/{id}/images/{imageID}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "tierlist id must be int", http.StatusBadRequest)
			return
		}
		imageID, err := strconv.ParseInt(r.PathValue("imageID"), 10, 64)
		if err != nil {
			http.Error(w, "image id must be int", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		tierID, err := strconv.ParseInt(r.FormValue("tier_id"), 10, 64)
		if err != nil {
			http.Error(w, "tier_id must be int", http.StatusBadRequest)
			return
		}
		position, err := strconv.Atoi(r.FormValue("position"))
		if err != nil {
			http.Error(w, "position must be int", http.StatusBadRequest)
			return
		}
		if err := db.MoveImage(r.Context(), *queries, id, tierID, imageID, position); err != nil {
			slog.ErrorContext(r.Context(), "unable to move image", "tierlistID", id, "imageID", imageID, "err", err)
			http.Error(w, "unable to move image", http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("DELETE /tierlists/{id}/images/{imageID}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "tierlist id must be int", http.StatusBadRequest)
			return
		}
		imageID, err := strconv.ParseInt(r.PathValue("imageID"), 10, 64)
		if err != nil {
			http.Error(w, "image id must be int", http.StatusBadRequest)
			return
		}
		if err := queries.DeleteTierImage(r.Context(), sqlc.DeleteTierImageParams{
			TierlistID: id,
			ImageID:    imageID,
		}); err != nil {
			slog.ErrorContext(r.Context(), "unable to remove image from tier", "tierlistID", id, "imageID", imageID, "err", err)
			http.Error(w, "unable to remove image", http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("GET /images", func(w http.ResponseWriter, r *http.Request) {
		page, perPage, offset := db.PaginationParams(r)
		images, total, err := db.GetImages(r.Context(), *queries, perPage, offset)
		if err != nil {
			slog.Error("unable to load images", "err", err)
			http.Error(w, "unable to load images", http.StatusInternalServerError)
			return
		}
		tmpl(w, "images", db.Paginate(images, page, perPage, total))
	})

	mux.HandleFunc("POST /images", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(5 << 20); err != nil { // 10MB
			http.Error(w, "file is too large", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("image")
		if err != nil {
			http.Error(w, "missing image", http.StatusBadRequest)
			return
		}
		defer file.Close()

		contentType := header.Header.Get("Content-Type")
		if contentType != "image/jpeg" && contentType != "image/png" {
			http.Error(w, "only jpeg/png allowed", http.StatusBadRequest)
			return
		}

		destPath := filepath.Join("data/images", header.Filename)

		dst, err := os.Create(destPath)
		if err != nil {
			slog.ErrorContext(r.Context(), "unable to create file", "err", err)
			http.Error(w, "unable to save image", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			slog.ErrorContext(r.Context(), "unable to write file", "err", err)
			http.Error(w, "unable to save image", http.StatusInternalServerError)
			return
		}

		image, err := queries.CreateImage(r.Context(), sqlc.CreateImageParams{
			ImagePath:        destPath,
			OriginalFilename: header.Filename,
		})
		if err != nil {
			slog.ErrorContext(r.Context(), "unable to save image in db", "err", err)
			http.Error(w, "unable to save image", http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusCreated)
		if r.Header.Get("HX-Request") != "" {
			templates["images"].ExecuteTemplate(w, "image", map[string]any{
				"Image":     db.ImageToImageView(image),
				"Draggable": true,
			})
		}
	})

	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(staticSub))))
	mux.Handle("GET /uploads/images/", http.StripPrefix("/uploads/images/", http.FileServer(http.Dir("data/images"))))

	log.Fatal(http.ListenAndServe(":8080", mux))
}
