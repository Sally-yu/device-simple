// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
// Copyright (C) 2018 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/edgexfoundry/device-simple/internal/common"
	"github.com/edgexfoundry/device-simple/internal/handler"
	"github.com/edgexfoundry/edgex-go/pkg/models"
	"github.com/gorilla/mux"
)

const (
	statusOK          string = "OK"
	headerContentType string = "Content-Type"
	contentTypeJson   string = "application/json"
)

func statusFunc(w http.ResponseWriter, req *http.Request) {
	result := handler.StatusHandler()
	io.WriteString(w, result)
}

func discoveryFunc(w http.ResponseWriter, req *http.Request) {
	if checkServiceLocked(w, req) {
		return
	}

	vars := mux.Vars(req)
	go handler.DiscoveryHandler(vars)
	io.WriteString(w, statusOK)
	w.WriteHeader(http.StatusAccepted)
}

func transformFunc(w http.ResponseWriter, req *http.Request) {
	if checkServiceLocked(w, req) {
		return
	}

	vars := mux.Vars(req)
	_, appErr := handler.TransformHandler(vars)
	if appErr != nil {
		w.WriteHeader(appErr.Code())
	} else {
		io.WriteString(w, statusOK)
	}
}

func callbackFunc(w http.ResponseWriter, req *http.Request) {
	if checkServiceLocked(w, req) {
		return
	}

	defer req.Body.Close()
	dec := json.NewDecoder(req.Body)
	cbAlert := models.CallbackAlert{}

	err := dec.Decode(&cbAlert)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		common.LoggingClient.Error(fmt.Sprintf("Invalid callback request: %v", err))
		return
	}

	appErr := handler.CallbackHandler(cbAlert, req.Method)
	if appErr != nil {
		http.Error(w, appErr.Message(), appErr.Code())
	} else {
		io.WriteString(w, statusOK)
	}
}

func commandFunc(w http.ResponseWriter, req *http.Request) {
	if checkServiceLocked(w, req) {
		return
	}
	vars := mux.Vars(req)

	body, ok := readBodyAsString(w, req)
	if !ok {
		return
	}

	event, appErr := handler.CommandHandler(vars, body, req.Method)

	if appErr != nil {
		http.Error(w, fmt.Sprintf("%s %s", appErr.Message(), req.URL.Path), appErr.Code())
	} else if event != nil {
		w.Header().Set(headerContentType, contentTypeJson)
		json.NewEncoder(w).Encode(event)
	}
}

func commandAllFunc(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	common.LoggingClient.Debug(fmt.Sprintf("Controller - Command: execute the Get command %s from all operational devices", vars["command"]))

	if checkServiceLocked(w, req) {
		return
	}

	body, ok := readBodyAsString(w, req)
	if !ok {
		return
	}

	events, appErr := handler.CommandAllHandler(vars["command"], body, req.Method)
	if appErr != nil {
		http.Error(w, appErr.Message(), appErr.Code())
	} else if len(events) > 0 {
		w.Header().Set(headerContentType, contentTypeJson)
		json.NewEncoder(w).Encode(events)
	}
}

func checkServiceLocked(w http.ResponseWriter, req *http.Request) bool {
	if common.ServiceLocked {
		msg := fmt.Sprintf("%s is locked; %s %s", common.ServiceName, req.Method, req.URL)
		common.LoggingClient.Error(msg)
		http.Error(w, msg, http.StatusLocked) // status=423
		return true
	}
	return false
}

func readBodyAsString(w http.ResponseWriter, req *http.Request) (string, bool) {
	defer req.Body.Close()
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		msg := fmt.Sprintf("commandFunc: error reading request body for: %s %s", req.Method, req.URL)
		common.LoggingClient.Error(msg)
		return "", false
	}

	if len(body) == 0 && req.Method == http.MethodPut {
		msg := fmt.Sprintf("no request body provided; %s %s", req.Method, req.URL)
		common.LoggingClient.Error(msg)
		http.Error(w, msg, http.StatusBadRequest) // status=400
		return "", false
	}

	return string(body), true
}
