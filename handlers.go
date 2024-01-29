package main

// handlers module holds all HTTP handlers functions
//
// Copyright (c) 2023 - Valentin Kuznetsov <vkuznet@gmail.com>
//

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	server "github.com/CHESSComputing/golib/server"
	services "github.com/CHESSComputing/golib/services"
	"github.com/gin-gonic/gin"
)

// DocParam defines parameters for uri binding
type DocParams struct {
	Name string `uri:"name" binding:"required"`
}

// helper function to check if HTTP request contains form-data
func formData(r *http.Request) bool {
	for key, values := range r.Header {
		if strings.ToLower(key) == "content-type" {
			for _, v := range values {
				if strings.Contains(strings.ToLower(v), "form-data") {
					return true
				}
			}
		}
	}
	return false
}

// PredictHandler handles GET HTTP requests
func PredictHandler(c *gin.Context) {
	r := c.Request

	rec, err := modelRecord(r)
	if err != nil {
		rec := services.Response("MLHub", http.StatusBadRequest, services.ReaderError, err)
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	if Verbose > 0 {
		log.Printf("InferenceHandler found %+v", rec)
	}
	data, err := Predict(rec, r)
	if err == nil {
		c.Data(http.StatusOK, "application/octet-stream", data)
		return
	}
	resp := services.Response("MLHub", http.StatusBadRequest, services.ReaderError, err)
	c.JSON(http.StatusBadRequest, resp)
	return
}

// DownloadHandler handles download action of ML model from back-end server via
// /models/:name?type=TensorFlow&version=123
func DownloadHandler(c *gin.Context) {
	var doc DocParams
	if err := c.ShouldBindUri(&doc); err != nil {
		rec := services.Response("MLHub", http.StatusBadRequest, services.BindError, err)
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	model := doc.Name
	mlType := c.Request.FormValue("type")
	version := c.Request.FormValue("version")
	// check if record exist in MetaData database
	records, err := metaRecords(model, mlType, version)
	if err != nil {
		rec := services.Response("MLHub", http.StatusBadRequest, services.ReaderError, err)
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	if len(records) != 1 {
		msg := fmt.Sprintf("Too many records for provide model=%s type=%s version=%s", model, mlType, version)
		rec := services.Response("MLHub", http.StatusBadRequest, services.ReaderError, errors.New(msg))
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	rec := records[0]
	// get bundle link
	val, ok := rec.MetaData["bundle"]
	if !ok {
		msg := fmt.Sprintf("No bundle file found for model=%s type=%s version=%s", model, mlType, version)
		rec := services.Response("MLHub", http.StatusBadRequest, services.NotFoundError, errors.New(msg))
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	fileName := val.(string)
	fname := findModelFile(fileName, mlType, version)
	// form link to download the model bundle
	bname := strings.Replace(fname, StorageDir, "", -1)
	downloadURL := fmt.Sprintf("/bundles%s", bname)
	if Verbose > 0 {
		log.Println("download", downloadURL)
	}

	//     http.Redirect(c.Writer, c.Request, downloadURL, http.StatusSeeOther)
	c.Redirect(http.StatusSeeOther, downloadURL)
}

// UploadHandler handles upload action of ML model to back-end server
func UploadHandler(c *gin.Context) {
	r := c.Request

	// check if we provided with proper form data
	if !formData(r) {
		rec := services.Response("MLHub", http.StatusBadRequest, services.ReaderError, errors.New("unable to get form data"))
		c.JSON(http.StatusBadRequest, rec)
		return
	}

	// handle upload POST requests
	var rec Record
	model := r.FormValue("model")
	mlType := r.FormValue("type")
	backend := r.FormValue("backend")
	bundle := r.FormValue("file")
	version := r.FormValue("version")
	reference := r.FormValue("reference")
	discipline := r.FormValue("discipline")
	description := r.FormValue("description")

	if Verbose > 0 {
		log.Printf("### model=%v type=%v bundle=%v version=%v ref=%v dis=%v desc=%v", model, mlType, bundle, version, reference, discipline, description)
	}

	// get file name bundle
	if bundle == "" {
		// parse incoming HTTP request multipart form
		err := r.ParseMultipartForm(32 << 20) // maxMemory
		if err != nil {
			rec := services.Response("MLHub", http.StatusBadRequest, services.ReaderError, err)
			c.JSON(http.StatusBadRequest, rec)
			return
		}
		for _, vals := range r.MultipartForm.File {
			for _, fh := range vals {
				bundle = fh.Filename
			}
		}
	}

	// we got HTML form request
	rec = Record{
		Model:       model,
		Type:        mlType,
		Backend:     backend,
		Version:     version,
		Description: description,
		Discipline:  discipline,
		Reference:   reference,
		Bundle:      bundle,
	}
	// assign oauth attributes to the record
	rec.UserName = "TODO-getuser from context"
	rec.UserID = "TODO-getuserid"
	rec.Provider = "TODO-get provider if necessary"

	// perform upload action
	err := Upload(rec, r)
	if err != nil {
		rec := services.Response("MLHub", http.StatusBadRequest, services.ReaderError, err)
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	c.JSON(http.StatusOK, services.Response("MLHub", http.StatusOK, 0, nil))
}

// GetHandler handles GET HTTP requests, this request will
// delete ML model in backend and MetaData database
func DeleteHandler(c *gin.Context) {
	model := c.Request.FormValue("model")
	var ok bool
	if ok {
		if Verbose > 0 {
			log.Printf("delete ML model %s", model)
		}
		// delete ML model in MetaData database
		err := metaRemove(model)
		if err != nil {
			rec := services.Response("MLHub", http.StatusInternalServerError, services.ReaderError, err)
			c.JSON(http.StatusInternalServerError, rec)
			return
		}
		c.JSON(http.StatusOK, services.Response("MLHub", http.StatusOK, 0, nil))
		return
	}
	rec := services.Response("MLHub", http.StatusBadRequest, services.ReaderError, errors.New("no model name is provided"))
	c.JSON(http.StatusBadRequest, rec)
}

// ModelsHandler provides information about registered ML models
func ModelsHandler(c *gin.Context) {
	// TODO: Add parameters for /models endpoint, eg q=query, limit, idx for pagination
	var records []map[string]any
	mRecords, err := metaRecords("", "", "")
	if err != nil {
		msg := fmt.Sprintf("unable to get meta-data, error=%v", err)
		rec := services.Response("MLHub", http.StatusInternalServerError, services.ReaderError, errors.New(msg))
		c.JSON(http.StatusInternalServerError, rec)
		return
	}
	for _, r := range mRecords {
		records = append(records, r.MetaData)
	}
	c.JSON(http.StatusOK, records)
}

// DocsHandler handles status of MLHub server
func DocsHandler(c *gin.Context) {
	var doc DocParams
	if err := c.ShouldBindUri(&doc); err != nil {
		rec := services.Response("MLHub", http.StatusBadRequest, services.BindError, err)
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	fname := fmt.Sprintf("%s/md/%s.md", StaticDir, doc.Name)
	content, err := server.MDToHTML(StaticFs, fname)
	if err != nil {
		rec := services.Response("MLHub", http.StatusInternalServerError, services.ReaderError, err)
		c.JSON(http.StatusInternalServerError, rec)
		return
	}
	tmpl := make(map[string]any)
	header := server.TmplPage(StaticFs, "header.tmpl", tmpl)
	footer := server.TmplPage(StaticFs, "footer.tmpl", tmpl)
	c.Data(http.StatusOK, "application/html", []byte(header+content+footer))
}
