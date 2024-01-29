package main

// client functions for ML backends
//
// Copyright (c) 2023 - Valentin Kuznetsov <vkuznet@gmail.com>
//

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	srvConfig "github.com/CHESSComputing/golib/config"
)

// Predict function fetches prediction for given uri, model and client's
// HTTP request. Code is based on the following example:
// https://golangbyexample.com/http-mutipart-form-body-golang/
func Predict(rec Record, r *http.Request) ([]byte, string, error) {
	mtype := ""
	log.Printf("search ML backend for record: %+v", rec)
	backend, err := mlBackend(rec.Backend, rec.Type)
	if err != nil {
		return []byte{}, mtype, err
	}
	if Verbose > 0 {
		log.Printf("found ML backend %+v", backend)
	}
	uri := backend.URI
	for _, rec := range backend.Apis {
		if rec.Name == "predict" {
			if r.Method != rec.Method {
				msg := fmt.Sprintf("method mismatch for %+v, got %s", backend, r.Method)
				return []byte{}, mtype, errors.New(msg)
			}
			uri = fmt.Sprintf("%s/%s", backend.URI, rec.Endpoint)
		}
	}
	if r.Header.Get("Accept") == "application/json" {
		return PredictJSONInput(uri, rec, r)
	} else if r.Header.Get("Accept") == "application/octet-stream" {
		return PredictMultipart(uri, rec, r)
	} else {
		msg := fmt.Sprintf("Unsupported mtime '%s' for uri %s", r.Header.Get("Accept"), uri)
		return []byte{}, mtype, errors.New(msg)
	}
}

func PredictJSONInput(uri string, rec Record, r *http.Request) ([]byte, string, error) {
	mtype := ""
	input := rec.Input
	if Verbose > 0 {
		log.Printf("Predict uri=%s rec %+v", uri, rec)
	}
	data, err := json.Marshal(input)
	if err != nil {
		return []byte{}, mtype, err
	}

	// form HTTP request
	client := &http.Client{
		Timeout: time.Second * 10,
	}
	if Verbose > 0 {
		log.Printf("POST request to %s with body\n%v", uri, string(data))
	}
	req, err := http.NewRequest("POST", uri, bytes.NewReader(data))
	if err != nil {
		return data, mtype, err
	}
	req.Header.Set("Content-Type", "application/json")
	rsp, err := client.Do(req)
	if rsp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("request to %s failed with response code: %d", rec.Backend, rsp.StatusCode)
		log.Println(msg, "error", err)
		return []byte{}, mtype, errors.New(msg)
	}
	mtype = rsp.Header.Get("Content-type")
	defer rsp.Body.Close()
	data, err = io.ReadAll(rsp.Body)
	if Verbose > 1 {
		log.Println("backend %s return %s error %v", rec.Backend, string(data), err)
	}
	return data, mtype, err
}

