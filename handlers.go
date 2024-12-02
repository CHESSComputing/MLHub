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

	authz "github.com/CHESSComputing/golib/authz"
	srvConfig "github.com/CHESSComputing/golib/config"
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

	var spec Record
	// check if we provided with proper form data
	if formData(r) {
		// handle form predict request, e.g. predict image
		model := r.FormValue("model")
		mlType := r.FormValue("type")
		backend := r.FormValue("backend")
		spec = Record{
			Model:   model,
			Type:    mlType,
			Backend: backend,
		}
		r.Header.Set("Accept", "application/octet-stream")
	} else {
		// hadle JSON predict request
		err := c.BindJSON(&spec)
		if err != nil {
			rec := services.Response("MLHub", http.StatusBadRequest, services.BindError, err)
			c.JSON(http.StatusBadRequest, rec)
			return
		}
	}

	rec, err := modelRecord(spec)
	if err != nil {
		rec := services.Response("MLHub", http.StatusBadRequest, services.GenericError, err)
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	if Verbose > 0 {
		log.Printf("InferenceHandler found %+v", rec)
	}
	data, mtype, err := Predict(rec, r)
	if err == nil {
		if mtype == "application/json" {
			c.JSON(http.StatusOK, data)
		} else {
			c.Data(http.StatusOK, mtype, data)
		}
		return
	}
	resp := services.Response("MLHub", http.StatusBadRequest, services.PredictError, err)
	c.JSON(http.StatusBadRequest, resp)
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
		rec := services.Response("MLHub", http.StatusBadRequest, services.MetaError, err)
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	if len(records) != 1 {
		msg := fmt.Sprintf("Too many records for provide model=%s type=%s version=%s", model, mlType, version)
		rec := services.Response("MLHub", http.StatusBadRequest, services.GenericError, errors.New(msg))
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	// form link to download model bundle file
	rec := records[0]
	fname := findModelFile(rec.Bundle, mlType, version)
	bname := strings.Replace(fname, StorageDir, "", -1)
	downloadURL := fmt.Sprintf("/bundles%s", bname)
	if Verbose > 0 {
		log.Println("download", downloadURL)
	}
	c.Redirect(http.StatusSeeOther, downloadURL)
}

// UploadHandler handles upload action of ML model to back-end server
func UploadHandler(c *gin.Context) {
	r := c.Request

	// check if we provided with proper form data
	if !formData(r) {
		rec := services.Response("MLHub", http.StatusBadRequest, services.FormDataError, errors.New("unable to get form data"))
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
	if version == "" {
		version = "latest"
	}
	reference := r.FormValue("reference")
	discipline := r.FormValue("discipline")
	description := r.FormValue("description")
	if mlType == "" || backend == "" || model == "" {
		msg := "Unable to upload your ML model"
		if mlType == "" {
			msg += ", ML type parameter is empty"
		} else if backend == "" {
			msg += ", ML backend parameter is empty"
		} else if model == "" {
			msg += ", ML model parameter is empty"
		}
		rec := services.Response("MLHub", http.StatusBadRequest, services.FormDataError, errors.New(msg))
		c.JSON(http.StatusBadRequest, rec)
		return
	}

	if Verbose > 0 {
		log.Printf("model=%v type=%v bundle=%v version=%v ref=%v dis=%v desc=%v", model, mlType, bundle, version, reference, discipline, description)
	}

	// get file name bundle
	if bundle == "" {
		// parse incoming HTTP request multipart form
		err := r.ParseMultipartForm(32 << 20) // maxMemory
		if err != nil {
			rec := services.Response("MLHub", http.StatusBadRequest, services.FormDataError, err)
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
	token := authz.BearerToken(r)
	claims, err := authz.TokenClaims(token, srvConfig.Config.Authz.ClientID)
	if err != nil {
		rec := services.Response("MLHub", http.StatusBadRequest, services.AuthError, err)
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	rec.UserName = claims.CustomClaims.User

	// perform upload action
	err = Upload(rec, r)
	if err != nil {
		rec := services.Response("MLHub", http.StatusBadRequest, services.UploadError, err)
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	c.JSON(http.StatusOK, services.Response("MLHub", http.StatusOK, 0, nil))
}

// GetHandler handles GET HTTP requests, this request will
// delete ML model in backend and MetaData database
func DeleteHandler(c *gin.Context) {
	// parse input JSON payload
	var spec Record
	c.BindJSON(&spec)
	model := spec.Model
	mlType := spec.Type
	version := spec.Version
	if version == "" {
		msg := "HTTP request does not provide ML model version"
		rec := services.Response("MLHub", http.StatusBadRequest, services.FormDataError, errors.New(msg))
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	if mlType == "" {
		msg := "HTTP request does not provide ML model type"
		rec := services.Response("MLHub", http.StatusBadRequest, services.FormDataError, errors.New(msg))
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	if Verbose > 0 {
		log.Printf("request to delete ML model %s type %s version %s", model, mlType, version)
	}
	records, err := metaRecords(model, mlType, version)
	if err != nil {
		rec := services.Response("MLHub", http.StatusBadRequest, services.MetaError, err)
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	for _, rec := range records {
		log.Printf("Remove %+v", rec)
		err = removeBundle(rec)
		if err != nil {
			rec := services.Response("MLHub", http.StatusInternalServerError, services.StorageError, err)
			c.JSON(http.StatusInternalServerError, rec)
			return
		}
		spec := map[string]any{"model": model, "type": mlType, "version": version}
		err := metaRemove(spec)
		if err != nil {
			rec := services.Response("MLHub", http.StatusInternalServerError, services.MetaError, err)
			c.JSON(http.StatusInternalServerError, rec)
			return
		}
	}
	c.JSON(http.StatusOK, services.Response("MLHub", http.StatusOK, 0, nil))
}

// ModelsHandler provides information about registered ML models
func ModelsHandler(c *gin.Context) {
	// TODO: Add parameters for /models endpoint, eg q=query, limit, idx for pagination
	mRecords, err := metaRecords("", "", "")
	if err != nil {
		msg := fmt.Sprintf("unable to get meta-data, error=%v", err)
		rec := services.Response("MLHub", http.StatusInternalServerError, services.MetaError, errors.New(msg))
		c.JSON(http.StatusInternalServerError, rec)
		return
	}
	c.JSON(http.StatusOK, mRecords)
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
		rec := services.Response("MLHub", http.StatusInternalServerError, services.MarkdownError, err)
		c.JSON(http.StatusInternalServerError, rec)
		return
	}
	tmpl := make(map[string]any)
	header := server.TmplPage(StaticFs, "header.tmpl", tmpl)
	footer := server.TmplPage(StaticFs, "footer.tmpl", tmpl)
	c.Data(http.StatusOK, "application/html", []byte(header+content+footer))
}
