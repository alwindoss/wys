package wys

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
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
	pagesPath      string
	layoutsPath    string
	InProduction   bool
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
	Render(w http.ResponseWriter, r *http.Request, tmpl string, data *TemplateData) error
}

type viewManager struct {
	templateCache map[string]*template.Template
	cfg           *Config
}

// Render implements ViewManager.
func (m *viewManager) Render(w http.ResponseWriter, r *http.Request, tmpl string, data *TemplateData) error {
	data = m.addDefaultData(r, data)
	var err error
	// Only in case of development usecase the library will fail when something
	// goes wrong with the parsing and caching of templates
	if !m.cfg.InProduction {
		m.templateCache, err = cacheTemplatesForDevelopment(m.cfg)
		if err != nil {
			err = fmt.Errorf("unable to cache templates for development: %w", err)
			return err
		}
	}
	t, ok := m.templateCache[tmpl]
	if !ok {
		err = fmt.Errorf("unable to find %s in template cache", tmpl)
		return err
	}
	buff := new(bytes.Buffer)
	err = t.Execute(buff, data)
	if err != nil {
		err = fmt.Errorf("error when executing template: %w", err)
		return err
	}
	_, err = buff.WriteTo(w)
	if err != nil {
		err = fmt.Errorf("error writing template to browser: %w", err)
		return err
	}
	return nil
}

func (m *viewManager) addDefaultData(r *http.Request, td *TemplateData) *TemplateData {
	td.CSRFToken = nosurf.Token(r)
	return td
}

func New(cfg *Config) (ViewManager, error) {
	// Not using filepath.Join because embed.FS does not work with windows path (ex: "\")
	pagesPath := cfg.PageLocation + "/" + cfg.PagePattern
	layoutsPath := cfg.LayoutLocation + "/" + cfg.LayoutPattern

	cfg.layoutsPath = layoutsPath
	cfg.pagesPath = pagesPath
	var cache map[string]*template.Template
	var err error
	if cfg.InProduction {
		cache, err = cacheTemplatesForProduction(cfg)
		if err != nil {
			return nil, err
		}
	} else {
		cache, err = cacheTemplatesForDevelopment(cfg)
		if err != nil {
			return nil, err
		}
	}
	return &viewManager{
		templateCache: cache,
		cfg:           cfg,
	}, nil
}

func cacheTemplatesForProduction(cfg *Config) (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}
	pages, err := fs.Glob(cfg.FS, cfg.pagesPath)
	if err != nil {
		err = fmt.Errorf("error glob: %w", err)
		return nil, err
	}
	for _, page := range pages {
		name := filepath.Base(page)
		ts, err := template.New(name).Funcs(cfg.FuncMap).ParseFS(cfg.FS, page)
		if err != nil {
			err = fmt.Errorf("unable to ParseFiles: %w", err)
			return nil, err
		}
		matches, err := fs.Glob(cfg.FS, cfg.layoutsPath)
		if err != nil {
			err = fmt.Errorf("unable to fs.Glob: %w", err)
			return nil, err
		}
		if len(matches) > 0 {
			ts, err = ts.ParseFS(cfg.FS, cfg.layoutsPath)
			if err != nil {
				err = fmt.Errorf("unable to ParseGlob: %w", err)
				return nil, err
			}
		}
		cache[name] = ts
	}
	return cache, nil
}

func cacheTemplatesForDevelopment(cfg *Config) (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}
	// pages, err := fs.Glob(filepath., cfg.pagesPath)
	pages, err := filepath.Glob(cfg.pagesPath)
	if err != nil {
		err = fmt.Errorf("error glob: %w", err)
		return nil, err
	}
	for _, page := range pages {
		name := filepath.Base(page)
		ts, err := template.New(name).Funcs(cfg.FuncMap).ParseFS(cfg.FS, page)
		if err != nil {
			err = fmt.Errorf("unable to ParseFiles: %w", err)
			return nil, err
		}
		matches, err := filepath.Glob(cfg.layoutsPath)
		// matches, err := fs.Glob(cfg.FS, cfg.layoutsPath)
		if err != nil {
			err = fmt.Errorf("unable to fs.Glob: %w", err)
			return nil, err
		}
		if len(matches) > 0 {
			ts, err = ts.ParseGlob(cfg.layoutsPath)
			if err != nil {
				err = fmt.Errorf("unable to ParseGlob: %w", err)
				return nil, err
			}
		}
		cache[name] = ts
	}
	return cache, nil
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