func PredictMultipart(uri string, rec Record, r *http.Request) ([]byte, string, error) {
	mtype := ""
	// parse incoming HTTP request multipart form
	err := r.ParseMultipartForm(32 << 20) // maxMemory

	// new multipart writer.
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// create new field
	for k, vals := range r.MultipartForm.Value {
		for _, v := range vals {
			writer.WriteField(k, v)
		}
	}
	// add mandatory model field
	writer.WriteField("model", rec.Model)

	// parse and recreate file form
	for k, vals := range r.MultipartForm.File {
		for _, fh := range vals {
			fname := fh.Filename
			fw, err := writer.CreateFormFile(k, fname)
			if err != nil {
				log.Printf("ERROR: unable to create form file for key=%s fname=%s", k, fname)
				break
			}
			file, err := fh.Open()
			if err != nil {
				log.Printf("ERROR: unable to open fname=%s", fname)
				break
			}
			_, err = io.Copy(fw, file)
			if err != nil {
				log.Printf("ERROR: unable to copy fname=%s to multipart writer", fname)
				break
			}
		}
	}
	writer.Close()

	// for TFaaS we need additional end-point path if we query image prediction
	if r.FormValue("name") != "image" && rec.Type == "TensorFlow" {
		uri += "/image"
	}
	if Verbose > 0 {
		log.Printf("Predict uri=%s HTTP request %+v", uri, r)
	}

	// form HTTP request
	var data []byte
	client := &http.Client{
		Timeout: time.Second * 10,
	}
	if Verbose > 0 {
		log.Printf("POST request to %s with body\n%v", uri, string(body.Bytes()))
	}
	req, err := http.NewRequest("POST", uri, bytes.NewReader(body.Bytes()))
	if err != nil {
		return data, mtype, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rsp, err := client.Do(req)
	if rsp.StatusCode != http.StatusOK {
		log.Printf("Request failed with response code: %d", rsp.StatusCode)
	}
	mtype = rsp.Header.Get("Content-type")
	defer rsp.Body.Close()
	data, err = io.ReadAll(rsp.Body)
	return data, mtype, err
}

// Upload function uploads record to MetaData database, then
// uploads file to server storage, and finally to ML backend
func Upload(rec Record, r *http.Request) error {
	err := uploadRecord(rec)
	if err != nil {
		return err
	}
	err = bundle2Storage(rec, r)
	if err != nil {
		return err
	}
	err = uploadBundle(rec, r)
	if err != nil {
		return err
	}
	return nil
}

// helper function to upload bundle tarball to ML backend
func uploadRecord(rec Record) error {
	// insert record into MetaData database
	if Verbose > 0 {
		log.Printf("uploadRecord %+v", rec)
	}
	err := metaInsert(rec)
	return err
}

// helper function to remove bundle from our storate
func removeBundle(rec Record) error {
	modelDir := fmt.Sprintf("%s/%s/%s/%s", StorageDir, rec.Type, rec.Model, rec.Version)
	return os.RemoveAll(modelDir)
}

// helper function to put bundle to the server storage
func bundle2Storage(rec Record, r *http.Request) error {
	if Verbose > 0 {
		log.Printf("bundle2Storage %+v", rec)
	}
	// parse incoming HTTP request multipart form
	err := r.ParseMultipartForm(32 << 20) // maxMemory
	if err != nil {
		return err
	}
	// extract file from HTTP request form
	file, handler, err := r.FormFile("file")
	if err != nil {
		return err
	}

	defer file.Close()
	modelDir := fmt.Sprintf("%s/%s/%s/%s", StorageDir, rec.Type, rec.Model, rec.Version)
	err = os.MkdirAll(modelDir, 0755)
	if err != nil {
		return err
	}
	fname := filepath.Join(modelDir, handler.Filename)
	dst, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		return err
	}
	return nil
}

// helper function to upload bundle tarball to ML backend
func uploadBundle(rec Record, r *http.Request) error {
	if rec.Type == "TensorFlow" {
		return uploadBundleTFaaS(rec, r)
	} else if rec.Type == "PyTorch" {
		return uploadBundleTorch(rec, r)
	} else if rec.Type == "ScikitLearn" {
		return uploadBundleScikit(rec, r)
	}
	msg := fmt.Sprintf("upload for %s backend is not implemented", rec.Type)
	return errors.New(msg)
}

// helper function to find ML backend record
func mlBackend(name, rtype string) (srvConfig.MLBackend, error) {
	var mlBackend srvConfig.MLBackend
	backends := srvConfig.Config.MLHub.ML.MLBackends
	for _, rec := range backends {
		if Verbose > 0 {
			log.Printf("### ML backend record %+v", rec)
		}
		if rec.Name == name && rec.Type == rtype {
			return rec, nil
		}
	}
	msg := fmt.Sprintf("No ML backend found with name=%s type=%s", name, rtype)
	return mlBackend, errors.New(msg)
}

