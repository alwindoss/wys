package wys

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"

	"github.com/justinas/nosurf"
)

type Config struct {
	FS             embed.FS
	PageLocation   string
	PagePattern    string
	LayoutLocation string
	LayoutPattern  string
	FuncMap        template.FuncMap
}

type TemplateData struct {
	CSRFToken       string
	StringSlice     []string
	StringMap       map[string]string
	IntMap          map[string]int
	FloatMap        map[string]float32
	Data            map[string]interface{}
	Flash           string
	Warning         string
	Error           string
	IsAuthenticated int
	Title           string
	InfoMsg         string
	WarnMsg         string
	ErrMsg          string
	// Form            *forms.Form
}

type ViewManager interface {
	Render(w http.ResponseWriter, r *http.Request, tmpl string, data *TemplateData)
}

type viewManager struct {
	templateCache map[string]*template.Template
	cfg           *Config
}

// Render implements ViewManager.
func (m *viewManager) Render(w http.ResponseWriter, r *http.Request, tmpl string, data *TemplateData) {
	data = m.addDefaultData(r, data)
	t, ok := m.templateCache[tmpl]
	if !ok {
		err := fmt.Errorf("unable to find %s in template cache", tmpl)
		log.Fatal(err)
	}
	buff := new(bytes.Buffer)
	err := t.Execute(buff, data)
	if err != nil {
		log.Fatal(err)
	}
	_, err = buff.WriteTo(w)
	if err != nil {
		log.Printf("error writing template to browser: %v", err)
		return
	}
}

func (m *viewManager) addDefaultData(r *http.Request, td *TemplateData) *TemplateData {
	td.CSRFToken = nosurf.Token(r)
	return td
}

func New(cfg *Config) (ViewManager, error) {
	cache := map[string]*template.Template{}
	// Not using filepath.Join because embed.FS does not work with windows path (ex: "\")
	pagesPath := cfg.PageLocation + "/" + cfg.PagePattern
	layoutsPath := cfg.LayoutLocation + "/" + cfg.LayoutPattern
	pages, err := fs.Glob(cfg.FS, pagesPath)
	if err != nil {
		log.Printf("error glob: %v", err)
		return nil, err
	}
	for _, page := range pages {
		name := filepath.Base(page)
		ts, err := template.New(name).Funcs(cfg.FuncMap).ParseFS(cfg.FS, page)
		if err != nil {
			log.Printf("unable to ParseFiles: %v", err)
			return nil, err
		}
		matches, err := fs.Glob(cfg.FS, layoutsPath)
		if err != nil {
			log.Printf("unable to fs.Glob: %v", err)
			return nil, err
		}
		if len(matches) > 0 {
			ts, err = ts.ParseFS(cfg.FS, layoutsPath)
			if err != nil {
				log.Printf("unable to ParseGlob: %v", err)
				return nil, err
			}
		}
		cache[name] = ts
	}
	return &viewManager{
		templateCache: cache,
		cfg:           cfg,
	}, nil
}

var BasicFunctions = template.FuncMap{
	// The name "inc" is what the function will be called in the template text.
	"inc": func(i int) int {
		return i + 1
	},
	"marshal": func(v interface{}) template.JS {
		a, _ := json.Marshal(v)
		return template.JS(a)
	},
}
