package main

// data module holds all data representations used in our package
//
// Copyright (c) 2023 - Valentin Kuznetsov <vkuznet@gmail.com>
//

// Record define ML meta record
type Record struct {
	Model       string `json:"model"`       // model name
	Type        string `json:"type"`        // model type
	Backend     string `json:"backend"`     // ML backend name
	Version     string `json:"version"`     // ML version
	Description string `json:"description"` // ML model description
	Reference   string `json:"reference"`   // ML reference URL
	Discipline  string `json:"discipline"`  // ML discipline
	Bundle      string `json:"bundle"`      // ML bundle file
	UserName    string `json:"username"`    // user name
	Input       any    `json:"input"`       // prediction input
}

// MLTypes defines supported ML data types
var MLTypes = []string{"TensorFlow", "PyTorch", "ScikitLearn"}
