// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package server implements the Aukera schedule server.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/deck"
	"github.com/google/aukera/schedule"
	"github.com/gorilla/mux"
)

func sendHTTPResponse(w http.ResponseWriter, statusCode int, message []byte) {
	w.WriteHeader(statusCode)
	i, err := w.Write(message)
	if err != nil {
		deck.Errorf("error writing response: [%d] %v", i, err)
	}
}

var fnSchedule = schedule.Schedule

func serve(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var req []string
	if vars["label"] != "" {
		req = append(req, vars["label"])
	}
	s, err := fnSchedule(req...)
	if err != nil {
		sendHTTPResponse(w, http.StatusInternalServerError, []byte(err.Error()))
	}
	b, err := json.Marshal(&s)
	if err != nil {
		sendHTTPResponse(w, http.StatusInternalServerError, []byte(err.Error()))
	}
	sendHTTPResponse(w, http.StatusOK, b)
}

func respondOk(w http.ResponseWriter, r *http.Request) {
	sendHTTPResponse(w, http.StatusOK, []byte("OK"))
}

func muxRouter() http.Handler {
	rtr := mux.NewRouter()
	rtr.HandleFunc("/status", respondOk)
	rtr.HandleFunc("/schedule", serve)
	rtr.HandleFunc("/schedule/{label}", serve)
	return rtr
}

// Run runs the internal schedule server on port.
func Run(port int) error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      muxRouter(),
	}
	return srv.ListenAndServe()
}