// helper functiont to upload bundle to TFaaS backend
func uploadBundleTFaaS(rec Record, r *http.Request) error {
	if Verbose > 0 {
		log.Println("uploadBundleTFaaS", rec)
	}
	backend, err := mlBackend(rec.Backend, rec.Type)
	if Verbose > 0 {
		log.Println("ML backend", backend)
	}
	if err != nil {
		return err
	}

	// form backe URI
	uri := fmt.Sprintf("%s/upload", backend.URI)
	if Verbose > 0 {
		log.Printf("upload model %s bundle to %s", rec.Model, uri)
	}
	// parse incoming HTTP request multipart form
	err = r.ParseMultipartForm(32 << 20) // maxMemory

	// construct proper request body
	var body io.Reader
	for _, vals := range r.MultipartForm.File {
		for _, fh := range vals {
			file, err := fh.Open()
			if err != nil {
				return err
			}
			body = io.NopCloser(file)
		}
	}

	// make HTTP request to remote TFaaS server
	client := &http.Client{
		Timeout: time.Second * 10,
	}
	req, err := http.NewRequest("POST", uri, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Content-Type", "application/octet-stream")
	if Verbose > 0 {
		log.Printf("New request %+v", req)
	}
	rsp, err := client.Do(req)
	if Verbose > 0 {
		log.Println("TFaaS response", rsp)
	}
	if err == nil {
		// check response status code
		if rsp.StatusCode != http.StatusOK {
			msg := fmt.Sprintf("TFaaS response status %s", rsp.Status)
			err = errors.New(msg)
		}
	}
	return err
}

// helper functiont to upload bundle to Torch backend
func uploadBundleTorch(rec Record, r *http.Request) error {
	return errors.New("upload for TorchServer backend is not implemented")
}

// helper functiont to upload bundle to Scikit backend
func uploadBundleScikit(rec Record, r *http.Request) error {
	return errors.New("upload for ScikitLearn backend is not implemented")
}

// helper function to get ML record for given HTTP request
func modelRecord(rec Record) (Record, error) {
	var record Record
	model := rec.Model
	version := rec.Version
	mtype := rec.Type

	// get ML meta-data
	records, err := metaRecords(model, mtype, version)
	if err != nil {
		msg := fmt.Sprintf("unable to get meta-data, error=%v", err)
		return rec, errors.New(msg)
	}
	// we should have only one record from MetaData
	if len(records) != 1 {
		msg := fmt.Sprintf("Incorrect number of MetaData records %+v", records)
		return rec, errors.New(msg)
	}
	record = records[0]
	if Verbose > 0 {
		log.Printf("meta-data for model=%s type=%s version=%s, record=%+v", model, mtype, version, record)
	}
	// assign input data to our meta-data record
	record.Input = rec.Input
	// convert mongo record (map[string]any) to Record data type
	data, err := json.Marshal(record)
	if err != nil {
		return record, err
	}
	var mRec Record
	err = json.Unmarshal(data, &mRec)
	if err != nil {
		return mRec, err
	}
	return mRec, nil
}

// helper function to find model file name for given parameters
func findModelFile(fileName, mlType, version string) string {
	var fname string
	if version == "" {
		version = "latest"
	}
	if mlType != "" {
		fname = fmt.Sprintf("%s/%s/%s/%s", StorageDir, mlType, version, fileName)
	}
	if Verbose > 0 {
		log.Printf("search fileName=%s mlType=%s version=%s", fileName, mlType, version)
	}
	// otherwise walk throught directory structure to find our file name
	pat := fmt.Sprintf("%s$", fileName)
	regExp, err := regexp.Compile(pat)
	err = filepath.Walk(StorageDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && regExp.MatchString(info.Name()) {
			fname = path
			return nil
		}
		return nil
	})
	if err != nil {
		log.Printf("Unable to find %s file in %s for type=%s version=%s", fileName, StorageDir, mlType, version)
	}
	return fname
}
