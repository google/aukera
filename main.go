// Copyright 2018 Google LLC
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

// Aukera provides a local http interface for querying locally-defined maintenance windows.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"flag"
	"github.com/google/aukera/auklib"
	"github.com/google/aukera/schedule"
	"github.com/gorilla/mux"
	"github.com/google/logger"
)

var (
	runInDebug = flag.Bool("debug", false, "Run in debug mode")
	port       = flag.Int("port", auklib.ServicePort, "Define listening port")
)

func sendHTTPResponse(w http.ResponseWriter, statusCode int, message []byte) {
	w.WriteHeader(statusCode)
	i, err := w.Write(message)
	if err != nil {
		logger.Errorf("error writing response: [%d] %v", i, err)
	}
}

func serve(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var req []string
	if vars["label"] != "" {
		req = append(req, vars["label"])
	}
	s, err := schedule.Schedule(req...)
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

func runMainLoop() error {
	rtr := mux.NewRouter()
	rtr.HandleFunc("/status", respondOk)
	rtr.HandleFunc("/schedule", serve)
	rtr.HandleFunc("/schedule/{label}", serve)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      rtr,
	}
	return srv.ListenAndServe()
}

func main() {
	// Initialize configuration directory
	exist, err := auklib.PathExists(auklib.ConfDir)
	if err != nil {
		logger.Errorf("unexpected error finding path %s: %v", auklib.ConfDir, err)
	}
	if exist == false {
		logger.Warning("Configuration directory does not exist. Attempting creation.")
		if err := os.MkdirAll(auklib.ConfDir, 0664); err != nil {
			logger.Warningf("Unable to create configuration directory: %v", err)
		}
	}
	// Initialize logger
	lf, err := os.OpenFile(auklib.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
	if err != nil {
		logger.Fatalln("Failed to open log file: ", err)
	}
	defer lf.Close()
	defer logger.Init("aukera", false, true, lf).Close()

	err = run()
	if err != nil {
		logger.Fatalln("Run exited with error: ", err)
	}
}
