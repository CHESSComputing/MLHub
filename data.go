package main

// data module holds all data representations used in our package
//
// Copyright (c) 2023 - Valentin Kuznetsov <vkuznet@gmail.com>
//

// Record define ML meta record
type Record struct {
	MetaData    map[string]any `json:"meta_data"`   // meta-data information about ML model
	Model       string         `json:"model"`       // model name
	Type        string         `json:"type"`        // model type
	Backend     string         `json:"backend"`     // ML backend name
	Version     string         `json:"version"`     // ML version
	Description string         `json:"description"` // ML model description
	Reference   string         `json:"reference"`   // ML reference URL
	Discipline  string         `json:"discipline"`  // ML discipline
	Bundle      string         `json:"bundle"`      // ML bundle file
	UserName    string         `json:"user_name"`   // user name
	UserID      string         `json:"user_id"`     // user id
	Provider    string         `json:"provider"`    // auth provider
}

// MLTypes defines supported ML data types
var MLTypes = []string{"TensorFlow", "PyTorch", "ScikitLearn"}

// MLBackend represents ML backend engine
type MLBackend struct {
	Name string `json:"name"` // ML backend name, e.g. TFaaS
	Type string `json:"type"` // ML backebd type, e.g. TensorFlow
	URI  string `json:"uri"`  // ML backend URI, e.g. http://localhost:port
}

// Predict performs predict action on upstream ML backend
func (m *MLBackend) Predict(data []byte) ([]byte, error) {
	return []byte{}, nil
}

// Upload performs upload of the given data to upstream ML backend
func (m *MLBackend) Upload(data []byte) error {
	return nil
}

// Download downloads ML model from backend server
func (m *MLBackend) Download(model string) ([]byte, error) {
	return []byte{}, nil
}

// Delete performs delete action of the ML model on ML backend
func (m *MLBackend) Delete(model string) error {
	return nil
}

// MLBackends represents map of ML backends records
type MLBackends map[string]MLBackend
